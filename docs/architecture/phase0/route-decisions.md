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
