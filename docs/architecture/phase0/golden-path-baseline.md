# Golden-path 基线（Phase 0）

`scripts/golden-path.sh` 是 Phase 0/1 的黄金链路回归脚本：从健康检查、登录、建库、上传、解析，到搜索、对话、keapi 与 MCP，共 11 个 stage 串行执行，任一 stage 失败即打印该 stage 名、HTTP 状态与响应体并以非 0 退出。本文档记录各 stage 的断言、依赖与基线状态。

**Phase 0 的完整 golden-path 目标是旧 `apps/corekg` 聚合单体**（挂载 kecore + kechat + account + keapi + keparser + kesearch，dev-up 后监听 `:8080`）。新 `cmd/corekg` 骨架当前只提供 `GET /healthz`，仅参与 stage 1。

## 用法

```bash
scripts/golden-path.sh --dry-run        # 打印 11 个 stage 与环境变量，不请求网络
scripts/golden-path.sh --self-test      # 离线自检：resp 同时解析 .Response/.response；stage 数 = 11
scripts/golden-path.sh --health-only    # 只跑 stage 1
scripts/golden-path.sh                  # 完整 11 stage（默认 CLEANUP=1，退出时删除所建森林）
scripts/golden-path.sh --no-cleanup     # 保留现场排障
```

退出码：`0` 全部通过；`1` 某 stage 失败（输出含 `stage=<name> FAILED`）；`2` 参数/依赖/环境变量错误。

## 环境变量与种子前置

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `BASE_URL` | 否 | 默认 `http://127.0.0.1:8080`（dev-up 后的旧 `apps/corekg`） |
| `TEST_DOMAIN` | 否 | 登录域名，可为空 |
| `TEST_USERNAME` | 完整跑必填 | 测试账号用户名（种子账号） |
| `TEST_PASSWORD_MD5` | 完整跑必填 | 密码的 md5 十六进制。`account.LoginByPassword` 挂有 `DecryptMD` 中间件，但无加密前缀的值原样透传，裸 md5 可用 |
| `MODEL_ID` | 完整跑必填 | 数字型 chat 模型 ID（keinit 初始化/后台配置的可用模型，LLM 需可达） |
| `KEAPI_API_KEY` | 完整跑必填 | keapi API Key（stage 10 与 stage 11 的 Bearer 鉴权） |
| `EXPECT_TEXT` | 否 | 写入测试文件的唯一 token，默认 `golden-path-$(date +%s)`；search/chat 用它断言命中 |
| `RUN_ID` | 否 | 森林名后缀，默认时间戳+PID |
| `PARSE_TIMEOUT` / `PARSE_INTERVAL` | 否 | parse 轮询上限/间隔，默认 300s / 5s |
| `HTTP_TIMEOUT` / `CHAT_TIMEOUT` | 否 | 普通请求 / chat 请求 curl 超时，默认 120s / 300s |

种子前置条件：

1. 已完成一次 `keinit` 初始化（建表 / ES 索引 / MinIO bucket / 系统设置 / 模型配置），见 `docs/local-development.md`、`docs/local-config-checklist.md`。
2. 存在可登录的测试账号（用户名 + 密码 md5）。
3. 存在可用的 chat 模型（`MODEL_ID`），且 LLM 上游可达。
4. 存在 keapi API Key（`KEAPI_API_KEY`），keapi 与 MCP（`/v3/keapi/mcp`）共用。
5. 测试文件必须小于分片（multipart）阈值：脚本生成的 `golden.txt` 只有几行文本；若 `files[0].multipart.enabled == true`，preupload stage 会直接报错提示。

## Stage 表

| # | stage | 端点 | 断言 | 依赖 | 基线耗时 |
| --- | --- | --- | --- | --- | --- |
| 1 | `health` | `GET /healthz` | HTTP 200 且 `.status == "ok"` | 仅 corekg 进程 | 9 ms（新 `cmd/corekg` 骨架，见下） |
| 2 | `login` | `POST /v3/account.LoginByPassword` | `(.response // .Response).jwt_token` 非空 | MySQL | 未测 |
| 3 | `create_forest` | `POST /v3/forest.CreateForest` | `(.Response // .response).forest_id` 非空 | MySQL | 未测 |
| 4 | `preupload` | `POST /v3/forest.PreUploadFile` | `files[0].upload_id` 非空、`upload_urls` 首值非空；`multipart.enabled == true` 则失败 | MySQL + MinIO（预签名 URL） | 未测 |
| 5 | `upload` | `PUT <presigned upload_url>` | curl PUT 成功（`-f`） | MinIO | 未测 |
| 6 | `callback` | `POST /v3/forest.UploadFileCallBack` | `(.Response // .response).file_id` 非空 | MySQL + MinIO + NATS（派发解析任务） | 未测 |
| 7 | `parse` | `POST /v3/forest.GetFileInfo`（5s 轮询，≤300s） | `parse_status == "success"` 且 `knowledge_status == "success"` | NATS + 宿主 worker（chunker 等）+ ES + embedding LLM | 未测 |
| 8 | `search` | `POST /v3/kesearch.ForestSearch` | `.code == 0` 且 `doc_search_result \| length > 0` | ES | 未测 |
| 9 | `chat` | `POST /v3/chat.SubmitChatQuestion` | `.code == 0` 且 `(.Response // .response).answer` 非空 | MySQL + ES + RAG 管线 + chat LLM | 未测 |
| 10 | `keapi_search` | `POST /v3/keapi.Search`（Bearer API Key） | `.code == 0` 且 `(.response // .Response).doc_search_result \| length > 0` | ES + MySQL（API Key 校验） | 未测 |
| 11 | `mcp` | `POST /v3/keapi/mcp`（Bearer API Key，`Accept: application/json, text/event-stream`） | HTTP 200 且响应体含 `"result"`；若返回 `Mcp-Session-Id` 头，再发 `tools/list` 并同样断言 | keapi MCP Server（与 keapi 同进程）+ MySQL（API Key 校验） | 未测 |

中间件依赖汇总：MySQL（2/3/4/6/9/10/11）、MinIO（4/5/6）、NATS + 宿主 worker（6/7）、ES（7/8/9/10）、LLM（7 embedding、9 chat）、Redis（会话/缓存，随 dev-up 启动）。

## Environment status（2026-11-01 记录）

**完整 11-stage 基线未在本环境执行。** 原因：

- `docker` 不可用（`command not found`）；`podman` 存在但宿主机根分区仅剩约 1.1 GB（99% 已用），无法拉取 compose 栈镜像（MySQL/ES/Redis/NATS/MinIO 等），因此 `make dev-up` 全栈不可行。
- 缺少种子数据（测试账号、可用 `MODEL_ID`、`KEAPI_API_KEY`）与可达的 LLM 上游。

本环境已验证的部分（真实输出，非伪造）：

- `bash -n scripts/golden-path.sh` 通过（exit 0）。
- `--dry-run` 通过：打印全部 11 个 stage 名与环境变量清单，exit 0，无网络请求。
- `--self-test` 通过：`resp` 对 `{"Response":{"x":1}}` 与 `{"response":{"x":2}}` 均解析成功，stage 数断言 11，exit 0。
- stage 1（`--health-only`）对新骨架 `bundles/corekg`（`cmd/corekg`，Task 1 产物）实测通过：

```text
$ ./bundles/corekg server --addr 127.0.0.1:18080 &
$ BASE_URL=http://127.0.0.1:18080 scripts/golden-path.sh --health-only
[golden-path] run_id=20261101210722-1437197 base_url=http://127.0.0.1:18080 expect_text=golden-path-1793538442 cleanup=1 health_only=1
[golden-path] stage=health ok elapsed_ms=9
[golden-path] summary:
[golden-path]   stage=health elapsed_ms=9
[golden-path] all 1 stage(s) passed
```

- 失败定位能力已验证：对只含 `/healthz` 的骨架跑完整脚本，stage 2 如期失败并打印状态与响应体，exit 1，cleanup 因未建森林而安全跳过：

```text
[golden-path] stage=health ok elapsed_ms=8
[golden-path] stage=login FAILED: expected http_status=200 got=404
[golden-path] stage=login http_status=404
[golden-path] stage=login response_body: 404 page not found
```

**stage 2–11 全部未测（unmeasured）**；上表"基线耗时"列除 stage 1 外均无数据，不得凭空填写。

## 后续执行完整基线的步骤

在有 docker 与足够磁盘的机器上：

```bash
# 1) 启动中间件 + 宿主旧 apps/corekg + 宿主 worker（首次需按 README 完成 keinit 初始化）
make dev-up                       # 转交 scripts/dev-up.sh；等价 ./scripts/dev-up.sh up
make dev-status                   # 确认 corekg:8080 与中间件端口就绪

# 2) 准备种子：测试账号 / 模型 / keapi API Key（keinit 或运营后台创建）
export TEST_USERNAME='<seed-user>'
export TEST_PASSWORD_MD5="$(printf %s '<password>' | md5sum | awk '{print $1}')"
export TEST_DOMAIN=''             # 如账号属于某域则填写
export MODEL_ID='<numeric-model-id>'
export KEAPI_API_KEY='<keapi-api-key>'

# 3) 跑完整 11 stage 并留存输出
BASE_URL=http://127.0.0.1:8080 scripts/golden-path.sh 2>&1 | tee /tmp/golden-path-baseline.log

# 4) 只验证新骨架 stage 1（可选）
go build -o bundles/corekg ./cmd/corekg
./bundles/corekg server --addr 127.0.0.1:18080 &
BASE_URL=http://127.0.0.1:18080 scripts/golden-path.sh --health-only
```

跑完后需要回填到本文档的内容：

1. Stage 表"基线耗时"列：逐 stage 抄录脚本输出的 `elapsed_ms`（parse stage 含轮询等待，需单独注明当时 worker/embedding 配置）。
2. 运行元数据：日期、git commit、`RUN_ID`、依赖版本（MySQL/ES/Redis/NATS/MinIO）、模型 ID 与 LLM 供应商（不含密钥）。
3. 任何失败与重试情况：失败 stage、响应体摘要、根因；基线只在 11 stage 全绿时才算建立。
4. Phase 1 迁移到新 `cmd/corekg` 后用同一脚本、同一组种子重跑，逐 stage 对比耗时与断言结果。
