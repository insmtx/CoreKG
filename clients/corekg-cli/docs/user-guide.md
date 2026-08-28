# CoreKG CLI 使用说明

本文说明当前 `corekg-cli` 的安装、浏览器登录、API Key 导入、Profile/多组织管理、知识库文件操作和自动化问答方式。

## 1. 安装和构建

源码构建当前平台：

```sh
make corekg-cli
./bundles/corekg-cli version
```

构建 Linux：

```sh
make corekg-cli-linux GOARCH=amd64
make corekg-cli-linux GOARCH=arm64
```

构建 npm 多平台预检包：

```sh
VERSION=0.1.0 make corekg-cli-npm-preflight
```

CLI 使用根目录已提交的 `vendor`，不在 `clients/corekg-cli` 下创建独立 `go.mod`。直接使用二进制不需要 Node.js；`@insmtx/corekg-cli` 包要求 Node.js 18 或更高版本。该单一 npm 包内含各支持平台的二进制，当前 npm 目录尚未表示已经发布到公共 registry。

发布后可通过 npm 全局安装，安装命令和实际执行命令分别为：

```sh
npm install --global @insmtx/corekg-cli
corekg-cli version
```

## 2. 全局参数

| 参数 | 说明 |
|---|---|
| `--config FILE_OR_JSON` | 本次运行从配置文件或 JSON 字符串加载配置。JSON 以 `{` 开头。 |
| `--profile NAME` | 只为本次命令选择 Profile，不修改当前 Profile。 |
| `--output, -o FORMAT` | `table`、`json` 或 `id`。 |
| `--timeout DURATION` | 通用 HTTP 请求超时，默认 `30s`；未指定命令专用超时时生效。 |

配置优先级为：内置默认值、`~/.corekg/config.json`、`--config` 指定的配置、`--output`/`--timeout` 单次覆盖。Profile 选择顺序为：命令行 `--profile`、环境变量 `COREKG_PROFILE`、`state.json` 中的 `current_profile`。

示例：

```sh
COREKG_PROFILE=staging corekg-cli kb list
corekg-cli --profile work -o json kb list
```

## 3. 初始化和浏览器登录

首次使用先初始化默认配置。初始化只做本地校验和写入，不访问服务器：

```sh
corekg-cli config init
```

默认配置保存在 `~/.corekg/config.json`。也可以让某次命令使用自定义文件或内联 JSON：

```sh
corekg-cli --config ./custom-config.json auth login
corekg-cli --config '{"server":"https://api.corekg.example.com","frontend":"https://corekg.example.com"}' auth login
```

### 3.1 交互式登录

```sh
corekg-cli auth login
```

CLI 会申请设备码并尝试打开浏览器。浏览器进入 `/cli/authorize` 后，完成账户登录和组织选择；CLI 不接收密码，也不处理网页登录 JWT。授权成功后，服务端创建专用的 `corekg_cli` API Key，CLI 将它保存为 Credential，并自动设置对应 Profile 为当前 Profile。

可以禁止自动打开浏览器：

```sh
corekg-cli auth login \
  --no-browser
```

可用 `--name` 指定 Profile 名称：

```sh
corekg-cli auth login \
  --name company-a
```

### 3.2 无浏览器或跳板机登录

先申请设备码并立即返回：

```sh
corekg-cli auth login \
  --no-wait \
  -o json
```

JSON 中包含 `device_code`、`user_code` 和 `verification_uri`。在有浏览器的设备打开 `verification_uri`，输入 `user_code`，登录并选择组织。授权完成后回到 CLI：

```sh
corekg-cli auth login --device-code <DEVICE_CODE>
```

未完成的设备码信息保存在 `auth.json` 的 `pending_logins` 中，因此继续命令可以省略 `--name`，并继续使用会话中记录的 Server。设备码默认 10 分钟有效，过期后重新执行 `auth login`。

### 3.3 多组织登录

每次网页登录选择一个组织，建议为不同组织指定不同 Profile：

```sh
corekg-cli auth login --name org-a
corekg-cli auth login --name org-b

corekg-cli profile use org-a
corekg-cli kb list
corekg-cli profile use org-b
corekg-cli kb list
```

这不会要求 CLI 保存或切换账户密码；每个 Profile 只保存对应组织的 API Key。

## 4. API Key 导入

已有 API Key 时，可以跳过浏览器登录。Server 来自有效配置，导入时只需指定 Profile 名称：

```sh
corekg-cli auth import \
	--name work
```

CLI 会隐藏输入并调用 `keapi.WhoAmI` 在线验证。无交互环境使用标准输入或显式环境变量：

```sh
printf '%s\n' "$COREKG_API_KEY" | corekg-cli auth import \
	--name work \
  --api-key-stdin

corekg-cli auth import \
	--name work \
  --api-key-env COREKG_API_KEY
```

`--api-key-stdin` 和 `--api-key-env` 不能同时使用。CLI 不提供明文 `--api-key` 参数，也不会自动读取某个固定环境变量。Profile 名称已存在时，导入会失败，应先重命名或删除原 Profile。

## 5. Profile 命令

Profile 是本地的 Server、Credential、组织和默认知识库组合。公开命令如下：

```sh
corekg-cli profile list
corekg-cli profile show
corekg-cli profile show --profile staging
corekg-cli profile use work
corekg-cli profile use -
corekg-cli profile rename work production
corekg-cli profile delete production --yes
```

`profile use -` 切换到上一次使用的 Profile。`profile delete` 只删除本地 Profile；当 Credential 不再被其他 Profile 引用时，也会删除本地 Credential。它不会撤销服务端 API Key。

单次执行可以用 `--profile`，不会改变当前 Profile：

```sh
corekg-cli kb list --profile staging
corekg-cli auth status --profile work
```

## 6. 认证状态和退出

查看本地认证记录（不会输出 API Key）：

```sh
corekg-cli auth list
corekg-cli auth list -o json
```

在线验证当前 Profile：

```sh
corekg-cli auth status
corekg-cli auth status --profile work
```

退出并删除本地记录：

```sh
corekg-cli auth logout --yes
```

如果同时需要撤销服务端 API Key：

```sh
corekg-cli auth logout --yes --revoke
```

`--revoke` 会先用当前 API Key 调用 `keapi.RevokeCurrentAPIKey`；服务端撤销失败时，CLI 不会继续删除本地记录。

## 7. 知识库命令

创建知识库。直接执行时会交互填写名称、描述和头像地址，并在调用服务端前展示摘要确认：

```sh
corekg-cli kb create
```

也可以提供名称和参数，未提供的字段仍会交互询问：

```sh
corekg-cli kb create "Product Docs" \
  --description "产品文档" \
  --avatar-url https://example.com/product.png \
  --use
```

创建的知识库类型固定为 `file`（标准/多模态知识库）。创建默认不会修改当前 Profile 的知识库，只有传入 `--use` 才会切换。自动化执行时使用 `--yes`；此时必须提供名称，未提供的描述和头像使用空值：

```sh
corekg-cli kb create "CI Docs" --description "构建资料" --yes -o json
```

列出当前组织可访问的知识库：

```sh
corekg-cli kb list
corekg-cli kb list --offset 0 --limit 50
corekg-cli kb list -o json
```

按 ID 或完整名称选择默认知识库：

```sh
corekg-cli kb use 20001
corekg-cli kb use "Product Docs"
```

名称必须完全匹配；多个同名知识库时使用 ID。`kb use` 当前只更新本地 Profile，后续文件、搜索和问答命令会复用该选择。

## 8. 文件和知识库问答

文件命令默认使用当前 Profile 选择的知识库，也可以用 `--kb` 传知识库 ID 或完整名称覆盖：

```sh
corekg-cli file list
corekg-cli file list --kb 20001 --all
corekg-cli file list --kb "Product Docs" -o json
corekg-cli file upload ./docs/handbook.pdf
corekg-cli file upload ./docs/handbook.pdf --parent-id 30001 --wait
```

`file upload` 使用流式 multipart 上传，不把文件整体读入内存。默认拒绝符号链接；确认目标后可显式使用 `--follow-symlinks`。`--wait` 会轮询文件状态，直到解析和索引成功或失败；上传已创建但等待失败时命令仍返回非零退出码，并在 JSON 错误详情中保留文件 ID。未使用 `--wait` 时状态 `accepted` 只表示服务端已接收。`--upload-timeout` 默认使用十分钟或更长的配置超时。`--all` 会按服务端最大分页大小读取全部文件；脚本建议使用 `-o json` 或 `-o id`。

`ask` 会为每个知识库保存最近一个服务端聊天会话，后续调用默认继续该会话：

```sh
corekg-cli ask "这份资料的关键风险是什么？"
corekg-cli ask "请给出相关证据"
corekg-cli ask --new "从管理层视角重新总结"
corekg-cli ask --session-id 12345 "继续讨论第一个结论"
corekg-cli ask --prompt-file question.txt
cat question.txt | corekg-cli ask -
```

`--new` 创建新的服务端会话，不删除旧会话；只有问答成功后，新会话才会成为该知识库的默认会话。问答失败不会覆盖原默认会话；错误详情会返回可恢复使用的 session ID。`--session-id` 必须属于目标知识库。`--ask-timeout` 默认使用两分钟或更长的配置超时。问答接口使用知识库范围而不是把当前全部文件 ID 复制到 CLI，本地只保存会话 ID。服务端文件仍需完成解析和索引后才会被检索。

## 9. 配置命令和文件

默认文件：

```text
~/.corekg/config.json
~/.corekg/state.json
~/.corekg/auth.json
```

查看实际路径：

```sh
corekg-cli config path
corekg-cli config path -o json
```

初始化或重新初始化静态配置。除了 API Server，还可以配置浏览器授权页面所在的前端地址：

```sh
corekg-cli config init
```

初始化提示依次包含 `CoreKG server`、`CoreKG frontend`、`Default output`、`HTTP timeout` 和保存确认。`config path` 只显示路径，不要求配置已经存在。`config init` 没有 `--file`、`--json` 等专属参数，并且始终写默认路径；`--config` 不能用于初始化。配置目录默认 `0700`，JSON 文件默认 `0600`，写入使用 `.lock`、临时文件和原子替换。

默认 API Server 是 `http://127.0.0.1:8080`，默认前端地址是 `http://localhost:3001`。

API Key 当前是 `auth.json` 中受文件权限保护的明文，建议不要打印、复制或提交到 Git。自定义配置只覆盖本次运行；登录、Profile 和知识库变更仍写入固定目录中的 `state.json`/`auth.json`。

如果未配置 `frontend`，登录会继续使用服务端返回的 `verification_uri`；配置后，CLI 使用 `{frontend}/cli/authorize?user_code=...` 打开浏览器。

## 10. 输出和退出码

默认是表格输出，不建议脚本解析表格。自动化使用 JSON：

```sh
corekg-cli kb list -o json
corekg-cli profile list -o id
```

JSON 成功结果使用 `{ "ok": true, "data": ... }`；错误写入标准错误并包含稳定错误码。常用退出码：

| 退出码 | 含义 |
|---:|---|
| 0 | 成功 |
| 1 | 运行时、网络或远程业务错误 |
| 2 | 参数或输入错误 |
| 10 | 缺少 `--yes` 确认 |

## 11. 故障排查

### `auth_required: no active profile`

没有当前 Profile。先登录或导入：

```sh
corekg-cli config init
corekg-cli auth login
corekg-cli auth import --name work --api-key-env COREKG_API_KEY
```

### `profile_not_found`

检查 Profile 名称：

```sh
corekg-cli profile list
```

### `credential_verification_failed`

检查 Server 是否对应当前 CoreKG 实例、API Key 是否被撤销、网络和 TLS 是否可用。可临时增加超时：

```sh
corekg-cli auth status --timeout 60s
```

### 浏览器授权过期或拒绝

重新执行 `auth login` 申请新的设备码；旧设备码不能重复使用。

### `configuration lock is held by another process`

另一个 CLI 进程正在修改同一配置目录。确认没有其他写入者后再重试。

### `kb_required` 或 `kb_not_found`

先选择一个当前 Profile 可访问的知识库，或在命令中显式传入 `--kb`：

```sh
corekg-cli kb list
corekg-cli kb use 20001
corekg-cli file list --kb 20001
```

### 文件仍处于 `pending`、`parsing` 或 `indexing`

上传输出为 `accepted` 只表示服务端已创建文件记录。使用 `file list -o json` 查看 `file_status`，或上传时使用 `--wait`；文件变为 `success` 后再进行问答。若等待超时，命令会失败但 JSON 错误详情仍包含文件 ID。

### 配置版本高于 CLI 支持版本

升级 CLI，不要手工降低 `version` 字段，否则可能丢失新字段。旧版本的 `current_context`/`contexts` 配置会被当前 CLI 读取并在下次保存时迁移为 `current_profile`/`profiles`。

## 11. 当前能力边界

| 命令域 | 已支持 | 后续计划 |
|---|---|---|
| 基础 | 版本、配置路径、默认设置 | 自动升级 |
| 认证 | 浏览器登录、Device Authorization、API Key 导入/验证/退出/撤销 | 刷新和更多认证提供商 |
| Profile | 列表、查看、切换、重命名、删除、多组织隔离 | 自动发现组织 |
| 知识库 | 列表、设置默认知识库 | 创建、修改、删除 |
| 文件和目录 | 文件列表、上传和解析状态 | 下载、目录操作 |
| 检索和聊天 | 知识库问答、会话复用和新会话 | 搜索、SSE 流式输出 |
| MCP | 不透传 MCP Tool | 不在当前 CLI 设计范围 |
