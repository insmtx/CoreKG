# CoreKG 本地基础环境搭建指南

本文档说明如何在本地快速搭建 CoreKG 所需的全部基础中间件（MySQL / Elasticsearch / Redis / MinIO / NATS / Nebula Graph / Coze 可选），并初始化数据。所有中间件均通过 Docker Compose 编排，镜像统一使用 **yygu 自建镜像**（`registry.cn-beijing.aliyuncs.com/yygu/corekg` 与 `registry.yygu.cn/library`），与私有化 Helm 部署（corekg-chart）同一套。

## 1. 总览

| 中间件 | 镜像 | 容器内部端口 | 宿主机映射端口 | 账号 / 密码 |
|---|---|---|---|---|
| MySQL | `mysql_8.4.5` | 3306 | **3308** | root / `123456`；corekg / `123456` |
| MySQL（opencoze 附加库） | - | - | - | 同一 MySQL |
| Elasticsearch | `elasticsearch_8.18.1-2`（内置 IK 分词） | 9200 / 9300 | **9202** / **9302** | elastic / `123456` |
| Redis | `redis_8.2` | 6379 | **6381** | 无密码 |
| MinIO | `minio_RELEASE.2025-04-22T22-12-26Z` | 9000 / 9001 | **9002** / **9003** | minioadmin / `minio123456` |
| NATS | `nats:2.12.7` | 4222 | **4225** | 无认证 |
| Nebula Graph（3 容器） | `nebula-metad` / `nebula-storaged` / `nebula-graphd`（`:v3.8.0`） | metad 9559 / storaged 9779 / graphd 9669 | **9559 / 9779 / 9669** | root / `nebula` |
| CoreKG 前端 | `corekg-frontend:latest` | 80 | **3001** | 使用 CoreKG 登录 |

> **镜像仓库说明**：上表简写镜像位于 `registry.cn-beijing.aliyuncs.com/yygu/corekg:<简写>`（除 `nebula-metad`、`nats` 在 `registry.yygu.cn/library/<镜像>:<tag>`）。完整镜像地址以 `docker-compose.yml` 为准。

> **端口设计说明**：容器内部保留各服务默认端口（互连时用服务名+内部端口）；宿主机端口统一在默认端口基础上 **+2**，以避免本地可能已安装的 MySQL/Redis/ES/MinIO 占用默认端口导致启动冲突。如某端口仍被占，可自行在 `docker-compose.yml` 中改映射端口，并同步修改对应 `config.yaml`。

> **Nebula 端口例外**：Nebula 是 3 容器集群（metad/storaged/graphd），三个进程之间用服务名+内部端口互连，故 **不遵循 +2**，宿主机端口与容器端口一致（9559 / 9779 / 9669）。DB settings（`knowledge/nebula`）里的 `address` 填 `graphd`、`port` 填 `9669`（容器内服务名）；宿主进程连图库走映射端口 `localhost:9669`。

> **凭据说明**：除 Nebula 外，所有中间件明文密码统一为 `123456`（本地开发默认值，欢迎直接使用）。Nebula 使用其内置默认账号 `root` / `nebula`。生产环境请勿使用本文件与默认密码。

## 2. 快速启动

```bash
# 1) 首次执行基础设施初始化（完成后 keinit 正常退出）
docker compose up --build keinit

# 2) 启动前端、corekg 及其依赖
docker compose up -d --build frontend

# 3) 确认状态；前端访问地址为 http://localhost:3001
docker compose ps
```

如需同时启动 pipeline、文档转换等全部服务，执行 `docker compose up -d --build`。前端容器同时代理 `/v2/*`、`/v3/*` 到 `corekg:8080`，以及 `/corekg-bucket/*` 到 `minio:9000`；可通过 `curl -fsS http://localhost:3001/healthz` 检查前端健康状态。本地 Compose 不编排 Coze 工作流前端。

- 首次启动 MySQL 时，`scripts/mysql-docker-init.sh` 会自动额外创建 `opencoze` 数据库（供 kechat / keinit / workflow 使用），并向 `corekg` 用户授权。
- Minio 启动后 `minio-init` 会自动创建 `corekg-bucket`（幂等，重复执行不报错）。
- Nebula 首次启动后 `nebula-activator` 会执行 `ADD HOSTS` 激活 storaged（幂等，重复不报错）；日志出现 `Nebula storaged activated.` 即完成。

### 各服务健康检查

```bash
# MySQL（本机通过映射端口 3308 访问）
mysql -h127.0.0.1 -P3308 -uroot -p123456 -e "SHOW DATABASES;"
# 应看到 corekg 与 opencoze

# Elasticsearch（注意带 Basic Auth，密码 123456）
curl -u elastic:123456 http://localhost:9202/

# Redis
redis-cli -p 6381 ping

# MinIO（Node 端口 9002；Web 控制台 9003，浏览器打开 http://localhost:9003）
# NATS

# Nebula Graph（确认 metad/storaged/graphd 均 healthy，且 storaged 已激活）
docker compose ps --filter name=corekg-nebula
docker compose logs corekg-nebula-activator   # 应看到 "Nebula storaged activated."
docker exec -it corekg-nebula-graphd /usr/local/nebula/bin/nebula-console \
  -addr 127.0.0.1 -port 9669 -u root -p nebula -e 'SHOW HOSTS;' | grep -i online
```

## 3. 各服务进程如何连接

- **容器间互连**：各 Docker 服务之间用 `服务名 + 容器内部端口`（如 `minio:9000`、`mysql:3306`、Nebula 用 `graphd:9669`）。
- **宿主机进程访问**：`make run` 启动的 Go 服务（keinit / corekg / keapi / ketask 等）跑在宿主机上，一律通过 **宿主机映射端口** 连接，因此 `config.yaml` 中连接的地址为 `localhost:3308`、`localhost:9202`、`localhost:6381`、`localhost:9002`、`nats://localhost:4225` 等。注意：**Nebula 连接不走 `config.yaml`，走 DB 的 `knowledge/nebula` settings**（见下方说明）。

### 连接字符串速查（写入各 config.yaml）

- **MySQL DSN**：`mysql://corekg:123456@localhost:3308/corekg?charset=utf8mb4&parseTime=true&loc=Local`
- **MySQL（opencoze）DSN**：`mysql://corekg:123456@localhost:3308/opencoze?charset=utf8mb4&parseTime=true&loc=Local`
- **Redis**：`addr: localhost:6381`
- **Elasticsearch**：`addresses: [http://localhost:9202]`，`username: elastic`，`password: 123456`
- **CoreKG 文件存储（Compose）**：`cos-ke.s3.end_point: http://minio:9000` 用于 CoreKG 容器内部访问，`cos-ke.s3.public_end_point: http://localhost:3001` 用于浏览器预签名 URL；宿主机直接运行 CoreKG 时，`end_point` 应改为宿主机映射地址 `http://localhost:9002`。`public_end_point` 不包含 Bucket，Bucket 由 `cos-ke.s3.bucket` 单独配置；未配置时回退到 `end_point`。
- **MinIO**：`end_point: http://localhost:9002`，`access_key_id: minioadmin`，`secret_access_key: minio123456`
  > ⚠️ **workflow 的 MinIO 连接是例外**：workflow 用 `minio-go` 客户端，其 `endpoint` 必须是**裸 host:port**（`localhost:9002`，不带 `://`），scheme 由 config 的 `storage.upload_http_scheme`（本地应填 `http`）决定。若写成 `http://localhost:9002` 会报 `Endpoint url cannot have fully qualified paths.`。凭证需与 docker-compose 的 `MINIO_ROOT_USER/MINIO_ROOT_PASSWORD`（`minioadmin` / `minio123456`）一致；`storage.bucket` 不存在时 workflow 会自动创建。
- **NATS**：`nats://localhost:4225`

### Nebula Graph 连接（图数据库）

- **连接来源**：Nebula 连接参数**不从 `config.yaml` 读取**，而是从 DB 的 `knowledge/nebula` settings（`core_settings` 表）读取，字段为 `address / port / username / password / prefix`，由 keinit 的 `core_setting.yaml` 下发。
- **本地默认值**（与 compose 一致）：`address: graphd`、`port: 9669`、`username: root`、`password: nebula`。**容器内 corekg** 用服务名 `graphd`；**宿主进程**若直接连图库，把 address 临时改为 `localhost`（映射端口 9669）即可。
- **图功能开关**：corekg 通过环境变量 `ENABLE_NEBULA_GRAPH` 决定是否初始化图库，值默认为 `true`。注意该开关在 `corekg/cmd/main.go` 是**强制语义**——置 `true` 且图库连不上时，corekg 会 `Fatal` 拒绝启动；关闭（`false`）则跳过初始化。compose 里已配为 `true`。
- **首次激活**：单节点 Nebula 首次运行后必须激活 storaged（`ADD HOSTS "storaged0":9779`），否则 `CREATE SPACE` 报 `Host not enough`。compose 的 `nebula-activator` 自动完成（幂等）。
- **space 自动创建**：知识图谱的 space 由应用按 `ke_graph_` + 20 位随机串自动创建，无需手动预建。

## 4. 初始化一次数据（keinit）

CoreKG 的数据初始化由 `keinit`(CLI / bootstrap 工具)完成,承担:MySQL 建表、SQL 种子数据、ES 索引、MinIO bucket、系统设置、聊天模型与 API Key。完整流程见上文对 `apps/keinit` 的说明。

```bash
# 准备 keinit 配置
cp apps/keinit/conf/test/config.yaml.example apps/keinit/conf/test/config.yaml
cp apps/keinit/conf/test/core_setting.yaml.example apps/keinit/conf/test/core_setting.yaml

# 编译并执行完整初始化（initDB -> chatmodel -> apikey -> migrate -> initES -> bucket）
make local APP=keinit ENV=test
./bundles/keinit -c apps/keinit/conf/test/config.yaml \
    --setting-file apps/keinit/conf/test/core_setting.yaml \
    --env-file apps/keinit/conf/test/kinit.env   # 可选:若需注入 LLM 模型/APIKEY 等变量
```

> `kechat` 等应用中的聊天模型(`chat_model`)与 `chat_agent` 种子数据依赖 `scripts/mysql` 的 SQL 脚本;其中 `@yg_*` 变量来自 `scripts/mysql/variable.sqltpl`(由 `--env-file` 或进程环境变量注入)。本地若暂时没有 LLM API Key,可先在 `variable.sqltpl` 对应 `@yg_llm_*` 填占位值并跳过模型连通性验证。

### 4.1 两种启动模式(宿主机 / docker-compose 容器)

`keinit` 的 `core_setting.yaml`(写入 `core_settings` 表)决定 corekg 运行时的**全部连接地址**,而 corekg 只读 `core_settings`(不下发 fallback 到 config.yaml)。因此**两种部署模式必须使用两套不同连接字段的 `core_setting.yaml`**:

| 部署模式 | corekg 如何运行 | 连接地址形式 | 使用的 `core_setting.yaml` |
|---|---|---|---|
| **宿主机模式**(推荐开发) | `make run APP=corekg ENV=test`(`:8080`) | 宿主机访问映射端口 | `apps/keinit/conf/test/core_setting.yaml`(`localhost:3308/6381/9202/9002`;Nebula `localhost:9669`) |
| **docker-compose 模式** | corekg 作为容器(`corekg`,compose bridge 网络 `:8080`) | compose 服务名 + 容器内端口 | `apps/keinit/conf/docker/core_setting.yaml`(`mysql:3306`/`redis:6379`/`elasticsearch:9200`/`minio:9000`;Nebula `graphd:9669`) |

两份模板(`conf/test/` 与 `conf/docker/`)除连接字段外其余内容**逐字一致**(key-set 与 `diff` 均可验证),只是在初始化**各自模式的 corekg 前**分别选用 `--setting-file` 指向对应文件:
- 在 compose 网络内初始化(fetch `mysql`/`redis` 等**服务名**)需把 `keinit` 以 **Linux 二进制跑在 compose 网络里的容器**中,或在网络内以服务名可达的环境执行;宿主机直接跑宿主版二进制即可。
- corekg 容器启动读取 `apps/corekg/conf/docker/config.yaml`(服务名 DSN),宿主进程读取 `apps/corekg/conf/test/config.yaml`(本地映射端口 DSN),两者与各自的 `core_settings` 保持一致。

#### 4.1.1 docker-compose 容器化 keinit（可选）

`docker-compose.yml` 额外提供了 `coze-static-loader` 与 `keinit` 两个一次性服务,把 keinit 也容器化到 compose 网络内,并顺带补齐 workflow(即 coze)所需的默认图标等静态资源:

- `coze-static-loader`:用 `registry.yygu.cn/library/minio-init:v0.1.2` 镜像把其内置的 `/app/scripts/minio`(`default_icon` / `official_plugin_icon`)拷入 `coze_static` 命名卷(幂等,卷非空则跳过)。
- `keinit`:本地构建(`apps/keinit/script/Dockerfile`),挂载 `coze_static` 卷到 `/app/scripts/minio`,使 `keinit` 的 `UploadCozeFile`(上传到 MinIO `opencoze` bucket)能读取到上述静态资源;并挂载 `conf/docker/` 的 `config.yaml` / `core_setting.yaml` / `kinit.env`。

```bash
# 依赖(包括 corekg 等)就绪后触发一次初始化,跑完退出:
docker compose up keinit
# 查看初始化日志:
docker compose logs keinit
```

> 这两个服务 `restart: no`,不会随 `docker compose up -d` 重复执行;手动 `docker compose up keinit` 即可。若需重跑初始化,先 `docker compose rm -sf keinit` 清理一次性容器。

> 镜像内无 `curl`,故 Nebula `metad/storaged/graphd` 的健康检查使用 `bash /dev/tcp` 探测各自 `ws_http_port` 的 `/status`(绑定在容器 IP 而非 127.0.0.1);首次启动需 `nebula-activator`(独立 `nebula-console_v3.8.0` 镜像)执行 `ADD HOSTS` 激活 `storaged0:9779`,否则 `CREATE SPACE` 报 `Host not enough`。

## 5. 初始管理员账号

初始化完成后，系统内置一个管理员账号，由 `scripts/mysql/v1.0_2__insert_account.sql` 写入：

| 字段 | 值 |
|---|---|
| 登录账号 | `admin@admin.com`（按邮箱登录） |
| 密码 | `admin123456` |
| identify | `admin` |

登录接口 `account.LoginByPassword`：当账号含 `@` 时按邮箱匹配 `user.email`，密码用 bcrypt 校验。

> 如需修改初始密码，请勿改已发布的基线 SQL（已写入 bcrypt 哈希）；应在系统运行后通过账号体系接口/后台修改。

## 5.5 一键自动化验证（知识库闭环）

`scripts/verify/verify-kb.sh` 自动跑通 **登录 → 新建知识库 → 上传文件 → 等待解析完成 → 基于文件问答** 的完整闭环，替代原来的 `verify-paths.sh`（只探路由）。模块化脚本位于 `scripts/verify/`：

| 文件 | 职责 |
|---|---|
| `verify-kb.sh` | 主入口，串起完整链路（两种模式：local / compose） |
| `lib.sh` | 公共库：HTTP/JSON 断言、计数、汇总 |
| `auth.sh` | `account.LoginByPassword` 登录取 JWT |
| `forest.sh` | `forest.CreateForest` / `ListFile` / `DeleteForest` |
| `upload.sh` | multipart `forest.UploadFile` 上传样例 |
| `wait_parse.sh` | 轮询 `forest.ListFile` 的 `knowledge_status` 直至 `success`（超时给排障提示） |
| `qa.sh` | 内部 `chat.*` 流式 RAG：建会话→提问→断言答案 + RAG 引用命中本知识库 |

### 运行前提

解析闭环**依赖 pipeline worker**（corekg 只负责写任务 `ke.prase_pdf_task → ke.knowledge_task` 到 `core_task` 表，真正的拆 chunk/向量/ES 入库由 `apps/pipeline` 的 analyser + chunker 完成）。两种模式对应两套启动方式：

**本地宿主模式**（corekg 是宿主二进制 `:8080`，pipeline 跑在宿主机 venv）：

```bash
docker compose up -d                                  # 中间件
make run APP=corekg ENV=test                          # 宿主 corekg :8080
cd apps/pipeline && python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt -r requirements_analyser.txt
python doc_worker_main.py   &                         # 消费 ke.prase_pdf_task
python chunk_worker_main.py &                         # 消费 ke.knowledge_task
cd ../..
./scripts/verify/verify-kb.sh --mode local --cleanup
```

**docker-compose 全容器模式**：

```bash
docker compose -f docker-compose.pipeline.yml up -d --build   # 中间件 + corekg + pipeline + mock-llm
./scripts/verify/verify-kb.sh --mode compose --cleanup
```

### 关键配置与依赖

- **向量化**：chunker 的向量化走 `apps/pipeline/config/chunk_config(.docker).yaml` 的 `Embedding` 节点（代码 `tools/llm_chat.py`），**不读 DB 的 `knowledge/embedding`**。默认已指向真实可达端点
  `http://embed-qwen3.003.yygu.cn:58080/v1`（模型 `Qwen3-Embedding-0.6B`）。本地无真实模型时可用
  `scripts/mock_embedding.py`（OpenAI 兼容 `/v1/embeddings`，返回确定性伪向量）。
- **task 上报主机名**：`.docker.yaml` 的 `Work.API_URl` / `analyser_config.docker.yaml` 的 `api_url`
  指 corekg 在本网络的暴露名——compose 拉起时是 `corekg`；若 corekg 是手动 `docker run` 的容器（无 `corekg`
  别名，容器名为 `corekg-app`），需改为 `http://corekg-app:8080/v3`。
- **文件类型**：解析阶段 analyser 对 `.txt/.md/.log/.csv/.json` 走 `others_process`（无需 MinerU）；
  PDF 需额外 `analyser_api_url`（MinerU）服务。

### 常见排障

1. `等待解析超时`：pipeline analyser/chunker 是否在跑且能连 corekg？核查 `core_task` 表
   `SELECT id,task_type,task_status FROM core_task WHERE subject_id=<file_id>;` 任务是否有推进。
2. `ES 删除失败: Empty value passed for parameter 'index'`：该知识库的 ES 索引未配置（`ke_forest.config_id` 为 0），
   需建好 `ke_N` 索引并让森林绑定（用 keinit / 面板创建知识库时选择索引）。
3. 引用为空：确认文件 `knowledge_status=success` 后 chunks 已进 ES 索引 `ke_N`，且 `query_reference_list` 能检索到。

## 6. 前端 / worker / pipeline 配置同步

- **前端**：Vite 开发模式使用 `frontend/corekg/.env.development.example`；Docker Compose 生产构建通过 Nginx 同源代理 API 与 MinIO，不需要填写 `VITE_API_URL`，未设置 `VITE_LOGIN_URL` 时使用浏览器当前站点。浏览器文件 URL 由 `cos-ke.s3.public_end_point` 控制，Compose 默认是 `http://localhost:3001`。
- **TS worker**：`apps/worker/.env.example`。
- **Python pipeline**：`apps/pipeline/config/*.yaml.example`。
- **workflow 应用**：其 `config.yaml`（`apps/workflow/conf/test/config.yaml`，以及聚合进 `apps/corekg/conf/test/config.yaml(.example)` 的 workflow 配置块）**不再依赖环境变量**，所有连接信息已收敛为与 `docker-compose.yml` 一致的**字面值**，直接运行即可：

  ```bash
  make run APP=workflow ENV=test          # 独立 workflow 服务
  make run APP=corekg  ENV=test           # 聚合服务（进程内拉启 workflow）
  ```

  各字段默认值如下（端口/凭据已按本方案统一偏移 +2、密码 `123456`；改动中间件时直接改这些字面值即可）：

  | 配置字段（`workflow:` 下） | 字面值 | 说明 |
  |---|---|---|
  | `main.database_conns.core` | `mysql://corekg:123456@localhost:3308/corekg` | core 库 DSN |
  | `main.database_conns.opencoze` | `mysql://corekg:123456@localhost:3308/opencoze` | opencoze 库 DSN |
  | `redis.addr` / `password` | `localhost:6381` / 空 | Redis（无密码） |
  | `elasticsearch.addr` / `username` / `password` | `http://localhost:9202` / `elastic` / `123456` | Elasticsearch |
  | `storage.minio.ak` / `sk` / `endpoint` / `region` / `api_host` | `minioadmin` / `minio123456` / `localhost:9002` / `us-east-1` / `localhost:9002` | MinIO 对象存储（`endpoint`/`api_host` 为**裸 host:port**，scheme 取自 `upload_http_scheme`） |
  | `mq.name_server` | `nats://localhost:4225` | NATS 消息总线 |

## 7. 常见问题

- **端口已被占用 / 容器启动失败**：因本方案已将宿主机端口统一 +2，绝大多数冲突已规避；若仍冲突，改 `docker-compose.yml` 端口映射并同步到对应 `config.yaml`。
- **MySQL 改了密码/清库后想重置**：删除数据卷后重新 `docker compose up -d`（首次初始化脚本会重建库与 opencoze）。
- **ES 鉴权失败**：确认使用 `-u elastic:123456`，且 `config.yaml` 中 `username/password` 为 `elastic/123456`。
- **corekg 启动失败 / 图功能报 `Host not enough`**：多为 Nebula storaged 未激活或未就绪。先确认 `nebula-activator` 日志有 `Nebula storaged activated.`；若失败，手动执行 `docker exec -it corekg-nebula-graphd /usr/local/nebula/bin/nebula-console -addr 127.0.0.1 -port 9669 -u root -p nebula -e 'ADD HOSTS "storaged0":9779'`。另确认 DB `knowledge/nebula` 的 `address` 为 `graphd`、密码为 `nebula`。
- **Nebula 端口占用**：若宿主机已有 N 台服务占用 9559/9779/9669，需改 compose 映射端口并同步改 `knowledge/nebula` settings 里的 `address`/`port`（容器内仍用服务名，改的是宿主映射）。
