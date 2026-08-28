# KEAPI 服务

## 服务定位

KEAPI 是 CoreKG 平台的**对外知识库 API 服务**，面向外部开发者和第三方集成，提供基于 API Key 认证的知识库管理、文档管理、AI 对话和知识检索能力。同时暴露 **MCP (Model Context Protocol) Server**，使 LLM Agent 可通过标准化协议直接操作知识库。

默认端口：`8086`，基础路径：`/v3`。

## 核心业务域

| 业务域 | 解决什么问题 | 关键能力 |
|--------|-------------|---------|
| 知识库管理 | 知识库生命周期 | CRUD、批量查询 |
| 文档管理 | 文档上传与组织 | 上传（秒传/SHA256去重）、预览、目录、Chunk 查询 |
| AI 对话 | 知识库问答 | OpenAI 兼容接口、流式 SSE、知识库范围会话、One-shot 模式、会话管理 |
| 知识检索 | 知识库搜索 | 多知识库聚合检索、文档/图片/视频分类结果 |
| MCP Server | Agent 程序化操作 | 21 个 Tool，StreamableHTTP 协议 |

## 核心业务概念

- **Forest（知识库）** — 知识容器，类型分 file（文档库）和 data（Excel 数据库）
- **File（文档/节点）** — 知识库内的文件或目录，支持树形结构
- **Chunk（分段）** — 文档 RAG 解析后的文本块，存储在 ES
- **ChatSession / ChatMessage** — 对话会话与消息
- **MCP Tool** — Agent 操作知识库的标准化接口

## MCP Server

挂载在 `ANY /v3/keapi/mcp`，StreamableHTTP 传输协议，21 个 Tool：

| 分组 | Tools |
|------|-------|
| 知识库 | list_forest, batch_get_forest, create_forest, update_forest, delete_forest |
| 文档 | list_file, batch_get_file, get_file_chunks, upload_file, preview_file_url |
| 节点 | create_dir, rename_path, delete_path |
| 对话 | create_chat, batch_get_chat_info, update_chat_name, delete_chat, create_chat_message, list_chat_messages, chat_completions |
| 检索 | search |

MCP 内部通过 HTTP 自调用自身 REST API，确保鉴权逻辑一致。

## API 路由

知识库、文件、对话和检索接口均需 API Key 鉴权（`RequireAPIKeyPrivilege`）。CLI 设备授权的 Start、Info、Poll 为公开的短时流程接口；授权批准/拒绝由 account 服务的已登录路由完成。

| 分组 | 代表接口 |
|------|---------|
| 知识库 | ListForest, BatchGetForest, CreateForest, UpdateForest, DeleteForest |
| 文档 | ListFile, BatchGetFile, GetFileChunks, UploadFile, PreviewFileByURL |
| 节点 | CreateDir, RenamePath, DeletePath |
| 对话 | CreateChat（支持 `forest_id` 或 `forest_file_ids`）、CreateChatMessage, chat/chat/completions（OpenAI 兼容） |
| 检索 | Search |
| CLI 身份 | WhoAmI、CLIAuthStart、CLIAuthInfo、CLIAuthPoll、RevokeCurrentAPIKey |
| 监控 | metrics（GET，Prometheus） |

## 代码架构

```
apps/keapi/
├── app.go                   # Routers/Migrates/RunJob
├── cmd/
│   ├── main.go              # Cobra 入口，初始化 DB/ES/Redis/Storage/i18n
│   └── init.go              # 数据库 + Redis 初始化
├── conf/
│   └── config.go            # MCP 配置加载
├── internal/
│   ├── apis/
│   │   ├── apis.go          # 统一路由注册
│   │   ├── forestctl/       # 知识库/文档/节点/对话 handler
│   │   └── searchctl/       # 检索 handler
│   ├── services/
│   │   └── svcforestchat/   # 对话核心业务（ChatWrapper、消息处理、SSE Printer）
│   ├── dto/dtokeapi/        # 请求/响应 DTO
│   ├── middleware/
│   │   └── apikey.go        # API Key 鉴权中间件
│   ├── mcp/                 # MCP Server
│   │   ├── server.go        # Server 创建 + 路由注册
│   │   └── tools/           # 21 个 Tool 定义
│   ├── mcpcommon/           # InternalClient（API 自调用）
│   └── docs/                # Swagger 自动生成
└── script/                  # 部署脚本
```

## 技术要点

- **OpenAI 兼容**：`chat/chat/completions` 完全兼容 OpenAI 格式，支持流式/非流式
- **无独立数据模型**：不维护自己的表，所有操作引用 kecore/kechat/kesearch 的 models/services
- **文件秒传**：上传时计算 SHA256，命中则跳过写入
- **知识库范围会话**：`CreateChat` 可传 `forest_id` 创建覆盖整个知识库的持久会话；仍兼容按 `forest_file_ids` 创建的会话
- **One-shot 模式**：不传 session_id 时自动创建临时会话，问答后清理
- **异步会话命名**：首次对话后异步调用 LLM 生成会话名称
- **回答内容过滤**：过滤内部标记，只保留可见内容

## 本地开发

```bash
make local APP=keapi
make run APP=keapi ENV=test
make generate-docs APP=keapi
```

## 与其他服务的关系

- 依赖 `apps/kecore` — 知识库/文档/节点核心 CRUD 与权限
- 依赖 `apps/kechat` — ChatWrapper RAG 引擎、会话/问题管理
- 依赖 `apps/kesearch` — ES 客户端、Chunk 查询、检索聚合
- 依赖 `apps/keparser` — 解析任务配置（SplitConfig）
- 依赖 `apps/kellm` — LLM 消息格式
- 依赖 `apps/account` — API Key 验证、用户信息
- 依赖 `pkgs/einotools` — Agent 消息打印器抽象
