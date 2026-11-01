# CoreKG 单体化 Phase 0 基础工程实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **执行者说明**：本计划假设你没有看过本仓库和设计文档。每个任务都给出文件、接口、测试和提交点；按任务顺序执行，不要合并任务。生产代码遵循 TDD：先写失败测试，再实现，再跑测试。文档类任务用“必需章节检查”代替单测。

**Goal（目标）**：在不删除旧 `apps/*`、不接入 PostgreSQL、不下线 Coze 的前提下，建立单体化的地基：新入口 `cmd/corekg`、Profile 配置、架构门禁、PG 方言清零、路由清单、golden-path 基线、迁移 dry-run 与切换 runbook。

**Architecture（架构）**：Phase 0 只做“可验证的地基”，不做业务迁移。旧二进制保持可构建；新 `cmd/corekg` 只提供 `server` 与 `migrate` 两个子命令。所有新代码放在仓库根的 `cmd/`、`internal/`、`configs/`、`scripts/`、`tools/` 下。

**Tech Stack（技术栈）**：Go 1.24.0、cobra、gin、gopkg.in/yaml.v3、database/sql + go-sql-driver/mysql（仅迁移 dry-run）、testify、bash、jq、docker compose。

**Spec（规格）**：`docs/superpowers/specs/2026-11-01-corekg-monolith-module-abstraction-design.md`

**Phase 0 范围**：

- 新入口骨架与 `/healthz`
- Profile 配置加载/校验 + 三档 YAML 草稿
- 架构 import 门禁
- PG 方言 ratchet 测试
- `FIND_IN_SET` / `REGEXP` 替换
- GORM MySQL 类型 tag 归一化
- 路由清单脚本 + keep/delete/reopen 规则
- golden-path 脚本
- PG driver vendor spike
- `corekg migrate from-mysql --dry-run`
- MySQL→PG 迁移评估
- 一次性切换/回滚 runbook

**非范围**：真实 PG 连接、AutoMigrate、数据搬迁、Coze 代码/前端删除、端口接口实现、`apps/*` 目录迁移、ChatEngine/opencode。这些在 Phase 1–4。

## Review Focus

以下五类问题最容易在 Phase 0 被“看起来通过”掩盖；每一条都在对应任务里用测试钉住：

1. **`tinyint(1)` 被误改成 `boolean`**：`pkgs/types.Bool` 是 `int32`，PG `boolean` 扫描会失败。Task 7 的 `RewriteTag` 用例固定映射为 `smallint`。
2. **`FIND_IN_SET` 替换后类型不匹配**：`?` 是整数，`string_to_array` 返回 `text[]`。Task 5 的 `ContainsCSV` 用例固定 `?::text = ANY(...)`。
3. **响应 envelope 大小写不一致**：多数结构体没有 `json:"response"` tag，实际 key 是 `Response`。Task 9 的 `--self-test` 同时验证 `.Response` 和 `.response`。
4. **codemod 改写 raw SQL/外部 Excel DDL**：外部 MySQL DDL 里的 `BIGINT UNSIGNED` 不能被改。Task 7 的 `RewriteFile` 用例断言 raw SQL 不变。
5. **golden-path 退化成只测 `/healthz`**：Task 9 的 `--dry-run` 必须列出 11 个 stage，完整运行必须覆盖上传、解析、检索、RAG、keapi 与 MCP。

## Global Constraints（全局约束）

- **单 module + vendor**：module 是 `github.com/insmtx/corekg`。不要执行 `go get`/`go mod tidy`，除非任务明确要求；Phase 0 不新增主模块依赖。PG driver spike 在 `/tmp` 的临时 module 做。
- **MySQL 只用于迁移源**：系统库新代码不得新增 MySQL 方言。外部数据源代码（`pkgs/plugins/dbplugins/mysqlplugin`、`SHOW CREATE TABLE`、`IFNULL`、Excel 外部库 DDL）允许保留，不纳入 ratchet。
- **生成文件只读**：不要编辑 `apps/*/internal/docs/*_docs.go`、`docs.go`、`swagger.json`、已发布的 `scripts/mysql/*.sql`。
- **测试方式**：Phase 0 的单测不得依赖 MySQL/ES/Redis。命令统一用 `go test ./internal/... ./pkgs/utils/sqlutil/... ./cmd/corekg/...`；迁移集成测试用 `CORekG_MYSQL_DSN` 显式开启，未设置则 `t.Skip`。
- **响应 envelope**：现有 yg-go handler 用 `ctx.JSON`，很多响应结构体的 `Response` 字段没有 `json` tag，实际 JSON key 是 `Response`（大写）；少数结构体显式写了 `json:"response"`。golden-path 脚本必须用辅助函数同时接受 `.Response` 和 `.response`。
- **提交粒度**：一个任务一个 commit。提交信息用 `phase0: <task>`，例如 `phase0: add corekg skeleton and healthz`。
- **不引入新框架**：Phase 0 只用仓库已有依赖；不要引入 wire/dig/fx。

## 文件地图

### 新建

- `cmd/corekg/main.go`、`cmd/corekg/server.go`、`cmd/corekg/migrate.go`
- `internal/bootstrap/app.go`、`internal/bootstrap/app_test.go`
- `internal/api/health/health.go`、`internal/api/health/health_test.go`
- `internal/config/profile.go`、`internal/config/profile_test.go`
- `internal/arch/imports_test.go`、`internal/arch/pgdialect_test.go`
- `internal/arch/testdata/pg_dialect_baseline.md`
- `internal/migrate/plan.go`、`internal/migrate/plan_test.go`、`internal/migrate/mysql.go`
- `configs/profiles/personal.yaml`、`configs/profiles/aio.yaml`、`configs/profiles/enterprise.yaml`
- `pkgs/utils/sqlutil/pg.go`、`pkgs/utils/sqlutil/pg_test.go`
- `scripts/inventory-routes.sh`、`scripts/golden-path.sh`
- `tools/pgtypes/main.go`、`tools/pgtypes/main_test.go`
- `docs/architecture/phase0/route-inventory.csv`（生成物）
- `docs/architecture/phase0/route-decisions.md`
- `docs/architecture/phase0/golden-path-baseline.md`
- `docs/architecture/phase0/pg-driver-spike.md`
- `docs/architecture/phase0/mysql-inventory.json`（脱敏盘点）
- `docs/architecture/phase0/pg-migration-assessment.md`
- `docs/architecture/phase0/cutover-runbook.md`

### 修改

- `Makefile`：新增 `corekg`、`arch-test`、`route-inventory` target
- `apps/kecore/models/forest/forest.go`：`FIND_IN_SET` 替换
- `apps/kesearch/models/globalsearch/forest.go`：`FIND_IN_SET` + `REGEXP` 替换
- `apps/kesearch/models/globalsearch/agent.go`：`REGEXP` 替换
- 约 51 个系统模型文件：GORM 类型 tag 归一化（由 `tools/pgtypes` 机械改写）
- `internal/arch/pgdialect_test.go`：随任务 5/6/7 逐步下调 baseline

## 执行前准备

- 由执行者使用 `using-git-worktrees` 创建隔离 worktree，分支 `phase0-foundation`，不要把 Phase 0 直接做在 `main`。
- 先把本计划与设计文档提交到该分支；确认 `git status` 干净后再开始 Task 1。
- Phase 0 不部署、不切流量、不改生产配置；所有验证在本机或本仓 `docker-compose` 完成。

## 任务依赖

- Task 1 → Task 3、Task 9
- Task 2 → Task 3
- Task 4 → Task 5、Task 6、Task 7
- Task 10 独立
- Task 11 → Task 12
- Task 8、Task 13 独立

---

## Task 1：`cmd/corekg` 骨架与 `/healthz`

**Files**

- Create `cmd/corekg/main.go`
- Create `cmd/corekg/server.go`
- Create `internal/bootstrap/app.go`
- Create `internal/bootstrap/app_test.go`
- Create `internal/api/health/health.go`
- Create `internal/api/health/health_test.go`
- Modify `Makefile`

**Interfaces（接口）**

- Consumes: 无（只依赖 gin）。
- Produces: `bootstrap.NewServer(addr string) *http.Server`、`health.Register(eng *gin.Engine)`、`health.Handler(c *gin.Context)`。

```go
// internal/bootstrap/app.go
package bootstrap

func NewServer(addr string) *http.Server
```

```go
// internal/api/health/health.go
package health

func Register(eng *gin.Engine)
func Handler(c *gin.Context)
```

`Handler` 返回 HTTP 200 和 `{"status":"ok"}`。

`cmd/corekg/server.go`：

```go
func newServerCmd() *cobra.Command
```

flag：`--addr`，默认 `:8080`。

`cmd/corekg/main.go`：root cobra `Use: "corekg"`，挂 `newServerCmd()`。

**步骤**

- [ ] 1. 写失败测试 `internal/api/health/health_test.go`：

```go
func TestHealthz(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    Register(r)
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    r.ServeHTTP(w, req)
    require.Equal(t, http.StatusOK, w.Code)
    require.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}
```

- [ ] 2. 运行 `go test ./internal/api/health`，确认因 `Register` 未定义而失败。
- [ ] 3. 实现 `health.go`：

```go
func Register(eng *gin.Engine) { eng.GET("/healthz", Handler) }

func Handler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

- [ ] 4. 实现 `internal/bootstrap/app.go`：

```go
func NewServer(addr string) *http.Server {
    gin.SetMode(gin.ReleaseMode)
    eng := gin.New()
    eng.Use(gin.Recovery())
    health.Register(eng)
    return &http.Server{
        Addr:              addr,
        Handler:           eng,
        ReadHeaderTimeout: 5 * time.Second,
    }
}
```

- [ ] 5. 写 `internal/bootstrap/app_test.go`：`NewServer(":0")`，用 `httptest` 请求 `/healthz`，断言 200。
- [ ] 6. 实现 `cmd/corekg/server.go`：`bootstrap.NewServer(addr)`；用 `signal.NotifyContext` 处理 `SIGINT/SIGTERM`，`Shutdown` 超时 5s。
- [ ] 7. 实现 `cmd/corekg/main.go`：root cmd + `newServerCmd()`；`Execute()` 出错时 `os.Exit(1)`。
- [ ] 8. 在 `Makefile` 的 `local` target 附近新增：

```make
corekg:
	mkdir -p bundles
	$(GO) build -ldflags="$(LD_FLAGS)" -v -o bundles/corekg ./cmd/corekg
```

- [ ] 9. 运行 `go test ./internal/api/health ./internal/bootstrap`、`go build ./cmd/corekg`、`make corekg`，全部通过。
- [ ] 10. Commit：`phase0: add corekg skeleton and healthz`

**验收**

- `go build ./cmd/corekg` 成功。
- `./bundles/corekg server --addr 127.0.0.1:18080` 启动后 `curl -fsS http://127.0.0.1:18080/healthz` 返回 `{"status":"ok"}`。
- 不修改旧 `apps/*/cmd`。

---

## Task 2：Profile 配置加载与校验

**Files**

- Create `internal/config/profile.go`
- Create `internal/config/profile_test.go`
- Create `configs/profiles/personal.yaml`
- Create `configs/profiles/aio.yaml`
- Create `configs/profiles/enterprise.yaml`

**Interfaces（接口）**

- Consumes: 无（只依赖 yaml.v3）。
- Produces: `config.Load(path string) (*Profile, error)`、`(*Profile).Validate() error`、三份 `configs/profiles/*.yaml`。

```go
package config

type Profile struct {
    Profile      string       `yaml:"profile"`
    Capabilities Capabilities `yaml:"capabilities"`
    Infra        Infra        `yaml:"infra"`
}

type Capabilities struct {
    Knowledge          bool `yaml:"knowledge"`
    Chat               bool `yaml:"chat"`
    Search             bool `yaml:"search"`
    Graph              bool `yaml:"graph"`
    OCR                bool `yaml:"ocr"`
    Video              bool `yaml:"video"`
    Writing            bool `yaml:"writing"`
    MCP                bool `yaml:"mcp"`
    Billing            bool `yaml:"billing"`
    ExternalDataSource bool `yaml:"external_datasource"`
}

type Infra struct {
    DB        DriverConfig `yaml:"db"`
    Vector    DriverConfig `yaml:"vector"`
    Keyword   DriverConfig `yaml:"keyword"`
    Object    DriverConfig `yaml:"object"`
    Cache     DriverConfig `yaml:"cache"`
    Lock      DriverConfig `yaml:"lock"`
    Stream    DriverConfig `yaml:"stream"`
    Queue     DriverConfig `yaml:"queue"`
    Embedding DriverConfig `yaml:"embedding"`
    Rerank    DriverConfig `yaml:"rerank"`
    LLM       DriverConfig `yaml:"llm"`
    Parser    DriverConfig `yaml:"parser"`
    Graph     DriverConfig `yaml:"graph"`
}

type DriverConfig struct {
    Driver  string            `yaml:"driver"`
    DSN     string            `yaml:"dsn,omitempty"`
    URL     string            `yaml:"url,omitempty"`
    Addr    string            `yaml:"addr,omitempty"`
    Root    string            `yaml:"root,omitempty"`
    Bucket  string            `yaml:"bucket,omitempty"`
    Mode    string            `yaml:"mode,omitempty"`
    Options map[string]string `yaml:"options,omitempty"`
}

func Load(path string) (*Profile, error)
func (p *Profile) Validate() error
```

**校验规则（必须全部实现）**

- `profile` ∈ `personal|aio|enterprise`。
- `infra.db.driver` 必须是 `postgres`；`infra.db.dsn` 非空。
- `infra.vector.driver` ∈ `pgvector|elasticsearch|milvus|qdrant`；`elasticsearch|milvus|qdrant` 要求 `addr` 或 `url`。
- `infra.keyword.driver` ∈ `postgres_fts|elasticsearch`；`elasticsearch` 要求 `addr`。
- `infra.object.driver` ∈ `localdir|minio|s3`；`localdir` 要求 `root`；`minio|s3` 要求 `url` 和 `bucket`。
- `infra.cache.driver` ∈ `memory|redis`；`redis` 要求 `addr`。
- `infra.lock.driver` ∈ `db|redis`；`redis` 要求 `addr`。
- `infra.stream.driver` ∈ `memory|redis`；`redis` 要求 `addr`。
- `infra.queue.driver` ∈ `db|nats`；`nats` 要求 `url`。
- `infra.embedding.driver` ∈ `http|local`；`http` 要求 `url`。
- `infra.rerank.driver` ∈ `none|http|local`；`http` 要求 `url`。
- `infra.llm.driver` ∈ `http|openai`；`http`/`openai` 要求 `url`。
- `infra.parser.driver` ∈ `local|http`；`http` 要求 `url`。
- `infra.graph.driver` ∈ `none|postgres|nebulagraph`；`nebulagraph` 要求 `addr`。
- 能力与驱动一致：`capabilities.chat=true` 要求 `infra.llm.driver != ""`；`capabilities.search=true` 要求 `infra.vector.driver != ""` 且 `infra.embedding.driver != ""`；`capabilities.graph=true` 要求 `infra.graph.driver != "none"`。
- 错误信息必须包含字段路径，如 `infra.db.driver`、`infra.db.dsn`。

**YAML 草稿**

`configs/profiles/personal.yaml`：

```yaml
profile: personal
capabilities:
  knowledge: true
  chat: true
  search: true
  graph: false
  ocr: false
  video: false
  writing: false
  mcp: false
  billing: false
  external_datasource: false
infra:
  db:
    driver: postgres
    dsn: postgres://corekg:corekg@127.0.0.1:5432/corekg?sslmode=disable
  vector:
    driver: pgvector
  keyword:
    driver: postgres_fts
  object:
    driver: localdir
    root: ./data/objects
  cache:
    driver: memory
  lock:
    driver: db
  stream:
    driver: memory
  queue:
    driver: db
  embedding:
    driver: http
    url: http://127.0.0.1:11434/v1/embeddings
  rerank:
    driver: none
  llm:
    driver: http
    url: http://127.0.0.1:11434/v1
  parser:
    driver: local
  graph:
    driver: none
```

`configs/profiles/aio.yaml`：与 personal 相同，但 `profile: aio`，`cache/lock/stream` 用 `redis` + `addr: 127.0.0.1:6379`，`mcp: true`。

`configs/profiles/enterprise.yaml`：`profile: enterprise`，全部能力为 `true`（`video` 可保持 `false`），`vector/keyword` 用 `elasticsearch` + `addr: http://127.0.0.1:9200`，`object` 用 `s3` + `url/bucket`，`cache/lock/stream` 用 `redis`，`queue` 用 `nats` + `url: nats://127.0.0.1:4222`，`rerank/parser` 用 `http` + `url`，`graph` 用 `nebulagraph` + `addr`。

**步骤**

- [ ] 1. 写失败测试 `profile_test.go`：

```go
func TestLoadPersonal(t *testing.T) {
    p, err := Load("../../configs/profiles/personal.yaml")
    require.NoError(t, err)
    require.NoError(t, p.Validate())
    require.Equal(t, "postgres", p.Infra.DB.Driver)
    require.Equal(t, "pgvector", p.Infra.Vector.Driver)
}

func TestValidateRejectsMySQL(t *testing.T) {
    p := validProfile()
    p.Infra.DB.Driver = "mysql"
    require.ErrorContains(t, p.Validate(), "infra.db.driver")
}

func TestValidateRejectsUnknownVector(t *testing.T) { /* vector.driver=sqlite_vec */ }
func TestValidateRequiresDSN(t *testing.T) { /* db.dsn="" */ }
func TestValidateRequiresLLMForChat(t *testing.T) { /* chat=true, llm.driver="" */ }
```

- [ ] 2. 运行 `go test ./internal/config`，确认失败。
- [ ] 3. 实现 `profile.go`：`os.ReadFile` + `yaml.Unmarshal`；`Validate` 用表驱动 + 辅助函数 `requireDriver(section, driver string, allowed ...string) error`。
- [ ] 4. 写三个 YAML 文件；确保 `Load + Validate` 全部通过。
- [ ] 5. 运行 `go test ./internal/config`，通过。
- [ ] 6. Commit：`phase0: add deployment profile config`

**验收**

- 三个 YAML 都能 `Load` + `Validate`。
- 非法 driver、缺失 DSN、能力与驱动不一致都有稳定错误信息。
- 不修改旧 `conf/*.yaml`，不接入 yg-go `config`。

---

## Task 3：架构 import 门禁

**Files**

- Create `internal/arch/imports_test.go`
- Modify `Makefile`

**Interfaces（接口）**

- Consumes: `internal/config`、`internal/api/health` 的包路径。
- Produces: `internal/arch/imports_test.go`、`make arch-test`。

**规则**

Phase 0 只约束新代码，不约束旧 `apps/*`：

- `./internal/config` 不得 import `github.com/insmtx/corekg/apps/...`。
- `./internal/api/...` 不得 import `github.com/insmtx/corekg/apps/...`。
- `./internal/api/...` 不得 import `github.com/insmtx/corekg/internal/infra/...`（Phase 2 才会出现，先立规则）。
- `./internal/config` 不得 import `./internal/api` 或 `./internal/bootstrap`。

**步骤**

- [ ] 1. 写 `imports_test.go`，用 `go list` 获取 imports：

```go
func listImports(t *testing.T, pkg string) []string {
    t.Helper()
    out, err := exec.Command("go", "list", "-f",
        `{{range .Imports}}{{println .}}{{end}}`, pkg).Output()
    require.NoError(t, err)
    return strings.Fields(string(out))
}

func TestConfigDoesNotImportApps(t *testing.T) {
    for _, imp := range listImports(t, "./internal/config") {
        require.False(t, strings.HasPrefix(imp, "github.com/insmtx/corekg/apps/"), imp)
    }
}
```

- [ ] 2. 运行 `go test ./internal/arch`，确认通过（此时新包没有 apps import）。
- [ ] 3. 增加 API 规则：

```go
func TestAPIDoesNotImportAppsOrInfra(t *testing.T) {
    for _, imp := range listImports(t, "./internal/api/...") {
        require.False(t, strings.HasPrefix(imp, "github.com/insmtx/corekg/apps/"), imp)
        require.False(t, strings.HasPrefix(imp, "github.com/insmtx/corekg/internal/infra/"), imp)
    }
}
```

- [ ] 4. 增加 `Makefile` target：

```make
arch-test:
	$(GO) test ./internal/arch ./internal/config ./internal/api/...
```

- [ ] 5. 运行 `make arch-test`，通过。
- [ ] 6. Commit：`phase0: add architecture import gate`

**验收**

- 规则只覆盖 `internal/config`、`internal/api/...`，不会让旧 `apps/*` 立刻失败。
- 新增违规 import 时测试失败并打印 import 路径。

---

## Task 4：PG 方言 ratchet 测试

**Files**

- Create `internal/arch/pgdialect_test.go`
- Create `internal/arch/testdata/pg_dialect_baseline.md`

**Interfaces（接口）**

- Consumes: 无。
- Produces: `baselineFindInSet`、`baselineRegexp`、`baselineMySQLTypTag` 常量，`countPattern`/`countMySQLTypeTags` 测试辅助函数。

**基线（Phase 0 开始时）**

| 模式 | 基线 | 范围 |
|---|---:|---|
| `FIND_IN_SET(` | 3 | `apps/kecore/models/forest`、`apps/kesearch/models/globalsearch` |
| `REGEXP` | 2 | `apps/kesearch/models/globalsearch` |
| MySQL GORM 类型 tag | 218 | `apps` + `pkgs`，排除 `apps/workflow`、`testdata`、`pkgs/plugins/dbplugins/mysqlplugin`、`apps/kecore/services/svcexcelforest`、`apps/kecore/services/excelparser`、`pkgs/einodocument/parser/xlsxparser/testdata` |

类型 tag 精确正则：

```text
type:(bigint unsigned|int unsigned|tinyint\(1\)|tinyint unsigned|tinyint|longtext|mediumtext|datetime\(3\)|datetime|int\(11\)|uint|TINYINT)([;"]|$)
```

计数只统计 `gorm:"..."` 内的 tag；跳过以 `//` 开头的行。`type:int`、`type:bigint`、`type:varchar`、`type:json`、`type:boolean` 是 PG 可接受的，不计入。

**步骤**

- [ ] 1. 写测试常量：

```go
const (
    baselineFindInSet   = 3
    baselineRegexp      = 2
    baselineMySQLTypTag = 218
)
```

- [ ] 2. 实现 `countPattern(t, roots, pattern, skipDirs)`：`filepath.WalkDir`，跳过目录名 `workflow`、`testdata`、`docs`，读取 `.go` 文件，按行匹配。类型 tag 用 `gormTagRe.FindAllStringSubmatch`。
- [ ] 3. 测试：

```go
func TestPGDialectRatchet(t *testing.T) {
    findInSet := countPattern(t, []string{"apps/kecore/models/forest", "apps/kesearch/models/globalsearch"}, `FIND_IN_SET\(`)
    regexp := countPattern(t, []string{"apps/kesearch/models/globalsearch"}, `\bREGEXP\b`)
    typeTags := countMySQLTypeTags(t)
    t.Logf("FIND_IN_SET=%d REGEXP=%d typeTags=%d", findInSet, regexp, typeTags)
    require.LessOrEqual(t, findInSet, baselineFindInSet)
    require.LessOrEqual(t, regexp, baselineRegexp)
    require.LessOrEqual(t, typeTags, baselineMySQLTypTag)
}
```

- [ ] 4. 运行 `go test ./internal/arch -run TestPGDialectRatchet -v`，确认计数为 3/2/218。
- [ ] 5. 写 `internal/arch/testdata/pg_dialect_baseline.md`，记录上表与排除理由（外部数据源保留 `IFNULL`/`SHOW CREATE TABLE`/`mysqlplugin`）。
- [ ] 6. Commit：`phase0: add pg dialect ratchet`

**验收**

- 测试只允许基线以内的出现次数；新增 MySQL 方言会失败。
- 排除项写进 baseline 文档，避免后续误删外部数据源插件。

---

## Task 5：`sqlutil.ContainsCSV` 与 `FIND_IN_SET` 替换

**Files**

- Create `pkgs/utils/sqlutil/pg.go`
- Create `pkgs/utils/sqlutil/pg_test.go`
- Modify `apps/kecore/models/forest/forest.go`
- Modify `apps/kesearch/models/globalsearch/forest.go`
- Modify `internal/arch/pgdialect_test.go`

**Interfaces（接口）**

- Consumes: `internal/arch` 的 `baselineFindInSet`。
- Produces: `sqlutil.ContainsCSV(column string) string`。

```go
package sqlutil

// ContainsCSV 返回 PostgreSQL 表达式：?::text = ANY(string_to_array(column, ','))
func ContainsCSV(column string) string
```

**替换规则**

- `FIND_IN_SET(?,f.manager_ids) > 0` → `?::text = ANY(string_to_array(f.manager_ids, ','))`
- `FIND_IN_SET(?, forest.manager_ids) > 0` → `?::text = ANY(string_to_array(forest.manager_ids, ','))`

占位符顺序不变：原来 `FIND_IN_SET(?, ...)` 的 `?` 对应参数列表里的 `uin`，替换后仍是第一个 `?`。

**步骤**

- [ ] 1. 写失败测试：

```go
func TestContainsCSV(t *testing.T) {
    require.Equal(t, "?::text = ANY(string_to_array(f.manager_ids, ','))", ContainsCSV("f.manager_ids"))
}
```

- [ ] 2. 运行 `go test ./pkgs/utils/sqlutil`，确认失败。
- [ ] 3. 实现 `ContainsCSV`：`fmt.Sprintf("?::text = ANY(string_to_array(%s, ','))", column)`。
- [ ] 4. 修改 `apps/kecore/models/forest/forest.go` 的 `CanUseForest`：

```go
Where("("+
    sqlutil.ContainsCSV("f.manager_ids")+" OR "+
    "(f.public_scope = ? AND f.company_id = ?) OR "+
    ...
```

- [ ] 5. 修改 `apps/kesearch/models/globalsearch/forest.go`：

```go
"(forest.uin = ? AND "+sqlutil.ContainsCSV("forest.manager_ids")+") OR "+
sqlutil.ContainsCSV("forest.manager_ids")+" OR "+
```

- [ ] 6. 把 `baselineFindInSet` 改为 `0`，运行 `go test ./pkgs/utils/sqlutil ./internal/arch`。若 `apps/kecore/models/forest` 有单测，运行 `go test ./apps/kecore/models/forest`；若因缺少配置跳过/失败，记录到 PR 说明，不强行修。
- [ ] 7. Commit：`phase0: replace find_in_set with pg array predicate`

**验收**

- `FIND_IN_SET` 在系统库代码中为 0。
- 两处权限 SQL 的占位符数量和顺序与原逻辑一致。
- `sqlutil` 不依赖 gin/gorm，只是字符串构造。

---

## Task 6：`sqlutil.RegexpInsensitive` 与 `REGEXP` 替换

**Files**

- Modify `pkgs/utils/sqlutil/pg.go`
- Modify `pkgs/utils/sqlutil/pg_test.go`
- Modify `apps/kesearch/models/globalsearch/forest.go`
- Modify `apps/kesearch/models/globalsearch/agent.go`
- Modify `internal/arch/pgdialect_test.go`

**Interfaces（接口）**

- Consumes: `pkgs/utils/sqlutil` 包（Task 5 已创建）。
- Produces: `sqlutil.RegexpInsensitive(column string) string`。

```go
// RegexpInsensitive 返回 PostgreSQL 大小写不敏感正则表达式：column ~* ?
func RegexpInsensitive(column string) string
```

MySQL `REGEXP` 对非二进制字符串默认大小写不敏感；PG 用 `~*` 保持兼容。

**步骤**

- [ ] 1. 写失败测试：

```go
func TestRegexpInsensitive(t *testing.T) {
    require.Equal(t, "forest.name ~* ?", RegexpInsensitive("forest.name"))
}
```

- [ ] 2. 运行 `go test ./pkgs/utils/sqlutil`，确认失败。
- [ ] 3. 实现 `RegexpInsensitive`。
- [ ] 4. 替换 `forest.go`：

```go
Where(sqlutil.RegexpInsensitive("forest.name")+" OR "+sqlutil.RegexpInsensitive("forest.description"), token_str, token_str)
```

- [ ] 5. 替换 `agent.go`：

```go
Where(sqlutil.RegexpInsensitive("a.show_name")+" OR "+sqlutil.RegexpInsensitive("v.description"), token_str, token_str)
```

- [ ] 6. 把 `baselineRegexp` 改为 `0`，运行 `go test ./pkgs/utils/sqlutil ./internal/arch`。
- [ ] 7. Commit：`phase0: replace mysql regexp with pg case-insensitive regexp`

**验收**

- `REGEXP` 在系统库代码中为 0。
- 搜索语义保持大小写不敏感。

---

## Task 7：GORM MySQL 类型 tag 归一化

**Files**

- Create `tools/pgtypes/main.go`
- Create `tools/pgtypes/main_test.go`
- Modify 约 51 个系统模型文件（由工具改写）
- Modify `internal/arch/pgdialect_test.go`

**Interfaces（接口）**

- Consumes: `internal/arch` 的 `baselineMySQLTypTag`。
- Produces: `main.RewriteTag(tag string) string`、`main.RewriteFile(src []byte) ([]byte, bool)`。

**映射表**

| MySQL tag | PG tag | 说明 |
|---|---|---|
| `type:bigint unsigned` | `type:bigint` | ID 不会超过 int64；迁移时校验 |
| `type:int unsigned` | `type:bigint` | 保留 0..4294967295 范围 |
| `type:tinyint unsigned` | `type:smallint` | 0..255 |
| `type:tinyint(1)` | `type:smallint` | `types.Bool` 是 int32，不要用 boolean |
| `type:tinyint` | `type:smallint` |  |
| `type:longtext` | `type:text` |  |
| `type:mediumtext` | `type:text` |  |
| `type:datetime(3)` | `type:timestamptz` | 先匹配带精度 |
| `type:datetime` | `type:timestamptz` |  |
| `type:int(11)` | `type:integer` |  |
| `type:uint` | `type:bigint` |  |
| `type:TINYINT` | `type:smallint` |  |

同时删除 GORM tag 里的 `ON UPDATE CURRENT_TIMESTAMP` / `ON UPDATE CURRENT_TIMESTAMP(3)`；`default CURRENT_TIMESTAMP` 和 `default:CURRENT_TIMESTAMP(3)` 在 PG 合法，保留。不要改 `DEFAULT CURRENT_TIMESTAMP` 为 `now()`，除非测试发现 GORM 生成失败。

**工具接口**

```go
package main

func RewriteTag(tag string) string
func RewriteFile(src []byte) ([]byte, bool)
```

`RewriteFile` 只改写 `` gorm:"..." `` 内的内容，不碰 raw SQL 字符串和 Excel 外部库 DDL。

**步骤**

- [ ] 1. 写失败测试 `tools/pgtypes/main_test.go`：

```go
func TestRewriteTag(t *testing.T) {
    cases := map[string]string{
        "type:tinyint(1);default:1":                  "type:smallint;default:1",
        "type:tinyint;not null":                      "type:smallint;not null",
        "type:tinyint unsigned;not null":             "type:smallint;not null",
        "type:bigint unsigned;not null":              "type:bigint;not null",
        "type:int unsigned;not null":                 "type:bigint;not null",
        "type:longtext;comment:x":                    "type:text;comment:x",
        "type:mediumtext;comment:x":                  "type:text;comment:x",
        "type:datetime(3);not null":                  "type:timestamptz;not null",
        "type:datetime;not null":                     "type:timestamptz;not null",
        "type:int(11);not null":                      "type:integer;not null",
        "type:uint;not null":                         "type:bigint;not null",
        "type:TINYINT;not null":                      "type:smallint;not null",
        "type:datetime(3);default:CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)": "type:timestamptz;default:CURRENT_TIMESTAMP(3)",
    }
    for in, want := range cases {
        require.Equal(t, want, RewriteTag(in))
        require.Equal(t, want, RewriteTag(want)) // idempotent
    }
}

func TestRewriteFileLeavesRawSQL(t *testing.T) {
    src := []byte("const ddl = \"BIGINT UNSIGNED NOT NULL\"\n" +
        "var x struct{ ID uint `gorm:\"type:bigint unsigned;not null\"` }\n")
    out, changed := RewriteFile(src)
    require.True(t, changed)
    require.Contains(t, string(out), "BIGINT UNSIGNED NOT NULL")
    require.Contains(t, string(out), "type:bigint;not null")
}
```

- [ ] 2. 运行 `go test ./tools/pgtypes`，确认失败。
- [ ] 3. 实现 `RewriteTag`：按顺序 `strings.ReplaceAll`，先长后短；用正则删除 `\s+ON UPDATE CURRENT_TIMESTAMP(\(3\))?`。
- [ ] 4. 实现 `RewriteFile`：正则 `gorm:"([^"]*)"`，对捕获组调用 `RewriteTag`；返回是否有变化。
- [ ] 5. 实现 `main`：walk `apps` + `pkgs`，跳过 `workflow`、`testdata`、`svcexcelforest`、`excelparser`、`xlsxparser/testdata`、`mysqlplugin`；改写后 `format.Source` 再写回。
- [ ] 6. 运行 `go run ./tools/pgtypes`，然后 `git diff --stat` 确认只改了模型 tag。
- [ ] 7. 把 `baselineMySQLTypTag` 改为 `0`，运行 `go test ./internal/arch ./tools/pgtypes`。
- [ ] 8. 运行编译：`go build ./apps/account/... ./apps/keapp/... ./apps/kechat/... ./apps/kecore/... ./apps/kesale/... ./apps/keinit/... ./pkgs/...`。
- [ ] 9. Commit：`phase0: normalize gorm types for postgres`

**验收**

- `pg_dialect_test.go` 的 type tag 基线为 0。
- 编译通过；`apps/workflow`、外部数据源代码未被改写。
- `tools/pgtypes` 可重复运行且幂等。

---

## Task 8：路由清单与 keep/delete/reopen 规则

**Files**

- Create `scripts/inventory-routes.sh`
- Create `docs/architecture/phase0/route-inventory.csv`（脚本生成）
- Create `docs/architecture/phase0/route-decisions.md`
- Modify `Makefile`

**Interfaces（接口）**

- Consumes: 无。
- Produces: `scripts/inventory-routes.sh`、`docs/architecture/phase0/route-inventory.csv`、`make route-inventory`。

**脚本行为**

- 扫描 `apps`，排除 `apps/workflow`、`*/internal/docs/`、`*_docs.go`、`testdata`。
- 匹配 `.P(`, `.G(`, `.Any(`, `.PRequireLogin(`, `.PRequireEmployee(`, `.GRequireLogin(`, `.HandleDoc(` 后的第一个字符串字面量。
- 输出 CSV：`file,line,method,action`，按 action 排序。
- 当前基线应 ≥ 400 行；已知 action：`forest.CreateForest`、`forest.PreUploadFile`、`forest.ForestSearch`、`chat.SubmitChatQuestion`、`keapi.Search`、`keapi.UploadFile`。

**步骤**

- [ ] 1. 写脚本：

```bash
#!/usr/bin/env bash
set -euo pipefail
out="${1:-docs/architecture/phase0/route-inventory.csv}"
mkdir -p "$(dirname "$out")"
printf 'file,line,method,action\n' > "$out"
grep -rnE '\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("[^"]+"' apps \
  --include='*.go' --exclude-dir=workflow --exclude-dir=testdata \
  | grep -v '/internal/docs/' \
  | sed -E 's#^([^:]+):([0-9]+):.*\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("([^"]+)".*#\1,\2,\3,\4#' \
  | sort -t, -k4,4 >> "$out"
```

- [ ] 2. 运行 `bash -n scripts/inventory-routes.sh`。
- [ ] 3. 运行 `scripts/inventory-routes.sh /tmp/routes.csv`；断言：

```bash
test "$(wc -l < /tmp/routes.csv)" -gt 400
grep -q 'forest.CreateForest' /tmp/routes.csv
grep -q 'forest.PreUploadFile' /tmp/routes.csv
grep -q 'forest.ForestSearch' /tmp/routes.csv
grep -q 'chat.SubmitChatQuestion' /tmp/routes.csv
grep -q 'keapi.Search' /tmp/routes.csv
```

- [ ] 4. 生成 `docs/architecture/phase0/route-inventory.csv`。
- [ ] 5. 写 `route-decisions.md`：

```markdown
# Route decisions

## Keep
- account.* 认证/组织/成员/API Key
- kecore forest.* / file.* / QA / graph
- kechat chat.* / chat model
- kesearch search
- keapi.*（含 MCP）
- ketask / kesale

## Delete with Coze
- action 包含 `Coze`、`coze` 的路由
- `apps/workflow` 全部路由
- `/v2` 挂载（`wecom` 除外，Phase 1 再定）

## Reopen
- `/v2` 仅在确认有外部消费者时重新开放
- `chat.ExternalToken` / `chat.GetExternalStatus` 需要 API 兼容性评审

## Exceptions
- 每个例外必须在 PR 中写明 owner 和理由。
```

- [ ] 6. 新增 `Makefile` target：

```make
route-inventory:
	./scripts/inventory-routes.sh
```

- [ ] 7. Commit：`phase0: inventory routes and record keep/delete decisions`

**验收**

- CSV 覆盖所有字面量 action；已知 action 存在。
- 删除规则明确覆盖 Coze 与 workflow；不逐条手工维护 400+ 路由。

---

## Task 9：golden-path 回归脚本

**Files**

- Create `scripts/golden-path.sh`
- Create `docs/architecture/phase0/golden-path-baseline.md`

**Interfaces（接口）**

- Consumes: `GET /healthz`（Task 1）。
- Produces: `scripts/golden-path.sh`、`docs/architecture/phase0/golden-path-baseline.md`。

**环境变量**

```text
BASE_URL             默认 http://127.0.0.1:8080
TEST_DOMAIN          登录域，可为空
TEST_USERNAME        测试账号
TEST_PASSWORD_MD5    密码的 md5 十六进制
MODEL_ID             可用的 chat 模型 ID
KEAPI_API_KEY        keapi API Key
EXPECT_TEXT          写入文件的唯一 token，默认 golden-path-$(date +%s)
```

依赖：`curl`、`jq`、`sha256sum`。

**阶段与断言**

1. `health`：`GET $BASE_URL/healthz`，`.status == "ok"`。
2. `login`：`POST /v3/account.LoginByPassword`，body：

```json
{"cmd":"account.LoginByPassword","request":{"domain_name":"$TEST_DOMAIN","username":"$TEST_USERNAME","password":"$TEST_PASSWORD_MD5"}}
```

取 `(.response // .Response).jwt_token`，非空。

3. `create_forest`：`POST /v3/forest.CreateForest`：

```json
{"cmd":"forest.CreateForest","request":{"name":"golden-path-$RUN_ID","description":"phase0 golden path","public_scope":"private","forest_type":"file","data_source_type":"standard","data_source_subtype":"standard"}}
```

取 `(.Response // .response).forest_id`。

4. `preupload`：`POST /v3/forest.PreUploadFile`：

```json
{"cmd":"forest.PreUploadFile","request":{"forest_id":$FOREST_ID,"parent_id":0,"files":[{"filename":"golden.txt","hash":"sha256:$SHA256","size":$SIZE,"content_type":"text/plain","split_config":{"split_mode":"auto","chunk_size":512,"split_mark":["\n\n"],"split_overlap":0,"preprocessing_rules":{"remove_empty_line":true,"remove_url":false,"remove_email":false}}}]}}
```

取 `(.Response // .response).files[0].upload_id` 和 `upload_urls` 的第一个 value。若 `files[0].multipart.enabled == true`，脚本报错并提示“测试文件必须小于分片阈值”。

5. `upload`：`curl -fsS -X PUT --data-binary @$TMP_FILE "$UPLOAD_URL"`。
6. `callback`：`POST /v3/forest.UploadFileCallBack`：

```json
{"cmd":"forest.UploadFileCallBack","request":{"forest_id":$FOREST_ID,"upload_id":$UPLOAD_ID,"filename":"golden.txt","hash":"sha256:$SHA256","parts":[]}}
```

取 `(.Response // .response).file_id`。

7. `parse`：每 5s 轮询 `POST /v3/forest.GetFileInfo`，body `{"cmd":"forest.GetFileInfo","request":{"file_id":$FILE_ID}}`，最多 300s；断言 `(.Response // .response).parse_status == "success"` 且 `knowledge_status == "success"`。
8. `search`：`POST /v3/kesearch.ForestSearch`：

```json
{"cmd":"kesearch.ForestSearch","request":{"forest_id":$FOREST_ID,"text":"$EXPECT_TEXT","is_semantics":false}}
```

断言 `.code == 0` 且 `(.Response // .response).doc_search_result | length > 0`。

9. `chat`：`POST /v3/chat.SubmitChatQuestion`：

```json
{"cmd":"chat.SubmitChatQuestion","request":{"session_id":0,"model_id":$MODEL_ID,"base_type":"standard","resource_type":"forest","resource_ids":[$FOREST_ID],"resource_names":["golden-path"],"question":"$EXPECT_TEXT 是什么？"}}
```

断言 `.code == 0`，且 `(.Response // .response).answer` 非空。

10. `keapi_search`：`POST /v3/keapi.Search`，Header `Authorization: Bearer $KEAPI_API_KEY`：

```json
{"cmd":"keapi.Search","request":{"forest_ids":[$FOREST_ID],"query":"$EXPECT_TEXT"}}
```

断言 `.code == 0` 且 `(.response // .Response).doc_search_result | length > 0`。

11. `mcp`：`POST /v3/keapi/mcp`，Header `Authorization: Bearer $KEAPI_API_KEY`、`Accept: application/json, text/event-stream`，body：

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"golden-path","version":"1.0"}}}
```

断言 HTTP 200 且响应体包含 `"result"`；若响应头有 `Mcp-Session-Id`，再发 `tools/list` 并断言之。

**脚本行为**

- `set -euo pipefail`。
- `--dry-run`：打印 11 个 stage 名称和所需环境变量，退出 0，不请求网络。
- `--self-test`：不请求网络，验证 `resp` 同时接受 `.Response` 和 `.response`，并断言 stage 列表长度为 11。
- `--health-only`：只跑 stage 1。
- 默认 `CLEANUP=1`：`trap` 在退出时调用 `forest.DeleteForest`；`--no-cleanup` 关闭。
- 每个 stage 打印 `[golden-path] stage=<name> ok`。
- 失败时打印响应体并退出非 0。

**步骤**

- [ ] 1. 写 `--dry-run` 骨架与 stage 函数；先不实现网络调用。
- [ ] 2. 运行 `bash -n scripts/golden-path.sh` 和 `scripts/golden-path.sh --dry-run`，断言输出包含 `health`、`search`、`chat`、`mcp`。
- [ ] 3. 实现 `--self-test`：

```bash
resp() { jq -c '(.response // .Response)' <<<"$1"; }
assert_eq "$(resp '{"Response":{"x":1}}')" '{"x":1}'
assert_eq "$(resp '{"response":{"x":2}}')" '{"x":2}'
test "${#STAGES[@]}" -eq 11
```

运行 `scripts/golden-path.sh --self-test`，必须退出 0。

- [ ] 4. 实现 stage 1–11，使用辅助函数：

```bash
resp() { jq -c '(.response // .Response)' <<<"$1"; }
post() { curl -fsS -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d "$2" "$BASE_URL/v3/$1"; }
```

- [ ] 5. 在本地 `make dev-up` 后准备种子账号/模型/API Key，运行完整脚本；把输出保存到 `docs/architecture/phase0/golden-path-baseline.md`。**Phase 0 的完整 golden-path 跑在旧 `apps/corekg` 上**；新 `cmd/corekg` 此时只有 `/healthz`，只参与 stage 1。
- [ ] 6. 记录每个 stage 的耗时和依赖（MySQL/ES/Redis/NATS/LLM），供 Phase 1 对比。
- [ ] 7. Commit：`phase0: add golden path regression harness`

**验收**

- `--dry-run` 不依赖服务即可通过。
- `--self-test` 通过，证明 `.Response`/`.response` 都可解析。
- 完整脚本在本地 compose 上 11 个 stage 全绿；失败时能定位到具体 stage 和响应。
- 脚本不硬编码租户/账号；全部通过环境变量注入。

---

## Task 10：PG driver vendor spike

**Files**

- Create `docs/architecture/phase0/pg-driver-spike.md`

**Interfaces（接口）**

- Consumes: 无。
- Produces: `docs/architecture/phase0/pg-driver-spike.md` 中的版本与接入结论。

**目的**：确认 Phase 1 接入 PG 的依赖路径，不在主模块执行 `go get`。

**步骤**

- [ ] 1. 在 `/tmp/pgspike` 建临时 module：

```bash
mkdir -p /tmp/pgspike && cd /tmp/pgspike
go mod init pgspike
go get gorm.io/gorm@latest gorm.io/driver/postgres@latest github.com/jackc/pgx/v5@latest
```

- [ ] 2. 写 `main.go`，只编译不连接：

```go
package main

import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    _, _ = gorm.Open(postgres.Open("postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"), &gorm.Config{})
}
```

- [ ] 3. 运行 `go build ./...`，记录 `go.mod` 里 `gorm.io/driver/postgres`、`pgx/v5` 的版本。
- [ ] 4. 检查 `vendor/github.com/ygpkg/yg-go/dbtools/v2` 是否支持注册非 MySQL driver；记录结论：若只支持 `mysqldrv`，Phase 1 在 `internal/infra/db` 自建连接注册，不修改 vendor。
- [ ] 5. 写 `pg-driver-spike.md`：版本、依赖树、vendor 增量、`pgvector` 扩展安装方式、CGO 结论、回退方案（自建 `pgdrv` vs 上游补丁）。
- [ ] 6. Commit：`phase0: document pg driver vendor spike`

**验收**

- 临时 module 编译通过。
- 文档明确 Phase 1 是“自建连接注册”还是“给上游补 `pgdrv`”，并列出需要新增的 vendor 包。
- 主仓库 `go.mod`/`vendor` 不变。

---

## Task 11：`corekg migrate from-mysql --dry-run`

**Files**

- Create `cmd/corekg/migrate.go`
- Create `internal/migrate/plan.go`
- Create `internal/migrate/mysql.go`
- Create `internal/migrate/plan_test.go`
- Modify `cmd/corekg/main.go`

**Interfaces（接口）**

- Consumes: `database/sql`、MySQL driver、`CORekG_MYSQL_DSN`。
- Produces: `migrate.MapType(mysqlType string) string`、`migrate.BuildPlan(...) (*Plan, error)`、`corekg migrate from-mysql --dry-run`。

```go
package migrate

type Plan struct {
    GeneratedAt time.Time    `json:"generated_at"`
    Schemas     []SchemaPlan `json:"schemas"`
}

type SchemaPlan struct {
    Name   string      `json:"name"`
    Tables []TablePlan `json:"tables"`
}

type TablePlan struct {
    Name      string       `json:"name"`
    RowCount  int64        `json:"row_count"`
    Columns   []ColumnPlan `json:"columns"`
    Warnings  []string     `json:"warnings,omitempty"`
}

type ColumnPlan struct {
    Name       string `json:"name"`
    MySQLType  string `json:"mysql_type"`
    PGType     string `json:"pg_type"`
    Nullable   bool   `json:"nullable"`
    Default    string `json:"default,omitempty"`
    Extra      string `json:"extra,omitempty"`
}

func MapType(mysqlType string) string
func BuildPlan(ctx context.Context, db *sql.DB, schemas []string, exactCount bool) (*Plan, error)
```

**MapType 规则**

- `tinyint(1)` / `tinyint` / `tinyint unsigned` → `smallint`
- `smallint unsigned` → `integer`
- `int` / `int(11)` / `mediumint` → `integer`
- `int unsigned` / `mediumint unsigned` → `bigint`
- `bigint` / `bigint unsigned` → `bigint`（`bigint unsigned` 加 warning：迁移前校验最大值 ≤ 9223372036854775807）
- `longtext` / `mediumtext` / `text` / `tinytext` → `text`
- `datetime` / `datetime(3)` / `timestamp` → `timestamptz`
- `date` → `date`；`time` → `time`；`year` → `smallint`
- `json` → `jsonb`
- `blob` / `longblob` / `mediumblob` / `tinyblob` / `binary` / `varbinary` → `bytea`
- `double` → `double precision`；`float` → `real`
- `decimal(p,s)` → `numeric(p,s)`
- `varchar(n)` / `char(n)` → 原样
- `enum(...)` / `set(...)` → `text` + warning
- 未知类型 → 返回空字符串，并在 `TablePlan.Warnings` 记录

**命令**

```bash
corekg migrate from-mysql \
  --dsn 'user:pass@tcp(127.0.0.1:3306)/?parseTime=true' \
  --schemas core,account,knownow,chat,sale \
  --out /tmp/mysql-plan.json \
  --dry-run
```

`--dry-run` 默认 true；Phase 0 不提供 `--apply`，传了返回错误 `apply is not implemented in phase 0`。

**步骤**

- [ ] 1. 写 `MapType` 表驱动测试，覆盖上面每条规则和未知类型。
- [ ] 2. 运行 `go test ./internal/migrate`，确认失败。
- [ ] 3. 实现 `MapType`（先小写、去 ` unsigned` 后缀、提取括号参数）。
- [ ] 4. 实现 `BuildPlan`：
  - `information_schema.tables` 查表名；
  - `information_schema.columns` 查列名/`column_type`/`is_nullable`/`column_default`/`extra`；
  - `exactCount=false` 时用 `information_schema.tables.table_rows`，`exactCount=true` 时逐表 `SELECT COUNT(*)`；
  - 每列调用 `MapType`，空结果写 warning。
- [ ] 5. 实现 `migrate.go` cobra 子命令，挂到 `cmd/corekg/main.go`。
- [ ] 6. 写集成测试 `TestBuildPlanIntegration`：`dsn := os.Getenv("CORekG_MYSQL_DSN")`，为空则 `t.Skip`；断言 `len(plan.Schemas) >= 5` 且所有 `PGType != ""`。
- [ ] 7. 运行 `go test ./internal/migrate ./cmd/corekg`、`go build ./cmd/corekg`。
- [ ] 8. 用测试 MySQL 跑一次 dry-run，把脱敏结果保存为 `docs/architecture/phase0/mysql-inventory.json`（只保留 schema/table/column 类型，不含数据）。
- [ ] 9. Commit：`phase0: add mysql to postgres dry-run planner`

**验收**

- 不需要 PG 即可运行；只读 MySQL。
- 输出 JSON 可被 `jq` 消费；所有列都有 PG 类型或 warning。
- 不执行任何写操作。

---

## Task 12：MySQL→PG 迁移评估

**Files**

- Create `docs/architecture/phase0/pg-migration-assessment.md`
- Read `docs/architecture/phase0/mysql-inventory.json`

**Interfaces（接口）**

- Consumes: `docs/architecture/phase0/mysql-inventory.json`（Task 11）。
- Produces: `docs/architecture/phase0/pg-migration-assessment.md`。

**必需章节**

```markdown
# MySQL → PostgreSQL 迁移评估

## 1. 盘点范围
## 2. 表/行数/大小
## 3. 类型映射
## 4. 默认值、自增与 sequence
## 5. 索引、唯一约束与外键
## 6. 排序规则与大小写
## 7. JSON / 文本 / 二进制
## 8. 空串、NULL、零日期
## 9. 方言替换清单
## 10. 迁移工具决策
## 11. 校验方案
## 12. 风险与回滚
```

**必须写清的内容**

- 5 个 schema：`core`、`account`、`knownow`、`chat`、`sale`；单库多 schema 为默认，`mode: databases` 为备选。
- 218 个 GORM 类型 tag 的映射结果（引用 Task 7 表）。
- `FIND_IN_SET` 3 处、`REGEXP` 2 处已替换；`IFNULL`/`SHOW CREATE TABLE`/`mysqlplugin` 属外部数据源，保留。
- 自增主键：GORM AutoMigrate 建表后，ETL 结束用 `setval(pg_get_serial_sequence(...), max(id))`。
- 索引：MySQL 唯一索引对 NULL 的语义与 PG 不同；逐索引确认。
- 排序规则：MySQL `utf8mb4_*_ci` 大小写不敏感；PG 默认区分大小写；搜索改 `ILIKE`/`~*` 或 ICU collation。
- JSON：MySQL JSON → PG `jsonb`；GORM `serializer:json` 保持。
- 零日期：MySQL `0000-00-00` → `NULL`；空串与 NULL 逐列确认。
- 工具决策：自研 `corekg migrate from-mysql` 为主；`pgloader` 仅作对照/兜底；理由写清（schema 映射、类型钩子、断点、校验）。
- 校验：每表行数、主键 min/max、关键字段 checksum、抽样查询对比、ES 重建后召回对比。
- 风险：`bigint unsigned` 溢出、`tinyint(1)` 语义、collation 导致搜索结果变化、sequence 未重置、部分索引/表达式索引丢失。

**步骤**

- [ ] 1. 用 `jq` 统计 `mysql-inventory.json`：schema 数、表数、列数、warning 数。
- [ ] 2. 按章节写文档；每个结论引用 inventory 的具体表/列。
- [ ] 3. 检查必需章节：

```bash
for h in "盘点范围" "类型映射" "自增与 sequence" "排序规则" "迁移工具决策" "校验方案"; do
  grep -q "$h" docs/architecture/phase0/pg-migration-assessment.md
done
```

- [ ] 4. Commit：`phase0: document mysql to postgres migration assessment`

**验收**

- 文档能直接指导 Phase 4 的 ETL 实现。
- 不出现“TBD/待定”；不确定项写成“需要在 Phase 4 dry-run 验证 + 具体查询”。

---

## Task 13：一次性切换与回滚 runbook

**Files**

- Create `docs/architecture/phase0/cutover-runbook.md`

**Interfaces（接口）**

- Consumes: Task 9 的 golden-path 基线、Task 12 的迁移评估。
- Produces: `docs/architecture/phase0/cutover-runbook.md`。

**必需章节**

```markdown
# CoreKG 单体一次性切换 Runbook

## 1. 范围与原则
## 2. 切换前检查清单
## 3. 备份与归档
## 4. 切换步骤
## 5. 验证门禁
## 6. 回滚步骤
## 7. 数据丢失边界
## 8. 沟通与责任人
## 9. 时间表模板
```

**必须写清的内容**

- 一次性切换，不做长期双写；内部里程碑 Phase 0–4，只发布一次。
- 分支策略：`monolith-v2` 集成分支；切换窗口前冻结 `main` 合并。
- 备份：MySQL 全量 dump + binlog 位点；对象存储清单；ES 索引快照；PG 基线备份。
- 切换步骤：停写 → 最终增量 ETL → 校验 → 启动新 `corekg` → 灰度/全量流量 → 观察。
- 验证门禁：golden-path 11 阶段全绿、行数/主键/checksum、搜索召回、聊天 P95、MCP `tools/list`。
- 回滚：保留旧镜像 tag、旧 MySQL、旧 ES 索引 ≥ 7 天；DNS/网关切回；明确“切换后写入 PG 的数据在回滚时会丢失”，需要业务确认。
- 数据归档：dump 放对象存储，记录 SHA256、保留 90 天。
- 责任人：应用、DBA、SRE、产品各一名；Go/No-Go 会议。
- 时间表模板：T-7 备份演练、T-1 冻结、T0 切换、T+1 观察、T+7 清理。

**步骤**

- [ ] 1. 写 runbook，所有命令用占位符（`<MYSQL_DSN>`、`<PG_DSN>`、`<IMAGE_TAG>`），不写真实密码。
- [ ] 2. 检查必需章节：

```bash
for h in "切换前检查清单" "备份与归档" "切换步骤" "验证门禁" "回滚步骤" "数据丢失边界"; do
  grep -q "$h" docs/architecture/phase0/cutover-runbook.md
done
```

- [ ] 3. 自审一遍；如果团队有 DBA/SRE，请其 review 并在文档末尾记录结论；没有则记录“待切换前由 DBA/SRE 确认”的检查项，不阻塞 Phase 0。
- [ ] 4. Commit：`phase0: add one-shot cutover and rollback runbook`

**验收**

- 任何一个未参与项目的人可以按 runbook 执行切换和回滚。
- 明确回滚的数据丢失边界，不隐藏在脚注里。

---

## Self-Review

- **Spec coverage**：设计文档 §9 Phase 0 的 8 项均有任务覆盖：骨架（Task 1）、架构测试（Task 3）、Profile（Task 2）、PG 方言（Task 4–7）、迁移工具（Task 11）、golden-path（Task 9）、切换策略（Task 13）、路由盘点（Task 8）。PG driver spike（Task 10）补上设计文档提到的 vendor 阻塞。
- **Step scan**：每个代码任务都是“写失败测试 → 跑失败 → 实现 → 跑通过 → commit”；文档任务用必需章节检查。没有“handle edge cases”式空步骤。
- **Type consistency**：`ContainsCSV`、`RegexpInsensitive`、`Load`、`Validate`、`NewServer`、`MapType`、`BuildPlan`、`RewriteTag` 在任务间命名一致；baseline 常量在 Task 4 定义、Task 5–7 修改。
- **Review Focus**：五条风险分别由 Task 7、Task 5、Task 9、Task 7、Task 9 的测试覆盖，见上文。
- **Proportion**：计划约 1200 行，设计文档约 1000 行；计划以签名、测试断言和命令为主，没有粘贴业务实现。

## Phase 0 完成标准

- [ ] `make corekg` 产出 `bundles/corekg`；`server` 子命令 `/healthz` 通过。
- [ ] `make arch-test` 通过；新代码无 `apps/*` 反向依赖。
- [ ] `go test ./internal/arch -run TestPGDialectRatchet -v` 输出 `FIND_IN_SET=0 REGEXP=0 typeTags=0`。
- [ ] `scripts/inventory-routes.sh` 产出 ≥ 400 行 CSV；`route-decisions.md` 覆盖 Coze/workflow 删除规则。
- [ ] `scripts/golden-path.sh --dry-run` 通过；本地 compose 完整运行 11 阶段全绿并记录基线。
- [ ] `corekg migrate from-mysql --dry-run` 可在测试 MySQL 上产出完整 JSON；`mysql-inventory.json` 已脱敏提交。
- [ ] `pg-driver-spike.md` 给出 Phase 1 的依赖版本与接入方式。
- [ ] `pg-migration-assessment.md`、`cutover-runbook.md` 通过必需章节检查。
- [ ] 旧 `apps/*` 二进制仍可构建；未删除任何 Coze 代码/前端/数据。

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| 类型 tag 机械替换破坏 GORM 行为 | `RewriteTag` 单测 + 定向编译 + Phase 1 AutoMigrate 空库验证 |
| `tinyint(1)` 改 `boolean` 导致 `types.Bool` 扫描失败 | 统一改 `smallint`，保留 int32 语义 |
| `ON UPDATE CURRENT_TIMESTAMP` 残留 | codemod 删除，`pg_dialect_test` 可增加该模式为 0 基线 |
| 路由清单漏掉动态注册 | 脚本断言已知 action；Phase 1 用实际 `ListAllRouters` 对比 |
| golden-path 只测健康检查 | 脚本强制 11 个 stage，缺环境变量直接失败 |
| 迁移 dry-run 只读但连生产 | 只允许测试 DSN；文档禁止生产 DSN；命令不提供写操作 |
| 回滚丢数据 | runbook 明确边界，切换前业务签字确认 |
