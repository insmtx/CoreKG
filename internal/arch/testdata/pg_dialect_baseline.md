# PG 方言 ratchet 基线（Phase 0）

`internal/arch/pgdialect_test.go` 中的 `TestPGDialectRatchet` 冻结 Phase 0 开始时遗留
MySQL-only SQL 模式与 GORM 类型 tag 的数量。后续 PG 迁移工作只能**减少**这些计数，
不能增加（断言使用 `require.LessOrEqual`）。

计数只统计 `.go` 文件；跳过 trimmed 后以 `//` 开头的注释行；根路径相对于模块根目录
（通过向上查找 `go.mod` 定位，测试与 cwd 无关）。

## 基线

| 模式 | 基线 | 计数方式 | 范围 |
|---|---:|---|---|
| `FIND_IN_SET\(` | 3 | 含匹配的代码行数 | `apps/kecore/models/forest`、`apps/kesearch/models/globalsearch` |
| `\bREGEXP\b` | 2 | 含匹配的代码行数 | `apps/kesearch/models/globalsearch` |
| MySQL GORM 类型 tag | 218 | `gorm:"..."` tag 内的出现次数 | `apps` + `pkgs`（含下方排除项） |

### 具体分布（基线核实时）

- `FIND_IN_SET(`（3 行）：
  - `apps/kecore/models/forest/forest.go:946` ×1
  - `apps/kesearch/models/globalsearch/forest.go:87,89` ×2
- `REGEXP`（2 行）：
  - `apps/kesearch/models/globalsearch/agent.go:119`（该行含 2 次出现，按行计 1）
  - `apps/kesearch/models/globalsearch/forest.go:77`（该行含 2 次出现，按行计 1）
  - 注意：`REGEXP` 的原始出现次数为 4；基线 2 按“含匹配的行数”统计
    （与核实时 `grep -rn '\bREGEXP\b' | wc -l` 的口径一致）。
- MySQL GORM 类型 tag（218 次）按顶层目录分布：

  | 目录 | 次数 |
  |---|---:|
  | `apps/kecore` | 102 |
  | `apps/kechat` | 39 |
  | `apps/keapp` | 21 |
  | `apps/account` | 18 |
  | `apps/kesale` | 16 |
  | `pkgs/platform` | 8 |
  | `pkgs/jobs` | 5 |
  | `pkgs/task` | 5 |
  | `apps/keinit` | 2 |
  | `pkgs/connectors` | 1 |
  | `pkgs/wecoms` | 1 |

## 类型 tag 精确正则

先以 `gorm:"([^"]*)"` 提取 tag 内容，再在 tag 内容上匹配：

```text
type:(bigint unsigned|int unsigned|tinyint\(1\)|tinyint unsigned|tinyint|longtext|mediumtext|datetime\(3\)|datetime|int\(11\)|uint|TINYINT)([;"]|$)
```

**不计入**（PG 可接受）：`type:int`、`type:bigint`、`type:varchar`、`type:json`、
`type:boolean`。

## 类型 tag 统计的目录排除项（按目录名跳过）

| 目录名 | 对应路径 | 排除理由 |
|---|---|---|
| `workflow` | `apps/workflow` | Coze 工作流应用，不接系统库，不属于系统库 PG 迁移范围 |
| `testdata` | 各级 `testdata/`（含 `pkgs/einodocument/parser/xlsxparser/testdata`） | 测试固件，非生产 schema |
| `docs` | 各应用 `internal/docs` 等 | swag 生成的文档代码，禁止手改，不应计入 |
| `mysqlplugin` | `pkgs/plugins/dbplugins/mysqlplugin` | 外部租户数据源（用户自有 MySQL 库）插件，**必须保留** |
| `svcexcelforest` | `apps/kecore/services/svcexcelforest` | Excel 森林导入服务，防御性排除（当前无 gorm 类型 tag） |
| `excelparser` | `apps/kecore/services/excelparser` | Excel 解析，防御性排除（当前无 gorm 类型 tag） |

## 保留项 vs ratchet 目标

**必须保留（外部租户数据源功能，不在 ratchet 范围内）：**

- `IFNULL`：`pkgs/plugins/dbplugins/mysqlplugin/mysql.go`（3 处）。
- `SHOW CREATE TABLE`：`apps/kechat/models/qachat/excel_chat.go`、
  `apps/kechat/models/qachat/mysql_chat.go`、`apps/kechat/models/excelchat/qa_excel.go`、
  `apps/kecore/models/keqa/qa_excel.go`（各 1 处）。
- `mysqlplugin` 插件整体。

这些查询的目标是**用户自有的外部 MySQL 数据源**（Excel/MySQL 问答的租户库），与系统库
迁移到 PostgreSQL 无关，迁移时不得删除或改写。

**ratchet 目标（只能减少）：**

- `FIND_IN_SET`：迁移为 PG 写法（如 `= ANY(string_to_array(...))`）。
- `REGEXP`：迁移为 PG 写法（`~` / 全文检索 / pg_trgm）。
- 系统 model 上的 MySQL 类型 tag：迁移为 PG 兼容类型
  （`boolean`、`timestamptz`/`timestamp`、`text`、`bigint`/`integer` 等）。

新增代码不得引入上述 MySQL-only 模式，否则 `TestPGDialectRatchet` 失败。
