# MySQL → PostgreSQL 迁移评估

> 元信息：Phase 0 / Task 12 产出。评估对象：系统库 5 个 schema（`core`、`account`、`knownow`、`chat`、`sale`）从 MySQL 到 PostgreSQL 的结构迁移与 ETL 方案。本文的代码事实均引用当前 `phase0-foundation` 分支（HEAD `1a6b913a6`）的文件路径；**本文不含任何来自生产/测试库的实测数值**（原因见第 1 节），所有行数、表数、列数、warning 数以 Phase 4 的 inventory 命令产出为准。

## 1. 盘点范围

### 1.1 范围定义

- 迁移范围为 5 个 schema：`core`、`account`、`knownow`、`chat`、`sale`。该清单与自研 planner 的默认值一致：`cmd/corekg/migrate.go:100` 中 `--schemas` 默认为 `"core,account,knownow,chat,sale"`。
- 代码侧对应的 DB 句柄入口为 `pkgs/utils/dbutil/mysql.go`（`Core()`/`Account()`/`Knownow()`/`Chat()`/`Sale()`，行 9–73）。
- **默认目标形态**：单个 PostgreSQL 数据库 + 库内 5 个 schema（MySQL 每个库 → PG 同名 schema），与应用现有"按库名取句柄"的模式（`dbtools.DB("knownow")` 等）最接近，search_path 管理集中在 PG 连接层。
- **备选形态 `mode: databases`**：每个 MySQL 库对应一个独立 PG 数据库。触发条件（满足其一即切换评估）：
  1. 需要按库独立做备份/恢复/权限边界（租户隔离要求）；
  2. 跨 schema 的 `search_path` 干扰造成无法在 Phase 1 pgdrv 连接层解决的问题（需要在 Phase 4 dry-run 验证：用 `pg-driver-spike.md` 第 4 节的 pgdrv factory 对 5 个 schema 各注册一个句柄，验证表名解析无串库）。

### 1.2 盘点（inventory）现状：被阻塞，文件不存在

**明确声明：`docs/architecture/phase0/mysql-inventory.json` 当前不存在，本文未运行也无法运行任何 MySQL 实测。** 当前环境（无 MySQL 服务、无 Docker、`CORekG_MYSQL_DSN` 未设置）无法连接任何 MySQL 实例；Task 11 的结论即为 `DONE_WITH_BLOCKER`（planner 代码完成、live dry-run 步骤被阻塞，见 `.superpowers/sdd/2026-11-01-phase0-foundation/task-11-report.md`）。因此：

- 本文**不引用、不编造**任何 schema/表/列/行数/大小数值，也不声明 jq 统计已成功执行；
- 凡结论依赖真实库数据之处，一律标注"需要在 Phase 4 dry-run 验证 + 具体查询"。

### 1.3 Phase 4 生成 inventory 的精确命令

```bash
# 构建二进制（AGENTS.md 口径：make local APP=<name>，产出 bundles/corekg）
make local APP=corekg

# 生成 inventory（只读 information_schema，不写任何数据；planner 现状为 dry-run，
# --apply 在 phase 0 被显式拒绝：cmd/corekg/migrate.go:44-46 "apply is not implemented in phase 0"）
corekg migrate from-mysql \
  --dsn "$CORekG_MYSQL_DSN" \
  --schemas core,account,knownow,chat,sale \
  --out docs/architecture/phase0/mysql-inventory.json

# 可选：用精确 COUNT(*) 替代 information_schema 估算行数（大库更慢但准确）
corekg migrate from-mysql --dsn "$CORekG_MYSQL_DSN" --exact-count \
  --schemas core,account,knownow,chat,sale \
  --out docs/architecture/phase0/mysql-inventory.json
```

产出 JSON 的结构由 `internal/migrate/plan.go:11-38` 固定：顶层 `schemas[]`，每 schema 含 `tables[]`，每表含 `row_count`、`columns[]`（`name/mysql_type/pg_type/nullable/default/extra`）、`warnings[]`。

### 1.4 生成后的 jq 统计（schema/表/列/warning 数）

```bash
# 总量：schema 数、表数、列数、warning 数
jq '{
  schemas:  (.schemas | length),
  tables:   ([.schemas[].tables[]] | length),
  columns:  ([.schemas[].tables[].columns[]] | length),
  warnings: ([.schemas[].tables[].warnings[]?] | length)
}' docs/architecture/phase0/mysql-inventory.json

# 分 schema 明细（schema / 表数 / 列数 / warning 数）
jq -r '.schemas[]
  | [.name, (.tables | length), ([.tables[].columns[]] | length), ([.tables[].warnings[]?] | length)]
  | @tsv' docs/architecture/phase0/mysql-inventory.json

# warning 全文（逐条，用于第 3/4/5/8 节的逐项核对）
jq -r '.schemas[].tables[] | .name as $t | .warnings[]? | "\($t): \(.)"' \
  docs/architecture/phase0/mysql-inventory.json
```

以上统计**本文未执行**（文件不存在），数值以 Phase 4 运行产出为准。

### 1.5 范围边界（不在本次迁移内，Phase 4 需确认）

| 项 | 位置 | 处置 |
|---|---|---|
| Coze 工作流库 | `apps/workflow`、`scripts/mysqlcoze/`、`dbutil.Coze()` | 与 `apps/workflow` 一致排除（ratchet 的 `typeTagSkipDirs` 已排除 `workflow`，`internal/arch/pgdialect_test.go:21`）；`dbutil.Coze()` 被 `apps/kechat/models/chat/coze_model_instance.go:27` 等使用，是否随系统库迁移需要在 Phase 4 dry-run 验证（查询 `SHOW DATABASES` 与 `dbutil` 名称清单比对，确认 coze 库是否有系统 model 依赖） |
| 外部租户 MySQL 数据源 | `pkgs/plugins/dbplugins/mysqlplugin/` | **必须保留**，不属于系统库迁移（见 9.2 节） |
| 其他命名库 | `dbutil` 中的 `aigc/zhy/cook/oa/llm/cluster/dryang/aiagents/opo/apigateway`（`pkgs/utils/dbutil/mysql.go:17-65`） | 逐一确认是否承载系统 model；grep 显示系统 model 主要走 `Core/Account/Knownow/Chat/Sale` 五个句柄，其余句柄的引用集中在 kechat 的 Coze 模型路径，需要在 Phase 4 dry-run 验证（具体查询：对每个库跑 planner 的 `--schemas`，非空即有表） |

## 2. 表/行数/大小

### 2.1 数据来源与方法

1. **行数（估算）**：planner 读取 `information_schema.tables.table_rows`（`internal/migrate/mysql.go:16` 的 `listTables` 查询）。注意：InnoDB 的 `table_rows` 是**估算值**（基于统计信息，误差可达 ±10–50%），只用于分批/容量初判，**不可用于验收**。建议生成 inventory 前先 `ANALYZE TABLE` 刷新统计。
2. **行数（精确）**：`--exact-count` 逐表 `SELECT COUNT(*)`（`internal/migrate/mysql.go:98-108`，标识符经 `quoteIdent` 反引号转义）。校验与第 11 节验收一律用精确值。
3. **大小**：当前 `TablePlan` 只有 `row_count`，**没有** `data_length`/`index_length`（`internal/migrate/plan.go:22-28` 字段仅 `Name/RowCount/Columns/Warnings`）。Phase 4 有两条路（推荐 a）：
   - a. 扩展 planner：在 `listTables` 查询中追加 `data_length`、`index_length`、`auto_increment` 三列并写入 `TablePlan`；
   - b. 直接在 MySQL 跑一次性 SQL 并人工归档：

```sql
SELECT table_schema, table_name, table_rows, auto_increment,
       ROUND(data_length/1024/1024, 2) AS data_mb,
       ROUND(index_length/1024/1024, 2) AS index_mb,
       ROUND((data_length + index_length)/1024/1024, 2) AS total_mb
FROM information_schema.tables
WHERE table_schema IN ('core','account','knownow','chat','sale')
  AND table_type = 'BASE TABLE'
ORDER BY (data_length + index_length) DESC;
```

### 2.2 结果模板（数值不本文填写，来源 = Phase 4 命令）

| schema | 表名 | 行数（exact） | data MB | index MB | auto_increment | 分批策略 | 备注 |
|---|---|---|---|---|---|---|---|
| （Phase 4 inventory 产出） | （同左） | （同左） | （同左） | （同左） | （同左） | （按 2.3 规则填） | （warning/特殊列） |

上表每一列的取值均以 2.1 的 Phase 4 命令产出为准；本文不预设任何数值，也不预设表清单。

### 2.3 用途（如何消费这份数据）

- **分批**：预计单表 > 1000 万行或 data+index > 2 GB 的表，ETL 必须按主键区间分批 + `COPY`（第 10 节）；阈值本身需要在 Phase 4 dry-run 验证（用实际数据量测 `COPY` 吞吐后定档）。
- **索引后建**：大表先 `DROP INDEX`/延迟建二级索引、载完再建（第 5.3 节）。
- **sequence 重置**：`auto_increment` 列用于第 4 节的 `setval` 上限对齐。
- **验收基线**：精确行数直接进入第 11 节的对账表。

## 3. 类型映射

### 3.1 完整映射表（以 `internal/migrate/plan.go:42-93` 的 `MapType` 为准）

| MySQL 类型 | PostgreSQL 类型 | 说明 / 风险 |
|---|---|---|
| `bigint unsigned` | `bigint` | PG 无 unsigned；**上限警告**：`internal/migrate/mysql.go:72-75` 对每列生成 warning（校验 `max value <= 9223372036854775807`）。Go 侧主键多为 `uint`/`int64`，若真实值可能超过 2^63-1 需要改 `numeric` 或 `text`（需要在 Phase 4 dry-run 验证 + 具体查询：`SELECT MAX(col) FROM ...` 逐列核对 warning 清单） |
| `int unsigned` / `integer unsigned` / `mediumint unsigned` | `bigint` | 同上，unsigned 上限 4294967295 在 bigint 内安全 |
| `int` / `integer` / `mediumint` / `int(11)` | `integer` | 显示宽度 `(11)` 被 `splitType` 剥离（`internal/migrate/plan.go:154-170`） |
| `bigint` | `bigint` | 原样 |
| `tinyint(1)` / `tinyint` / `tinyint unsigned` | `smallint` | **不映射为 boolean**。原因：`pkgs/types/boolean.go:10-19` 定义 `type Bool int32`，取值 `-1(False)/1(True)/2(Any)` 三态，并非真布尔；GORM tag 已由 Task 7 归一为 `type:smallint` |
| `smallint` | `smallint` | 原样 |
| `smallint unsigned` | `integer` | unsigned 上限 65535 在 integer 内安全 |
| `year` | `smallint` | 数值语义保留 |
| `longtext` / `mediumtext` / `text` / `tinytext` | `text` | PG `text` 无长度上限（MySQL text 族 64KB/16MB/4GB 限制消失，属放宽，无风险） |
| `datetime` / `datetime(3)` | `timestamptz` | 时区语义变化，见第 8.4 节 |
| `timestamp` / `timestamp(3)` | `timestamptz` | MySQL `timestamp` 本身带 UTC 转换，语义最接近 `timestamptz` |
| `date` / `time` | `date` / `time` | 原样 |
| `json` | `jsonb` | 见第 7 节（jsonb 规范化 key 顺序/空白） |
| `blob` / `longblob` / `mediumblob` / `tinyblob` / `binary` / `varbinary` | `bytea` | Go 侧 `[]byte` 直通；`binary(n)` 的定长补零语义**不保留**（PG `bytea` 变长），依赖定长语义的列需要在 Phase 4 dry-run 验证（具体查询：比对 `octet_length` 分布） |
| `double` | `double precision` | 原样 |
| `float` | `real` | PG `real` 为 4 字节单精度，与 MySQL `float` 对齐 |
| `decimal(p,s)` | `numeric(p,s)` | 裸 `decimal` → `numeric`（`internal/migrate/plan.go:80-85`） |
| `varchar(n)` / `char(n)` | `varchar(n)` / `char(n)` | 原样保留长度参数（`plan.go:86-87` 返回 `name + params`）；MySQL 行宽 65535 限制在 PG 不存在，属放宽 |
| `enum(...)` / `set(...)` | `text` | **warning**：`internal/migrate/mysql.go:77-79` 逐列生成 "mapped to text; verify allowed values"。set 的多值语义由应用层维护（本仓库 CSV 列已有 `sqlutil.ContainsCSV` 先例，见第 9 节） |
| 未知类型（如 `geometry`、`point`） | `""` + warning | `internal/migrate/mysql.go:70-71`："unmapped MySQL type"。出现即人工处理，不允许静默通过 |

### 3.2 代码侧 218 个 GORM 类型 tag 的归一结果（已完成，非计划）

Task 7 已把系统 model 的 218 处 MySQL-only GORM 类型 tag 全部改写为 PG 兼容类型，并把 ratchet 基线降为 0：

- 基线记录：`internal/arch/testdata/pg_dialect_baseline.md`（Task 7 前的基线：MySQL 类型 tag **218** 次，分布：kecore 102 / kechat 39 / keapp 21 / account 18 / kesale 16 / platform 8 / jobs 5 / task 5 / keinit 2 / connectors 1 / wecoms 1）。
- 当前断言：`internal/arch/pgdialect_test.go:16-18` 三个基线全部为 0（`FIND_IN_SET=0`、`REGEXP=0`、`typeTags=0`），`TestPGDialectRatchet` 用 `require.LessOrEqual` 保证只减不增。
- tag 级映射（`tools/pgtypes`，Task 7）：`type:bigint unsigned→type:bigint`、`type:int unsigned→type:bigint`、`type:tinyint(1)/tinyint unsigned/tinyint/TINYINT→type:smallint`、`type:longtext/mediumtext→type:text`、`type:datetime(3)/datetime→type:timestamptz`、`type:int(11)→type:integer`、`type:uint→type:bigint`，并剥离 `\s+ON UPDATE CURRENT_TIMESTAMP(\(3\))?`；`default:CURRENT_TIMESTAMP` 保留不动。
- **两侧映射一致性要求**：Phase 4 必须 diff 两份结果——(a) AutoMigrate 在 PG 实际建出的列类型（`information_schema.columns`）与 (b) planner `MapType` 的 `pg_type`——逐列比对，不一致即修 `MapType` 或 tag。具体查询：

```sql
-- PG 侧（AutoMigrate 建表后）
SELECT table_schema, table_name, column_name, data_type, udt_name, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema IN ('core','account','knownow','chat','sale')
ORDER BY table_schema, table_name, ordinal_position;
```

与 `mysql-inventory.json` 的 `columns[].pg_type` 逐列 diff（jq 导出 CSV 后 `diff`）。

### 3.3 enum/set 与未知类型的处理原则

- enum/set → `text` 后，**校验层补约束**：Phase 4 对每个 enum 列在 PG 侧评估 `CHECK (col IN (...))`（ALTER TABLE 加约束，不进 GORM tag，避免影响 MySQL 双跑期）。允许值清单从 inventory 的 `mysql_type` 原文（`enum('a','b')`）解析。
- 未知类型 → 一律阻塞迁移（planner 已保证产生 warning），按 warning 清单逐列人工决定；禁止默认 fallback。

## 4. 默认值、自增与 sequence

### 4.1 建表链路（PG 侧表从哪来）

PG 侧**不用 SQL DDL 脚本建表，而是 GORM AutoMigrate**：`keinit` 的 `initDatabase`（`apps/keinit/cmd/init.go:22-47`）→ `dbtools.DoInitModels(foresttype.InitDB, settings.InitDB, chattype.InitDB, accounttype.InitDB, wecomoms.InitDB, login_setting.InitDB)` → `dbtools.InitModel` 逐 model `db.AutoMigrate(v)`（`vendor/github.com/ygpkg/yg-go/dbtools/v2/helpers.go:12-20`）；model 清单见 `apps/kecore/models/foresttype/db.go:58-107` 等。种子数据走 `apps/keinit/cmd/mysql.go:75-86` 的 `mysql.InitMysqlData`，读取 `scripts/mysql/v1.0_1__insert_chat.sql`、`v1.0_2__insert_account.sql`、`v1.0_3__insert_setting.sql`（MySQL 方言 INSERT，含反引号）——**这些种子脚本需要 PG 化版本**（反引号去掉/改双引号、表名加 schema 前缀），属于 Phase 4 交付项。

### 4.2 自增主键与 sequence 重置（核心步骤）

1. MySQL 侧自增列在 GORM model 上是 `autoIncrement` tag（如 `apps/keapp/models/web/crawl_task.go` 的 `ID uint gorm:"...;type:bigint;primaryKey;autoIncrement"`）；PG 侧 AutoMigrate 会生成 `bigserial`/`serial`（或 identity，取决于 gorm postgres driver 版本，即 `docs/architecture/phase0/pg-driver-spike.md` 推荐的 `driver/postgres v1.6.0`）。**serial 与 identity 的具体形态需要在 Phase 4 dry-run 验证**（建表后跑下面查询）：

```sql
SELECT table_schema, table_name, column_name, column_default, is_identity
FROM information_schema.columns
WHERE table_schema IN ('core','account','knownow','chat','sale') AND column_name = 'id'
ORDER BY 1, 2;
```

2. **ETL 按原值搬运 id**（不做 id 重映射），载完后**每表重置 sequence**，否则新写入报主键冲突。基础步骤（只按表内最大 id，不含 AUTO_INCREMENT 对齐）逐表执行（schema 限定，必需）：

```sql
-- 以 core.ke_forest 为例（表名常量见 apps/kecore/models/foresttype/db.go:17）
SELECT setval(pg_get_serial_sequence('core.ke_forest', 'id'),
              COALESCE(MAX(id), 1),
              MAX(id) IS NOT NULL)
FROM core.ke_forest;
```

   - `pg_get_serial_sequence` 对 serial 与 identity 列都有效；表名参数必须 schema 限定（单库多 schema 形态下不带 schema 会按 search_path 解析，可能命中错误表）；含大写/关键字的标识符用 `'"core"."KeTable"'` 形式。
   - 第三参数 `is_called`：非空表 `MAX(id) IS NOT NULL` → `true`，下一次插入取 `MAX(id)+1`；空表 → `setval(seq, 1, false)`，`false` 表示该值尚未被用过，下一次插入取 1（用 `true` 会跳过 1 直接从 2 开始）。
   - **任何情况下都不得用 0 调用 `setval`**：PG sequence 默认 `MINVALUE 1`，`setval(seq, 0, ...)` 直接报 `setval: value 0 is out of bounds for sequence`。下面所有写法都靠 `COALESCE(..., 1)` / `GREATEST(..., 1)` 兜住这一条。
3. **对齐 MySQL 的 AUTO_INCREMENT 计数器**：MySQL 删行后计数器可能已超过 `MAX(id)`，只按 MAX 重置会产生重复 id。ETL 前记录 MySQL 侧 `information_schema.tables.auto_increment`（2.1 节 SQL 已含该列；语义是"下一个待用值"，故已用最大值为 `auto_increment - 1`）。

   **守卫条件：仅当 `auto_increment - 1 > MAX(id)` 时才执行本步**；否则步骤 2 的结果已是正确上界，直接跳过（不重复调用 `setval`）。`auto_increment` 为 NULL（表从未插入）时同样跳过。按表是否为空分两种安全写法：

```sql
-- (a) 非空表：推进到 MAX(id) 与 auto_increment-1 的较大者（守卫条件已保证结果 > 0）
SELECT setval(pg_get_serial_sequence('core.ke_forest', 'id'),
              GREATEST(MAX(id), <mysql_auto_increment> - 1),
              true)
FROM core.ke_forest;

-- (b) 空表：没有 MAX(id) 可用，直接把 auto_increment 作为"下一个待用值"
--     GREATEST(...,1) 防止 auto_increment 为 0/NULL 时以 0 调用 setval
SELECT setval(pg_get_serial_sequence('core.ke_forest', 'id'),
              GREATEST(<mysql_auto_increment>, 1),
              false);
```

   - (a)/(b) 的选择由守卫查询决定：`SELECT COUNT(*) FROM core.ke_forest;` 为 0 走 (b)，否则走 (a)。ETL 工具应把"守卫条件 + (a)/(b) 分支"实现为代码路径，不手工挑选。
4. PG ≥ 10 的 identity 列等价替代：`ALTER TABLE core.ke_forest ALTER COLUMN id RESTART WITH <n>;`（仅 identity 列适用；`<n>` 同样受 0 值约束，与 `setval` 二选一，以 4.2.1 的 dry-run 结果决定）。

### 4.3 CURRENT_TIMESTAMP 默认值（保留）

Task 7 改写**保留**了 `default:CURRENT_TIMESTAMP(3)`（唯一带毫秒的例子：`apps/keapp/models/web/crawl_task.go:28-29`，现为 `type:timestamptz;not null;default:CURRENT_TIMESTAMP(3)`）。PG 完全接受 `CURRENT_TIMESTAMP(3)`（精度参数合法），默认值无需转换。需要在 Phase 4 dry-run 验证建表后的 `column_default` 与 MySQL 一致（4.2.1 的查询已含 `column_default`）。

### 4.4 `ON UPDATE CURRENT_TIMESTAMP`（已删除，语义迁移到应用层）

- MySQL 的 `ON UPDATE CURRENT_TIMESTAMP` 在 PG 没有等价物。Task 7 的 codemod 已把唯一一处（`crawl_task.go` 的 `updated_at`）从 tag 中剥离。
- **补偿路径**：GORM 对名为 `UpdatedAt` 的 `time.Time` 字段在 Create/Update 时自动维护时间戳（GORM 约定字段），model 层已天然覆盖 ORM 路径；**但原生 SQL（`Exec`/批量 `UPDATE ... WHERE`）不会触发**。Phase 4 必须逐一排查原生 UPDATE 语句并显式 `SET updated_at = now()`，验证方式（golden path 场景）：

```sql
-- 应用跑一轮更新后，比对 updated_at 是否前进
SELECT id, updated_at FROM core.ke_forest WHERE id = <id>;
```

不前进的原生 SQL 站点清单需要在 Phase 4 dry-run 验证（grep 口径：`grep -rn "UPDATE " --include="*.go" apps pkgs | grep -v workflow`，逐个核对是否走 ORM）。

## 5. 索引、唯一约束与外键

### 5.1 索引盘点来源（MySQL 侧）

AutoMigrate 建 MySQL 表时同样创建 GORM tag 声明的索引（代码侧粗略计数：`uniqueIndex/unique_index` tag 28 行、`index` 类 tag 152 行，grep 口径，非精确 tag 数）。**线上实际索引以 MySQL information_schema 为准**（可能存在手工加的索引），Phase 4 用以下查询盘点：

```sql
-- 全量索引（含列序、唯一性、前缀长度 sub_part、函数索引表达式 expression）
SELECT table_schema, table_name, index_name, non_unique, seq_in_index, column_name, sub_part, expression
FROM information_schema.statistics
WHERE table_schema IN ('core','account','knownow','chat','sale')
ORDER BY table_schema, table_name, index_name, seq_in_index;

-- 或逐表 DDL（注意：SHOW CREATE TABLE 在仓库内是外部数据源专用方言，见 9.2 节；
-- 这里仅作为 DBA 盘点手段，不进代码）
SHOW CREATE TABLE core.ke_forest;
```

### 5.2 索引类别对照与处理

| MySQL 索引形态 | PG 处理 | 动作 |
|---|---|---|
| PRIMARY KEY | PRIMARY KEY | AutoMigrate 直接生成，无需处理 |
| 普通/复合唯一索引（`uniqueIndex` tag） | UNIQUE 索引 | AutoMigrate 生成；NULL 语义见 5.4 |
| 普通二级/复合索引 | btree 索引 | AutoMigrate 生成 |
| **前缀索引** `KEY (col(10))`（`sub_part` 非空） | PG 无前缀索引 | 二选一：a) 整列索引（存储变大）；b) 表达式索引 `CREATE INDEX ... ON t (left(col, 10))`（保持选择性）。前缀索引清单需要在 Phase 4 dry-run 验证（5.1 查询过滤 `sub_part IS NOT NULL`），逐个决定 |
| 函数/表达式索引（MySQL ≥ 8.0.13，`expression` 非空） | PG 表达式索引（原生支持） | 语法改写（MySQL 双括号 `((col))` → PG `(col)`），逐个翻译 |
| **部分索引**（MySQL 不支持，仓库不会有） | PG `WHERE ...` 部分索引 | 无来源，无需处理；若 Phase 4 想为软删除列加部分索引（`WHERE deleted_at IS NULL`），属 PG 侧优化项，须与 AutoMigrate 流程协调（AutoMigrate 不管理手写索引，重建时不会被删） |
| 全文索引 FULLTEXT | PG 无内建 FULLTEXT（tsvector/pg_trgm 需扩展） | 本仓库全文检索走 ES（`ik_max_word` 分词、索引按公司/森林命名 `ke_0`、`ke_1`，见 AGENTS.md），**预期系统库无 FULLTEXT 索引**；是否为零需要在 Phase 4 dry-run 验证（5.1 查询过滤 `index_type = 'FULLTEXT'`） |

### 5.3 AutoMigrate 建索引 vs 载数后建索引

- **默认（小/中表）**：直接让 AutoMigrate 建好全部索引再 ETL——保证 PG schema 与应用模型一致，链路最简单。
- **大表**：载数前删除/延迟二级索引，载完再建，避免逐行维护索引。实现路径：AutoMigrate 建表（含索引）→ `DROP INDEX` 记录在 checkpoint → `COPY` → `CREATE INDEX`（或 `CREATE INDEX CONCURRENTLY`，避免锁表，但不能在事务内）。**哪些表走此路径**由 2.2 节的数据量在 Phase 4 决定（阈值见 2.3）；DROP/CREATE 语句由 ETL 工具从 5.1 的盘点结果自动生成，不手写。
- 一致性兜底：载数完成后用 PG 侧查询比对索引集合与 MySQL 盘点一致：

```sql
SELECT schemaname, tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname IN ('core','account','knownow','chat','sale')
ORDER BY schemaname, tablename, indexname;
```

### 5.4 唯一约束的 NULL 语义（逐索引确认）

- 语义基线：MySQL 唯一索引对 `NULL` 不判重（允许多行 NULL）；PG 默认（`NULLS DISTINCT`，PG 15 之前的唯一行为）**同样**允许多行 NULL。因此"多 NULL"行为两侧通常一致。
- 差异点与确认动作：
  1. **PG 15 `NULLS NOT DISTINCT`**：若 Phase 4 在唯一索引上显式用了 `NULLS NOT DISTINCT`（或不小心用错默认），NULL 判重行为与 MySQL 相反——不要默认使用；确需使用（例如业务上 NULL 应唯一）必须单列决策。
  2. **复合唯一索引**：MySQL 复合唯一中含 NULL 的行不判重；PG 同样按 `NULLS DISTINCT` 处理（任一列为 NULL 即不冲突）。语义一致，但**ETL 时可能撞上历史脏数据**：MySQL 里 `('a', NULL)` 重复行载入 PG 也会被拒吗？——不会（两行都含 NULL，不判重）。真正的风险是**大小写**：MySQL `utf8mb4_*_ci` 下 `'Foo'` 与 `'foo'` 冲突，PG 默认排序规则下不冲突（见第 6 节）；反之 MySQL 里已存在的 `Foo`/`foo` 两行在 ci collation 下不可能共存，所以 PG 侧不会新增冲突，但 ci 下"看似唯一"的列在 PG 变为可重复插入。逐唯一索引确认查询（PG 侧，载数后）：

```sql
-- 唯一索引涉及的列中哪些可空 + 是否 NULLS NOT DISTINCT + 完整索引定义
SELECT n.nspname AS schema, t.relname AS table, i.relname AS index_name,
       k.ord AS key_part, k.attnum, a.attname AS column, a.attnotnull,
       x.indnullsnotdistinct, pg_get_indexdef(x.indexrelid) AS indexdef
FROM pg_index x
JOIN pg_class i ON i.oid = x.indexrelid
JOIN pg_class t ON t.oid = x.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
CROSS JOIN LATERAL unnest(x.indkey) WITH ORDINALITY AS k(attnum, ord)
LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE n.nspname IN ('core','account','knownow','chat','sale') AND x.indisunique
ORDER BY 1, 2, 3, 4;
```

- **表达式索引的键位会被 inner join 丢掉**：`pg_index.indkey` 对表达式键位（如 `lower(name)`、`left(col,10)`）填 `0`，而 `pg_attribute` 里没有 `attnum = 0` 的行，因此上面的 `pg_attribute` 关联**必须用 `LEFT JOIN`**；用 inner join 时这些键位整行消失，审计会误判为"该唯一索引只覆盖普通列"。表达式键位以 `attnum = 0`、`column` 为 NULL 呈现，其判重/可空语义改由 `pg_get_indexdef(indexrelid)` 的完整定义人工判断。
- `indnullsnotdistinct` 仅 PG 15+ 存在，低版本查询需去掉该列。

### 5.5 外键（FK）

- 代码事实：系统 model 的 GORM tag **没有** `constraint:OnDelete`/`references`（grep 计数为 0），`scripts/mysql/*.sql` 也没有 `FOREIGN KEY`/`REFERENCES`——表结构全部由 AutoMigrate 生成，引用完整性在应用层维护。预期线上没有系统库 FK，**必须在 Phase 4 dry-run 验证**（排除手工加约束）：

```sql
SELECT tc.table_schema, tc.table_name, tc.constraint_name, kcu.column_name,
       ccu.table_name AS referenced_table
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
LEFT JOIN information_schema.constraint_column_usage ccu
  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
WHERE tc.constraint_type = 'FOREIGN KEY'
  AND tc.table_schema IN ('core','account','knownow','chat','sale');
```

- 处理：若盘点为空 → PG 侧同样不加 FK（保持两侧一致，避免 AutoMigrate 载数顺序问题）；若发现存量 FK → PG 侧在建表后、**载数全部完成后**再 `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` + `VALIDATE CONSTRAINT`（NOT VALID 避免载数中途被引用检查卡死）。

## 6. 排序规则与大小写

### 6.1 语义差异（核心风险）

- MySQL：`utf8mb4_general_ci` / `utf8mb4_0900_ai_ci` 等 `_ci` 排序规则下，`=`、`LIKE`、`ORDER BY`、唯一索引**均不区分大小写**。
- PG：默认排序规则（`C`/`en_US.UTF-8`，deterministic）下，`=`、`LIKE`、唯一索引、`ORDER BY` **区分大小写**。这不是 bug 而是语义变化，影响所有 `WHERE name = ?` 类等值查询与唯一性。
- 已完成的代码适配：`REGEXP` 查询已改为大小写不敏感的 `~*`（`sqlutil.RegexpInsensitive`，`pkgs/utils/sqlutil/pg.go:11-12`），覆盖 `apps/kesearch/models/globalsearch/agent.go:120` 与 `forest.go:78`。

### 6.2 查询层适配（首选）

按查询形态替换：

| MySQL 行为 | PG 写法 | 适用 |
|---|---|---|
| `col = 'x'`（ci 匹配） | `lower(col) = lower('x')`（配 `lower(col)` 函数索引） | 等值查询 |
| `col LIKE 'x%'`（ci 匹配） | `col ILIKE 'x%'` | 前缀/模式匹配 |
| `col REGEXP 'x'`（ci 匹配） | `col ~* 'x'`（已有封装） | 正则 |
| 大小写不敏感排序 | `ORDER BY lower(col)` | 列表排序 |

需要在 Phase 4 dry-run 验证的站点：等值/排序类查询没有统一封装，必须逐个 grep（`apps`+`pkgs` 的 `.Where("` 与 `.Order("` 调用）核对哪些列在 MySQL 语义下依赖 ci；每处给出具体改写。

### 6.3 列级 collation 方案（备选，慎用）

- ICU nondeterministic collation：`CREATE COLLATION ci (provider = icu, locale = 'und-u-ks-level2', deterministic = false);` 后 `col varchar(255) COLLATE ci`，可让 `=`/唯一索引恢复 ci 语义。
- **限制**：nondeterministic collation **不支持 `LIKE`/`ILIKE` 等模式匹配操作符**，且要求 PG ≥ 12；与本仓库大量 LIKE/正则查询冲突。仅当某列的 ci 等值语义无法在查询层改写时才对该列使用；是否需要需要在 Phase 4 dry-run 验证（对候选列跑 6.4 的差异计数，非零才考虑）。

### 6.4 体检查询（迁移前后都要跑）

```sql
-- MySQL：大小写折叠后重复的"唯一列"（迁移到 PG 后 ci 唯一性消失，可再插入重复值）
SELECT company_id, LOWER(name), COUNT(*) AS cnt
FROM core.ke_forest GROUP BY company_id, LOWER(name) HAVING COUNT(*) > 1;
-- （前提：该列上确实存在 ci 排序规则下的唯一索引——是否如此以 5.1 节的 statistics 盘点为准。
--  若存在这类唯一索引，MySQL 侧此查询应为空，PG 侧同查询非空即说明迁入数据被折叠出重复；
--  若该列没有唯一索引，MySQL 侧也可能非空，此时两侧结果应逐组相等，而不是都为空。）

-- PG：载数后同样查询，与 MySQL 侧结果 diff（有 ci 唯一索引时 diff 应为空）
SELECT company_id, lower(name), COUNT(*) AS cnt
FROM core.ke_forest GROUP BY company_id, lower(name) HAVING COUNT(*) > 1;

-- MySQL：值中含大写字母的行数 = 依赖 ci 检索的规模（评估召回变化面）
-- 注意必须强制二进制比较：在 ci 排序规则下 name <> LOWER(name) 恒为假（比较本身不区分大小写）
SELECT COUNT(*) FROM core.ke_forest WHERE name <> LOWER(name) COLLATE utf8mb4_bin;
```

ES 召回不受 PG collation 影响（ES 用自己的 `ik_max_word` 分词器），但 DB 直查路径（如 `globalsearch` 走 `~*`）需要做迁移前后召回对比（第 11.5 节）。

## 7. JSON / 文本 / 二进制

### 7.1 三种 JSON 存储形态（本仓库的实际分类）

| 形态 | 例子（代码事实） | MySQL 列类型 | PG 目标类型 | 迁移动作 |
|---|---|---|---|---|
| MySQL `json` 列 | inventory 中 `mysql_type = json` 的列 | `json` | **`jsonb`**（`internal/migrate/plan.go:72-73`） | ETL 直拷；见 7.2 规范化 |
| GORM `serializer:json` + `type:text` | `apps/kecore/models/foresttype/forestqa.go:79-80`（`query_reference_list`/`chat_reference_list`）、`forest_db.go:15`（`db_meta`）、`forest.go:194`（`file_config`）、`ke_package.go:65`（`extra`） | `text` | **`text`**（保持） | **保持 `serializer:json` + `text`，不改 jsonb**：序列化由 GORM 完成，值以字符串形态存储，不受 jsonb 规范化影响 |
| GORM `type:json` tag（4 处） | `apps/keapp/models/web/resource.go:23,29`（`manifest`/`metadata`，Go 类型 `json.RawMessage`）、`apps/keapp/models/apptype/application.go:24,26`（`config`/`crawl_config`） | `json` | **当前 AutoMigrate 会建成 PG `json`**，与 planner 的 `jsonb` **不一致** | Phase 4 必须对齐：把 4 处 tag 改为 `type:jsonb`（推荐，语义与 planner 一致），或把 `MapType("json")` 改回 `json`（不推荐，丢失 jsonb 索引能力）。对齐后在 PG `information_schema.columns` 复查 `data_type` |

### 7.2 JSON 规范化与校验差异

- **前提（易误解处）**：MySQL 原生 `json` 列**不保存原始文本**。服务端在写入时解析并存为优化的二进制格式，key 已排序、无意义空白已去除、重复 key 已折叠（保留最后一个）。所以"MySQL 保留原文、PG jsonb 才规范化"的对比不成立：**两侧都是规范化存储，只是规范化规则不完全相同**（数字字面量的存储与回显、转义与非 ASCII 处理、key 排序的平局规则等存在版本相关差异）。
- **结论不变：JSON 列不得进入 SQL checksum。** 语义等价的两个值，其 `col` 文本输出在两侧仍可能字节不等，第 11 节的 SQL 摘要会因此产生假阳性。统一做法是在 Go 校验工具中"解析后重序列化"：两侧取值 `json.Unmarshal` 到同一类型（`map[string]any` 或强类型），再用**同一份** `json.Marshal` 代码序列化后比对——同一份 Go 代码保证规范化规则一致。
  - PG 侧 `col::text` 即 jsonb 规范化文本；MySQL 侧没有与之对齐的规范化输出函数（`JSON_UNQUOTE(JSON_EXTRACT(col, '$'))` 返回的是 MySQL 自己的规范化文本，不保证与 PG 一致），因此不要在 SQL 层强行对齐，直接在 Go 层对齐。
- **真正保留原文的是 text 列**：7.1 节第二类（`serializer:json` + `type:text`）在 MySQL 侧就是原文存储（含 key 顺序、空白、可能的重复 key），迁到 PG `text` 后仍是原文，**不做任何规范化**。这类列可以按普通 text 列进 checksum 做字节级比对；前提是它们保持 `text`、不被改成 jsonb（7.1 节的 `type:json` 对齐项只针对真 json 列）。
- **重复 key 的正确表述**：MySQL `json` 列中不可能存在重复 key（写入时已折叠），PG jsonb 同样折叠，因此迁移**不改变**重复 key 数量。jsonb 的 `jsonb_typeof`/key 计数抽检用于发现**类型漂移**（如 MySQL 侧存的是标量/数组、PG 侧被写成对象），不是用于发现重复 key。
- Go 侧读写：`json.RawMessage` 字段（`resource.go:23`）写入 jsonb 列时 pgx 可直接传 `[]byte`；`serializer:json` 字段是 text 列不受影响。兼容性需要在 Phase 4 dry-run 验证（golden path 跑一遍 resource 的写读）。

### 7.3 文本族

- `longtext/mediumtext/text/tinytext → text`（3.1 节）；MySQL text 族长度上限在 PG 消失，无风险。
- `varchar(n)`/`char(n)` 保留长度；PG `char(n)` 的尾部空格填充语义与 MySQL `char` 一致（blanks padded），无需处理。
- 排序/比较差异全部归入第 6 节处理。

### 7.4 二进制族

- `blob` 族 / `binary` / `varbinary` → `bytea`；Go `[]byte` 直通，`COPY` 二进制模式无转义问题。
- **`binary(n)` 定长补零语义丢失**（PG bytea 变长、不补 `\0`）。依赖补零的列（如 `binary(16)` UUID 存储）在 MySQL 里读出即带补零、PG 不带，长度变化需要用 `octet_length` 分布比对。是否受影响需要在 Phase 4 dry-run 验证（inventory 过滤 `binary(` 的列，逐列 `SELECT octet_length(col), COUNT(*) GROUP BY 1` 两侧比对）。
- 长度校验：`octet_length(col)`（PG）与 `LENGTH(col)`/`OCTET_LENGTH(col)`（MySQL）逐表 sum 比对。

## 8. 空串、NULL、零日期

### 8.1 零日期（`0000-00-00 00:00:00`）

- PG 拒绝零日期（插入直接报错）。MySQL 允许取决于 `sql_mode`（非 STRICT 模式允许）。
- **迁移规则：零日期 → `NULL`**（列为 NOT NULL 时改为 `'1970-01-01 00:00:00'`+warning 人工决策，不允许静默）。
- 盘点与计数（Phase 4 执行）：

```sql
-- 1) MySQL 当前 sql_mode（决定零日期是否可能存在）
SELECT @@global.sql_mode, @@session.sql_mode;

-- 2) 零日期列清单 + 每列计数（datetime/timestamp/date 列逐列跑）
SELECT table_schema, table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema IN ('core','account','knownow','chat','sale')
  AND data_type IN ('datetime','timestamp','date');

-- 3) 每列零值/早于 epoch 的行数（<col> 替换为列名）
SELECT COUNT(*) FROM <schema>.<table>
WHERE <col> = '0000-00-00 00:00:00' OR <col> < '1970-01-01';
```

### 8.2 空串 vs NULL（逐列确认）

- MySQL 与 PG 的 `''` 与 `NULL` 语义本身一致（都区分），**但应用层常把 `''` 当"未设置"**，且 MySQL ci collation 下 `'' = ''` 两侧行为相同——真正的风险是**迁移期间做归一化决策时把语义改掉**。
- 规则：ETL **原样搬运**（`''` 保持 `''`，NULL 保持 NULL），不做任何 `'' ↔ NULL` 转换；归一化只作为 Phase 4 的显式决策项（每列单独批准 + 应用代码同步改）。
- 盘点（找出"空串当 NULL 用"的候选列，供决策）：

```sql
-- MySQL：每个 text/varchar 列的空串/NULL 计数（Phase 4 生成全列清单后逐列跑）
SELECT COUNT(*) AS total,
       SUM(col = '') AS empty_str,
       SUM(col IS NULL) AS nulls
FROM <schema>.<table>;
```

（候选信号：`empty_str` 与 `nulls` 同时 > 0，且业务语义上"空"应唯一。）

### 8.3 NOT NULL 与默认值一致性

- 列的 `is_nullable`/`column_default` 由 planner 原样记录（`internal/migrate/mysql.go:46` 查询取 `column_default`/`extra`）。ETL 后比对两侧 `information_schema.columns` 的 `is_nullable` 与默认值表达式（4.2.1 / 3.2 的 PG 查询），不一致清单逐列处理。
- MySQL 隐式默认（非 STRICT 下 `''`/`0` 填充）在 PG 不存在；若 8.1 查询显示 `sql_mode` 非 STRICT，需对 NOT NULL 无默认列做专项检查（是否有历史隐式默认数据）。

### 8.4 时区（datetime → timestamptz 的 8 小时风险）

- MySQL `datetime` 是"无时区挂钟时间"，实际值按服务器/会话时区写入；PG `timestamptz` 存绝对时刻。ETL 解析时**必须显式按源时区打标签**。
- **源时区是待实测项，不是已知事实**：`+08:00`（`Asia/Shanghai`）只是基于部署环境的**假设**，本文后续 SQL 中出现的 `Asia/Shanghai` 一律按此假设书写。假设错了 = 整库时间偏移 8 小时，因此 Phase 4 必须先用下面的查询确认真实时区，再把它填入 ETL 配置与本文 SQL：
  1. 确认源时区（Phase 4 执行）：`SELECT @@global.time_zone, @@session.time_zone;`（返回 `SYSTEM` 时再查 `SHOW VARIABLES LIKE 'system_time_zone';`，并核对 OS 时区与 `mysql_tzinfo_to_sql` 是否已加载时区表）。
  2. go-sql-driver 读取侧 DSN 必须 `parseTime=true&loc=<步骤 1 的实测时区，URL 编码>`（假设成立时为 `loc=Asia%2FShanghai`），得到正确的 `time.Time`；写入 PG 时值即带正确偏移。
  3. **禁止**用 `parseTime=true` 不带 `loc`（默认 UTC，会把挂钟时间当 UTC，产生 8 小时偏移）。
  4. PG 侧渲染口径同理：`to_char(col AT TIME ZONE '<实测时区>', ...)`；ETL 与校验（11.2/11.4）必须用同一个时区名，且该名字来自步骤 1 的实测结果。
- 校验：抽样行两侧比对（MySQL `DATE_FORMAT(col, '%Y-%m-%d %H:%i:%s.%f')` vs PG `to_char(col AT TIME ZONE '<实测时区>', 'YYYY-MM-DD HH24:MI:SS.US')`，见 11.4）。

## 9. 方言替换清单

### 9.1 已替换（Phase 0 完成，ratchet 基线为 0）

| MySQL 方言 | 出现次数（基线） | 替换为 | 代码位置（现状） |
|---|---|---|---|
| `FIND_IN_SET(?, col)` | 3 行（基线，`internal/arch/testdata/pg_dialect_baseline.md:14`） | `?::text = ANY(string_to_array(col, ','))`，封装 `sqlutil.ContainsCSV(column)` | 封装：`pkgs/utils/sqlutil/pg.go:6-8`；调用：`apps/kecore/models/forest/forest.go:947`（`f.manager_ids`）、`apps/kesearch/models/globalsearch/forest.go:88,90`（`forest.manager_ids`） |
| `col REGEXP ?` | 2 行（基线，同上 :15） | `col ~* ?`（大小写不敏感，补 MySQL ci 语义），封装 `sqlutil.RegexpInsensitive(column)` | 封装：`pkgs/utils/sqlutil/pg.go:11-12`；调用：`apps/kesearch/models/globalsearch/agent.go:120`（`a.show_name OR v.description`）、`forest.go:78`（`forest.name OR forest.description`） |
| MySQL GORM 类型 tag | 218 处（基线，同上 :16） | PG 兼容 tag（见 3.2 的 12 条映射） | 57 个 model 文件已改写（Task 7，`tools/pgtypes` codemod）；ratchet 现基线 0：`internal/arch/pgdialect_test.go:16-18` |

守门：`TestPGDialectRatchet`（`internal/arch/pgdialect_test.go:100-112`）断言上述三项计数 ≤ 0，新增 MySQL-only 写法会被 CI/本地测试拒绝。

### 9.2 保留（外部数据源功能，不属于系统库迁移，禁止"顺手清理"）

| 保留项 | 位置 | 保留理由 |
|---|---|---|
| `IFNULL`（3 处） | `pkgs/plugins/dbplugins/mysqlplugin/mysql.go:145-147`（`IFNULL(DATA_LENGTH+INDEX_LENGTH,0)` 等） | 查询目标是**用户自有的外部 MySQL 数据源**（租户库容量统计），永远连 MySQL |
| `SHOW CREATE TABLE`（4 处） | `apps/kecore/models/keqa/qa_excel.go`、`apps/kechat/models/qachat/excel_chat.go`、`apps/kechat/models/qachat/mysql_chat.go`、`apps/kechat/models/excelchat/qa_excel.go`（各 1 处） | Excel/MySQL 问答功能读取租户自有 MySQL 库的表结构 |
| `mysqlplugin` 插件整体 | `pkgs/plugins/dbplugins/mysqlplugin/`（ratchet `typeTagSkipDirs` 排除项，`internal/arch/pgdialect_test.go:21`） | 外部租户 MySQL 连接插件，系统库切 PG 后仍需连 MySQL |

（清单与理由的权威记录：`internal/arch/testdata/pg_dialect_baseline.md` 第 66-77 节"必须保留"。）这些保留项**不进入**迁移替换范围；评估结论：无遗漏的方言清理项，MySQL-only SQL 在系统库路径上已清零（ratchet 保证）。

## 10. 迁移工具决策

### 10.1 决策：自研 `corekg migrate from-mysql` 为主，pgloader 仅对照/兜底

**主路径：自研工具。** 现状：`corekg migrate from-mysql` 已实现只读 planner/dry-run（`cmd/corekg/migrate.go`；`--apply` 在 phase 0 显式报错"apply is not implemented in phase 0"，行 44-46），产出带类型映射与 warning 的 JSON 计划（`internal/migrate/`）。

选择自研的硬理由（每条都对应本仓库的具体约束）：

1. **5 schema → 1 库多 schema 的映射**：目标形态是单 PG 库 + `core/account/knownow/chat/sale` 五 schema（1.1 节），需要把"MySQL 库 → PG schema"的映射、search_path、schema 限定序列名（4.2 节）全部纳管；外部工具对此默认行为不同且不可编程。
2. **类型钩子必须复用 `MapType`**：`tinyint(1) → smallint`（因 `types.Bool` 三态，3.1 节）、`json → jsonb`、`bigint unsigned` 溢出 warning 等规则是仓库自定义的，planner（`internal/migrate/plan.go:42-93`）已是唯一权威实现；用外部工具等于维护第二套映射，必然漂移。
3. **sequence 重置**：ETL 后逐表 `setval`（并与 MySQL `auto_increment` 对齐，4.2 节）必须与装载批次联动，需要工具内聚。
4. **断点（checkpoint）**：大表分批 + 可恢复重放（12.3 节回滚依赖）需要每表/每批状态持久化，外部工具无此概念。
5. **校验内嵌**：行数/PK min-max/checksum/抽样对账（第 11 节）与装载同进程，mismatch 直接停止并给出定位分桶。
6. **PG 侧索引/排序规则修正**：5.2/5.3 节的"载数后建索引"、6 节的 collation 决策需要按表定制 DDL 顺序，工具化可控。

**pgloader 的角色：对照与兜底。** 用一次独立 pgloader 全量导入到临时 schema，对自研 ETL 的行数做三方对账（MySQL / 自研 PG / pgloader PG）；仅当自研工具在 Phase 4 出现无法按期修复的缺陷时，才考虑用 pgloader 完成一次性装载（随后仍需手工执行 `setval` 与索引修正）。pgloader 不作主路径的具体限制：默认映射规则与本仓库规则冲突（如 `tinyint(1)` 默认映射 boolean，而本仓库要求 `smallint`）、类型映射不可编程、5-schema 目标映射与断点/校验需求不匹配；其具体行为需要在 Phase 4 以小库实测验证后再定对照口径。

### 10.2 Phase 4 需要给自研工具补齐的能力（当前 planner → ETL 的差距）

| 能力 | 现状 | Phase 4 交付 |
|---|---|---|
| apply 模式 | 显式拒绝（`cmd/corekg/migrate.go:44-46`） | 新增 `--apply`：按计划装载（建表仍走 AutoMigrate，工具不建表） |
| 批量装载 | 无 | 按主键区间分批 + PG `COPY`（二进制协议）；批大小由 2.3 节数据量定档 |
| 断点 | 无 | checkpoint 文件（表名 + 已完成最大 id + 批次号），重跑跳过已完成批次 |
| sequence 重置 | 无 | ETL 完成钩子执行 4.2 节 `setval`（含 auto_increment 对齐） |
| 校验 | 无 | 第 11 节五项校验内嵌，mismatch 即失败退出 |
| 索引时序 | 无 | 大表按 5.3 节延迟建索引（DROP/CREATE 语句从盘点自动生成） |
| 估算→精确 | `--exact-count` 已有 | 行数复用；data/index 大小按 2.1 节 a 方案扩展进 plan JSON |

## 11. 校验方案

校验在 ETL 完成后、切换流量前执行，全部通过才允许 cutover。以下 SQL 中以 `core.ke_forest` 为例（表名常量 `TableNameKnownowForest`，`apps/kecore/models/foresttype/db.go:17`），实际对 1.4 节 inventory 的每张表执行。

### 11.1 行数与主键 min/max（每表，硬门槛）

```sql
-- MySQL
SELECT COUNT(*), COALESCE(MIN(id), 0), COALESCE(MAX(id), 0) FROM core.ke_forest;
-- PG（schema 限定）
SELECT COUNT(*), COALESCE(MIN(id), 0), COALESCE(MAX(id), 0) FROM core.ke_forest;
```

**验收阈值：三项必须完全相等（差异 = 0）。** 不满足即 fail，进入 11.3 的差异定位。

### 11.2 关键字段 checksum（每表，硬门槛）

两侧用同一摘要做聚合 XOR，行序无关。**摘要宽度固定为 MD5 前 8 个 hex 字符（= 32 bit）**：MySQL 侧 `CAST(... AS UNSIGNED)` 配合 `BIT_XOR` 得到无符号值，PG 侧 `bit(32)::bigint` 得到有符号 bigint 内的正数，两侧取值域都是 `0..2^32-1`，异或结果同样落在该区间，**必然可直接比较**。若取 16 个 hex 字符再 `::bit(32)` 会发生静默截断（bit 串长度超出 `bit(32)` 声明宽度），得到与 MySQL `CONV(...,16,10)` 不同的数值；若两侧都按 64 bit 处理，则 MySQL 无符号与 PG 有符号在高位（≥ 2^63）会符号错位。因此本仓库校验统一用 8 hex / 32 bit。

```sql
-- MySQL
SELECT BIT_XOR(CAST(CONV(SUBSTR(MD5(CONCAT_WS('|',
  COALESCE(id, ''), COALESCE(company_id, ''), COALESCE(uin, ''), COALESCE(name, ''),
  COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s.%f'), ''),
  COALESCE(DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s.%f'), ''),
  COALESCE(DATE_FORMAT(deleted_at, '%Y-%m-%d %H:%i:%s.%f'), '')
  /* …关键字段按表扩展；JSON 列除外，见 7.2 */), 1, 8), 16, 10) AS UNSIGNED))
FROM core.ke_forest;

-- PG（≥ 14，bit_xor 聚合需 14+）
SELECT bit_xor(('x' || substr(md5(concat_ws('|',
  COALESCE(id::text, ''), COALESCE(company_id::text, ''), COALESCE(uin::text, ''), COALESCE(name, ''),
  COALESCE(to_char(created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS.US'), ''),
  COALESCE(to_char(updated_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS.US'), ''),
  COALESCE(to_char(deleted_at   AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS.US'), '')
  )), 1, 8))::bit(32)::bigint)
FROM core.ke_forest;
```

要点：
- **每个参与哈希的列都包一层 `COALESCE(...,'')`**：两侧 `CONCAT_WS` 都是**跳过** NULL 参数而不是产出空位，NULL 与非 NULL 混合时字段边界发生位移（`(NULL,'a')` 与 `('a',NULL)` 都得到 `'a'`），不同行会哈希出相同串；PG 侧 `concat_ws` 还要求参数为文本，数值列需显式 `::text`（MySQL 侧自动转换，不必加）；
- 时间列（含 `deleted_at`）两侧统一渲染到同一时区、同一格式（8.4 节）。`deleted_at` **不得**写成 `deleted_at::text`——该写法走会话 `TimeZone` 输出、格式也与 MySQL `DATE_FORMAT` 不一致；
- SQL 里的 `'Asia/Shanghai'` 是 8.4 节的**待实测假设**，Phase 4 用实测时区整体替换后再执行；
- **JSON/jsonb 列不进 SQL checksum**（两侧规范化规则不同，见 7.2），改在 Go 校验工具中"解析后重序列化"比对；
- **验收阈值：checksum 相等。** 不相等 → 11.3 分桶定位。

### 11.3 差异定位（mismatch 处理流程）

```sql
-- MySQL：按 id 分桶（每 10 万 id 一桶）算局部 checksum
SELECT (id DIV 100000) AS bucket, COUNT(*),
       BIT_XOR(CAST(CONV(SUBSTR(MD5(CONCAT_WS('|', /* 同 11.2 字段与 COALESCE 口径 */)), 1, 8), 16, 10) AS UNSIGNED))
FROM core.ke_forest GROUP BY bucket ORDER BY bucket;

-- PG：整数除法已按整除语义，桶宽必须与 MySQL 完全一致
SELECT (id / 100000) AS bucket, COUNT(*),
       bit_xor(('x' || substr(md5(concat_ws('|', /* 同 11.2 字段与 COALESCE 口径 */)), 1, 8))::bit(32)::bigint)
FROM core.ke_forest GROUP BY bucket ORDER BY bucket;
```

分桶口径：**两侧必须是同一表达式、同一桶宽**，否则桶号无法对齐。MySQL 的 `id / 100000` 返回小数（DECIMAL），不能当桶号；必须用 `id DIV 100000`（或 `FLOOR(id / 100000)`）。PG 侧 `id / 100000` 在 `id` 为整数时已是整除，保持原样。两侧输出 `diff`，对不一致桶内的 id 逐行比对（11.4 抽样口径同样适用于桶内）。

处理规则：定位到行后以 **MySQL 为准**修复 PG（重载该行/该桶），修复后重跑 11.1/11.2。

### 11.4 抽样确定性比对（软门槛，抽检语义正确性）

固定模数抽样（确定性、两侧同集合，不用随机数）：

```sql
-- 两侧执行（模数 97 为示例，Phase 4 按行数取 1%/5% 定档）
SELECT * FROM core.ke_forest WHERE id % 97 = 0 ORDER BY id;
```

导出两侧结果为 JSON（Go 工具统一序列化：时间按 8.4 节渲染、jsonb 解析后重序列化、bytea 十六进制），逐行 `diff`。**验收阈值：抽样行零差异**（差异即 fail，处理同 11.3）。

### 11.5 ES 重建与召回对比

- ES 索引数据全部来自系统库（按公司/知识库命名 `ke_0`、`ke_1`，`ik_max_word` 分词，见 AGENTS.md）；PG 迁移完成后必须**全量重建 ES 索引**（从 PG 回灌），重建前后各做一次召回对比。
- 对比载体：`scripts/golden-path.sh` 黄金链路（`docs/architecture/phase0/golden-path-baseline.md`，11 个 stage，含上传→解析→搜索→对话）。执行：迁移前对旧 MySQL 栈完整跑一遍记录基线（含搜索命中断言，`EXPECT_TEXT` 唯一 token）；PG + ES 重建后再跑一遍。
- **验收阈值：11 个 stage 全部通过（退出码 0），且搜索 stage 对同一 `EXPECT_TEXT` 的命中与迁移前一致。** 不一致 → 检查 ES 重建完整性与 6 节的 DB 直查召回（`~*`/`ILIKE` 改写是否漏站点）。

### 11.6 汇总对账表（每表一行，人工签收）

| schema | 表 | MySQL 行数 | PG 行数 | PK min/max 一致 | checksum 一致 | 抽样一致 | sequence 已重置 | 结论 |
|---|---|---|---|---|---|---|---|---|
| （Phase 4 填） | （Phase 4 填） | （Phase 4 填） | （Phase 4 填） | 是/否 | 是/否 | 是/否 | 是/否 | pass / fail(+定位桶) |

## 12. 风险与回滚

### 12.1 风险清单与缓解

| # | 风险 | 影响 | 缓解 / 触发条件 |
|---|---|---|---|
| 1 | **`bigint unsigned` 溢出**：值 > 2^63-1 载入 `bigint` 报错 | ETL 失败 | planner 已逐列 warning（`internal/migrate/mysql.go:72-75`）；Phase 4 对 warning 清单逐列 `SELECT MAX(col)`，超限列改 `numeric` 并同步改 model tag |
| 2 | **`tinyint(1)` 语义**：映射 `smallint` 而非 `boolean` | 若有第三方按 boolean 读写该列会混淆 | 保持 `smallint`（`types.Bool` 三态 -1/1/2，`pkgs/types/boolean.go:10-19`）；README/接口文档明示；禁止任何"顺手改 boolean" |
| 3 | **collation 搜索变化**：ci → 区分大小写 | 等值查询召回下降、ci 唯一性消失 | 6.2 查询层改写（`lower()`/`ILIKE`/`~*`）+ 6.4 体检 + 11.5 召回对比兜底 |
| 4 | **sequence 未重置**：ETL 后新写入主键冲突 | 切流后写入失败 | 4.2 节 `setval` 为 ETL 完成钩子（必执行步骤，进 11.6 对账表的"sequence 已重置"列） |
| 5 | **部分索引/表达式索引丢失**：MySQL 前缀索引（`sub_part`）与函数索引（`expression`）在 AutoMigrate 重建时丢 | 查询性能退化、隐性约束消失 | 5.1 盘点 + 5.3 载数后重建 + 5.3 一致性兜底查询比对索引集合 |
| 6 | **JSON 规范化规则差异**：MySQL `json` 与 PG `jsonb` 都已规范化（key 排序、空白去除、重复 key 折叠保留最后一个），但数字字面量/转义/key 排序平局规则不同 | 字节级比对假阳性、对账失败 | 7.2：JSON 列不进 SQL checksum，改 Go 侧"解析后重序列化"比对；确认应用不依赖原始字节（grep 序列化比较逻辑，需要在 Phase 4 dry-run 验证） |
| 7 | **时区/零日期**：8 小时偏移、`0000-00-00` 拒载 | 数据错误、ETL 中断 | 8.4 DSN `loc` 强制 + 8.1 零日期→NULL 规则 + 11.4 抽样时间字段逐值比对 |
| 8 | **空串/NULL 混淆**：迁移期归一化改变语义 | 业务判断分支反转 | 8.2 原样搬运铁律，归一化必须逐列显式批准 |
| 9 | **唯一索引 NULL 语义**：`NULLS NOT DISTINCT` 误用 | 多 NULL 行被拒/判重 | 5.4 逐唯一索引确认（PG15 特性不默认启用）+ 复合唯一索引含 NULL 列专项核对 |
| 10 | **一次性 cutover**：全量切换窗口长、失败回退成本高 | 停机超时 | 12.2 冻结 + 12.3 断点重放；大表先演练一次全流程（Phase 4 dry-run 即演练） |

### 12.2 回滚原则（MySQL 权威，直到签收）

1. **切流前 MySQL 始终是权威库**：ETL/校验期间 MySQL 保持可写（或只读冻结，见下），PG 仅作副本校验目标；校验 11.1–11.5 全部通过并人工签收 11.6 对账表之前，任何应用不切 PG。
2. **cutover 冻结**：切换窗口内 `SET GLOBAL super_read_only = ON;`（冻结 MySQL 写入，保证 PG 追平后无增量丢失），应用把 dbtools DSN 从 MySQL 切到 PG（pgdrv 注册见 `docs/architecture/phase0/pg-driver-spike.md` 第 4 节）；冻结期间业务写失败需产品侧预告。
3. **快照**：切流前对 PG 库做快照（`pg_dump` 全库或存储层快照），同时记录 MySQL 侧 `master` 位点（GTID/binlog 位点）——回滚后如需增量补偿，以此为起点。

### 12.3 回滚步骤（切流后发现问题的执行序列）

1. 应用 dbtools DSN 切回 MySQL（配置回滚，无数据操作）；
2. MySQL `SET GLOBAL super_read_only = OFF;` 恢复写入；
3. PG 侧处置二选一：
   - **整库重来**：`DROP SCHEMA core CASCADE;`（5 个 schema 逐一）→ 重跑 AutoMigrate + checkpoint 重放 ETL（自研工具断点保证从已完成批次续传，避免全量重导）；
   - **定点修复**：用 12.2 的快照 `pg_restore` 恢复受损表，再按 checkpoint 重放该表之后的批次；
4. 回滚期间 PG 上已产生的业务写入：逐表导出（`pg_dump --table`）人工合入 MySQL（仅限签收后发现的数据问题场景，写入前逐行 review）；
5. 复盘后重试 cutover 前必须重跑 11.1–11.5 全套校验（不因"只改了一点"跳过）。

### 12.4 回滚成本与演练要求

- 回滚 = 配置切换（分钟级）+ 数据处置（视 PG 上写入量，checkpoint 重放为大表全程的增量部分）；冻结窗口内的回滚最便宜（PG 无业务写入，直接丢弃重来）。
- **Phase 4 dry-run 必须至少演练一次完整回滚序列**（含 `DROP SCHEMA ... CASCADE` + checkpoint 重放），记录各步耗时，作为 cutover 预案的时间参数；未演练回滚不得切流。
