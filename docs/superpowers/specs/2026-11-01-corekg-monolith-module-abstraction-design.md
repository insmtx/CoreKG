# CoreKG 单体化与模块接口化设计（基础梳理）

> 状态：设计基线（2026-11-01 决策已确认）
> 日期：2026-11-01
> 参照实现：`/root/go/src/lework`（`github.com/insmtx/Lework`）
> 关联现状文档：`docs/product-architecture-current-state.md`

---

## 0. 一句话目标

把当前"多 app、各自启动、包级网状耦合"的 CoreKG，收敛为**一个 composition root 的单体**；把**基础设施选型**从业务代码中抽成端口 + Profile/Capability 装配；同时**下线 Coze**，把会话引擎替换能力延后到下一阶段（opencode 本期不接）。

**成功判据：**

1. 一个二进制 `corekg`，通过子命令运行 `server` / `worker` / `init` / `migrate`；删除 `apps/*` 的独立 `cmd/` 入口。
2. 个人/一体机档位使用 **PostgreSQL + pgvector + 本地目录 + 进程内缓存 + DB 队列**，不依赖 Redis/NATS/ES/Nebula。
3. 企业档位使用 **PostgreSQL + ES 或专用向量库 + Redis + NATS + MinIO + Nebula**；**MySQL 退出运行期**，只作为一次性迁移源。
4. 新增一种 DB/向量/检索/模型/解析实现 = 实现一个接口 + 注册 + 配置，**不改调用方**。
5. Coze 全栈下线（`apps/workflow`、`opencoze` 库/桶、kechat/kecore 的 Coze 耦合）。
6. 一次性切换，不保留 `/v2` 兼容层；允许不兼容旧接口。

---

## 1. 目标与非目标

### 1.1 目标

| # | 目标 | 说明 |
|---|---|---|
| G1 | 单进程、单 composition root | 启动顺序、依赖装配、生命周期只在一个地方 |
| G2 | 基础设施端口化 | DB / 向量 / 关键词 / 对象存储 / 缓存 / 锁 / 队列 / LLM / Embedding / Rerank / 解析 |
| G3 | Profile + Capability 装配 | 个人 / 企业 / 一体机三档，能力可裁剪 |
| G4 | 业务模块契约化（Phase 3 起） | `KnowledgeService`、`SearchService`、`IdentityService`；`ChatEngine` 仅预留 |
| G5 | 依赖方向可强制检查 | 用 `go list` 测试守住分层，而不是靠约定 |
| G6 | 路由与部署收敛 | 只保留 `/v3`；删除 `/v2` 双挂载；一次性切换 |
| G7 | 数据库统一为 PostgreSQL | MySQL 退出运行期；方言、迁移脚本、模型类型全部对齐 PG |

### 1.2 非目标

- 不做微服务拆分、不引入 RPC/服务网格。
- 不做"运行时按租户热切换后端"（本期是**部署期选型**；多租户共享一套基础设施）。
- 不重写业务逻辑、不重做前端（只删 Coze 相关页面/入口）。
- **本期不接 opencode**；`ChatEngine` 只预留接口，不实现替换。
- 不保留独立 `kecore`/`kechat`/`keapi`/`account` 二进制。
- **不保留 MySQL 运行期支持**；不做 MySQL/PG 双写、不长期双跑。
- 不做长期双轨兼容；一次性迁移。

### 1.3 已确认的关键决策

| # | 决策 | 结论 |
|---|---|---|
| A1 | 选型时机 | 部署期选型，不做租户级热切换 |
| A2 | 统一数据库 | **PostgreSQL 作为唯一运行期数据库**（个人/一体机/企业）；MySQL 弃用，仅作一次性迁移源 |
| A3 | 向量分档 | 个人/一体机：**pgvector**；企业：**ES 或专用向量数据库**（Milvus/Qdrant 等） |
| A4 | CGO | 允许按需启用（镜像/构建可带 CGO） |
| A5 | 独立 app 入口 | 全部收敛为单二进制；只保留 `cmd/corekg` 与 `cmd/corekg-cli` |
| A6 | Coze | **全栈下线** |
| A7 | opencode | **下一阶段再考虑，本期不接** |
| A8 | 企业检索 | 元数据 PG；向量 ES/专用向量库；关键词 ES 或 PG FTS（由 Profile 决定） |
| A9 | 外部数据源/Excel | 保留为企业能力；长期规划接入 chat2db 类"智能问数" |
| A10 | Helm | 本仓库维护 |
| A11 | `/v2` 兼容 | 可以重开、可以不兼容；本期只保留 `/v3` |
| A12 | 迁移节奏 | 一次性切换 |
| A13 | 企业 NATS | 保留，但抽象为队列能力 |
| A14 | MySQL 数据 | 一次性迁移到 PG；迁移后 MySQL 仅保留只读快照用于回滚 |

---

## 2. 从 lework 借什么、不借什么

### 2.1 值得直接借鉴的 6 个模式

| 模式 | lework 证据 | 对 CoreKG 的价值 |
|---|---|---|
| **单一 composition root** | `backend/cmd/lework/{main,server,worker}.go`，cobra 子命令，生命周期只在 cmd | 取代 7 份重复的 `cmd/init.go` |
| **严格分层 + 可执行检查** | `AGENTS.md` 分层表；`cmd/lework/architecture_test.go` | 用测试守住 `api → service → infra`，禁止反向依赖 |
| **契约驱动、轻抽象** | `internal/api/contract/*.go`；明确"不搞 Clean/DDD/Repository" | 接口按能力定义，不给每个实体套 Repository |
| **Runtime 可替换** | `agent.Runtime` + `Registry`；`native/claude/codex/opencode` 四个 runtime | 为下一阶段"会话换 opencode"提供结构参考 |
| **基础设施适配器** | `internal/infra/{db,mq,filestore,providers}`；`pkg/messaging` 线协议 | DB/存储/消息可替换 |
| **AIO 打包** | `deployments/aio/`：Compose + Nginx 同源 + install/backup/restore/upgrade | 一体机交付模板 |

### 2.2 不照搬的 3 点

| lework 做法 | 为什么不照搬 |
|---|---|
| NATS 仍是 AIO 硬依赖 | 个人/一体机不用 NATS，改用 DB 队列；企业档保留 NATS，但抽象为队列能力 |
| 领域是 项目/任务/技能/工作区 | CoreKG 是知识库/RAG，领域模型不同，只借结构不借模型 |
| `types/` 放全部 GORM 模型 | CoreKG 模型有 50+ 张表、跨 5 个逻辑库，应按逻辑库拆分，共享类型才下沉 |

> PG 是**统一数据库**（所有档位），MySQL 退出运行期；向量按档位分流：轻量 pgvector，企业 ES/专用向量库。

### 2.3 一个关键判断

lework 的 AIO **并没有追求零依赖**：它把 PG/NATS/server/web/nginx 打进一台机器，减少的是**部署单元**，不是中间件数量。

对应到 CoreKG，按已确认决策：

- **一体机（AIO）**：PostgreSQL（含 pgvector）+ 本地目录 + 进程内缓存 + DB 队列；**不带** Redis/NATS/ES/Nebula；用一个 Compose 栈 + 单二进制。
- **个人版**：同上，单机 PG；允许能力裁剪（图谱/OCR/视频/Coze 等关闭）。
- **企业版**：PostgreSQL（元数据）+ ES 或专用向量库（向量）+ Redis + NATS + MinIO + Nebula；关键词可用 ES 或 PG FTS。MySQL 不再参与运行期。

---

## 3. 现状模块框架（抽象前）

### 3.1 当前结构的问题（按严重度）

| 问题 | 证据 | 后果 |
|---|---|---|
| 7 份重复 bootstrap | `apps/{corekg,kecore,kechat,account,keapi,kesearch,ketask}/cmd/init.go` | 初始化顺序敏感、能力开关分散、单体里重复初始化 |
| 业务模块互相 import | account→kecore(16)/kechat(8)；kechat→kecore(56)；kesearch→kecore(17)/kechat(20) | 无法独立替换任一模块 |
| handler 直连 model/DB/ES | kecore 内 40 处 handler 直接 `dbutil.*`，`qapair`/`fileparse` 直接查 ES | 端口无法在边界注入 |
| 全局单例 | `fs.Forest`、`essearch.embeddingConfig`、`chunk.escli`、`task.natsBridge`、`dbutil` 注册表 | 换实现必须改代码 |
| 检索无接口 | `RerankSearchQuestionChunk`、`GlobalSearch` 是具体函数 | 换向量库要改 kechat/kesearch/kecore 多处 |
| 任务强依赖 NATS | `PushTaskQueue` bridge 为 nil 直接报错；`CreateForestTask` 上抛 | 去掉 NATS 需要改任务创建语义 |
| 启动硬失败 | 各 app 缺 ES/Nebula 直接 fatal | 轻档位无法启动 |

### 3.2 各核心模块的现状骨架

| 模块 | 现状结构 | 已有抽象 | 主要耦合点 |
|---|---|---|---|
| **kecore** | `internal/apis/<ctl>` → `services/svc*` / 直接 `models/*`；`models/foresttype` 是事实模型层 | 11 个小接口（`ResourceScopeHandler`、`UsageProvider`、`FileConvertStrategy` 等） | `dbutil` 313 处、`fs.Forest` 34 处、ES 59 处、Nebula 139 处 |
| **kechat** | `internal/apis` → `services/svcchat` → `chat/wrapper` → `chat/modes/*` | `core.ChatMode` 单方法接口、`Printer`、Eino Tool | ES 历史、Redis SSE、直接构造 ChatModel、无 registry |
| **kesearch** | `internal/apis` → `services/svc*` → `models/{reranksearch,globalsearch,chunk}` | 仅 wrapper 结构体，无端口 | ES DSL 深绑、直接 import kecore/kechat 模型 |
| **account** | `internal/apis` → `services` / `models/accounttype` | `accountmds` 中间件 | 反向 import kecore/kechat；平台与业务混合 |
| **ketask** | `cmd/worker_*.go` 轮询 + `internal/jobs` 结果消费 | `task` 回调注册表 | NATS 启动硬依赖；只挂在 corekg 上提供 HTTP 拉取 |
| **keapi** | handler 直接 import 各模块 service | 无 | MCP 走 HTTP 自调用；权限校验绕开 API 级 |
| **corekg** | `app.go` 挂 7 个 Router + License + task 回调 | 无 | 只是"拼装"，不是 composition root |

### 3.3 现有可复用的扩展点（不要推倒重来）

| 现有机制 | 位置 | 复用方式 |
|---|---|---|
| `storage.Storager` 多驱动 | `vendor/.../yg-go/storage` | 直接作为 `ObjectStore` 端口 |
| `kellm` driver registry | `apps/kellm/drivers/driver.go` | 扩成通用 `LLMProvider` registry |
| `ChatMode` | `apps/kechat/chat/core/interface.go` | 作为 `ChatEngine` 的 `native` 实现 |
| `svcresource` handler registry | `apps/kecore/services/svcresource/handler_registry.go` | 作为模块注册表模板 |
| `task.RegisterCallBack` | `pkgs/task/task.go` | 作为任务处理器注册表 |
| `settings` 分组配置 | `vendor/.../settings` | 过渡期继续用；新增 Profile 文件优先 |
| `Eino` 工具/Agent | `pkgs/einotools`、`apps/einonodes` | 作为 `native` 引擎内部实现 |

---

## 4. 目标目录结构

```text
corekg/
├── cmd/
│   ├── corekg/                    # 唯一进程入口（cobra）
│   │   ├── main.go
│   │   ├── server.go              # corekg server
│   │   ├── worker.go              # corekg worker --types=...
│   │   ├── init.go                # corekg init（建库/索引/桶/默认配置）
│   │   ├── migrate.go             # corekg migrate
│   │   └── version.go
│   └── corekg-cli/                # 现有 clients/corekg-cli
├── internal/
│   ├── bootstrap/                 # 装配：初始化顺序、依赖容器、启动校验
│   ├── config/                    # 配置结构 + profile/capability manifest
│   ├── api/                       # 唯一 HTTP 面
│   │   ├── router.go
│   │   ├── middleware/
│   │   ├── dto/
│   │   ├── handler/<domain>/      # 从各 apps 的 internal/apis 迁入
│   │   └── external/              # keapi 外部 API + MCP
│   ├── identity/                  # account 域（用户/组织/RBAC/API Key）
│   ├── knowledge/                 # kecore 域（forest/file/chunk/graph/article/quota）
│   ├── chat/                      # kechat 域
│   │   ├── contract/              # ChatEngine / ChatRequest / ChatEvent（预留）
│   │   └── engine/native/         # 现有 ChatMode 封装
│   ├── search/                    # kesearch 域（混合检索编排）
│   ├── task/                      # 任务域（core_task + worker runners）
│   ├── app/                       # keapp 域（可选能力）
│   ├── billing/                   # kesale
│   ├── agent/                     # Eino 执行内核（对齐 lework agent/）
│   ├── module/                    # 模块注册表 + Capability 声明
│   └── infra/                     # 适配器实现
│       ├── db/                    # postgres（+ mysqlimport：仅一次性迁移）
│       ├── cache/
│       ├── lock/
│       ├── queue/                 # db | nats
│       ├── objectstore/           # localdir | minio | s3
│       ├── vector/                # pgvector | elasticsearch | milvus | qdrant
│       ├── keyword/               # postgres_fts | elasticsearch
│       ├── llm/
│       ├── embedding/
│       ├── rerank/
│       ├── parser/
│       └── graph/
├── pkg/
│   ├── contracts/                 # 跨模块接口与 DTO（无业务依赖）
│   ├── types/                     # 共享领域类型
│   ├── errs/                      # 统一错误码
│   └── event/                     # 事件/任务 wire contract
├── workers/                       # 非 Go worker（保留独立镜像）
│   ├── pipeline/                  # 现 apps/pipeline
│   └── worker-ts/                 # 现 apps/worker
├── contrib/                       # 可选卫星，不编进主二进制
│   ├── webfetch/
│   └── websearch/
├── scripts/
├── deployments/
│   ├── aio/                       # 一体机（PG + 单二进制 + Web + Nginx）
│   ├── enterprise/                # 企业 Compose + Helm
│   ├── personal/                  # 个人版
│   └── helm/                      # 本仓库维护
└── configs/
    ├── profiles/
    │   ├── personal.yaml
    │   ├── enterprise.yaml
    │   └── aio.yaml
    └── capabilities.yaml
```

> **不搬进主进程的**：`pipeline`（Python）、`worker`（TS）保持独立镜像/进程，通过契约与本体交互。Coze 已下线，不再有 `workflow` 目录。
>
> **Go 服务入口收敛**：本期只保留 `cmd/corekg`（服务端）与 `cmd/corekg-cli`（客户端）。`webfetch`/`websearch` 默认折叠为 `internal/agent/tools` 的 HTTP 客户端；若因网络隔离必须独立部署，才保留 `contrib/*` 的 cmd，并在 Profile 中声明为可选卫星。

**与 lework 目录的对应关系：**

| lework | CoreKG 目标 | 说明 |
|---|---|---|
| `backend/cmd/lework/{main,server,worker}.go` | `cmd/corekg/{main,server,worker,init,migrate}.go` | cobra 子命令 + 生命周期 |
| `backend/internal/api/{contract,dto,handler,middleware,router}` | `internal/api/*` + `pkg/contracts` | 契约下沉到 `pkg/contracts` |
| `backend/internal/service` | `internal/{identity,knowledge,chat,search,task,billing}` | 按领域拆，不用单一 service 目录 |
| `backend/internal/infra/{db,mq,filestore}` | `internal/infra/*` | 适配器实现 |
| `backend/agent/{runtime,executor,registry}.go` | `internal/agent/*` | Eino 执行内核 + 未来 Runtime |
| `backend/config` | `internal/config` | 配置结构 + Profile |
| `backend/types` | `internal/model/<db>` + `pkg/types` | GORM 实体按库拆；共享类型下沉 |
| `backend/pkg/{cache,event,messaging}` | `pkg/{contracts,event,errs}` | 公共契约 |
| `backend/internal/worker` | `cmd/corekg worker` + `internal/task/worker` | 内置 worker 子命令 |
| `deployments/aio` | `deployments/aio` | 一体机模板 |

---

## 5. 分层与依赖规则

| 层 | 路径 | 可以做 | 不可以做 |
|---|---|---|---|
| 进程入口 | `cmd/corekg/` | cobra、配置加载、依赖装配、信号/生命周期、`log.Fatal` | 业务逻辑 |
| HTTP 适配 | `internal/api/` | 路由、DTO、鉴权中间件、调用 contract | 直接操作 DB/ES、写业务规则 |
| 业务域 | `internal/{identity,knowledge,chat,search,task,billing,app}/` | 业务编排、调用端口 | import 其他域的 `internal` 实现 |
| 执行内核 | `internal/agent/` | Runtime/Executor/NodeEvent/Tool 契约 | 访问 DB/NATS、发业务事件 |
| 基础设施 | `internal/infra/*` | 实现端口、驱动适配 | import 业务域 |
| 公开契约 | `pkg/contracts/`、`pkg/types/` | 接口、DTO、错误码 | 依赖 `internal` 或 vendor 框架 |
| 共享契约 | `pkg/event/` | 任务/事件 wire struct | 业务逻辑 |

**共享内核（GORM 实体的归属）**

当前跨域耦合几乎都是**模型**，不是行为：`kechat` 19 个文件 import `kecore/models/foresttype`，`kesearch` 13 个文件 import `kechat/models/chattype`，`account` 同时 import `kecore`/`kechat` 的多个模型包。如果按"每个实体抽接口"来做，会是数月工作量，且与 §6.1"不做 Repository"冲突。

**决策（builder call）：沿用 lework 的共享内核，但按逻辑库拆分**：

```text
internal/model/
├── core/        # core_upload_files, core_task
├── account/     # user, company, employee, api_key...
├── knownow/     # forest, file, graph, article, quota...
├── chat/        # session, question, model, agent...
└── sale/        # order, payment
```

- GORM 实体、表名常量、纯数据方法放 `internal/model/<db>`。
- 域包（`internal/knowledge` 等）只放行为：service、handler、编排。
- 跨域允许 import `internal/model/*`，**禁止** import 其他域的 service/handler。
- Coze 相关模型随 `apps/workflow` 一并删除，不进入共享内核。

**强制规则（写成 `internal/arch` 测试）：**

1. `internal/infra/**` 不得 import `internal/{identity,knowledge,chat,search,task}`。
2. 域 A 不得 import 域 B 的 `internal` 子包（`internal/model` 除外）；跨域行为只能走 `pkg/contracts`。
3. `internal/api/**` 不得 import `internal/infra/**`。
4. `cmd/**` 是唯一允许同时 import 所有层的地方。
5. `pkg/**` 不得 import `internal/**`。

> 过渡期可以用"兼容包"重导出旧路径，但每次迁移必须在 3 个 PR 内删除，不做长期双路径。

---

## 6. 基础设施端口（Phase 1-2）

### 6.1 设计原则

1. **不做 Repository 层**（对齐 lework）：元数据仍然直接用 GORM；只抽象**连接/方言**，不抽象每个实体。
2. **端口按能力切，不按中间件切**：业务只知道"向量召回""关键词召回""重排"，不知道 ES/Postgres。
3. **接口小**：一个端口 3-6 个方法；组合放在域服务里。
4. **错误统一**：端口返回 `pkg/errs` 的可判别错误，不泄漏驱动错误。
5. **注册表 + 工厂**：新增实现只改 composition root。

### 6.2 端口清单

#### DB

```go
// internal/infra/db
type Provider interface {
    Core(ctx context.Context) *gorm.DB
    Account(ctx context.Context) *gorm.DB
    Knownow(ctx context.Context) *gorm.DB
    Chat(ctx context.Context) *gorm.DB
    Sale(ctx context.Context) *gorm.DB
    Dialect() Dialect // postgres
}

type Dialect interface {
    Name() string
    SupportsSkipLocked() bool // PG: true
    SupportsRegexp() bool     // PG: true（~ / ~*）
    // 方言差异集中在这里，业务代码禁止拼 MySQL 专有 SQL
}
```

- 现状锚点：`pkgs/utils/dbutil/mysql.go`、`database_conns`。
- **目标态运行期只注册 `db/postgres`**。Phase 1-3 过渡期可保留旧 `db/mysql` adapter 给未迁移路径，但新代码只能通过 `DBProvider`，不得直接 import MySQL 驱动。`db/mysqlimport` 仅供一次性迁移命令使用，不进入 `server` 装配。
- **不做**：`UserRepository`、`ForestRepository` 这类 god interface。
- **范围**：`DBProvider` 只覆盖 5 个系统库（core/account/knownow/chat/sale）。按租户的外部数据源（`ForestDBInstance`/`ForestDB`/`ForestTable`/`CompanyDB`，见 `apps/kecore/models/foresttype`）作为独立 `external_datasource` capability，**不进入**该端口；个人档关闭，企业档保留。
- **会话模型/Agent 配置**统一存主库的 `chat` 逻辑库；Coze 下线后不再有 `opencoze` 库。
- **前置依赖（当前仓库不具备）**：`gorm.io/driver/postgres` 未 vendor，`dbtools/v2` 只有 `mysqldrv`。Phase 1 必须二选一：
  - 在核心仓实现自己的 PG 连接注册（推荐，避免等上游）；
  - 或给 `yg-go/dbtools/v2` 补 `pgdrv` 并升级依赖。
  无论哪条，都要执行 `go mod tidy && go mod vendor` 并提交 `vendor/`。

**MySQL→PG 方言盘点（已核对当前代码）：**

| 构造 | 命中 | 位置 | PG 处理 |
|---|---|---|---|
| `FIND_IN_SET` | 3 | `kecore/models/forest/forest.go:946`、`kesearch/models/globalsearch/forest.go:87/89` | 改用 PG 数组列 + `= ANY(...)`；过渡期用 `?::text = ANY(string_to_array(manager_ids, ','))` |
| `REGEXP` | 2 | `kesearch/models/globalsearch/{agent,forest}.go` | 改 `~` / `~*` |
| `IFNULL` | 3 | `pkgs/plugins/dbplugins/mysqlplugin/mysql.go` | **外部数据源插件（租户 MySQL），不是系统库**；保留，不改为 PG |
| `SHOW CREATE TABLE` | 4 | `kecore/models/keqa/qa_excel.go`、`kechat/models/qachat/{excel_chat,mysql_chat}.go`、`kechat/models/excelchat/qa_excel.go` | **外部数据源查询，不是系统库**；由 `ExternalDataSource` 按方言处理，Phase 0 不改 |
| MySQL 类型 tag | 218 | `type:tinyint` / `tinyint(1)` / `bigint unsigned` / `int unsigned` / `longtext` / `mediumtext` / `datetime` / `datetime(3)` / `int(11)` / `uint` / `TINYINT` | 归一化为 `smallint` / `bigint` / `integer` / `text` / `timestamptz`；`tinyint(1)` 保持 `smallint`（`types.Bool` 是 int32）；Phase 0/1 全量替换 |
| GORM `SKIP LOCKED` | 1 | `pkgs/task/crud.go:43` | PG 原生支持，保留 |
| Raw SQL 反引号 | 6118（含 struct tag） | 全仓 | 只审计 `Raw/Exec` 字符串；改为双引号或 GORM builder |

**迁移策略（一次性，Phase 4 执行）：**

1. **新建库用 GORM AutoMigrate**：`corekg migrate` 在空 PG 上建表；扩展/索引（pgvector、FTS、部分索引）放 `scripts/postgres/`，因为 AutoMigrate 表达不了。
2. **存量 MySQL 用离线 ETL**：`corekg migrate from-mysql --dsn ... --dry-run`，逐表搬迁（按外键顺序），统一处理：
   - `tinyint(1)` → `smallint`（不要用 `boolean`，`types.Bool` 是 int32）；`datetime`/`datetime(3)` → `timestamptz`；`unsigned` → 去掉并校验范围；`longtext/mediumtext` → `text`；`int(11)` → `integer`；`uint` → `bigint`；`TINYINT` → `smallint`；
   - JSON 字符串保持 `jsonb` 或 `text`（按查询需求定）；
   - 自增主键迁移后重置 sequence；
   - 空字符串与 `NULL` 的语义差异逐列确认。
3. **校验**：每表行数、主键范围、关键字段 checksum；抽样业务查询对比；ES 索引重建后再切流量。
4. **切换**：停写 → 追平增量 → 切 PG → 保留 MySQL 只读快照（回滚用）→ 观察期后下线。
5. **旧脚本**：`scripts/mysql/` 冻结，不再新增；新变更只写 `scripts/postgres/`。已发布 MySQL 脚本不修改。

#### 对象存储

```go
type ObjectStore = storage.Storager
```

- 已有接口与 localdir/minio/s3/cos 驱动，直接复用。
- 只把 `fs.Forest` 全局改为注入。

#### 向量 / 关键词

```go
type Chunk struct {
    ID        string
    ForestID  uint
    FileID    uint
    Content   string
    Embedding []float32
    Type      string
    Sequence  int
    Metadata  map[string]string
}

type VectorIndex interface {
    Upsert(ctx context.Context, index string, chunks []Chunk) error
    DeleteByFile(ctx context.Context, index string, forestID, fileID uint) error
    Search(ctx context.Context, req VectorSearchRequest) ([]ChunkHit, error)
    Health(ctx context.Context) error
}

type KeywordIndex interface {
    Upsert(ctx context.Context, index string, chunks []Chunk) error
    DeleteByFile(ctx context.Context, index string, forestID, fileID uint) error
    Search(ctx context.Context, req KeywordSearchRequest) ([]ChunkHit, error)
    Aggregate(ctx context.Context, req AggregateRequest) (AggregateResult, error)
}
```

- **向量/关键词分档：**

| 档位 | `vector.driver` | `keyword.driver` | 说明 |
|---|---|---|---|
| 个人 / 一体机 | `pgvector` | `postgres_fts` | 只依赖 PG；中文分词用 `zhparser` 或应用侧分词 |
| 企业（默认） | `elasticsearch` | `elasticsearch` | 沿用现有 ES 混合检索，行为最接近现状 |
| 企业（大库/专用） | `milvus` / `qdrant` | `postgres_fts` 或 `elasticsearch` | 专用向量库只负责向量；关键词不强制依赖它 |

- **检索编排**（邻居扩展、rerank、阈值、按文件聚合）放在 `internal/search`，不放适配器。
- Postgres 侧需要处理：`pgvector` 扩展、FTS 分词（中文需 `zhparser` 或应用侧分词）、索引名映射（单库内用 `forest_id` 分区/过滤，不再有 `ke_<id>` 索引概念）。
- 适配器统一用 `namespace`（如 `forest_<id>`）而不是 ES index 概念；ES 适配器映射到 index，pgvector 映射到分区/过滤列，Milvus/Qdrant 映射到 collection。
- **前置依赖**：`pgvector-go` 未 vendor；可选用原生 SQL（`<=>`/`<->`）+ `database/sql` 避免新驱动，或在 Phase 2 引入并 vendor。PG 镜像必须预装 `vector` 扩展（AIO/个人版镜像负责）。
- `corekg init` 负责 `CREATE EXTENSION IF NOT EXISTS vector` 与 FTS 配置；扩展不可用时 pgvector 驱动**启动失败并明确提示**，不静默降级为全表扫描。
- 专用向量库适配器（Milvus/Qdrant）作为 Phase 3 可选项，不阻塞 Phase 2；接口先按最小方法集固定。

#### Embedding / Rerank / LLM

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
}

type Reranker interface {
    Rerank(ctx context.Context, query string, docs []string) ([]float64, error)
}

type ChatModelFactory interface {
    New(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error)
}
```

- 现状锚点：`essearch.GetEmbedding`、`reranksearch.GetRerank`、`modelhelper.NewToolCallingChatModel`、`kellm` driver registry。
- 个人档：远程 API（默认）或本地 Ollama/ONNX（可选能力）。
- 嵌入维度必须显式校验：切换 embedding 模型时，维度变化要让启动失败，而不是写出脏向量。

#### 缓存 / 锁 / 流

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
    Delete(ctx context.Context, keys ...string) error
}

type Locker interface {
    Lock(ctx context.Context, key string, ttl time.Duration) (UnlockFunc, error)
}

type StreamStore interface {   // SSE 停止/回放
    Append(ctx context.Context, streamID string, event []byte, ttl time.Duration) error
    Range(ctx context.Context, streamID string, from int64) ([][]byte, error)
    Stop(ctx context.Context, streamID string) error
    Stopped(ctx context.Context, streamID string) (bool, error)
}
```

- 企业档：Redis；个人档：进程内 map + 定期清理；多副本需求出现时再上 DB 表。
- 当前所有调用点必须替换 `redispool.Redis()` 直连。

#### 任务队列

```go
type TaskQueue interface {
    Enqueue(ctx context.Context, taskID uint, taskType string) error // 可选推送
    Kind() string                                                     // nats | db | memory
}
```

- **默认实现 `db`**：只写 `core_task`，worker 走 HTTP 拉取。NATS 变为可选加速器。
- 这是去掉 NATS 的最小改动：`PushTaskQueue` 不再是错误源，`CreateTask` 不再因无 bridge 失败。
- 企业档保留 `nats`，但语义降级为"通知"，事实源仍是 `core_task`。

#### 会话历史 / 模型配置

```go
type HistoryStore interface {
    Append(ctx context.Context, q *QuestionRecord) error
    Update(ctx context.Context, q *QuestionRecord) error
    Get(ctx context.Context, requestID string) (*QuestionRecord, error)
    ListBySession(ctx context.Context, sessionID string, limit int) ([]QuestionRecord, error)
    Stats(ctx context.Context, filter StatsFilter) (StatsResult, error)
}

type ModelRegistry interface {
    List(ctx context.Context, companyID uint) ([]ModelSpec, error)
    Get(ctx context.Context, id uint) (*ModelSpec, error)
    ResolveDefault(ctx context.Context, companyID uint, role string) (*ModelSpec, error)
}
```

- 现状：聊天历史在 ES `ke_chat_history`（`chatquestion`），模型配置读 `chat` 库 + Coze 同步。
- 企业档：`HistoryStore = elasticsearch`（现有）或 PostgreSQL（简化档），`ModelRegistry = postgres`。
- 个人档：两者都用 PostgreSQL；**不再需要 opencoze 库**。
- 这是个人档能启动聊天的前提，必须在 Phase 1 完成（见 §9 修正）。

#### 文档解析

```go
type Parser interface {
    Parse(ctx context.Context, req ParseRequest) (ParseResult, error)
    Supports(mime string) bool
}
```

- 现状锚点：`fileconvert`、`decoupler`、`pipeline` 的 MinerU 调用。
- 个人档：纯文本/Markdown 直读；Office 用轻量转换；PDF 可选远程解析。
- 企业档：MinerU + GPU worker（现状）。

#### 图存储

```go
type GraphStore interface {
    EnsureSpace(ctx context.Context, space string) error
    UpsertNodes/Edges(...)
    Search(ctx context.Context, req GraphSearchRequest) (GraphResult, error)
    DropSpace(ctx context.Context, space string) error
}
```

- 企业档：Nebula；个人档：关闭能力或退化为 PostgreSQL 邻接表（仅小图）。

#### 外部数据源（企业能力，不进 `DBProvider`）

```go
type ExternalDataSource interface {
    Test(ctx context.Context, ds DataSourceSpec) error
    ListTables(ctx context.Context, ds DataSourceSpec) ([]TableSpec, error)
    Query(ctx context.Context, ds DataSourceSpec, query QuerySpec) (QueryResult, error)
    Schema(ctx context.Context, ds DataSourceSpec) (SchemaSpec, error)
}
```

- 现状：`ForestDBInstance`/`ForestDB`/`ForestTable`/`CompanyDB`（`apps/kecore/models/foresttype`），支持外部 MySQL/PG/Excel 入库。
- 个人档关闭；企业档保留。**不并入** `DBProvider`，避免系统库方言被租户数据源污染。
- 长期扩展点：在其上做 chat2db 类"智能问数"（NL → SQL），作为独立 capability，不改变本期范围。

### 6.3 装配：Profile + Capability

```yaml
# configs/profiles/personal.yaml
profile: personal
capabilities:
  knowledge: true
  chat: true
  search: true
  graph: false
  ocr: false
  video: false
  writing: false
  mcp: true
  billing: false
  external_datasource: false
infra:
  db:
    driver: postgres
    url: "postgres://corekg:...@localhost:5432/corekg?sslmode=disable"
    mode: schemas          # 单库 + 5 schema；或 databases（5 个独立库）
  vector:    { driver: pgvector }
  keyword:   { driver: postgres_fts }
  object:    { driver: localdir, root: "./data/files" }
  cache:     { driver: memory }
  lock:      { driver: memory }
  stream:    { driver: memory }
  queue:     { driver: db }
  embedding: { driver: openai, base_url: "...", model: "..." }
  rerank:    { driver: http, base_url: "..." }
  llm:       { driver: openai }
  parser:    { driver: builtin }
  graph:     { driver: none }
```

```yaml
# configs/profiles/enterprise.yaml
profile: enterprise
capabilities: { knowledge: true, chat: true, search: true, graph: true, ocr: true, video: true, writing: true, mcp: true, billing: true, external_datasource: true }
infra:
  db:
    driver: postgres
    url: "postgres://corekg:...@pg:5432/corekg?sslmode=disable"
    mode: schemas          # 推荐；也支持 5 个独立 PG 库
  vector:    { driver: elasticsearch, url: "...", username: "...", password: "..." }
  # 大库可改为：vector: { driver: milvus, addr: "...", collection_prefix: "corekg_" }
  keyword:   { driver: elasticsearch, url: "..." }   # 若向量走专用库，可改为 postgres_fts
  object:    { driver: minio, endpoint: "...", bucket: "..." }
  cache:     { driver: redis, addr: "..." }
  lock:      { driver: redis }
  stream:    { driver: redis }
  queue:     { driver: nats, url: "nats://..." }
  embedding: { driver: http, base_url: "...", model: "..." }
  rerank:    { driver: http, base_url: "..." }
  llm:       { driver: openai }
  parser:    { driver: mineru, base_url: "..." }
  graph:     { driver: nebula, addr: "..." }
```

> `aio.yaml` 与 `personal.yaml` 基本一致，差异只在部署编排（Compose + Nginx + Web），不在能力/驱动。
>
> **逻辑库映射**：推荐单 PG 实例 + 单数据库 + 5 个 schema（`core`/`account`/`knownow`/`chat`/`sale`），每个逻辑连接通过 DSN/连接参数设置 `search_path`；也支持 5 个独立 PG 库（`mode: databases`）。前者备份/迁移只面对一个库，后者隔离更强。MySQL 的 5 库结构不再保留。

**启动校验规则：**

1. Profile 引用的每个 `driver` 必须已注册，否则启动失败（fail-fast）。
2. Capability 为 true 时，其依赖端口必须可用；不可用则失败并给出明确提示。
3. Capability 为 false 时，路由不注册、worker 不启动、后台循环不跑。
4. 允许 `required: false` 的降级声明（例如 websearch 不可用时工具直接不可见）。

> 这替代当前散落的 `ENABLE_NEBULA_GRAPH`、`workflow.enabled`、`DEPLOY_MODE`、`main.env` 判断。

### 6.4 配置权威顺序（必须明确）

现在的配置有三层，且互相覆盖：`conf/*.yaml` → PostgreSQL `core_settings`（`settings.GetYaml`，TTL 5 分钟）→ 环境变量/ldflags（`DEPLOY_MODE`、`ENABLE_NEBULA_GRAPH`）。同一项可能在三处出现。

**决策（builder call）：按用途分层，而不是按来源分优先级。**

| 配置类型 | 权威来源 | 例子 |
|---|---|---|
| **基础设施连接** | Profile YAML（部署期） | db/vector/keyword/cache/queue/object/parser 的 driver 与地址 |
| **能力开关** | Profile YAML + Capability 清单 | graph/ocr/video/billing/external_datasource 是否启用 |
| **业务可调参数** | DB `settings`（运行期） | 检索 topn/threshold、提示词、配额、公告 |
| **密钥** | Profile 引用的环境变量或 secret 文件 | DB 密码、API Key |

规则：

1. `corekg config validate -f profile.yaml` 只读文件，不依赖 DB。
2. 启动时先加载 Profile，建立"驱动注册表"，再连 DB；DB 不可用时要能给出明确错误，而不是"设置表缺失"。
3. `settings` 只保留业务参数；基础设施项即使存在也**不再读取**。
4. `DEPLOY_MODE` 降级为 `profile.edition`（builtin/enterprise/private），不再作为散落的运行时开关。
5. 提供一次性命令 `corekg config import-settings`，把旧 `core_settings` 中的基础设施项迁进 Profile，供存量客户升级。

### 6.5 Coze 全栈下线

**决策：** Coze 不再是产品的一部分；本期一次性删除，不保留兼容开关。

**必须清理的耦合点（已核对代码）：**

| 位置 | 内容 | 处理 |
|---|---|---|
| `apps/workflow/` | vendored Coze Studio 后端（约 70 万行 Go） | 删除；`go.mod` 清理独占依赖 |
| `frontend/workflow/` | Coze UI monorepo | 删除 |
| `apps/kechat/models/coze/` | Coze 消息/映射模型 | 删除，消息改走 `HistoryStore` |
| `apps/kechat/mds/cozemds.go` | Coze Bearer 鉴权 | 删除 |
| `apps/kechat/services/svccoze/` | 空间/成员同步 | 删除 |
| `apps/kechat/services/svcmodel/llm_model.go` | 模型 Create/Update 无条件调用 Coze 同步 | **先解耦**，再删 |
| `apps/kechat/models/chat/coze_model_instance.go` | 模型实例跨库写入 | 删除，模型统一存 `chat` 库 |
| `apps/kechat/internal/apis/*` | Coze 插件/映射/回调路由 | 删除并更新 action 清单 |
| `apps/kecore/services/svccoze/`、`models/coze/` | 绑定/同步 | 删除 |
| `apps/kecore` 路由 `forest.BindCozeModel` 等 | Coze 绑定 | 删除 |
| `apps/account` | `employee_manage.go` 引用 `svccoze`；`chatagent` 模型 | 解耦后删除 |
| `apps/keinit` | `CreateCozeBucket`、`--coze-sql-dir`、`coze` DB 配置、`migrator/cozemodelsync.go` | 删除 |
| `scripts/mysqlcoze/` | Coze 建表脚本 | 删除 |
| `docker-compose.yml` | `coze-static-loader`、workflow 服务、`opencoze` 桶 | 删除 |
| `resource/`、`docs/` | Coze 静态资源与文档 | 清理 |

**执行顺序（避免双库写坏数据）：**

0. 先确认耦合面：主模块中只有 `apps/corekg/cmd/{main,init}.go` 直接 import `apps/workflow`；其余耦合集中在 kechat/kecore 的 Coze service/model。删除顺序如下。

1. **解耦模型管理**：`chat_model` 的增删改不再触发 Coze 同步；删除 `coze_model_instance` 读写。
2. 删除 kechat/kecore/account 的 Coze 路由与 service；前端去掉 `/coze` 入口与 iframe。
3. 删除 `apps/workflow`、`frontend/workflow`、Coze 镜像/Compose 服务/静态资源。
4. 数据侧一次性迁移：导出并归档 `opencoze` 库与桶，然后 drop；清理 `core_settings` 中的 Coze 键。
5. 回归：模型 CRUD、Agent CRUD、知识库问答、开放 API（确认没有隐藏依赖）。

**Exit：** `rg -i 'coze'` 在 `apps/`、`frontend/corekg/`、`deployments/` 中为 0（历史文档与 CHANGELOG 除外）；企业档/个人档均可启动。

---

## 7. 业务模块契约（Phase 3，ChatEngine 仅预留）

### 7.1 契约边界

| 契约 | 输入 | 输出 | 当前实现 | 可替换场景 |
|---|---|---|---|---|
| `KnowledgeService` | forest/file/tree/parse 命令 | 领域 DTO | kecore | 换存储后端、裁剪能力 |
| `SearchService` | `SearchRequest` | `SearchResult` | kesearch | 换检索实现 |
| `ChatEngine` | `ChatRequest` | `ChatEvent` 流 | kechat native | 下一阶段换 opencode（本期不实现） |
| `IdentityService` | 认证/授权请求 | 身份上下文 | account | 对接企业 IAM |
| `TaskService` | 任务命令 | 任务状态 | pkgs/task + ketask | 换队列/调度 |

### 7.2 ChatEngine 契约（下一阶段设计输入，本期不实现）

参考 lework 的 `agent.Runtime`（`ExecutionRequest` / `ExecutionResult` / `NodeObserver`）与 CoreKG 现有的 `ChatContext` / `ChatResult` / `Printer`：

```go
// pkg/contracts/chat.go
type ChatEngine interface {
    Run(ctx context.Context, req ChatRequest) (ChatStream, error)
    Stop(ctx context.Context, requestID string) error
    Resume(ctx context.Context, requestID string, fromEventID string) (ChatStream, error)
    Capabilities() EngineCapabilities
}

type ChatStream interface {
    Next(ctx context.Context) (ChatEvent, error) // io.EOF 表示结束
    Close() error
}

type ChatEvent struct {
    ID        string
    Type      EventType // delta | reasoning | tool_call | tool_result | reference | usage | done | error
    Content   string
    Data      any
    CreatedAt time.Time
}

type ChatRequest struct {
    RequestID   string
    SessionID   string
    UIN         uint
    CompanyID   uint
    Engine      string        // native | opencode
    Model       ModelSpec
    Messages    []Message     // 历史 + 本轮
    Attachments []Attachment
    Tools       []ToolSpec
    Retrieval   RetrievalScope
    Prompt      PromptSpec
    Options     ChatOptions   // enable_reference / enable_web_search / stop key...
}

type EngineCapabilities struct {
    Streaming   bool
    ToolCalls   bool
    References  bool
    Resume      bool
    Multimodal  bool
}
```

**必须补齐的现状缺口：**

1. 现在 `ChatResult` 只有最终答案/用量/引用，**没有 tool_call 事件流**；要支持 opencode 必须补事件。
2. 现在引用只在结束时返回；流式引用依赖 `MsgTypeKnowledgeSearch`，要纳入 `ChatEvent.Reference`。
3. 现在 stop/replay 依赖 Redis；由 `StreamStore` 端口承接，引擎不直接碰 Redis。
4. 现在每个 mode 各自拷贝 usage；统一由引擎返回 `usage` 事件。

**opencode 接入方式（下一阶段再做）：**

- 本期只在 `internal/chat/engine/native` 封装现有 `ChatMode`，并保留 `RegisterEngine(name, factory)` 注册点。
- 下一阶段新增 `internal/chat/engine/opencode/`，内部通过 ACP 调 opencode 可执行文件（参考 `lework/backend/agent/runtime/opencode/acp_invoker.go`）。
- 切换方式：`capabilities.chat.engine: opencode`；不可用时降级到 `native`。
- **本期验收不包含 opencode**；不得因预留接口而增加运行期分支。

### 7.3 模块注册表

```go
// internal/module
type Module interface {
    Name() string
    Capabilities() []string
    RegisterRoutes(r *api.Router)
    Migrate(ctx context.Context, db *gorm.DB) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

- 每个域一个 `Module`，替代 `apps/*/app.go` 的 `Routers/Migrates/RunJob`。
- `bootstrap` 读取 Profile，按 Capability 过滤模块，再按依赖顺序 `Migrate → Start`。
- `Stop` 逆序执行，统一处理退出。

---

## 8. 现状 → 目标 迁移映射

| 现路径 | 目标路径 | 处理方式 |
|---|---|---|
| `apps/corekg/*` | `cmd/corekg` + `internal/bootstrap` | 重写为 composition root |
| `apps/kecore/internal/apis/*` | `internal/api/handler/*` | 按域拆分迁入 |
| `apps/kecore/{services,models}` | `internal/knowledge/*` | 保留业务代码，改依赖注入 |
| `apps/kechat/chat/*` | `internal/chat/*` | `ChatMode` → `ChatEngine/native` |
| `apps/kechat/{internal,models}` | `internal/api/handler/chat` + `internal/chat` | 拆分 |
| `apps/kesearch/*` | `internal/search` + `internal/infra/{vector,keyword}` | 检索编排与适配器分离 |
| `apps/account/*` | `internal/identity` | 去掉对 kecore/kechat 的反向 import |
| `apps/keapi/*` | `internal/api/external` + `internal/api/external/mcp` | MCP 改为进程内调用 |
| `apps/ketask/*` | `cmd/corekg/worker.go` + `internal/task/worker` | 变成子命令 |
| `apps/kesale/*` | `internal/billing` | 保持库形态 |
| `apps/keapp/*` | `internal/app`（可选模块） | 能力开关控制 |
| `apps/kellm/*` | `internal/infra/llm/gateway` | 保留 driver registry |
| `apps/nodes`、`apps/einonodes`、`pkgs/einotools` | `internal/agent` | 合并为执行内核 |
| `apps/keinit/*` | `cmd/corekg/init.go` + `internal/bootstrap` | CLI 子命令 |
| `apps/workflow/*` | — | **删除**（Coze 下线） |
| `frontend/workflow/*` | — | **删除**（Coze UI） |
| `apps/pipeline/*` | `workers/pipeline` | 独立镜像 |
| `apps/worker/*` | `workers/worker-ts` | 独立镜像 |
| `apps/webfetch`、`apps/websearch` | `internal/agent/tools`（默认）或 `contrib/*` | 优先折叠；需要网络隔离才保留独立进程 |
| `clients/corekg-cli` | `cmd/corekg-cli` | 独立二进制 |
| `pkgs/utils/dbutil/*`、`database_conns` | `internal/infra/db` | 改为 PG；去掉全局 `dbutil.X()` |
| `scripts/mysql/*` | `scripts/postgres/*` | 冻结旧脚本；新脚本用 PG 方言；数据迁移走 `corekg migrate from-mysql` |
| `pkgs/plugins/dbplugins/mysqlplugin` | 保留 | **外部数据源**（租户 MySQL）插件，不属于系统库；不随系统库切换删除 |
| `apps/*/models` 的 `type:tinyint/unsigned/longtext/datetime(3)` | 同路径 | 归一化为 PG 可移植类型（218 处，不含 `apps/workflow`；Phase 0/1 完成） |

---

## 9. 迁移路线（内部里程碑，一次性发布切换）

> 已确认 A12：不搞长期双轨、不做灰度共存。最终**一次发布**切换；旧 `apps/*` 在新单体通过全部门禁前保持可构建，切换后删除。数据库侧是一次性 MySQL→PG 迁移，切换后 MySQL 只留只读快照。

### Phase 0：盘点、骨架与迁移预研（2-3 周）

> 可执行任务见 `docs/superpowers/plans/2026-11-01-phase0-foundation.md`。

- 盘点 action/URL/SSE/前端调用，产出"保留 / 删除 / 重开"清单（允许不兼容，但必须可追溯）。
- 新建 `cmd/corekg` 与 `internal/bootstrap` 骨架，能启动空 HTTP + 健康检查。
- 建立 `internal/arch` 依赖测试（先允许旧路径，逐步收紧）。
- 产出 `configs/profiles/{personal,enterprise,aio}.yaml` 草案。
- **PG 方言盘点**：按 §6.2 清单逐项改（`FIND_IN_SET`/`REGEXP`/类型 tag），先做机械替换与单测；`IFNULL`/`SHOW CREATE TABLE`/`mysqlplugin` 属外部数据源，不在本期改。
- **迁移工具预研**：对比 `pgloader` 与自研 `corekg migrate from-mysql`；确定类型映射、序列重置、JSON/时间/布尔处理。
- **建立 golden-path 回归脚本**：`init → 建库 → 上传 → 解析 → 切块 → 检索 → RAG → 开放 API → MCP`，跑在本仓 `docker-compose`（enterprise profile），不依赖外部 staging。
- 确定一次性切换的分支策略、回滚方式、数据归档范围（Coze `opencoze` 库/桶、MySQL 快照）。

**Exit：** `go build ./cmd/corekg` 成功；旧 `apps/*` 仍可独立构建；golden-path 基线在改造前跑通并留档；PG 方言替换清单闭环。

### Phase 1：Coze 下线 + 单体装配 + PG 接入（4-5 周）

- **先解耦 Coze**：按 §6.5 顺序执行，模型管理不再写 `opencoze`；删除 Coze 路由/service/前端入口。
- 合并 7 份 bootstrap 为一份；启动顺序集中声明。
- 实现 `DBProvider`（PG 为唯一目标；过渡期保留 MySQL adapter 给旧路径，新代码禁止使用）、`Cache`、`Locker`、`StreamStore`、`TaskQueue(db|nats)`、`ObjectStore`、`HistoryStore`、`ModelRegistry` 注入。
- 队列能力化：个人/一体机默认 `db`，企业默认 `nats`；`PushTaskQueue` 不再阻断创建任务。
- Redis 直连替换为 `Cache/StreamStore` 接口（企业保留 Redis 实现）。
- 会话历史/模型配置端口化：企业档 ES/PostgreSQL，个人档 PG。
- MCP 改为**进程内调用**（去掉 `mcp.addr` HTTP 自调用）。
- `corekg server/worker/init/migrate` 子命令可用；`corekg migrate from-mysql --dry-run` 可在测试库跑通。
- 删除 `apps/workflow`、`frontend/workflow` 及 Compose 中的 Coze 服务/桶；执行 `go mod tidy && go mod vendor` 清理 Coze 独占依赖。

**Exit：** 企业档（仍连 MySQL）golden-path 通过；个人档（PG + localdir + memory cache + db queue + PG history）能启动并完成"直聊 + 会话历史"；迁移工具 dry-run 无类型错误；`rg -i coze` 在业务代码中为 0。

### Phase 2：检索端口化 + 向量分档（3-4 周）

- 实现 `VectorIndex` / `KeywordIndex` / `Embedder` / `Reranker`。
- ES 适配器包住现有 DSL；把 `ke_0` 全部改为 `forest.EsIndex()`。
- `internal/search` 承接邻居扩展、rerank、阈值、聚合。
- 新增 `pgvector` 适配器 + Postgres FTS 关键词实现（个人/一体机默认）。
- 企业档保持 `elasticsearch`；预留 `milvus`/`qdrant` 适配器接口（Phase 3 可选实现）。
- **并入当前 P0 止血项**：`running` 超时回收、NATS 最大投递/DLQ、切块"先写新索引再切换"。顺序：先定 `TaskQueue` 语义，再做 reaper/DLQ。

**Exit：** 切换 `vector.driver` 不改调用方；多 Forest 索引隔离测试通过；个人档在无 ES 情况下完成"上传 → 解析 → 切块 → 向量检索 → 问答"。

### Phase 3：业务域拆分 + 单入口收敛（4-6 周）

- 按 §8 把包迁入 `internal/{identity,knowledge,chat,search,task}`，GORM 实体沉到 `internal/model/<db>`。
- 引入 `pkg/contracts`，消灭跨域 `internal` import。
- 把 `apps/*/app.go` 替换为 `module.Module`；**删除 `apps/*/cmd/` 与独立二进制**。
- 外部数据源保留为 `ExternalDataSource` 独立端口（不进 `DBProvider`），企业档启用；为后续 chat2db 类"智能问数"留扩展点。
- Helm chart 落在 `deployments/helm`（本仓维护），与 Compose 共用同一份 Profile。
- 依赖测试收紧到"域间只能走 contracts（model 除外）"。
- **MySQL→PG 全量迁移演练**：在 staging/本地用生产规模快照跑 2 次，记录耗时、失败点、校验结果。

**Exit：** `go list -deps` 无跨域 service 依赖；`go build ./cmd/corekg` 是唯一后端产物；迁移演练连续两次通过。

### Phase 4：一次性切换与发布（3-4 周）

- **数据迁移**：停写 → MySQL 增量追平 → `corekg migrate from-mysql` → 行数/主键/checksum 校验 → 切换 PG。
- **检索重建**：按新 `vector.driver` 重建 ES/pgvector 索引；旧索引保留只读用于对比。
- **配置迁移**：`corekg config import-settings` 把 `core_settings` 的基础设施项迁进 Profile；清理 Coze 键。
- **部署切换**：AIO Compose、企业 Helm、个人版安装包统一用同一二进制 + Profile。
- **收尾**：清理旧 `apps/*` 残留、旧镜像构建、旧 CI、`scripts/mysql`（冻结归档）、系统库 MySQL 连接适配器；**保留** `mysqlplugin`（外部数据源）。
- **回归**：enterprise golden-path + personal golden-path + `rg -i coze` + 关键业务查询对比。

**Exit：** 一次发布完成切换；MySQL 仅保留只读快照；回滚方案为上一版镜像 + MySQL 快照（观察期内可回退）；`/v2` 已下线或按需重开。

### 下一阶段：ChatEngine + opencode（不在本期）

- 定义 `ChatEngine` 契约与事件流（§7.2 已给出草案）。
- `native` 封装现有 `ChatMode`；新增 `opencode` ACP 适配器。
- 验收：`engine: native|opencode` 可切换，SSE/引用/stop/usage 一致。

---

## 10. 风险与反模式

| 风险 | 表现 | 对策 |
|---|---|---|
| God interface | `MetadataStore` 几十个方法 | 不做 Repository；只抽象连接/方言 |
| 过早抽象 | 业务还在变就定死接口 | Phase 1 只抽"外部依赖"，业务契约放 Phase 3；ChatEngine 延后 |
| 双写/双路径长期化 | 新旧 API 并存不删 | 一次性切换；旧 `apps/*/cmd` 在 Phase 3 删除 |
| 方言残留 | MySQL 专有 SQL/类型未清干净 | 系统库：`FIND_IN_SET` 3 处、`REGEXP` 2 处、类型 tag 218 处；外部数据源的 `IFNULL`/`SHOW CREATE TABLE`/mysqlplugin 保留 | Phase 0 机械替换 + `rg` 门禁 + PG 集成测试；`scripts/postgres` 为唯一新脚本入口 |
| 数据迁移丢失/类型漂移 | `tinyint(1)`/`unsigned`/`datetime`/JSON/NULL 语义不同 | 逐列映射表 + 行数/主键/checksum 校验 + 两次全量演练；空串与 NULL 单独确认 |
| 迁移停机与回滚 | 一次性切换窗口内失败 | 停写 → 增量追平 → 校验 → 切换；保留 MySQL 只读快照 + 上一版镜像 |
| 向量重建不一致 | ES/pgvector 索引 chunk 数、维度、分词与旧库不一致 | 双索引并行、抽样对比、embedding `Dimensions()` 启动校验、按 Forest 分批重建 |
| 隐式全局 | 新代码继续用 `dbutil.X()`/`fs.Forest` | 依赖测试 + code review 规则 |
| 索引维度漂移 | 换 embedding 后写入维度不一致 | 启动校验 `Dimensions()`，并记录索引版本 |
| NATS 语义变化 | 以为"发出去就一定被消费" | 明确 `core_task` 是事实源，NATS 只是通知/加速 |
| Coze 删除不彻底 | 残留跨库写、前端死链、模型同步失败 | 先解耦模型管理，再删路由/前端；`rg -i coze` 门禁 |
| 一次性切换回滚难 | 发布后才发现主链路缺口 | Phase 0 golden-path + Phase 3 迁移演练 + 切换前双跑对比；MySQL 快照可回滚 |
| AIO 交付复杂 | 组件多、备份恢复难 | 复用 lework 的 install/backup/restore/upgrade 脚本形态 |
| 私有化 License 耦合 | 启动顺序与 license 校验冲突 | license 作为 `capability`，在 bootstrap 明确顺序 |

---

## 11. 验收标准（可执行）

1. `go build ./cmd/corekg` 产出**唯一**后端二进制；`corekg server`、`corekg worker`、`corekg init`、`corekg migrate` 均可运行。
2. 个人/一体机启动不要求 Redis/NATS/ES/Nebula；**只要求 PostgreSQL（含 pgvector）**。缺失能力时日志明确说明"capability disabled"，进程不崩溃。
3. `go test ./internal/arch` 通过（依赖方向规则）。
4. `rg 'dbutil\.' internal/{identity,knowledge,chat,search}` 为 0（或仅允许在 infra 适配器）。
5. `rg '"ke_0"'` 为 0。
6. 新增一个 `VectorIndex` 假实现（测试用）不需要改 `internal/search` 之外的代码。
7. 企业档与个人档的 golden-path 脚本全绿：`init → 建库 → 上传 → 解析 → 切块 → 检索 → RAG → 开放 API → MCP`；企业档在本仓 `docker-compose` 上跑，不依赖外部 staging。
8. `configs/profiles/*.yaml` 能通过 `corekg config validate`。
9. 个人档在干净机器上按文档完成安装（PG + pgvector + 单二进制 + 本地目录）。
10. `rg -i 'coze'` 在 `apps/`、`frontend/corekg/`、`deployments/` 中为 0（历史文档与 CHANGELOG 除外）。
11. `apps/*/cmd/` 已删除；CI 不再构建 kecore/kechat/keapi/account 独立镜像。
12. AIO Compose 与企业 Helm 均能用同一二进制 + 同一份 Profile 启动。
13. **MySQL 退出运行期**：`configs/profiles/*.yaml` 中无 `driver: mysql`；运行期只 import `db/postgres`；`mysqldrv` 仅出现在 `db/mysqlimport`。
14. **方言清零（系统库）**：`rg 'FIND_IN_SET|REGEXP'` 在系统库业务代码中为 0；GORM 模型无 `type:tinyint|unsigned|longtext|mediumtext|datetime\\(3\\)`；外部数据源代码（`mysqlplugin`、`SHOW CREATE TABLE`、`IFNULL`）允许保留。
15. **迁移校验**：MySQL 与 PG 逐表行数一致；主键范围/关键字段 checksum 一致；所有 sequence 已重置；抽样业务查询结果一致。
16. **检索重建**：企业档在 ES/专用向量库上重建后，chunk 数、维度、TopK 抽样与旧库对比在阈值内；个人档 pgvector 全链路通过。

---

## 12. 决策记录（2026-11-01 已确认）

| # | 决策 | 结论 |
|---|---|---|
| D1 | 统一数据库 | **PostgreSQL 作为所有档位的唯一运行期数据库**；MySQL 弃用 |
| D2 | 轻量向量 | **pgvector**（个人/一体机） |
| D3 | CGO | **允许按需启用** |
| D4 | 独立部署 | **全部收敛为单二进制**，只保留 `cmd/corekg` 与 `cmd/corekg-cli` |
| D5 | opencode | **本期不接**，下一阶段再考虑 |
| D6 | Coze | **全栈下线** |
| D7 | 企业向量 | **ES 或专用向量数据库**（Milvus/Qdrant 等）；元数据统一 PG |
| D8 | 企业回归 | **无现成 staging/签字人**；Phase 0 用本仓 Compose 建 golden-path，自动门禁 |
| D9 | 外部数据源/Excel | **不下线**；长期规划接入 chat2db 类"智能问数" |
| D10 | Helm | **本仓库维护**（`deployments/helm`） |
| D11 | `/v2` 兼容 | **可以不兼容**；本期只保留 `/v3`，需要时可重开 |
| D12 | 迁移节奏 | **一次性切换** |
| D13 | 企业 NATS | **保留**，抽象为队列能力的一种实现 |
| D14 | MySQL 数据 | **一次性迁移到 PG**；切换后 MySQL 只保留只读快照用于回滚 |
| D15 | 企业关键词 | 默认 `elasticsearch`；若向量走专用库，可切 `postgres_fts`，不强制保留 ES |

> 本期不再保留"企业 MySQL"选项。所有 Profile 的 `db.driver` 只能是 `postgres`；`mysql` 只出现在迁移工具与历史脚本中。

---

## 13. 已做的默认决策（builder calls，可推翻）

| # | 决策 | 理由 |
|---|---|---|
| C1 | GORM 实体进 `internal/model/<db>` 共享内核，域包只放行为 | 跨域耦合主要是模型；避免为 50+ 实体抽接口 |
| C2 | 不做 Repository 层，只抽象连接/方言 | 对齐 lework"轻抽象"；防止 god interface |
| C3 | 外部数据源不进 `DBProvider`，作为独立 `ExternalDataSource` 端口 | 控制方言层范围；为智能问数留扩展点 |
| C4 | 会话历史与模型配置在 Phase 1 端口化，个人档用 PG | 否则个人档无法启动聊天 |
| C5 | MCP 改为进程内调用 | 单二进制下 `:8086` 自调用不成立 |
| C6 | Profile YAML 是基础设施权威，DB settings 只保留业务参数 | 避免同一项三处覆盖 |
| C7 | Phase 1 不解除企业档 ES 依赖；个人档 Phase 2 用 pgvector | 让"低风险端口"与"高风险检索"分开 |
| C8 | P0 止血项（reaper/DLQ/`ke_0`）并入 Phase 2 | 与 `TaskQueue`/`VectorIndex` 同文件，避免二次改 |
| C9 | Phase 0 必须产出 golden-path 回归脚本 | 当前没有可用的企业回归门禁 |
| C10 | `pipeline`/`worker-ts` 不进主二进制；`webfetch`/`websearch` 默认折叠为 `internal/agent/tools` | 控制主进程复杂度；需要网络隔离时才保留 contrib |
| C11 | Coze 清理顺序：先解耦模型管理，再删路由/前端/代码/数据 | 避免先删代码导致跨库写失败 |
| C12 | 一次性切换仍分内部里程碑，但只发布一次 | 降低大爆炸风险，同时不搞长期双轨 |
| C13 | 个人档关键词用 Postgres FTS；中文分词需 `zhparser` 或应用侧分词 | pgvector 只解决向量，不解决 BM25 类召回 |
| C14 | Helm/AIO/个人版共用同一份 Profile，只换编排 | 避免三套配置漂移 |
| C15 | 先做 GORM 类型归一化（218 处），再开 AutoMigrate | 否则 PG 建表直接失败 |
| C16 | 单 PG 实例 + 单数据库 + 5 schema 为默认（`mode: schemas`），保留 `mode: databases` | 备份/迁移简单；隔离需求可切 5 库 |
| C17 | 迁移工具优先自研 `corekg migrate from-mysql`，`pgloader` 作为对照/兜底 | 需要业务级类型映射与校验，pgloader 不够可控 |
| C18 | ES 索引按 Forest 分批重建，保留旧索引只读对比 | 降低一次性重建失败的影响面 |

---

## 附录 A：端口与现有锚点对照

| 端口 | 现状锚点 | 风险 |
|---|---|---|
| `DBProvider` | `pkgs/utils/dbutil/mysql.go`、`database_conns`；MySQL→PG 需新增 `db/mysqlimport` | 高 |
| `ObjectStore` | `kecore/models/fs/file.go`、`yg-go/storage` | 低 |
| `Cache/Locker` | `redispool`、`settings` 缓存 | 低-中 |
| `StreamStore` | `pkgs/einotools/printer/sse_printer.go`、`llmchat/stop_stream.go` | 高 |
| `TaskQueue` | `pkgs/task/task_queue.go` | 中 |
| `HistoryStore` | `kechat/models/chatquestion/*`、ES `ke_chat_history` | 中 |
| `ModelRegistry` | `kechat/models/chatmodel`、`services/svcmodel`（Coze 同步下线后统一存 `chat` 库） | 中 |
| `VectorIndex/KeywordIndex` | `apps/kesearch/models/{chunk,essearch,reranksearch,globalsearch}`；适配器：pgvector/ES/Milvus/Qdrant | 高 |
| `Embedder` | `essearch/embedding.go` | 低 |
| `Reranker` | `reranksearch/rerank.go` | 低 |
| `ChatModelFactory` | `kechat/chat/modelhelper`、`kellm/drivers` | 中 |
| `Parser` | `kecore/services/fileconvert`、`models/decoupler`、`apps/pipeline` | 中 |
| `GraphStore` | `models/nbgraph`、`models/nebulagraph` | 中 |

## 附录 B：Profile 能力矩阵

| Capability | personal | aio | enterprise |
|---|---|---|---|
| knowledge（文本） | ✅ | ✅ | ✅ |
| chat（native） | ✅ | ✅ | ✅ |
| chat（opencode） | — | — | — |
| search（向量） | ✅ | ✅ | ✅ |
| search（关键词/混合） | 可选 | ✅ | ✅ |
| OCR / PDF 高级解析 | ❌ | 可选 | ✅ |
| 视频解析 | ❌ | 可选 | ✅ |
| 图谱 | ❌ | 可选 | ✅ |
| AI 写作 | ❌ | 可选 | ✅ |
| Coze / workflow | ❌ | ❌ | ❌ |
| MCP | ✅ | ✅ | ✅ |
| 计费/套餐 | ❌ | 可选 | ✅ |
| 外部数据源 / Excel 知识库 | ❌ | 可选 | ✅ |
| 智能问数（chat2db 类） | 长期 | 长期 | 长期 |
| License | ❌ | 可选 | ✅ |

> `chat（opencode）` 本期全部关闭；下一阶段以 Runtime 适配器方式接入，不改变上表其余能力。

**基础设施档位矩阵：**

| 基础设施 | personal | aio | enterprise |
|---|---|---|---|
| 元数据库 | PostgreSQL | PostgreSQL | PostgreSQL |
| 向量 | pgvector | pgvector | ES / Milvus / Qdrant |
| 关键词 | PG FTS | PG FTS | ES 或 PG FTS |
| 对象存储 | 本地目录 | 本地目录 | MinIO/S3 |
| 缓存/锁/流 | 进程内 | 进程内 | Redis |
| 队列 | DB | DB | NATS |
| 图 | 关闭 | 可选 | Nebula |
| 外部数据源 | 关闭 | 可选 | 开启 |

---

> 下一步：决策已确认（§12），进入 `writing-plans`，按 Phase 0 / Phase 1 拆成可执行任务；ChatEngine/opencode 单独作为下一阶段设计。
