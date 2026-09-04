# CoreKG MCP Server 接入文档

> 本页为 `keapi` 服务内置 **MCP (Model Context Protocol) Server** 的完整接入指南：接入信息、三种客户端接入方式、全部可用 Tool 及快速验证。
> 概述与快速入口见[项目 README](../README.md#mcp-server)。

`keapi` 提供 MCP Server，将知识库 API 封装为 MCP Tool，供 AI 代理或客户服务端通过 MCP 协议调用。基于 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)，使用 **StreamableHTTP** 传输协议（MCP 2025-03-26 规范），支持远程接入。

## 接入信息

| 项目 | 说明 |
|---|---|
| Endpoint URL | `http://<host>:<port>/v3/keapi/mcp`（与 keapi HTTP API 共用端口，默认 `8086`） |
| 鉴权方式 | 每次请求携带 `Authorization: Bearer <api_key>` Header，与 HTTP API 共用同一套 API Key 鉴权 |
| 传输协议 | StreamableHTTP（支持 POST/GET/DELETE） |
| 依赖库 | mark3labs/mcp-go v0.43.0 |
| Tool 数量 | 21 个，按功能分为 5 组 |

> 实现说明：MCP Tool 内部通过 HTTP 自调用 keapi 自身 REST API，确保与对外 API 的鉴权逻辑完全一致。
> 在 `corekg` 聚合单体部署模式下，keapi 路由被挂载进聚合进程，MCP endpoint 位于聚合服务端口（默认 `:8080`）的 `/v3/keapi/mcp`。

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

## 相关文档

- [项目 README](../README.md) — CoreKG 概述与快速开始
- [`keapi` 服务说明](../apps/keapi/README.md) — 对外 API 服务定位与路由
- [核心业务流程](./core-business-flow.md) — RAG / 对话深层架构
