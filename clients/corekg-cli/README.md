# CoreKG CLI

`corekg-cli` 是 CoreKG 的独立 Go 命令行客户端，通过 `/v3/<action>` REST API 工作，不依赖 MCP 传输。源码位于 `clients/corekg-cli`，复用仓库根 `go.mod` 和 `vendor`，可编译为单独二进制。

## 当前能力

```text
corekg-cli version
corekg-cli config init|path
corekg-cli profile list|show|use|rename|delete
corekg-cli auth login|import|list|status|logout
corekg-cli kb create|list|use
corekg-cli file list|upload
corekg-cli ask [QUESTION...]
```

认证支持两种方式：

- `auth login` 启动浏览器 Device Authorization。浏览器内完成账户登录和组织选择，CLI 只接收服务端为该组织创建的 API Key。
- `auth import` 从终端、标准输入或显式环境变量导入已有 API Key，适合 CI 和无浏览器环境。

文件列表、文件上传和知识库问答已支持；当前 CLI 不提供 MCP Tool 透传。

## 快速开始

构建当前平台二进制：

```sh
make corekg-cli
./bundles/corekg-cli version
```

初始化默认配置（只在本机写入 `~/.corekg/config.json`）：

```sh
corekg-cli config init
```

交互式浏览器登录：

```sh
corekg-cli auth login
corekg-cli auth status
```

无浏览器环境可以先申请设备码，再在另一台设备打开 URL 完成授权：

```sh
corekg-cli auth login --no-wait -o json
corekg-cli auth login --device-code <DEVICE_CODE>
```

导入已有 API Key：

```sh
corekg-cli auth import \
	--name work \
  --api-key-env COREKG_API_KEY
```

选择组织对应的 Profile 和知识库：

```sh
corekg-cli profile list
corekg-cli profile use work
corekg-cli kb list
corekg-cli kb use 20001

# 交互创建知识库；依次填写名称、描述和头像地址后确认
corekg-cli kb create

# 创建后切换为当前知识库；类型固定为 file
corekg-cli kb create "Product Docs" --use

# 脚本模式：跳过交互和确认，-o id 输出新知识库 ID
corekg-cli kb create "CI Docs" --description "持续集成资料" --yes -o id

# 文件和知识库问答
corekg-cli file list
corekg-cli file upload ./handbook.pdf --wait
corekg-cli ask "这个知识库的核心结论是什么？"
corekg-cli ask --new "换一个角度总结"
```

上传和问答分别支持 `--upload-timeout`、`--ask-timeout`；命令专用参数优先于全局 `--timeout`。

## 本地配置

默认配置目录为 `~/.corekg`：

- `config.json` 保存 API Server、前端地址、默认输出和超时。
- `state.json` 保存 Profile、当前 Profile、知识库选择和每个知识库最近的聊天会话。
- `auth.json` 保存 Credential、API Key 元数据和未完成的设备登录状态。
- 目录默认 `0700`，JSON 文件默认 `0600`，写入采用锁、临时文件和原子替换。

Profile 是 CLI 的本地执行上下文，一个 Profile 对应一个 Server、Credential 和组织身份。多组织用户为每个组织重复 `auth login` 并使用不同 Profile；命令行公开参数是 `--profile`，只影响当前命令，也可用 `COREKG_PROFILE` 设置一次性选择。其他命令默认读取 `~/.corekg/config.json`，也可通过全局 `--config` 指定配置文件或 JSON 字符串；该配置只对本次运行生效。

详细设计、认证协议、配置格式、命令示例和故障排查见：

- [架构设计](docs/architecture.md)
- [使用说明](docs/user-guide.md)

## 构建和测试

```sh
make corekg-cli
make corekg-cli-test
make corekg-cli-linux GOARCH=amd64
VERSION=0.1.0 make corekg-cli-npm-preflight
```

当前 npm 目录生成单一 `@insmtx/corekg-cli` 包，内含各支持平台的二进制，尚未表示已发布到公共 registry。直接使用 Go 二进制不需要 Node.js。
