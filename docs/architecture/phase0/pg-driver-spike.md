# PG Driver Vendor Spike（Phase 0 / Task 10）

**目的**：在不修改主仓库 `go.mod` / `go.sum` / `vendor/` 的前提下，用临时 module（`/tmp/pgspike`）验证 Phase 1 接入 PostgreSQL 的依赖路径：GORM postgres driver 的版本选择、依赖树、vendor 增量、`dbtools/v2` 注册方式、`pgvector` 与 CGO 结论。本文所有结论均来自 2026-11-01 的实测（命令与输出见文末），标注"估计"处除外。

**范围**：仅编译验证（compile-only），不连接任何数据库；主仓库零改动（本文件是本次唯一新增文件）。

## 结论速览

| 事项 | 结论 |
| --- | --- |
| 推荐版本组合 | `gorm.io/gorm v1.30.1`（仓库现有）+ `gorm.io/driver/postgres v1.6.0` + `github.com/jackc/pgx/v5 v5.7.2` + `github.com/pgvector/pgvector-go v0.3.0` |
| Go 版本约束 | 上述组合 go directive 均 ≤ 1.23，`go1.24.13 + GOTOOLCHAIN=local` 编译通过；`@latest`（pgx v5.12.0 / driver v1.6.3）要求 go ≥ 1.25，会迫使主仓库升 go directive |
| dbtools/v2 | 上游 registry 已开放注册（`Register(scheme, factory)`），**Phase 1 自建 `postgres` factory 自注册，不改 vendor** |
| vendor 增量 | 主仓库净新增 7 个包 ≈ **1.6 MB**（实测 du，见下） |
| pgvector | Go 包 `github.com/pgvector/pgvector-go` v0.3.0（go 1.23）实测可编译；扩展需 `CREATE EXTENSION IF NOT EXISTS vector;` |
| CGO | pgx 纯 Go 实现，PG 驱动**无 CGO 依赖**；CGO_ENABLED 维持现状即可 |

## 1. 环境与版本

- 工具链：`go version go1.24.13 linux/amd64`（与仓库 `go.mod` 的 `go 1.24.0` 一致）
- 临时 module：`/tmp/pgspike`（`go mod init pgspike`），编译缓存 `GOCACHE=/dev/shm/gocache`、`TMPDIR=/dev/shm/tmp`（根盘近满，未污染仓库）
- 网络：`GOPROXY=https://goproxy.cn,direct` 可用，`@latest` 解析成功

### 版本探测结果（实测）

| 模块 | latest | go directive（latest） | go1.24 兼容的已缓存版本 | go directive |
| --- | --- | --- | --- | --- |
| `gorm.io/gorm` | v1.31.2 | go 1.18 | **v1.30.1**（仓库现有） | go 1.18 |
| `gorm.io/driver/postgres` | v1.6.3 | go 1.25.0 | **v1.6.0** | go 1.20 |
| `github.com/jackc/pgx/v5` | v5.12.0 | go 1.25.0 | **v5.7.2**（v5.6.0 亦可，见"pgvector 联动"） | go 1.21 |
| `github.com/pgvector/pgvector-go` | v0.4.1 | go 1.25.0 | **v0.3.0** | go 1.23.0 |

关键发现：**`pgx/v5 ≥ v5.10.0` 与 `driver/postgres ≥ v1.6.2` 的 go.mod 均声明 `go 1.25.0`**。用 `GOTOOLCHAIN=local`（go1.24.13）编译该组合会直接失败：

```
go: go.mod requires go >= 1.25.0 (running go 1.24.13; GOTOOLCHAIN=local)
```

默认 `GOTOOLCHAIN=auto` 下虽可自动下载 go1.25+/go1.26+ 工具链编译成功，但会把主仓库 `go.mod` 的 go directive 抬到 1.25.0，迫使所有开发者/CI 镜像升级工具链（仓库 Makefile 与 `.gitlab-ci.yml` 均未固定 GOTOOLCHAIN，存在隐式下载风险）。因此 Phase 1 推荐**钉在 go1.24 兼容组合**（上表加粗版本），全部已在 `/root/go/pkg/mod` 缓存中。

## 2. 依赖树（实测 `go mod graph` / `go list -m all` 摘录）

推荐组合下 `go.mod`（临时 module）：

```
require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/pgvector/pgvector-go v0.3.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.30.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)
```

关键依赖边：

```
gorm.io/driver/postgres@v1.6.0  → github.com/jackc/pgx/v5、gorm.io/gorm（≥ v1.25.10，仓库 v1.30.1 满足）
github.com/jackc/pgx/v5@v5.7.2  → pgpassfile、pgservicefile、puddle/v2、golang.org/x/{crypto,text}
github.com/pgvector/pgvector-go@v0.3.0 → pgx/v5（≥ v5.7.2，MVS 会把 v5.6.0 抬到 v5.7.2）、x448/float16
```

注意（pgvector 联动）：只加 driver/postgres v1.6.0 时解析到 pgx **v5.6.0**（go 1.20）；一旦加入 pgvector v0.3.0（其 go.mod require pgx ≥ v5.7.2），MVS 抬到 **v5.7.2**（go 1.21），仍兼容 go1.24。仓库现有 `golang.org/x/crypto v0.41.0 / x/sync v0.16.0 / x/text v0.28.0` 均高于所需（v0.36.0 / v0.12.0 / v0.23.0），MVS 保留仓库版本，**x/* 与 jinzhu 无需任何变动**。

## 3. Vendor 增量（实测，临时 module `go mod tidy && go mod vendor`）

临时 module 自身 vendor 总量 9.0M（含 gorm 636K、`golang.org/x/*` 6.7M——后者仓库已 vendored 且版本更高，非增量）。**主仓库真正需要新增的包**（仓库 `vendor/` 中目前不存在任何 `github.com/jackc/*`，实测 du -sk）：

| 新增 vendor 目录 | 大小（实测） | 说明 |
| --- | --- | --- |
| `vendor/github.com/jackc/pgx`（v5） | 1,344 KB | pgconn / pgproto3 / pgtype / pgxpool / stdlib 等 |
| `vendor/github.com/jackc/puddle`（v2） | 80 KB | 连接池 |
| `vendor/github.com/pgvector/pgvector-go` | 68 KB | vector / halfvec / sparsevec + pgx 子包 |
| `vendor/github.com/jackc/pgpassfile` | 20 KB | `~/.pgpass` 解析 |
| `vendor/github.com/x448/float16` | 36 KB | halfvec 的 float16 编码 |
| `vendor/github.com/jackc/pgservicefile` | 16 KB | pg_service.conf 解析 |
| `vendor/gorm.io/driver/postgres` | 56 KB | dialector（postgres.go / migrator.go / error_translator.go） |
| **合计** | **≈ 1.6 MB（实测求和）** | 估计：加上 modules.txt/go.sum 行与文件系统开销，仓库 vendor 总增量 < 2 MB |

Phase 1 需要做的：`go.mod` 增加 1 条 direct require（`gorm.io/driver/postgres`）+ 4 条 indirect（pgx、puddle、pgpassfile、pgservicefile；若同时接 pgvector 再加 2 条 direct + x448/float16 indirect），然后 `go mod tidy && go mod vendor` 并提交 vendor（按 AGENTS.md 流程）。

## 4. dbtools/v2 注册结论（实测代码）

`vendor/github.com/ygpkg/yg-go/dbtools/v2/registry.go`（已读全文）：

```go
type DriverFactory func(dsn string) (gorm.Dialector, error)

var drivers = map[string]DriverFactory{}

func Register(scheme string, factory DriverFactory) {
	drivers[strings.ToLower(scheme)] = factory
}

func Open(dburl string) (*gorm.DB, error) { /* 按 URL scheme 查表 → factory(dsn) → gorm.Open */ }
```

`mysqldrv`（`dbtools/v2/mysqldrv/driver.go`）只是**当前唯一的注册方**，通过 `init()` 自注册：

```go
func init() {
	dbtools.Register("mysql", func(dsn string) (gorm.Dialector, error) { ... })
}
```

仓库侧的既有用法（apps/keapp、keapi、corekg、kesearch、kechat、ketask 等约 30+ 处）均为 blank import 触发注册：

```go
import (
	dbtools "github.com/ygpkg/yg-go/dbtools/v2"
	_ "github.com/ygpkg/yg-go/dbtools/v2/mysqldrv"
)
...
db, err := dbtools.Open(dburl) // scheme "mysql" 命中 factory
```

**结论：上游 registry 是开放注册表，不是仅支持 mysqldrv 的硬编码。** Phase 1 决策：在 `internal/infra/db` 新建 `pgdrv` 包，仿照 mysqldrv 的 `init()` 自注册：

```go
func init() {
	dbtools.Register("postgres", func(dsn string) (gorm.Dialector, error) {
		return postgres.Open(dsn), nil // driver/postgres 原生接受 postgres:// URL
	})
}
```

业务侧只需新增 `_ "…/internal/infra/db/pgdrv"` blank import，`dbtools.Open("postgres://…")` 即可工作。**不需要修改 vendor、不需要给上游补 pgdrv**。

## 5. pgvector

- **PostgreSQL 扩展**（服务端，Phase 1 环境准备）：在目标库执行 `CREATE EXTENSION IF NOT EXISTS vector;`（需宿主已安装 `pgvector` 扩展包，如 Debian 的 `postgresql-16-pgvector` 或官方 `ankane/pgvector` Docker 镜像自带）。
- **Go 包**：`github.com/pgvector/pgvector-go`。实测解析：latest = **v0.4.1**（go directive 1.25.0，不兼容仓库 go1.24）；**v0.3.0**（go 1.23.0）在 spike 中与 pgx v5.7.2 一起 `GOTOOLCHAIN=local` 编译通过。
- API 形态（v0.3.0 实测源码）：根包 `pgvector.Vector / NewVector / HalfVector / NewHalfVector / SparseVector` 实现 `driver.Valuer` + `sql.Scanner`，可直接作 GORM 字段类型；pgx 原生协议需子包 `pgvector/pgx` 的 `RegisterTypes(ctx, *pgx.Conn)`（按 OID 注册 vector/halfvec/sparsevec）。
- 版本已如实解析成功（非"not resolved"），但 v0.4.1 的 go1.25 要求与第 1 节同一约束：若仓库不升 go directive，钉 v0.3.0。

## 6. CGO 结论

`gorm.io/driver/postgres` 基于 `jackc/pgx/v5` 纯 Go 实现的 PostgreSQL 协议（无 libpq 绑定、无 cgo import，spike 全程 `go build` 未涉及 cgo）。PG 驱动接入**不引入 CGO 依赖**，`CGO_ENABLED` 保持仓库现状即可；CGO 在其他组件（如 sqlite）中仍是允许项，与 PG 无关。

## 7. 回退方案（rollback options）

| 方案 | 内容 | 适用场景 |
| --- | --- | --- |
| (a) 裸 `pgx` / `database/sql` | 绕过 GORM postgres driver，直接 `pgxpool` 或 `database/sql` + pgx stdlib 注册 | 若 GORM dialector 与 dbtools 集成出现不可解的问题；代价是失去 GORM 迁移/模型层 |
| (b) **自建 `pgdrv` factory（推荐）** | `internal/infra/db/pgdrv` 内 `dbtools.Register("postgres", postgres.Open 适配)`，vendor 增量 ≈1.6MB，零上游改动 | **Phase 1 首选**：与现有 mysqldrv 模式完全同构，代码量 < 50 行，可测试、可回滚（删包即可） |
| (c) 上游贡献 `pgdrv` | 向 ygpkg/yg-go 提 PR，在 `dbtools/v2/pgdrv` 官方维护 | 仅当 registry 出现扩展性限制（如需要非 URL 形态 DSN、特殊连接池参数）且自建包无法满足时 |

推荐 (b)：registry 已验证开放（第 4 节代码），自建包是仓库内普通 Go 代码，不碰 vendor 里的上游代码，回滚 = 删除一个包 + 一次 blank import。

## 8. 实际执行的命令与输出（节选）

```bash
# 1. 临时 module（/tmp/pgspike）
mkdir -p /tmp/pgspike && cd /tmp/pgspike
go mod init pgspike

# 2a. 先试 @latest（网络可用，解析成功但抬 go directive）
go get gorm.io/gorm@latest gorm.io/driver/postgres@latest github.com/jackc/pgx/v5@latest
#   → gorm v1.31.2 / driver/postgres v1.6.3 / pgx v5.12.0
#   → "go: github.com/jackc/pgx/v5@v5.12.0 requires go >= 1.25.0; switching to go1.26.10"
#   → go: upgraded go 1.24.13 => 1.25.0
go build ./...            # GOTOOLCHAIN=auto 下成功，但 GOTOOLCHAIN=local 报：
#   "go: go.mod requires go >= 1.25.0 (running go 1.24.13; GOTOOLCHAIN=local)"

# 2b. 回退钉缓存版本（仓库兼容组合；brief 预设的 fallback 路径）
rm -f go.mod go.sum && go mod init pgspike
go get gorm.io/gorm@v1.30.1 gorm.io/driver/postgres@v1.6.0 github.com/jackc/pgx/v5@v5.6.0
go get github.com/pgvector/pgvector-go@v0.3.0   # MVS 将 pgx 抬至 v5.7.2

# 3. 编译（main.go 见 brief，仅 gorm.Open 不连接）
GOTOOLCHAIN=local go build ./...
#   → 退出码 0，无输出（成功）；go version: go1.24.13 linux/amd64

# 4. 依赖树 / vendor 增量
go list -m all | grep -E "pgvector|pgx|driver/postgres|gorm.io/gorm"
go mod graph | grep -E "^(gorm.io/driver/postgres|github.com/jackc/pgx/v5) "
go mod tidy && go mod vendor
du -sk vendor/gorm.io/driver/postgres vendor/github.com/jackc/{pgx,pgpassfile,pgservicefile,puddle} \
        vendor/github.com/pgvector vendor/github.com/x448
#   → 56 / 1344 / 20 / 16 / 80 / 68 / 36（KB）

# 5. pgvector 版本探测
go list -mod=mod -m -json github.com/pgvector/pgvector-go@latest  # v0.4.1, GoVersion 1.25.0
go list -mod=mod -m -json github.com/pgvector/pgvector-go@v0.3.0  # GoVersion 1.23.0
```

pgvector 编译性验证用了独立子包 `pgv/main.go`（import `pgvector` 根包与 `pgvector/pgx` 子包），与 brief 的主 `main.go` 分开，不影响其原样编译。

## 9. 主仓库未改动声明

本次 spike 仅新增本文档；主仓库 `go.mod`、`go.sum`、`vendor/` 零改动（`git status --short` 仅一个新文档，`git diff -- go.mod go.sum vendor` 为空）。临时产物 `/tmp/pgspike`、`/dev/shm/gocache`、`/dev/shm/tmp` 及 spike 触发的 module-cache 新增项已在记录完成后清理。
