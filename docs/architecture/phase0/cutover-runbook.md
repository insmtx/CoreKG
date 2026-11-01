# CoreKG 单体一次性切换 Runbook

> 元信息：Phase 0 / Task 13 产出。执行对象：把当前"`apps/*` 多服务 + MySQL 权威"形态一次性切换为"`cmd/corekg` 单体 + PostgreSQL 权威"形态（Phase 1–4 的最终动作）。配套文档：`docs/architecture/phase0/pg-migration-assessment.md`（Task 12，校验口径与回滚原则，下称"评估文档"）、`docs/architecture/phase0/golden-path-baseline.md`（Task 9，回归基线）、`docs/architecture/phase0/pg-driver-spike.md`（Task 10，PG 接入）、`docs/architecture/phase0/route-decisions.md`（Task 8，路由处置）。
>
> **本文所有命令都是占位符形状（command shapes）**：`<MYSQL_DSN>`、`<PG_DSN>`、`<NEW_IMAGE_TAG>`、`<OLD_IMAGE_TAG>`、`<S3_ENDPOINT>` 等尖括号内容在执行时替换为真实值；**本文不含、也不应含任何真实密码、密钥、DSN**。执行时的真实凭据一律通过环境变量或密钥管理系统（`<SECRET_MANAGER>`）注入，绝不写入仓库、工单、IM 或 shell 历史（见 §1.6）。
>
> **当前状态（2026-11-01，Phase 0）：本文是可审阅的执行模板，尚不可执行。** 五个硬 blocker 见 §2（`cmd/corekg` 镜像管线缺失、golden-path 11/11 未跑、live inventory 缺失、ETL `--apply` 未实现、DBA/SRE review 未进行）。在 blocker 关闭与 Phase 4 交付前，任何人不得以本文为依据发起切换。

## 1. 范围与原则

### 1.1 切换模式：一次性切换，不做长期双写

- **T0 维护窗口内完成**：停写 → 最终备份 → 增量 ETL → 校验 → ES 重建 → 启动新栈 → 灰度放量（§4.1）。此后 PostgreSQL 是唯一权威写库，MySQL 转为只读归档。
- **不做长期 MySQL↔PG 双写**、不做按租户分批切、不做"读 PG 写 MySQL"过渡态。理由：
  1. 双写需要两套一致性协议与冲突裁决，本仓库没有现成组件，引入成本高于冻结窗口内追平（自研 ETL + checkpoint 重放 + 内嵌校验，评估文档 §10.1/§12.3）；
  2. 双写期的回滚面比一次性切换更大（两侧都可能有写），数据丢失边界反而更难向业务说清；
  3. 回滚的兜底是"保留旧栈 ≥ 7 天 + 归档 90 天"（§3/§6），不依赖双写。
- 若业务最终给出"不可接受任何写入丢失"的目标（RPO = 0 且不接受的场景见 §7.5），一次性切换模式不满足，需升级架构评审另立方案，**不修改本 runbook 去拼凑双写**。

### 1.2 发布与分支策略：内部 Phase 0–4，对外一次发布

- Phase 0–4 是**内部里程碑**，不对外发布中间版本。对外只有一个新版本：`<NEW_IMAGE_NAME>:<NEW_IMAGE_TAG>`（含 Phase 0–4 全部改动）。
- **镜像管线现状（易错点）**：legacy 目标 `make local/linux/push-image APP=corekg` 构建的是**旧** `apps/corekg` 聚合单体（`apps/corekg/script/Dockerfile`，`IMAGE_NAME = docker.io/corekg/<APP>-api`）；**新**单体 `cmd/corekg` 目前只有 `make corekg` 本地二进制目标（产出 `bundles/corekg`），**没有 Dockerfile，也没有 CI 镜像管线**——`<NEW_IMAGE_NAME>:<NEW_IMAGE_TAG>` 是 Phase 4 交付（§2 第 2 项 ⛔）。部署一律引用 `<NEW_IMAGE_TAG>`，不得把本地 `bundles/corekg` 当作部署产物。
- **⚠️ Phase 0 代码分支合并顺序（merge-ordering constraint，非实测结果）**：`phase0-foundation` 分支为 **PostgreSQL-only**——Task 5–7 已使旧 `apps/*` 与 MySQL **运行时不兼容**（`?::text = ANY(...)`、`~*` 等 PG 方言 SQL；`timestamptz` 等 GORM 类型 tag 在 MySQL AutoMigrate 建表直接失败），**不可部署到 MySQL 环境**。该分支只允许合入本节的不部署集成分支 `monolith-v2`；**在 Phase 1 的 PG 运行时接入（pgdrv，`pg-driver-spike.md`）与本 runbook 的切换完成之前，不得合入 `main`、不得以任何形式部署**。此约束与上一条"新镜像管线是 Phase 4 交付"并行不悖：前者约束代码分支流向，后者约束部署产物来源。
- 分支策略：
  - 日常开发在集成分支 **`monolith-v2`**；
  - **T-1 起 `main` 冻结合并**：不接受功能合并，仅接受 cutover 阻塞修复（需 Go/No-Go 会议批准，且同步 cherry-pick 到 `monolith-v2`）；
  - 切换完成签收（T+1）后 `monolith-v2` 合回 `main`，解冻；若切换失败，`main` 保持冻结至重试窗口确定。
- 回滚不按 phase 拆分：回滚 = 部署 `<OLD_IMAGE_TAG>` + 流量指回 MySQL（§6），一个动作覆盖全部 Phase 0–4 改动。

### 1.3 范围内 / 范围外

| 范围 | 内容 |
|---|---|
| **内** | 系统库 5 个 schema（`core`/`account`/`knownow`/`chat`/`sale`）→ 单 PG 库同名 schema（评估文档 §1.1）；ES `ke_*` 索引全量重建（从 PG 回灌，`ik_max_word`）；keapi/MCP 流量随单体切换 |
| **外** | 对象存储（MinIO/S3）数据**不搬迁**（桶与对象原样保留，仅记录清单，§3.2）；外部租户 MySQL 数据源（`pkgs/plugins/dbplugins/mysqlplugin`，永久保留 MySQL 连接，评估文档 §9.2）；Coze/workflow 库（按 `route-decisions.md` 删除规则处置，不迁移）；`/v2` 挂载等 Reopen 项按 `route-decisions.md` |

### 1.4 权威性原则

MySQL 在切换签收前始终是权威库（评估文档 §12.2）：ETL/校验期间 PG 仅是校验目标；T0 停写（`super_read_only`）后 PG 追平；G1–G9 全绿 + T+1 签收后权威转移完成。签收前任何"PG 上的数据"都不得当作业务数据源。

### 1.5 执行者假设与前提

目标执行者是**未参与 Phase 0–4 开发的工程师**：所有步骤自包含，命令形状、判据与失败处置都在本文，不要求执行者理解迁移代码。执行前提：

1. 跳板机 `<OPS_HOST>` 可访问 MySQL、PG、ES、对象存储、网关管理面、NATS；
2. 仓库 checkout 到 `<MONOLITH_V2_COMMIT>`（`monolith-v2` 分支），`scripts/golden-path.sh` 可用；**新单体二进制用 `make corekg` 产出 `bundles/corekg`**（构建 `cmd/corekg`）。注意：`make local APP=corekg` 构建的是**旧** `apps/corekg` 且写入同一路径 `bundles/corekg`——两个目标互斥，切换机上取证/执行一律用 `make corekg`，不得混用；部署不走本地 `bundles/`，走 `<NEW_IMAGE_TAG>`（§1.2）；
3. 凭据经 `<ENV_FILE>`（权限 600，运行时从 `<SECRET_MANAGER>` 生成，用后删除）注入；MySQL 客户端命令统一使用 `--defaults-extra-file=<MYSQL_DEFAULTS_FILE>`（同为 600，`[client]` 段，运行时生成，§3.1）；
4. 产物目录 `<ARTIFACT_DIR>` 可写且容量 ≥ dump 大小 × 2。

### 1.6 记录纪律

- 门禁未通过就是未通过，禁止"看起来差不多"放行；所有门禁产物（log/CSV/截图）存 `<ARTIFACT_DIR>` 并归档；
- 真实凭据不得出现在命令行参数（MySQL 用 `--defaults-extra-file=<MYSQL_DEFAULTS_FILE>`，PG 用 `PGPASSWORD` 环境变量，对象存储用 `"$S3_SECRET_KEY"` 环境变量）、不得粘贴进工单与 IM；
- 本文任何"当前状态"标注（如 §2 的 ⛔）只描述写文档当天的事实，后续更新必须随事实修改，不得提前写"已通过"。

## 2. 切换前检查清单

图例：✅ 已完成（有据可查）｜⬜ 未做（按时间表排期）｜⛔ **blocker（不关闭不得进入 T-7 时间表）**。owner 列对应 §8.1 角色。

### 2.1 代码与镜像

| # | 检查项 | owner | 完成标准 | 现状（2026-11-01） |
|---|---|---|---|---|
| 1 | `monolith-v2` 集成分支可交付 | 应用 | CI 绿；**`make corekg` 产出 `bundles/corekg`（新单体 `cmd/corekg`）**；`make arch-test` 通过。注意 `make local APP=corekg` 构建旧 `apps/corekg` 到同一路径，不得用作新单体证据 | ✅ Phase 0 骨架可构建；Phase 1–4 完成度以各 phase 交付为准 |
| 2 | 新单体镜像管线 | SRE | `cmd/corekg` 的 Dockerfile + CI 镜像管线交付；推送 `<NEW_IMAGE_NAME>:<NEW_IMAGE_TAG>` 并记录 digest（`sha256:<...>`）。**当前没有任何 `cmd/corekg` 的 Dockerfile/CI 管线**（legacy `make push-image APP=corekg` 走 `apps/corekg/script/Dockerfile`，构建的是旧单体） | ⛔ **缺失**：Phase 4 交付；镜像名约定（是否沿用 `docker.io/corekg/<app>-api` 形态）须 T-7 显式决定（§9） |
| 3 | 回滚镜像 | SRE | 记录当前生产在跑的 `<OLD_IMAGE_TAG>`（legacy 管线产物）与 digest，确认保留 ≥ 7 天 | ⬜ |
| 4 | `main` 合并冻结 | SRE | T-1 生效；冻结时点 PR 列表为空或全部延后 | ⬜ 计划内（§9） |

### 2.2 回归与迁移基线（诚实标注 blocker）

| # | 检查项 | owner | 完成标准 | 现状（2026-11-01） |
|---|---|---|---|---|
| 5 | Task 9 golden-path 11/11 | 应用 | 在 Docker 可用宿主机对旧栈完整跑通 11 stage 并回填基线（`golden-path-baseline.md` 的回填清单）；种子账号/模型/`KEAPI_API_KEY` 就绪。**基线代码必须是 Task 5 之前的 commit `630ba7b9f`（或当前生产在跑的镜像），不是分支 HEAD**——HEAD 的 Task 5–7 已把旧 `apps/*` 改为 PG-only（MySQL 上 AutoMigrate 与 stage 8/全局搜索路径直接失败），在 HEAD 上跑出的"旧栈基线"无效（见 `golden-path-baseline.md` §"后续执行完整基线的步骤"） | ⛔ **未执行**：当前环境无 docker（宿主根分区 99% 用满）、无种子数据，仅 stage 1 实测通过。详见 `golden-path-baseline.md` §"Environment status" |
| 6 | Task 11 live inventory | DBA | `docs/architecture/phase0/mysql-inventory.json` 已用 `corekg migrate from-mysql`（只读）生成、脱敏、提交 | ⛔ **缺失**：Task 11 为 DONE_WITH_BLOCKER（无可达 MySQL），文件不存在 |
| 7 | Task 12 评估文档 | DBA | `pg-migration-assessment.md` 已提交；其 §11 校验口径并入本文 §5、§12 回滚原则并入本文 §6；DBA 实际 review 归第 25 项 | ✅ 文档已提交（Task 12）；其数值列全部留待 Phase 4 inventory（诚实标注，未伪造） |
| 8 | ETL `--apply` 与 ES 重建命令 | 应用 | `corekg migrate from-mysql --apply` 实现（批量 COPY、checkpoint、setval 钩子、校验内嵌，评估文档 §10.2）；`<ES_REBUILD_CMD>` 按 §4.3 行为规格交付 | ⛔ **两者均未实现**：`--apply` 在 Phase 0 显式拒绝（`cmd/corekg/migrate.go` 报 "apply is not implemented in phase 0"，当前只有只读 dry-run）；ES 重建命令不存在（§4.3 为行为规格占位） |
| 9 | 回滚演练 | DBA | 评估文档 §12.4：至少一次完整回滚序列演练（含 `DROP SCHEMA ... CASCADE` + checkpoint 重放），记录各步耗时 | ⬜ Phase 4 |
| 10 | MySQL 时区实测 | DBA | 评估文档 §8.4：实测 `@@global.time_zone` 并填入 ETL 配置 | ⬜ Phase 4 |

### 2.3 决策定稿

| # | 检查项 | owner | 完成标准 | 现状（2026-11-01） |
|---|---|---|---|---|
| 11 | 路由/Coze 决策 | 应用 | `route-decisions.md` 定稿；Reopen 项（`/v2` 挂载、`chat.ExternalToken` 等）全部关闭或书面延期到切换后 | 部分完成：决策文档 ✅；Reopen 项 ⬜ |
| 12 | pgvector | DBA | 若 Phase 1+ 使用（`pg-driver-spike.md` 结论：`pgvector-go v0.3.0` 可用）：目标库执行 `CREATE EXTENSION IF NOT EXISTS vector;` 并确认在备份范围内 | ⬜ 是否使用在 Phase 1 决策 |

### 2.4 基础设施与运维准备

| # | 检查项 | owner | 完成标准 | 现状（2026-11-01） |
|---|---|---|---|---|
| 13 | PG 目标库就绪 | DBA | 网络/账号/权限/schema search_path 验证；`pg_dump`/`pg_dumpall`/`psql` 客户端在 `<OPS_HOST>` 可用 | ⬜ |
| 14 | PG 快照与 globals | DBA | globals（roles/tablespaces）T-1 dump；**post-ETL 快照（§3.4）流程**在 T-7 演练库验证，T0 在校验通过后、首个灰度请求前执行（§4.1 步骤 7）——不在 ETL 前对空库/半成品库 dump | ⬜ |
| 15 | secrets 注入通道 | SRE | `CORekG_MYSQL_DSN`、`<PG_DSN>`、`KEAPI_API_KEY`、S3/ES 凭据全部经 `<SECRET_MANAGER>` → `<ENV_FILE>` 注入；MySQL 客户端另配 `<MYSQL_DEFAULTS_FILE>`（§3.1，600 权限）；仓库/文档零明文 | ⬜ |
| 16 | DNS TTL | SRE | 对外域名 TTL 降至 ≤ 60 秒（T-1 生效） | ⬜ |
| 17 | 证书 | SRE | 网关 TLS 证书剩余有效期 > 窗口时长 + 30 天 | ⬜ |
| 18 | 监控/大盘/告警 | SRE | 错误率、P95、DB 连接、队列深度、ES 集群健康的大盘与告警就绪并指向 on-call | ⬜ |
| 19 | on-call | SRE | 窗口值班表 + 每角色备份人（§8.1） | ⬜ |
| 20 | 备份恢复演练 | DBA | §3.6 至少一次成功（T-7） | ⬜ |
| 21 | 磁盘容量 | DBA | PG 数据盘 ≥ 预估 × 1.5；`<DUMP_DIR>` ≥ dump 大小 × 2；ES 重建空间 ≥ 现索引 × 2 | ⬜ |
| 22 | ES snapshot repo | DBA | 快照仓库注册成功 + 一次快照成功（T-7） | ⬜ |
| 23 | MCP/keapi API Key | 应用 | 现有 keapi API Key 随 `account` schema 迁移后仍可鉴权（golden-path stage 10/11 依赖）；是否重签发已决策并（如需）公告 | ⬜ |
| 24 | 队列排空流程 | SRE | NATS 暂停 consumer + pending 清零的流程已演练（§4.2） | ⬜ |

### 2.5 签收

| # | 检查项 | owner | 完成标准 | 现状（2026-11-01） |
|---|---|---|---|---|
| 25 | DBA/SRE review 本文 | DBA/SRE | 文末"附：DBA/SRE review 记录"两行均 approve | ⛔ **未进行**（见文末待确认清单） |
| 26 | 业务签收 | 产品 | 维护窗口对外公告已发；数据丢失边界（§7）已向业务说明并理解 | ⬜ T-1 |

**结论：当前为 NO-GO。** 上述 ⛔（第 2、5、6、8、25 项）全部关闭且 Phase 4 交付后，才可进入 §9 的 T-7 时间表。**本文不声称 golden-path 已通过、inventory 已生成——它们没有。**

## 3. 备份与归档

备份在两个时点执行：**T-7 演练**（全量 + 恢复演练）与 **T0 停写后**（最终全量，§4.1 步骤 3）。命令一致，产物带日期后缀区分。**例外：PG 快照（§3.4）在 T0 的执行时点是 ETL/校验之后、首个灰度请求之前（§4.1 步骤 7），不在最终备份步骤里。**所有产物统一落 `<DUMP_DIR>`，归档到对象存储并保留 **90 天**。

MySQL 客户端命令统一使用 `--defaults-extra-file=<MYSQL_DEFAULTS_FILE>`（避免凭据进命令行/进程列表；文件权限 600，运行时从 `<SECRET_MANAGER>` 生成，用后删除）。其形状：

```ini
# <MYSQL_DEFAULTS_FILE>（生成时替换占位符；本文件不进仓库）
[client]
host=<MYSQL_HOST>
port=<MYSQL_PORT>
user=<MYSQL_USER>
password=<MYSQL_PASSWORD>
```

### 3.1 MySQL 全量 dump 与 binlog 位点

```bash
# 前置：确认 binlog 开启（位点记录的前提）
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> -e "SELECT @@log_bin AS log_bin;"
# 断言：log_bin == 1。为 0 时无增量补偿能力 → 停止并交 DBA 决策（结果记入签收表备注）。

# 全量 dump（5 个系统库 = dbutil 五句柄，评估文档 §1.1）。
# --single-transaction 仅对 InnoDB 一致；若 Phase 4 inventory 发现 MyISAM 表，
# 改用 --lock-all-tables 并由 DBA 确认停写窗口（本 runbook 默认停写后执行，可接受）。
# --source-data=2 把 binlog 位点以注释写进 dump 头（MySQL ≤ 8.0.25 为 --master-data=2），
# 该头部位点是本节的权威位点。
# --set-gtid-purged=OFF 是本 runbook 的显式选择：归档只用于本环境恢复/取证，不复制到要求
# GTID 一致性的实例；若 DBA 要求 GTID 恢复，改为 AUTO 并在签收表记录（影响恢复实例复制状态）。
mysqldump --defaults-extra-file=<MYSQL_DEFAULTS_FILE> \
  --single-transaction --source-data=2 --hex-blob \
  --default-character-set=utf8mb4 --set-gtid-purged=OFF \
  --routines --triggers --events \
  --databases core account knownow chat sale \
  --result-file=<DUMP_DIR>/mysql-full-<DATE>.sql

# 完整性断言（SHA256/上传之前必须通过；截断的 dump 不得归档）
tail -n 1 <DUMP_DIR>/mysql-full-<DATE>.sql
# 断言：该行以 "-- Dump completed" 开头。

# dump 完成后单独执行 SHOW BINARY LOG STATUS，与 dump 头部位点一起记录
# （回滚后增量补偿的起点，评估文档 §12.2）。
# MySQL ≥ 8.4 用 SHOW BINARY LOG STATUS；更早版本回退 SHOW MASTER STATUS（DBA 按 server 版本确认）。
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> \
  -e "SHOW BINARY LOG STATUS;" | tee <DUMP_DIR>/binlog-coord-<DATE>.txt
# 若报 Unknown statement，改用：
#   -e "SHOW MASTER STATUS;" | tee <DUMP_DIR>/binlog-coord-<DATE>.txt
# 断言：单独位点 >= dump 头部 CHANGE REPLICATION SOURCE 注释位点（两个值都写入文件；
# 停写后通常相等，但断言必须是 >= 而不是 ==——若 >，说明 dump 期间仍有写入，排查后再继续，
# 绝不假设两者相等）。以 dump 头部位点为权威。
```

### 3.2 对象存储清单（数据不搬，只留清单）

```bash
mc alias set <S3_ALIAS> <S3_ENDPOINT> <S3_ACCESS_KEY> "$S3_SECRET_KEY"   # 密钥走环境变量
mc ls --recursive --json <S3_ALIAS>/<S3_BUCKET> > <DUMP_DIR>/s3-manifest-<DATE>.jsonl
mc du --recursive <S3_ALIAS>/<S3_BUCKET> | tee <DUMP_DIR>/s3-du-<DATE>.txt
# 断言：jsonl 行数 > 0；对象总数与总字节数记入 s3-du（回滚后孤儿清理的基线，§7.3）。
```

### 3.3 ES 索引快照

```bash
# 一次性注册快照仓库（fs 类型需 elasticsearch.yml 的 path.repo 包含 <SNAPSHOT_DIR>，SRE 确认）
curl -sS -X PUT "<ES_ENDPOINT>/_snapshot/<SNAPSHOT_REPO>" \
  -u "<ES_USER>:$ES_PASSWORD" -H 'Content-Type: application/json' \
  -d '{"type":"fs","settings":{"location":"<SNAPSHOT_DIR>"}}' | tee <DUMP_DIR>/es-repo-<DATE>.json

# 全量快照（ke_* 及全部业务索引；大索引用后台模式）
curl -sS -X PUT "<ES_ENDPOINT>/_snapshot/<SNAPSHOT_REPO>/pre-cutover-<DATE>?wait_for_completion=false" \
  -u "<ES_USER>:$ES_PASSWORD" | tee <DUMP_DIR>/es-snapshot-<DATE>.json

# 轮询直至 state == SUCCESS（不委派给执行者现场发挥；上限 <ES_SNAPSHOT_TIMEOUT_S>）
deadline=$(( SECONDS + <ES_SNAPSHOT_TIMEOUT_S> ))
until curl -sS "<ES_ENDPOINT>/_snapshot/<SNAPSHOT_REPO>/pre-cutover-<DATE>" \
    -u "<ES_USER>:$ES_PASSWORD" | tee <DUMP_DIR>/es-snapshot-<DATE>.json \
    | grep -q '"state":"SUCCESS"'; do
  if (( SECONDS >= deadline )); then
    echo "ES snapshot not SUCCESS after <ES_SNAPSHOT_TIMEOUT_S>s"; exit 1
  fi
  sleep 30
done
```

### 3.4 PG 快照（post-ETL，回滚取证与丢失面计量的基线）

**T0 执行时点：最终 ETL + 数据校验全部通过之后、首个灰度请求之前（§4.1 步骤 7）。**此时 PG 内容 = MySQL final dump 的完整迁移副本 + 校验通过状态，是回滚取证（§6.1）与数据丢失面计量（§7.4）的基线；切换前对空库/半成品库做 dump 没有取证价值，不执行。T-7 在演练库上验证本流程（§3.6）。roles/tablespaces 等 globals 在 T-1 单独 dump（§2.4 第 14 项）。

```bash
# post-ETL 状态快照（custom 格式，pg_restore 可选择性恢复）
PGPASSWORD="$PG_PASSWORD" pg_dump --format=custom \
  --host=<PG_HOST> --port=<PG_PORT> --user=<PG_USER> --dbname=<PG_DBNAME> \
  --file=<DUMP_DIR>/pg-postetl-<DATE>.dump

# 全局对象（roles/tablespaces；pg_dump 不含；T-1 执行，此处为形状）
PGPASSWORD="$PG_ADMIN_PASSWORD" pg_dumpall \
  --host=<PG_HOST> --port=<PG_PORT> --user=<PG_ADMIN_USER> \
  --globals-only --file=<DUMP_DIR>/pg-globals-<DATE>.sql
```

### 3.5 归档与保留（90 天）

```bash
cd <DUMP_DIR>
sha256sum mysql-full-<DATE>.sql binlog-coord-<DATE>.txt \
  s3-manifest-<DATE>.jsonl s3-du-<DATE>.txt \
  pg-postetl-<DATE>.dump pg-globals-<DATE>.sql \
  es-repo-<DATE>.json es-snapshot-<DATE>.json > SHA256SUMS-<DATE>
# 前置：§3.1 的 "Dump completed" 断言已通过——截断的 dump 不得进入归档。

mc cp --recursive <DUMP_DIR>/ <S3_ALIAS>/<BACKUP_BUCKET>/cutover/<DATE>/
mc cat <S3_ALIAS>/<BACKUP_BUCKET>/cutover/<DATE>/SHA256SUMS-<DATE> | tee <DUMP_DIR>/SHA256SUMS-verify-<DATE>
# 断言：SHA256SUMS-verify 与本地 SHA256SUMS-<DATE> 逐行一致（上传未损坏）。

# 保留 90 天：对象存储生命周期规则或日历提醒删除；删除前需 DBA 复核（§8.4 第 8 项不覆盖此归档）。
```

### 3.6 恢复演练（T-7，隔离实例）

```bash
# MySQL 恢复到演练实例后抽查行数（§5 G2 口径）
mysql --defaults-extra-file=<MYSQL_DRILL_DEFAULTS_FILE> < <DUMP_DIR>/mysql-full-<DATE>.sql
PGPASSWORD="$PG_PASSWORD" pg_restore \
  --host=<DRILL_PG_HOST> --port=<DRILL_PG_PORT> --user=<PG_USER> --dbname=<DRILL_PG_DBNAME> \
  <DUMP_DIR>/pg-postetl-<DATE>.dump
# 断言：两侧关键表 COUNT(*) 与源一致；ES 快照可 restore 到演练集群（含 §6.3 的恢复序列与 _count 断言）；
# MySQL 写探针表（cutover_write_probe，§6.3）在演练库预建并跑通一次写探针。
```

演练结果（耗时、失败与修复）写入 `<ARTIFACT_DIR>/restore-drill-<DATE>.md`，作为 T0 时间参数依据。

## 4. 切换步骤

### 4.1 步骤总览（T0 当天，按序执行）

| # | 步骤 | 内容 | 通过条件 | 失败处置（abort） |
|---|---|---|---|---|
| 1 | 冻结与通知 | 维护公告生效、网关维护页开、`main` 冻结确认（T-1 已做） | 公告可见、维护页生效 | — |
| 2 | 停写（顺序不可换） | a. 网关维护/只读；b. **NATS 排空（worker 保持运行，`NumPending`→0）**；c. 停 worker（`keworker`/`keparser` 等）；d. **停全部旧 HTTP 服务（kecore/kechat/keapi/keparser/kesearch/account）**；e. MySQL `super_read_only = ON`；f. processlist 无业务连接残留 | 每步断言通过（§4.2） | 排空超时 `<DRAIN_TIMEOUT_MIN>` → 中止切换（**零数据丢失**）：失败发生在冻结（e 小步）之前 → 解除网关维护、**仅重启已停的 worker/旧 HTTP 服务**；MySQL 已 `super_read_only=ON` → 解除只读必须走 §6.3 步骤 3 的**双开关程序**（`super_read_only` + `read_only` + 写探针），不得只执行 `SET GLOBAL super_read_only = OFF` |
| 3 | 最终备份 | §3 全套（final dump + binlog 位点 + S3 清单 + ES 快照）+ `Dump completed`/位点断言 | SHA256 归档核验一致 | 中止切换（同上，无损） |
| 4 | 最终增量 ETL | `corekg migrate from-mysql --apply`（Phase 4 交付）+ sequence 重置 | 工具退出码 0 | 按评估文档 §11.3 定位；超过 `<MAX_WINDOW_HOURS>` → 回滚（无损，PG 未接流量） |
| 5 | 数据校验 | §5 G2–G4 | 全绿 | 修复后**重跑全套**；超时 → 回滚（无损） |
| 6 | **ES 重建** | 从 PG 全量重建 `ke_<company>` 全部索引（`ik_max_word`）：next-index 写入 + 别名切换（或文档化等价方案）；逐索引 `_count` 断言 + 完成断言；**旧索引原地保留**（§6.1，≥ 7 天） | `<ES_REBUILD_CMD>` 退出 0 + `_count` 与 PG 侧计数一致（§4.3） | 修复重跑；超时 → 回滚（无损） |
| 7 | PG post-ETL 快照 | §3.4：校验与 ES 重建完成后、**首个灰度请求之前** | dump 完成 + SHA 归档 | 中止/回滚（无损） |
| 8 | 启动新栈 | 部署 `<NEW_IMAGE_TAG>`（**不是** `make push-image APP=corekg` 的 legacy 镜像）；`/healthz`；golden-path 11/11（G1）；G5 召回（对步骤 6 重建的索引）；G7/G8 | 退出码 0 | 窗口内修复重跑；超时/不可修复 → 回滚（无损） |
| 9 | 灰度放量 | **旧 HTTP 服务保持停止**；1% → 10% → 100% 指路由到新栈的真实流量比例，其余流量维持维护页（不存在"旧栈服务 99%"的阶段——MySQL 已只读，旧栈无法承接写请求）；每档观察 `<CANARY_OBSERVE_MIN>` 分钟 | 每档 G6–G9 达标 | 触发 §6.2 条件 → 停止升档 / 回滚（**此时已有数据丢失面**，§7） |
| 10 | 观察 | 全量后观察 `<OBSERVE_HOURS>`（默认 1 小时），T+1 全天；观察项含 PG 重复主键错误率（SQLSTATE `23505`，漏 `setval` 的信号） | 无 §6.2 触发 | 按 §6 |
| 11 | 解冻 | **100% 档 G6/G9 达标后才解除维护页**；DNS TTL 恢复、`main` 解冻、完成通知。**成功路径旧 MySQL 保持 `super_read_only = ON` 直至 T+7 清理**（见下） | — | — |

**成功路径 MySQL 保持只读的原因**：切换后 PG 是唯一权威写库（§1.4）。若旧 MySQL 重新可写而旧镜像/旧服务被误启，会形成两套权威（split-brain）；保持只读让任何误操作直接报错而不是静默分流。只有回滚路径（§6.3 步骤 3）才解除只读，或 T+7 清理时随下线流程处置（§8.4 第 8 项）。

**止损原则**：步骤 2–8 之间任何失败都可无损中止（PG 未接业务流量；首个灰度请求在步骤 9）；步骤 9 之后失败进入 §6 回滚，必须先读 §7。窗口总预算 `<MAX_WINDOW_HOURS>`（由 T-7 演练数据回填），超预算一律中止/回滚。

### 4.2 停写命令形状（顺序不可换：先网关、再排空、后停机、最后冻结）

```bash
# (1) 网关切维护页/只读（占位形状：以实际网关机制为准，SRE 替换）
curl -sS -X PUT "<GATEWAY_ADMIN_URL>/mode" \
  -H "Authorization: Bearer <GATEWAY_ADMIN_TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{"mode":"maintenance"}'

# (2) NATS 排空：worker 保持运行，等 pending 清零（先排空、后停机，顺序不可换——
#     停机后再排空是不可能的）
nats --server <NATS_URL> consumer info <NATS_STREAM> <NATS_CONSUMER>
# 断言：每个 consumer 的 NumPending 与 NumAckPending 均为 0（逐 consumer 轮询，
# consumer 清单 T-7 冻结）。超过 <DRAIN_TIMEOUT_MIN> 未清零 → abort：
# 解除网关维护、保持 worker 运行继续消化，按 §4.1 步骤 2 的失败处置中止切换。

# (3) 停 worker（真实清单：keworker、keparser（解析 worker，含 chunker 等宿主 worker），
#     以及部署中存在的其他 NATS 消费者；以下 <OLD_WORKER_UNIT> 为占位，逐个执行）
ssh <DEPLOY_HOST> 'systemctl stop <OLD_WORKER_UNIT>'
# 停机后的 parked 消息语义：
#   abort  → 重启 worker，parked 消息继续消费（MySQL 恢复写后正常处理）；
#   success → 排空已在 (2) 验证为 0，正常无 parked 消息；若事后发现存量，
#             记入 §6.4 postmortem 处置，不得静默丢弃。

# (4) 停全部旧 HTTP 服务（split-brain guard：这些服务此后不得在 MySQL 只读期间承接任何流量）
#     清单：kecore、kechat、keapi、keparser、kesearch、account（及部署中的其他 legacy HTTP 服务）
ssh <DEPLOY_HOST> 'systemctl stop <OLD_SERVICE_UNIT>'   # 逐服务执行
# 断言：全部旧服务端口不再监听（ss -ltnp | grep <OLD_SERVICE_PORT> 为空）；
#       对外只剩维护页。此后到 §4.1 步骤 11（100% 门禁通过）前，用户侧 100% 是维护页。

# (5) MySQL 冻结写（评估文档 §12.2）
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> \
  -e "SET GLOBAL super_read_only = ON; SELECT @@global.super_read_only AS frozen;"
# 断言：frozen == 1。失败（权限/复制拓扑）→ DBA 处理或中止。

# (6) split-brain 兜底核查：MySQL 侧无业务连接残留
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> -e "SHOW PROCESSLIST;"
# 断言：除本会话与运维连接外，无旧服务/worker 的业务连接（有 → 回到 (4) 排查所停清单）。
```

### 4.3 最终增量 ETL 与 ES 重建命令形状

```bash
# ETL 二进制：make corekg 产出 bundles/corekg（新单体 cmd/corekg，含 migrate 子命令）。
# 不要用 make local APP=corekg——那构建旧 apps/corekg 且覆盖同一路径 bundles/corekg。
make corekg

# Phase 4 交付后的形状。当前 --apply 显式拒绝（§2 第 8 项 ⛔）；
# --checkpoint 同为 Phase 4 的提议 flag（断点重放，评估文档 §10.2 交付物），尚未实现，
# 实际 flag 名以 Phase 4 工具文档为准。
./bundles/corekg migrate from-mysql \
  --dsn "$CORekG_MYSQL_DSN" \
  --schemas core,account,knownow,chat,sale \
  --apply \
  --checkpoint <CHECKPOINT_DIR>/etl-<DATE>.json
# sequence 重置（评估文档 §4.2 的守卫写法）与 §5 G2–G4 校验内嵌于该工具（评估文档 §10.2）；
# 若 Phase 4 交付时拆分为独立步骤，则按评估文档 §4.2/§11 手动执行，顺序不可换。
```

ES 重建（§4.1 步骤 6）。**当前没有现成命令**——Phase 4 交付 `<ES_REBUILD_CMD>` 前，以下为必须满足的行为规格，不是可执行命令：

- **数据源**：PG（ETL/校验完成后的权威副本），不读 MySQL；
- **范围**：全部 `ke_<company>` 索引（按公司/知识库命名，`ik_max_word` 分词，AGENTS.md）；系统索引不在重建范围；
- **切换方式**：next-index 写入（如 `ke_<company>-v<N>`）+ 别名 `ke_<company>` 切换，或文档化的等价方案（逐索引 rebuild/新建+删旧），**必须避免"边删边建"的不可用窗口**；
- **断言**：逐索引 `GET <ES_ENDPOINT>/ke_*/_count` 与 PG 侧对应文档计数一致；重建命令退出码 0；
- **旧索引**：原地保留 ≥ 7 天（§6.1，回滚与 G5 旧基线对比都依赖它）。

```bash
# Phase 4 交付后的形状（<ES_REBUILD_CMD> 为占位，行为按上述规格验收）
<ES_REBUILD_CMD> --pg-dsn "$PG_DSN" --index-pattern 'ke_*' \
  --analyzer ik_max_word --alias-swap | tee <ARTIFACT_DIR>/es-rebuild-<DATE>.log

# 断言（现在即可执行的部分）：逐索引文档数
curl -sS "<ES_ENDPOINT>/ke_*/_count" -u "<ES_USER>:$ES_PASSWORD" \
  | tee <ARTIFACT_DIR>/es-rebuild-count-<DATE>.json
# 与 PG 侧文档计数（Phase 4 工具产出）逐索引比对，差异 = 0 才进入步骤 7。
```

### 4.4 启动新栈与灰度命令形状

```bash
# 部署：<NEW_IMAGE_NAME>:<NEW_IMAGE_TAG>（Phase 4 镜像管线产物，§2 第 2 项 ⛔）。
# 注意：make push-image APP=corekg 是 legacy apps/corekg 的镜像目标（docker.io/corekg/corekg-api），
# 构建的是旧单体——新栈部署绝不使用它，也不得用本地 bundles/corekg 直接部署。
ssh <DEPLOY_HOST> '<DEPLOY_SCRIPT> <NEW_IMAGE_NAME>:<NEW_IMAGE_TAG>'

# 健康检查（golden-path stage 1 同款）
curl -sS <COREKG_BASE_URL>/healthz    # 断言：HTTP 200 且 .status == "ok"

# golden-path 全量 11 stage（对 PG 新栈；凭据经 <ENV_FILE> 注入，600 权限，用后删）
# shellcheck disable=SC1090
source <ENV_FILE>    # TEST_USERNAME / TEST_PASSWORD_MD5 / MODEL_ID / KEAPI_API_KEY / EXPECT_TEXT
BASE_URL=<COREKG_BASE_URL> scripts/golden-path.sh 2>&1 | tee <ARTIFACT_DIR>/golden-path-pg-<DATE>.log
# 断言：退出码 0（11/11）；非 0 时 log 内含 stage=<name> FAILED 定位。

# 灰度（占位形状：以实际网关/负载均衡机制为准）
# 语义：weight_percent = 真实用户流量路由到新栈的百分比；其余流量继续看维护页。
# 旧 HTTP 服务在 §4.2(4) 已停止且不重启——不存在"旧栈服务 99% 流量"的阶段
# （MySQL 已 super_read_only，旧栈无法承接写请求，只能报错）。
# 100% 档且 G6/G9 达标后，才按 §4.1 步骤 11 解除维护页。
curl -sS -X PUT "<GATEWAY_ADMIN_URL>/routes/corekg/canary" \
  -H "Authorization: Bearer <GATEWAY_ADMIN_TOKEN>" -H 'Content-Type: application/json' \
  -d '{"weight_percent":1}'          # 依次 1 → 10 → 100；每档间隔 <CANARY_OBSERVE_MIN> 分钟
# split-brain guard（灰度全程生效）：旧 HTTP 服务保持停止、旧 MySQL 保持 super_read_only=ON、
# 只有新栈 + PG 接受写入；任何"临时把旧服务拉起来扛流量"的操作都违反本 runbook。
```

### 4.5 全局中止标准

无论当前处于哪一步，出现以下任一情况**立即中止**（未放流量走 §4.1 abort；已放流量走 §6）：

1. 超过 `<MAX_WINDOW_HOURS>` 未完成步骤 8（新栈启动 + 全量门禁预检）；
2. 备份不可核验（SHA256 不一致、dump 不完整）；
3. 发现评估文档 §12.1 风险清单中的数据损坏类事件（如 `bigint unsigned` 溢出批量出现）；
4. 值班人判断继续的风险大于收益（记录判断依据，事后复盘）。

## 5. 验证门禁

### 5.1 门禁清单

| # | 门禁 | 通过标准 | 执行时点 | 产出物 |
|---|---|---|---|---|
| G1 | golden-path 11 stage | 退出码 0，11/11 全绿（`scripts/golden-path.sh`） | 启动新栈后、灰度前；T+1 复跑 | log |
| G2 | 行数 + PK min/max | 每表 `COUNT`/`MIN`/`MAX` 三项两侧完全相等（评估文档 §11.1） | ETL 后 | 对账 CSV |
| G3 | 关键字段 checksum | 每表 `BIT_XOR` 相等（评估文档 §11.2 的 8 hex/32bit 口径；JSON 列除外，走 Go 重序列化比对） | ETL 后 | 对账 CSV |
| G4 | 抽样确定性比对 | 抽样行零差异（评估文档 §11.4） | ETL 后 | 行级 diff |
| G5 | ES 重建 + 搜索召回 | Top-10 命中重叠率下降 ≤ 2pp（§5.2）；**测量对象是 §4.1 步骤 6 重建出的索引** | 新栈启动后、灰度前 | 召回报告 |
| G6 | chat P95 | ≤ 基线 × 1.2 且 ≤ 5000ms（或产品 SLO `<CHAT_SLO_MS>`，取更严者）（§5.3）；**判定样本量 ≥ `<MIN_REQUESTS>`** | 灰度每档 | 监控截图/压测报告 |
| G7 | MCP | `initialize` HTTP 200 含 `"result"`；`tools/list` 的 `result.tools` 非空（§5.4）。**比 golden-path stage 11 更严**：stage 11 在响应无 `Mcp-Session-Id` 头时会跳过 `tools/list` 仍判通过，G7 不允许——tools/list 必须实际执行且非空 | 启动新栈后 | log |
| G8 | keapi search | `.code == 0` 且 `doc_search_result` 非空（§5.4） | 启动新栈后 | log |
| G9 | 错误率/延迟/主键冲突 | 5xx 率 ≤ 基线；全接口 P95 ≤ 基线 × 1.2；**PG 重复主键错误率 = 0（SQLSTATE `23505`——漏执行 `setval` 的最早信号，评估文档 §12.1 风险 4）** | 灰度每档 + 观察期 | 监控截图 |

G2–G4 汇总进评估文档 §11.6 的每表对账表，DBA 逐行签收。**任一门禁失败：未放流量 → 修复后重跑全套（不得只重跑失败项）；已放流量 → 按 §6.2 触发条件处置。**

### 5.2 搜索召回对比口径（G5）

- **query 集**：`<RECALL_QUERY_SET>`（T-7 冻结；来源：迁移前 30 天高频搜索 query 抽样 `<RECALL_N>` 条 + golden-path 的 `EXPECT_TEXT` 唯一 token；清单存 `<ARTIFACT_DIR>/recall-queries-<DATE>.txt`，每行一条 query，配套 `<RECALL_FOREST_ID>` 记录目标森林）。
- **方法**：迁移前（旧栈）与 ES 从 PG 重建后（新栈，§4.1 步骤 6 产物）各执行同一 query 集，取每条 Top-10 文档 id 集合；指标 = 平均重叠率 `Σ|new ∩ old| / (10 × N)`。
- **通过标准**：平均重叠率 ≥ 0.98（即下降 ≤ 2 个百分点）。不达标先查 ES 重建完整性（§4.3 的 `_count` 断言），再查评估文档 §6 的 DB 直查改写（`~*`/`ILIKE` 站点遗漏）。
- **两侧命令必须同款**：先登录取 JWT，再带 `Authorization: Bearer` 调 `ForestSearch`，请求体用 yg-go 信封 `{cmd, request}`（与 `scripts/golden-path.sh` stage 2/8 一致）。

```bash
# (1) 登录取 JWT（golden-path stage 2 同款；凭据经 <ENV_FILE> 注入，占位符不落盘）
JWT="$(curl -sS <BASE_URL>/v3/account.LoginByPassword \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg domain "<TEST_DOMAIN>" --arg user "$TEST_USERNAME" --arg pass "$TEST_PASSWORD_MD5" \
    '{cmd:"account.LoginByPassword", request:{domain_name:$domain, username:$user, password:$pass}}')" \
  | jq -r '((.response // .Response).jwt_token) // empty')"
# 断言：JWT 非空（空 = 登录失败，先修复登录，召回对比无意义）。

# (2) 逐条 query 调 ForestSearch，取 Top-10 文档 id
while IFS= read -r q; do
  curl -sS <BASE_URL>/v3/kesearch.ForestSearch \
    -H 'Content-Type: application/json' -H "Authorization: Bearer $JWT" \
    -d "$(jq -n --argjson forest_id <RECALL_FOREST_ID> --arg text "$q" \
      '{cmd:"kesearch.ForestSearch", request:{forest_id:$forest_id, text:$text, is_semantics:false}}')" \
    | jq '(.Response // .response).doc_search_result | map(.id) | map(select(. != null)) | .[0:10]'
done < <ARTIFACT_DIR>/recall-queries-<DATE>.txt > <ARTIFACT_DIR>/recall-<SIDE>-<DATE>.jsonl
# <SIDE> = old（迁移前，BASE_URL=<OLD_BASE_URL>）/ new（重建后，BASE_URL=<COREKG_BASE_URL>）；
# id 字段 = ForestSearch DTO 的 json:"id"（apps/kesearch/models/globalsearch/file.go，注意不是 file_id），两侧同款命令。
# 守卫（必须执行）：任一侧任一 query 的非空 id 少于 10 个或列表为空 → G5 直接判失败（fail），
# 不得剔除该条继续算重叠率——空列表对空列表会算出 100% 重叠的假阳性。守卫通过后，
# 两侧 jsonl 逐行做交集统计，按上述口径算平均重叠率。
```

### 5.3 chat P95 口径（G6）

- 基线 = 迁移前 7 天监控的 chat 接口 P95 `<CHAT_P95_BASELINE_MS>`；监控缺历史则用压测（`<LOAD_TOOL>` 以 `<CHAT_RPS>` 持续 15 分钟）。
- 目标 = `min(基线 × 1.2, 5000ms)`；若产品已定 SLO `<CHAT_SLO_MS>`，取更严格者。
- **最小样本量**：判定需要该档累计样本 ≥ `<MIN_REQUESTS>`；样本不足时不作通过/失败判定，**继续观察**（1% 档流量小，正常情况下样本不足——保持观察推进到 10%/100% 档再判定，而不是用 20 个样本宣布 P95 达标）。
- 注意：golden-path stage 9 是**单次请求**，只作可用性断言（`.answer` 非空），不能当 P95 依据。

### 5.4 MCP 与 keapi 验证命令（G7/G8）

```bash
# initialize（协议版本与 scripts/golden-path.sh 一致）
curl -sS -D <ARTIFACT_DIR>/mcp-headers-<DATE>.txt <COREKG_BASE_URL>/v3/keapi/mcp \
  -H "Authorization: Bearer <KEAPI_API_KEY>" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"cutover-check","version":"1.0"}}}'
# 断言：HTTP 200 且响应含 "result"。

# tools/list：从 mcp-headers-<DATE>.txt 取 Mcp-Session-Id（解析方式与 scripts/golden-path.sh 一致）；
# 注意：G7 严于 golden-path stage 11——stage 11 在无 Mcp-Session-Id 头时跳过 tools/list 仍算通过，
# G7 要求 tools/list 必须实际执行且 result.tools 非空（无会话头时视为 G7 失败，人工排查会话管理）。
MCP_SESSION_ID=$(grep -i '^mcp-session-id:' <ARTIFACT_DIR>/mcp-headers-<DATE>.txt \
  | head -n1 | sed 's/^[^:]*:[[:space:]]*//; s/\r$//')
curl -sS <COREKG_BASE_URL>/v3/keapi/mcp \
  -H "Authorization: Bearer <KEAPI_API_KEY>" \
  -H "Mcp-Session-Id: $MCP_SESSION_ID" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
# 断言：HTTP 200 且 result.tools 为非空数组（当前 21 个工具，见 docs/mcp-server.md）。

# keapi search（golden-path stage 10 同款）
curl -sS <COREKG_BASE_URL>/v3/keapi.Search \
  -H "Authorization: Bearer <KEAPI_API_KEY>" -H 'Content-Type: application/json' \
  -d @<KEAPI_QUERY_FILE>
# 断言：.code == 0 且 (.response // .Response).doc_search_result | length > 0。
```

### 5.5 Go/No-Go 定义

- **会议**：T0 前 `<GO_NOGO_TIME>`（默认 T-1 21:00），§8.1 四方 owner 到场（可远程）。
- **GO 条件（同时满足，缺一不可）**：
  1. §2 清单无 ⛔，⬜ 项仅剩"窗口内才能执行"的动作（T-1/T0 项）。**基线口径**：第 5 项的 golden-path 11/11 旧栈基线必须在 Task 5 之前的 commit `630ba7b9f`（或生产镜像）上采集——本分支 HEAD 已把旧 `apps/*` 改为 PG-only，在 HEAD 上跑出的结果不构成旧栈基线；据此口径不满足时同样判 NO-GO；
  2. Phase 4 演练记录：G1–G5 在演练环境全绿（含 `<ES_REBUILD_CMD>` 已按 §4.3 规格交付并在演练环境跑通）且记录在 `<ARTIFACT_DIR>`；
  3. 回滚演练完成（§2 第 9 项，评估文档 §12.4 要求）；
  4. 业务确认维护窗口已公告、数据丢失边界（§7）已说明。
- **NO-GO 处置**：窗口顺延 `<NEXT_WINDOW_DATE>`；若 `main` 已冻结且未开始停写，立即解冻；若已停写则按 §4.1 步骤 2 的 abort 无损恢复——未冻结（e 小步之前）仅解除网关维护并重启已停服务；MySQL 已 `super_read_only=ON` 时，解除只读必须走 §6.3 步骤 3 的**双开关程序**（`super_read_only` + `read_only` + 写探针）。
- **灰度升档 mini Go/No-Go**：每档观察期末，SRE owner（应用 owner 备份）依据 G6–G9 决定"升档 / 保持观察 / 回滚"；判断与依据记入 status 记录。

## 6. 回滚步骤

### 6.1 保留物（不提前清理）

| 保留物 | 保留方式 | 最短期限 |
|---|---|---|
| 旧镜像 `<OLD_IMAGE_TAG>` | 镜像仓库不删 tag（legacy 管线产物，§1.2） | ≥ 7 天 |
| 旧 MySQL | **成功与回滚两种路径都先保持 `super_read_only = ON`**：成功路径保持只读直至 T+7 清理（§4.1 步骤 11——PG 已是唯一权威，旧库可写 = split-brain 风险）；回滚路径才在 §6.3 步骤 3 解除。T0 前的 final dump + binlog 归档另存 90 天（§3.5） | ≥ 7 天 |
| 旧 ES 索引 | 索引未删除则原地保留；若重建覆盖同名索引，用 pre-cutover 快照恢复（§6.3） | ≥ 7 天 |
| PG 库 | **回滚后不删除**：只读留存取证（仅 DBA/排障可访问），直到 postmortem 关闭 + 数据处置方案签收，**可能 > 7 天** | ≥ 7 天，至 postmortem 关闭 |
| 对象存储 bucket | 原样不动（无回滚动作；孤儿处置见 §7.3） | 不限 |

### 6.2 触发条件

**硬触发**（值班人直接执行 §6.3，事后 30 分钟内通报；无需等会议）：

1. G1–G4 任一失败且 30 分钟内未定位或未修复（数据正确性不可妥协）；
2. 新栈 5xx 错误率 > 5% 持续 10 分钟；
3. chat P95 > 基线 × 2 持续 15 分钟；
4. 出现数据损坏证据（运行期校验漂移、业务报错指向数据错误）。

**软触发**（召开 Go/No-Go 会议（§5.5）决定回滚或继续）：

5. G5 召回下降在 2pp 边缘（重跑确认后仍不达标）；
6. P95 退化在基线 × 1.2 至 × 2 之间；
7. 非核心功能异常（数据正确、个别边缘 action 报错）。

**冻结窗口内**（已停写、未放流量，即 §4.1 步骤 2–8 期间）：任何门禁失败直接走 §4.1 的 abort——此时**零业务数据丢失**，是最便宜的"回滚"（评估文档 §12.4）。

### 6.3 回滚执行序列（顺序不可换）

1. **冻结 PG 写入**（先于一切切回动作，防止丢失面扩大）：停新 `corekg`（或网关摘除 100% 流量）+ PG 侧只读兜底 + 终断既有连接；
2. **业务确认数据丢失边界**（§7.4）：产品 owner 签字；未签字不得继续。若 PG 已接受写入且业务不接受丢失 → 停在"PG 只读保持"状态，升级 Go/No-Go 会议（可选前向修复，§7.4 例外）；
3. **恢复 MySQL 写**：`super_read_only` 与 `read_only` **两个开关都显式关**（`SET GLOBAL super_read_only = OFF` 不会连带清除 `read_only`，只关一个会导致回滚后写仍失败），断言两个读数均为 0，并做**真实写探针**（BEGIN/INSERT/ROLLBACK）后才继续；
4. **启动旧栈**：启动旧 HTTP 服务与 worker（§4.2 停掉的 `<OLD_SERVICE_UNIT>`/`<OLD_WORKER_UNIT>`，反向 split-brain guard：此时新栈已停/已摘流量），健康确认（`<OLD_BASE_URL>/healthz`）通过前**不解除维护页**；
5. **流量切回**：网关 upstream 切回 `<OLD_IMAGE_TAG>`，DNS（TTL ≤ 60s）指回 `<OLD_LB_IP>`，解除维护页；
6. **ES 处置**：旧索引原地保留（§4.1 步骤 6 的 alias-swap 方案）→ 直接把别名切回旧索引；若旧索引曾被覆盖/删除 → 从 pre-cutover 快照恢复（命令见下，含轮询与 `_count` 断言）；
7. **PG 留存取证**：保持只读 + 访问审计，**不得 DROP/清空**（直到 postmortem 关闭）；
8. **事后处置**：PG 增量导出评估（§7.3）、对象孤儿清理评估（§7.3）、`monolith-v2` 继续修复（`main` 保持冻结）；
9. **重试切换**：**以 postmortem 关闭为前置 gate**。先重置 PG：逐个 `DROP SCHEMA core CASCADE;`（5 个 schema，评估文档 §12.3）→ checkpoint 重放/重跑 ETL → 重跑 G1–G9 全套（不得"只测改动点"）→ 重新走 §5.5 Go/No-Go。

```bash
# 1) 冻结 PG 写
ssh <DEPLOY_HOST> '<STOP_COMMAND>'
PGPASSWORD="$PG_ADMIN_PASSWORD" psql "<PG_ADMIN_DSN>" \
  -c "ALTER DATABASE <PG_DBNAME> SET default_transaction_read_only = on;"
PGPASSWORD="$PG_ADMIN_PASSWORD" psql "<PG_ADMIN_DSN>" \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '<PG_DBNAME>' AND pid <> pg_backend_pid();"

# 3) 恢复 MySQL 写：两个开关都要关（super_read_only=OFF 不清除 read_only）
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> \
  -e "SET GLOBAL super_read_only = OFF; SET GLOBAL read_only = OFF;"
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> \
  -e "SELECT @@global.super_read_only AS sro, @@global.read_only AS ro;"
# 断言：sro == 0 且 ro == 0（任一为 1，回滚后写请求会继续报错——开关读数不是写能力的证明）。

# 3b) 写探针（切回流量前必做）：对 T-7 预建的探针表做真实写（事务内 INSERT 后 ROLLBACK，
#     验证整条写路径：权限、引擎、触发器、只读状态，而不只是开关读数）
mysql --defaults-extra-file=<MYSQL_DEFAULTS_FILE> <PROBE_DB> \
  -e "BEGIN; INSERT INTO cutover_write_probe(id, ts) VALUES (1, NOW()); ROLLBACK; SELECT COUNT(*) FROM cutover_write_probe;"
# 断言：无报错（任何只读残留/权限问题都会在这里暴露）；探针表 DDL（T-7 预建）：
#   CREATE TABLE IF NOT EXISTS cutover_write_probe (id BIGINT PRIMARY KEY, ts DATETIME(3));

# 4) 启动旧栈（逐服务；健康确认通过前不解除维护页）
ssh <DEPLOY_HOST> 'systemctl start <OLD_SERVICE_UNIT>'
ssh <DEPLOY_HOST> 'systemctl start <OLD_WORKER_UNIT>'
curl -sS <OLD_BASE_URL>/healthz    # 断言：HTTP 200

# 5) 流量切回（网关 + DNS；以实际机制为准）
curl -sS -X PUT "<GATEWAY_ADMIN_URL>/routes/corekg/upstream" \
  -H "Authorization: Bearer <GATEWAY_ADMIN_TOKEN>" -H 'Content-Type: application/json' \
  -d '{"upstream":"<OLD_IMAGE_TAG>"}'
# DNS：对外域名 A 记录指回 <OLD_LB_IP>（TTL 已降，生效 ≤ 5 分钟；以 <DNS_PROVIDER> 实际为准）
# 解除维护页后，golden-path 对旧栈复跑确认恢复（<OLD_BASE_URL>）。

# 6) ES 快照恢复（仅当旧索引被覆盖/删除时；安全分支：旧索引原地保留且完好 → 不 restore，
#    仅把别名切回旧索引，见步骤 6）：
#    处置目标索引首选 DELETE 而非仅 _close——别名会存留在已关闭（closed）的索引上，
#    与快照恢复回来的同名别名冲突（restore 失败或别名指向错误）。删除对象含重建产生的
#    -v<N> 目标索引（当前持别名；ke_* 通配符同时匹配 ke_<company> 与 ke_<company>-v<N>）；
#    快照在手可重复恢复，删除可重放。若确需保守动作（网关尚未完全摘流量等），_close 为次选，
#    但 restore 报别名冲突时回退 DELETE 重试。
curl -sS -X DELETE "<ES_ENDPOINT>/ke_*" -u "<ES_USER>:$ES_PASSWORD"
curl -sS -X POST "<ES_ENDPOINT>/_snapshot/<SNAPSHOT_REPO>/pre-cutover-<DATE>/_restore?wait_for_completion=false" \
  -u "<ES_USER>:$ES_PASSWORD" -H 'Content-Type: application/json' \
  -d '{"indices":"ke_*","include_global_state":false}'
# 轮询恢复状态直至 SUCCESS（上限 <ES_RESTORE_TIMEOUT_S>；不委派给执行者现场发挥）
deadline=$(( SECONDS + <ES_RESTORE_TIMEOUT_S> ))
until curl -sS "<ES_ENDPOINT>/_snapshot/<SNAPSHOT_REPO>/pre-cutover-<DATE>/_status" \
    -u "<ES_USER>:$ES_PASSWORD" | grep -q '"state":"SUCCESS"'; do
  if (( SECONDS >= deadline )); then
    echo "ES restore not SUCCESS after <ES_RESTORE_TIMEOUT_S>s"; exit 1
  fi
  sleep 30
done
# 断言：逐索引文档数与快照前记录一致（§3.3/§4.3 的 _count 基线）
curl -sS "<ES_ENDPOINT>/ke_*/_count" -u "<ES_USER>:$ES_PASSWORD" | tee <ARTIFACT_DIR>/es-restore-count-<DATE>.json

# 9) 重试前的 PG 重置（postmortem 关闭后执行；逐 schema）
PGPASSWORD="$PG_ADMIN_PASSWORD" psql "<PG_ADMIN_DSN>" \
  -c "DROP SCHEMA IF EXISTS core CASCADE; DROP SCHEMA IF EXISTS account CASCADE; \
      DROP SCHEMA IF EXISTS knownow CASCADE; DROP SCHEMA IF EXISTS chat CASCADE; \
      DROP SCHEMA IF EXISTS sale CASCADE;"
# 随后 checkpoint 重放 / 重跑 ETL（§4.3）→ 全套门禁 → §5.5 Go/No-Go。
```

### 6.4 回滚后动作

- 48 小时内 postmortem（§9 表末行），结论进 `<ARTIFACT_DIR>/postmortem-<DATE>.md`；
- postmortem 关闭前：PG 不删、旧 MySQL/ES/镜像保留期不开始倒计时清理；**重试的 PG 重置（`DROP SCHEMA ... CASCADE` + checkpoint 重放，§6.3 步骤 9）同样不得执行**——重置即销毁取证现场；
- 对外公告恢复（产品 owner）；
- 重试窗口由 Go/No-Go 会议重新排期（回到 §9 的 T-7 项——**恢复演练等项不因"刚做过"而免做**，仅可缩短）；重试序列固定为：PG 重置 → 重跑 ETL → 全套门禁 → Go/No-Go（§6.3 步骤 9）。

## 7. 数据丢失边界

> **⚠️ 全文最重要的章节：回滚 = 丢数据。签字前必须读完本节。**
>
> **切换（T0 放流量）之后被 PostgreSQL 接受的所有业务写入，在回滚时不会自动回到 MySQL——它们会丢失。** 本方案没有反向同步（不存在 PG → MySQL 同步通道，也不做双写，§1.1）；回滚后 PG 上的新数据只能事后人工导出合入，且不保证完整（§7.3）。这不是脚注，是回滚的定价。

### 7.1 边界的精确定义

- **"PG 接受的写入"**：从灰度第一笔流量（§4.1 步骤 9 的 1%）起，到回滚冻结 PG 写（§6.3 步骤 1）止，所有落库的业务数据——新建森林/文件/账号、chat 会话与消息、QA、API Key 与设置变更等**全部写路径**。
- **零丢失窗口**：§4.1 步骤 2–8 期间（已停写、未放流量）中止/回滚 = MySQL 权威未变、PG 未接流量，**零业务数据丢失**。这是最便宜的回滚窗口，宁可在此窗口内保守中止。
- **ETL/校验/ES 重建失败后放弃**（步骤 4–7 的 abort）= 无业务数据丢失：PG 上只有迁移数据，丢弃或 checkpoint 重放即可。

### 7.2 保留 vs 丢失（回滚后）

| 数据 | 回滚后状态 | 说明 |
|---|---|---|
| MySQL 侧 T0 前全部数据 | ✅ 保留 | 权威从未离开 MySQL |
| 对象存储 T0 前已有对象 | ✅ 保留 | 对象层未做任何操作（§1.3） |
| 备份归档（dump/binlog/ES 快照/PG post-ETL 快照） | ✅ 保留 | 90 天（§3.5） |
| 对象存储 T0 后新上传对象 | ⚠️ 物理保留，但成**孤儿** | bucket 不回滚；DB 无对应记录（§7.3） |
| PG 在 T0 后接受的全部业务写入 | ❌ **丢失** | 无反向同步；人工合入不保证完整（§7.3） |
| 切换后新建 chat 会话 | ❌ **丢失** | 引用链（消息/文件 id）断裂，部分合入也无法恢复完整性 |
| 从 PG 重建的 ES 索引内容 | ⚠️ 随回滚失效 | 回滚后以旧索引/pre-cutover 快照为准 |
| NATS 未消费消息 | 视停写策略 | 冻结前排空（§4.2）则无；否则逐条业务处置 |

### 7.3 孤儿数据与人工合入

- **对象孤儿**：T0 后上传的对象在回滚后 DB 无记录。不影响正确性，只占存储；清理是事后专项（§3.2 的 manifest 与 DB 引用做差集，人工确认后删）。**不阻塞回滚**。
- **PG-only 行人工合入**（仅当业务明确要求）：`pg_dump --table` 导出 → 逐行 review → 手工写回 MySQL（评估文档 §12.3 第 4 步）。已知限制：id 冲突（MySQL `AUTO_INCREMENT` 未前进，需先对齐）、引用完整性断裂（合入消息不合入其引用的 PG-only 文件 = 断链）。**默认预期是不合入、接受丢失**——这正是需要业务签字的原因。
- **chat 会话**：即使逐行合入消息，会话与 PG-only 文件的引用关系无法整体恢复；建议整体放弃，除非单表独立可合入。

### 7.4 回滚前置条件（顺序不可换）

1. 冻结 PG 写入（§6.3 步骤 1，先止损失边界）；
2. 向产品 owner 呈现实际丢失面：按 §7.2 表 + PG 真实写入量统计，例如逐关键表 `SELECT COUNT(*) FROM knownow.ke_forest WHERE created_at > '<T0_TS>';`。**`knownow.ke_forest` 只是示意表**——inventory 当前缺失（§2 第 6 项 ⛔），实际表清单以 Phase 4 inventory 产出为准；
3. 产品 owner 在 §8.4 签收表签字，三选一：**接受丢失** / **要求尝试人工合入**（拉长 RTO，§7.5）/ **拒绝回滚改为前向修复**；
4. 之后才执行 §6.3 步骤 3（恢复 MySQL 写）及其后步骤。

例外：业务选"前向修复"（fix-forward）时不走回滚序列，转入事故响应（§8.3 升级路径），数据保留在 PG 由修复过程处置。

### 7.5 RPO / RTO（占位，切换前由业务与 SRE 填写并签收）

- **目标 RPO = `<RPO>`**：零丢失窗口内回滚 RPO = 0；放流量后回滚 RPO = PG 已接受的全部写入（上界 ≈ 放量时长 × 写入速率）。
- **目标 RTO = `<RTO>`**：技术序列本身分钟级（停新栈 + 恢复 MySQL 写（双开关 + 写探针）+ 启动旧栈 + 网关/DNS 切回；DNS 生效 ≤ TTL = 60s）；主要变量是 §7.4 的**决策时间**，因此丢失面呈现材料需在 T-7 准备好模板。
- 若业务给出"不可接受任何丢失"（RPO = 0 且不接受 §7.4 签字流程）：一次性切换模式不满足该目标，升级架构评审（§1.1），**不得临时拼凑双写**。

## 8. 沟通与责任人

### 8.1 角色与 owner（T-7 前指定，一人一角色，各有备份）

| 角色 | owner | 备份 | 职责 |
|---|---|---|---|
| 应用 owner | `<APP_OWNER>` | `<APP_BACKUP>` | ETL 执行、G1/G7/G8 门禁、代码修复决策 |
| DBA | `<DBA_OWNER>` | `<DBA_BACKUP>` | 备份/binlog/`super_read_only`/setval、G2–G4、PG 冻结与留存 |
| SRE | `<SRE_OWNER>` | `<SRE_BACKUP>` | 镜像/网关/DNS/灰度/监控、G6/G9、回滚序列执行 |
| 产品 | `<PRODUCT_OWNER>` | `<PRODUCT_BACKUP>` | 维护公告、数据丢失边界签字、G5/G6 业务验收 |

### 8.2 会议、事故桥与节奏

- **Go/No-Go 会议**：T-1 21:00（§5.5）+ 灰度升档即时决策 + T+1 复盘会；回滚触发时临时召开（硬触发先执行后开会，§6.2）。
- **事故桥**：`<BRIDGE_URL>`（电话兜底 `<BRIDGE_PHONE>`），T0 全程在线；任何 §6.2 触发即入桥。
- **节奏**：窗口内每 30 分钟在 `<NOTIFY_CHANNEL>` 发 status（当前步骤号 + 门禁状态 + 下一动作）；异常即时上报不等整点。

### 8.3 升级路径

on-call 值班 → 角色 owner（15 分钟未响应即升级）→ 部门负责人 →（数据正确性事件）CTO。**数据丢失边界签字只能由 `<PRODUCT_OWNER>` 或其书面授权人给出**，其余任何人不得代签。

### 8.4 签收表（模板；空白 = 未签）

| # | 事项 | 签收人 | 时间 | 结果 |
|---|---|---|---|---|
| 1 | §2 检查清单无未关闭 blocker | SRE + DBA | | GO / NO-GO |
| 2 | 备份与归档完成（SHA256 已核） | DBA | | |
| 3 | 维护窗口与数据丢失边界已向业务说明 | 产品 | | |
| 4 | GO 决定（切换开始） | 四方 | | |
| 5 | 灰度 1% → 10% → 100% 每档放行 | SRE | | |
| 6 | 切换完成（T+1 无回滚） | 四方 | | |
| 7 | 回滚数据丢失边界确认（仅回滚时） | 产品 | | 接受丢失 / 要求合入 / 前向修复 |
| 8 | 清理批准（T+7，旧栈下线/归档转入常规流程） | DBA + SRE | | |

## 9. 时间表模板

| 时间 | 事项 | owner | 产出物 | 决策点 |
|---|---|---|---|---|
| T-7 | 备份恢复演练（§3.6，含 MySQL 写探针表预建与探针演练）；ES snapshot repo 注册 + 快照验证；golden-path 基线复核（**必须已 11/11 绿**）；召回 query 集冻结（§5.2）；**旧栈侧召回基线执行**：query 集冻结后立即按 §5.2 以 `<SIDE>=old`、`BASE_URL=<OLD_BASE_URL>` 跑完并留存产物（趁旧栈仍在线——T0 停写后旧 HTTP 服务已停，届时无法再取基线）；角色 owner 指定（§8.1）；`<MAX_WINDOW_HOURS>` 回填；**镜像 registry/repo 约定确认**：记录 legacy 管线约定（Makefile `IMAGE_NAME = docker.io/corekg/<APP>-api`）并**显式决定**新单体镜像名 `<NEW_IMAGE_NAME>`（registry 可达、登录凭据通道可用），同时记录 `<OLD_IMAGE_TAG>` 与 `<NEW_IMAGE_TAG>` 的 digest | DBA/SRE/全部 | 演练报告、`recall-queries-<DATE>.txt`、`recall-old-<DATE>.jsonl`、owner 表、镜像约定与 digest 记录 | 演练失败 / 镜像约定未决 → 窗口顺延 |
| T-1 | `main` 冻结；DNS TTL 降至 ≤ 60s；§2 清单终检；Go/No-Go 会议（§5.5）；维护公告发布；`<ENV_FILE>`/`<MYSQL_DEFAULTS_FILE>` 通道验证 | SRE/全部 | 冻结记录、会议纪要、公告链接 | **GO / NO-GO** |
| T0 | §4 全步骤：停写（含停旧 HTTP 服务）→ 最终备份 → ETL → 校验 → ES 重建 → PG 快照 → 新栈 → 灰度 1%/10%/100%（其余流量维护页）→ 观察 | 全部 | 门禁产物、每 30 分钟 status 记录 | 每步 abort（§4.1）；触发回滚（§6.2） |
| T0+1h | 初步稳定性判定 | SRE | 观测报告 | 保持 / 回滚 |
| T+1 | 观察日：全天监控对比（含 SQLSTATE `23505` 重复键错误率）、召回抽检、chat P95 对比；切换完成签收（§8.4 第 6 项）；golden-path 复跑一次 | 全部 | 对比报告、签收表 | **完成签收** |
| T+7 | 清理：旧 MySQL/ES 归档或下线流程启动（前提：≥ 7 天保留期满且无未决 postmortem；成功路径 MySQL 在此才解除只读并随下线处置）；对象孤儿清理评估（§7.3）；`monolith-v2` 合回 `main`、解冻；本 runbook 复盘修订 | DBA/SRE/应用 | 清理清单、复盘文档 | 清理批准（§8.4 第 8 项） |
| 若回滚 | 48h 内 postmortem；PG/旧栈保留期冻结（§6.1）；postmortem 关闭后才执行 PG 重置 + checkpoint 重放（§6.3 步骤 9）；重试窗口决策（回 T-7 项，可缩短不可免做） | 全部 | `postmortem-<DATE>.md` | 重试 / 延期 |

---

### 附：DBA/SRE review 记录（当前未签署）

本文要求在切换前由 DBA 与 SRE 各一名 review 并在下表记录结论。**当前（2026-11-01，Phase 0）本表为空：本文未经 DBA/SRE review，不构成任何"已批准"的声明。** 以下为"待切换前由 DBA/SRE 确认"清单（其缺失不阻塞 Phase 0 交付，但**阻塞切换执行**，即 §2 第 25 项 ⛔）：

DBA 待确认：

1. §3.1 mysqldump 参数（`--single-transaction` 的引擎前提、`--source-data=2` 的版本/权限）与 binlog 位点语句选择（`SHOW BINARY LOG STATUS` vs `SHOW MASTER STATUS`）；
2. §4.2 `super_read_only` 冻结对业务与复制拓扑的影响；§6.3 的双开关解除（`super_read_only` + `read_only`）与写探针表（`cutover_write_probe`）预建；
3. §6.3 PG 只读冻结方式（`ALTER DATABASE ... default_transaction_read_only` vs 仅停进程）与连接终断的影响面；
4. ETL sequence 重置是否内嵌（评估文档 §4.2 守卫写法为兜底）；
5. 备份容量与时长预估、`<DUMP_DIR>` 空间（§2 第 21 项）；
6. 保留期成本（90 天归档 + 旧栈 ≥ 7 天）；
7. RPO/RTO 目标值（§7.5）与业务对齐。

SRE 待确认：

1. 网关维护页/只读模式与灰度权重的实际机制（替换本文占位命令）；
2. DNS TTL 可控性与实际生效时间；
3. 镜像 digest 记录与 `<OLD_IMAGE_TAG>` 保留策略（§6.1）；
4. 监控大盘/告警对 §5 G6/G9 指标的覆盖；
5. NATS 排空的实际命令与超时（§4.2 第 (2) 步）、停机清单（worker/旧 HTTP 服务）的实际单元名；
6. 部署通道（SSH/CI）与回滚部署的实测耗时。

| reviewer | 角色 | 日期 | 结论（approve / changes requested） | 备注（指向当时 commit） |
|---|---|---|---|---|
| （空） | DBA | | | |
| （空） | SRE | | | |

review 记录随 git 历史保留；任何"approve"必须指向评审时的 commit hash，文档修订后需重新评审。
