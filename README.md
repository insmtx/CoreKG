# CoreKG —— 知识库 / RAG 对话服务

CoreKG 是一个知识平台：以知识库（Forest）承载文档（File），并在其上提供基于知识库的对话（Chat）与检索（Search），以及配套的内部运营服务。服务端为 Go 单体仓库（module `github.com/insmtx/corekg`），摄入解析（拆 chunk / 向量化 / 写索引）由独立 Python pipeline 承担，文档/对话前端见 `frontend/`。知识库异步任务经 NATS JetStream 分发；`keapi` 额外提供 MCP Server 将知识库 API 封装为 MCP Tool。

## 文档

- [前端开发指南](frontend/README.md)
- [本地开发指南](docs/local-development.md)

### 共用账号

### 开发、测试环境

* 测试域名 example.com
* 生产域名（自定）

## 开始之前

```bash
git clone <本仓库>
cd CoreKG-oss
git config pull.rebase true
```

## 本地开发起步（开源）

### 配置文件约定

所有含密钥/连接串的运行配置一律**不入库**，仓库仅提供 `*.example` 模板：

- Go 服务：`apps/<app>/conf/<env>/config.yaml.example`
- 前端：`frontend/corekg/.env.development.example`、`.env.production.example`
- TS worker：`apps/worker/.env.example`
- Python pipeline：`apps/pipeline/config/*.yaml.example`

使用方式（以 `corekg` 聚合服务为例）：

```bash
cp apps/corekg/conf/test/config.yaml.example apps/corekg/conf/test/config.yaml
# 编辑 config.yaml，将 change-me 换成自己的真实值
make run APP=corekg ENV=test
```

> **两种启动模式**：corekg 既可用宿主机进程跑(上方 `make run`,读 `apps/corekg/conf/test/config.yaml` + `conf/test/core_setting.yaml`),也可以作为 **docker-compose 容器**跑(读 `apps/corekg/conf/docker/config.yaml` + `conf/docker/core_setting.yaml`)。因为 corekg 的运行连接全部来自 DB `core_settings`(由 keinit 初始化写入),两模式的 `core_setting.yaml` 连接字段不同(宿主用 `localhost:PORT`,容器用 compose 服务名)。两种模式的初始化与部署差异见 `docs/local-development.md`「4.1 两种启动模式（宿主机 / docker-compose 容器）」。

### 构建多架构（amd64 / arm64）镜像

默认单平台（`linux/amd64`）构建，保持向后兼容。需要同时产出两种架构时，通过 `BUILD_PLATFORMS` 指定并用 buildx 构建：

```bash
# 1) 准备 docker-container driver 的 buildx builder（默认 docker driver 不支持多平台）
docker buildx create --name corekg-multi --driver docker-container --platform linux/amd64,linux/arm64

# 2) 多架构构建并推送（需具备 buildx 与目标 registry 的推送权限）
make push-image APP=keapi BUILD_PLATFORMS='linux/amd64,linux/arm64' BUILDER=corekg-multi
```

- 单平台：仍走原 `docker build --platform`，行为不变。
- 多平台：`BUILD_PLATFORMS` 含逗号时自动切到 `docker buildx build ... --platform <list> --push`。
- 多平台要求 buildx builder 使用 `docker-container` driver（否则报 `Multi-platform build is not supported for the docker driver`）；通过 `BUILDER` 指定 builder。
- 应用 Go 二进制的架构由各应用 `script/Dockerfile` 中的 `ARG TARGETARCH` + `GOOS=${TARGETOS} GOARCH=${TARGETARCH}` 决定（已统一），无需手工指定。

### 本地依赖（MySQL / ES / Redis / MinIO / NATS）

```bash
docker compose up -d
```

本地基础环境（含中间件端口/凭据、如何启动、各服务初始化）请见 **[docs/local-development.md](docs/local-development.md)**。关键约定如下：

- **宿主机端口统一偏移（规避本机已占用）**：MySQL `3308`(:3306)、Redis `6381`(:6379)、ES `9202`(:9200)/`9302`(:9300)、MinIO `9002`(:9000)/`9003`(:9001)、NATS `4225`(:4222)。
- **所有中间件明文密码统一为 `123456`**（本地开发默认值）。
- 容器之间经服务名+容器内端口互连；宿主机进程（各 `make run` 启动的 Go 服务）经上述映射端口访问。
- 首次启动会通过 `scripts/mysql-docker-init.sh` 额外创建 `opencoze` 库。

以上默认值已与各 `apps/*/conf/*/config.yaml.example` 保持一致；生产部署请勿使用这些默认值。

### Docker Compose 启动 CoreKG 前端

前端以独立 Nginx 容器运行，并通过同一站点代理 CoreKG API 与 MinIO。首次部署先执行一次初始化，再启动前端及其依赖：

```bash
# 首次初始化（完成后 keinit 正常退出）
docker compose up --build keinit

# 启动前端、corekg 及其依赖
docker compose up -d --build frontend

# 如需同时启动 pipeline、文档转换等全部服务
docker compose up -d --build
```

浏览器访问 `http://localhost:3001`。前端将 `/v2/*`、`/v3/*` 转发到 `corekg:8080`，将 `/corekg-bucket/*` 转发到 `minio:9000`；`http://localhost:3001/healthz` 用于前端容器健康检查。本地 Compose 不包含 Coze 工作流前端，`/coze` 不在本部署方式的验收范围内。

> 初始化所需补齐的环境变量与占位符清单（对话/视觉/Embedding 模型地址、JWT、PDF 转换服务等），及对应的初始化命令，见 **[docs/local-config-checklist.md](docs/local-config-checklist.md)**。

**中间件镜像统一使用 yygu 自建镜像**（`registry.cn-beijing.aliyuncs.com/yygu/corekg` 与 `registry.yygu.cn/library`），与私有化 Helm 部署（corekg-chart）同一套镜像：`mysql_8.4.5`、`elasticsearch_8.18.1-2`（内置 IK 分词插件，无需额外 es-init 容器）、`redis_8.2`、`minio_RELEASE.2025-04-22T22-12-26Z`、`nebula-*_v3.8.0`、`nats:2.12.7`；corekg / pipeline / ketask 等业务镜像仍本地构建。可验证：

```bash
# 确认官方 minio 同时发布 amd64 与 arm64
docker manifest inspect minio/minio:latest | grep '"architecture"'
# 起 minio 并等待 bucket 初始化完成（minio-init 会自动创建 corekg-bucket）
docker compose up -d minio minio-init
docker compose ps
```

**NATS 是本项目唯一的消息中间件**：既承担知识库异步任务分发（`ketask`/`kecore`/`keapp` 的 JetStream 任务系统），也是 `workflow` 的事件总线（`workflow.mq.type: nats`）。无需额外部署 NSQ/Kafka/Pulsar/RocketMQ/RabbitMQ。

启用 `workflow` 应用时，直接运行即可——`apps/workflow/conf/test/config.yaml` 及 `apps/corekg/conf/test/config.yaml(.example)` 中嵌入的 workflow 配置块已把连接信息收敛为与 `docker-compose.yml` 一致的**字面值**（本地端口 +2、密码一律 `123456`），不再依赖任何环境变量：

```bash
# 依赖就绪：docker compose up -d
make run APP=workflow ENV=test   # 或 make run APP=corekg ENV=test（聚合进程内拉启 workflow）
```

如需改动中间件端口/凭据，直接改 `docker-compose.yml` 与上述 `config.yaml` 里的字面值即可，无需导出环境变量。

## 一键自动化验证（知识库闭环）

`scripts/verify/verify-kb.sh` 可自动跑通**登录 → 新建知识库 → 上传文件 → 等待解析(拆 chunk/向量/入库) → 基于文件问答**的完整闭环。

前提：中间件 + corekg + **pipeline 摄入 worker** 均已在跑（解析依赖 pipeline 的 analyser/chunker；
若要验证 Office(.docx/.ppt/.ofd) 上传预览/解析，还需要 **doc2pdf(ketask)** worker）：
`make dev-up` 会一并拉起。手动启动：

```bash
# 本地宿主模式（corekg 二进制 :8080，pipeline 跑在宿主机 venv）
cd apps/pipeline && source .venv/bin/activate
python doc_worker_main.py &   # 消费 ke.prase_pdf_task
python chunk_worker_main.py & # 消费 ke.knowledge_task（拆 chunk + 向量 + 写 ES）
cd ../..
make local APP=ketask
./bundles/ketask doc2pdf -c apps/ketask/conf/test/config.yaml \
  -t ke.doc_to_pdf_task -b http://localhost:8080/ &  # 消费 ke.doc_to_pdf_task（Office → PDF 预览）
./scripts/verify/verify-kb.sh --mode local --cleanup

# docker-compose 全容器模式（含 doc2pdf 服务）
docker compose -f docker-compose.pipeline.yml up -d --build
./scripts/verify/verify-kb.sh --mode compose --cleanup
```

- 样例文件 `testdata/verify_sample.txt`、问题 `testdata/verify_question.txt` 可用 `--file/--question` 覆盖。
- `--cleanup` 结束自动删除本次创建的知识库；`VERIFY_PARSE_TIMEOUT` 可调解析等待时长（默认 180s）。
- 关键依赖：向量化走 `apps/pipeline/config/chunk_config(.docker).yaml` 的 `Embedding` 节点（默认指向真实可达的
  `embed-qwen3.003.yygu.cn`）；无真实模型时可改用 `scripts/mock_embedding.py`。详见 `docs/local-development.md`。

# KEAPI MCP Server

keapi 提供 MCP (Model Context Protocol) Server，将知识库 API 封装为 MCP Tool，供 AI 代理或客户服务端通过 MCP 协议调用。基于 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)，使用 StreamableHTTP 传输协议（MCP 2025-03-26 规范），支持远程接入。

## 接入信息

| 项目 | 说明 |
|---|---|
| Endpoint URL | `http://<host>:<port>/v3/keapi/mcp`（与 keapi HTTP API 共用端口，默认 8086） |
| 鉴权方式 | 每次请求携带 `Authorization: Bearer <api_key>` Header，与 HTTP API 共用同一套 API Key 鉴权 |
| 传输协议 | StreamableHTTP（支持 POST/GET/DELETE） |
| 依赖库 | mark3labs/mcp-go v0.43.0 |

## 服务端接入配置

客户服务端接入 keapi MCP Server 有三种方式：

### 方式一：直接 URL 接入（最简单）

适用于支持 StreamableHTTP 的 MCP Client，直接指定 endpoint URL 和鉴权 Header：

```json
{
  "mcpServers": {
    "keapi": {
      "url": "http://<host>:<port>/v3/keapi/mcp",
      "headers": {
        "Authorization": "Bearer <your_api_key>"
      }
    }
  }
}
```

### 方式二：Go 服务端接入（mark3labs/mcp-go Client）

适用于 Go 服务端程序，使用 mcp-go Client 库连接：

```go
import (
    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/mcp"
)

func connectKEAPIMCP(apiKey string) (*client.Client, error) {
    mcpClient := client.NewStreamableHTTPClient("http://<host>:<port>/v3/keapi/mcp",
        client.WithStreamableHTTPHeaders(map[string]string{
            "Authorization": "Bearer " + apiKey,
        }),
    )

    ctx := context.Background()
    session, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{
        Params: mcp.InitializeParams{
            ClientInfo: mcp.Implementation{
                Name:    "my-app",
                Version: "1.0.0",
            },
        },
    })
    if err != nil {
        return nil, err
    }
    // session 可用于后续 CallTool / ListTools 等操作
    return mcpClient, nil
}
```

### 方式三：eino-ext MCP 工具接入

适用于使用 eino AI 框架的服务端，项目已依赖 `cloudwego/eino-ext/components/tool/mcp`：

```go
import (
    "github.com/cloudwego/eino-ext/components/tool/mcp"
)

func createKEAPIMCPTool(apiKey string) (*mcp.Tool, error) {
    tool, err := mcp.GetTool(ctx, &mcp.Config{
        URL: "http://<host>:<port>/v3/keapi/mcp",
        Headers: map[string]string{
            "Authorization": "Bearer " + apiKey,
        },
    }, "search") //指定要使用的 Tool 名称，如 "search"
    if err != nil {
        return nil, err
    }
    return tool, nil
}
```

## 可用 Tool 列表

keapi MCP Server 提供全部 21 个 Tool，按功能分为 5 组：

### 知识库管理 (Forest)

| Tool Name | 描述 | 必填参数 |
|---|---|---|
| `list_forest` | 列出知识库列表 | offset, limit |
| `batch_get_forest` | 批量查询知识库信息 | forest_ids |
| `create_forest` | 创建知识库 | name |
| `update_forest` | 更新知识库信息 | forest_id, name 或 description |
| `delete_forest` | 删除知识库 | forest_id |

### 文档管理 (File)

| Tool Name | 描述 | 必填参数 |
|---|---|---|
| `list_file` | 列出知识库下的文档列表 | forest_id |
| `batch_get_file` | 批量查询文档信息 | forest_file_ids |
| `get_file_chunks` | 查询文档的 Chunk 分段内容 | forest_file_id, chunk_sequences |
| `upload_file` | 上传文档到知识库（文件内容需 base64 编码） | forest_id, file_name, file_base64 |
| `preview_file_url` | 获取文档的预览或下载 URL | forest_file_id |

### 目录操作 (Node)

| Tool Name | 描述 | 必填参数 |
|---|---|---|
| `create_dir` | 在知识库中创建文件夹 | forest_id, name |
| `rename_path` | 重命名文件或文件夹 | forest_file_id, name |
| `delete_path` | 删除文件或文件夹 | forest_file_ids |

### 对话 (Chat)

| Tool Name | 描述 | 必填参数 |
|---|---|---|
| `create_chat` | 创建对话会话，关联指定文档 | forest_file_ids |
| `batch_get_chat_info` | 批量查询对话会话信息 | session_ids |
| `update_chat_name` | 更新对话会话名称 | session_id, name |
| `delete_chat` | 删除对话会话 | session_id |
| `create_chat_message` | 在对话会话中创建用户消息 | session_id, content |
| `list_chat_messages` | 查询对话会话的消息列表 | session_id |
| `chat_completions` | 基于知识库文档进行对话补全（非流式） | forest_file_ids 或 session_id |

### 搜索 (Search)

| Tool Name | 描述 | 必填参数 |
|---|---|---|
| `search` | 在知识库中检索相关内容 | forest_ids, query |

## 注意事项

- **chat_completions** 强制 `stream=false`，不支持 MCP 层面的流式输出，返回完整对话结果
- **upload_file** 需将文件内容 base64 编码后通过 `file_base64` 参数传入，同时需指定 `file_name`
- **preview_file_url** 返回的预览 URL 由客户端自行访问，URL 有效期有限
- **鉴权** 所有 MCP Tool 调用均需有效的 API Key，无效或过期 Key 将返回 `unauthorized` 错误
- **端口共用** MCP Server 与 keapi HTTP API 共用同一服务端口，MCP endpoint 路径为 `/v3/keapi/mcp`

## 快速验证

使用 curl 验证 MCP Server 连通性：

```bash
# 1. Initialize 建立会话
curl -i -X POST "http://127.0.0.1:8086/v3/keapi/mcp" \
  -H "Authorization: Bearer <your_api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"initialize",
    "params":{
      "protocolVersion":"2025-03-26",
      "capabilities":{},
      "clientInfo":{
        "name":"curl",
        "version":"1.0"
      }
    }
  }'

# 2. 使用返回的 mcp-session-id 调用 Tool（替换为实际 session id）
curl -s -X POST "http://127.0.0.1:8086/v3/keapi/mcp" \
  -H "Authorization: Bearer <your_api_key>" \
  -H "Content-Type: application/json" \
  -H "mcp-session-id: mcp-session-<your_session_id>" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"list_forest",
      "arguments":{
        "limit":20
      }
    }
  }'
```

# 数据库迁移脚本规范

## 脚本规范

### MySQL
* **已发布版本的脚本除了有导致完全无法执行的语法错误外，都不允许修改，一律通过新版本变更实现回滚**
* **脚本文件仅执行一遍**
* 文件名： 全小写、数字、下划线
* ${version} 主版本号，可重复值，例如 v1.6
* ${seq} 序号，可重复值，例如 1, 2, 3
* ${action} 动作，例如 create_table，insert_data，alter_table 等
文件： `/scripts/mysql/${version}_${seq}__${action}.sql`
实例： `/scripts/mysql/v1.6_1__create_table.sql`， `/scripts/mysql/v1.6_2__insert_data.sql`， `/scripts/mysql/v1.7_1__alter_table.sql`

#### 脚本规范
##### 索引
前缀 `uk_`, `idx_`

##### 对于多条有外键关联的数据插入的脚本，例如
```sql
INSERT INTO `t1` (`id`, `name`) VALUES (1, 'a');
SET @t1_id = LAST_INSERT_ID();
INSERT INTO `t2` (`id`, `name`, `t1_id`) VALUES (1, 'b', @t1_id);
```

##### 对于需要用变量替换字段中部分内容的，使用占位符先创建，再替换，例如
```sql
INSERT INTO `t1` (`id`, `name`) VALUES (1, 'a-xxxxyyyyzzzz-b');
SET @t1_id = LAST_INSERT_ID();
UPDATE `t1` SET `name` = REPLACE(`name`, 'xxxxyyyyzzzz', @yg_VAR_NAME) WHERE `id` = @t1_id;
```

