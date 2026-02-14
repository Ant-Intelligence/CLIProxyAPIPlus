# cpa-client

CLIProxyAPI Plus 管理 API 的命令行客户端。用于远程管理代理服务器上的凭据、查看帐号用量和健康状态。

## 安装

从源码编译：

```bash
go build -o cpa-client ./cmd/client
```

## 快速开始

```bash
# 1. 保存服务器连接配置（只需执行一次）
cpa-client config --server https://your-server.example.com --api-key YOUR_KEY

# 2. 之后所有命令自动使用已保存的配置
cpa-client auth-health
cpa-client kiro-usage
cpa-client upload-kiro --file credential.json
```

## 全局参数

所有子命令均支持以下参数，可覆盖已保存的配置：

| 参数 | 说明 |
|------|------|
| `--server` | 服务器地址，例如 `https://your-server.example.com` |
| `--api-key` | 管理 API 密钥 |

参数优先级：命令行参数 > 已保存的配置文件。

## 命令

- [`config`](config.md) — 保存或查看服务器连接配置
- [`upload-kiro`](upload-kiro.md) — 上传 Kiro 凭据到服务器
- [`kiro-usage`](kiro-usage.md) — 查看 Kiro 帐号额度用量
- [`auth-health`](auth-health.md) — 检查 OAuth 帐号健康状态

## 配置文件

配置保存在 `~/.cli-proxy-api/client.yaml`，格式：

```yaml
server: https://your-server.example.com
api-key: YOUR_KEY
```

文件权限为 `0600`，目录权限为 `0700`。
