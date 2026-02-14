# upload-kiro

上传 Kiro (AWS CodeWhisperer) 凭据到代理服务器。支持两种凭据格式和多种输入方式。

## 用法

```bash
cpa-client upload-kiro [flags]
```

## 参数

| 参数 | 短写 | 说明 |
|------|------|------|
| `--file` | `-f` | Kiro Account Manager 格式的 JSON 文件路径 |
| `--token-file` | `-t` | AWS SSO token 文件路径 |
| `--client-file` | `-c` | AWS SSO client 文件路径（可省略，从 clientIdHash 自动发现） |
| `--name` | `-n` | 指定上传的目标文件名（覆盖自动命名） |

`--file` 和 `--token-file` 互斥，不能同时使用。

## 输入模式

### 模式 1：Kiro Account Manager 格式

适用于从 Kiro Account Manager 浏览器扩展导出的凭据。JSON 格式使用 camelCase 字段名。

**从文件上传（推荐）：**

```bash
cpa-client upload-kiro --file credential.json
```

**从 stdin 读取：**

```bash
cat credential.json | cpa-client upload-kiro
```

> **注意：** 不建议手动粘贴 JSON 到 stdin，因为超长字段（如 `clientSecret`）在终端中会被换行，
> 导致 JSON 字符串内出现非法换行符。请先保存为文件再用 `--file` 上传。

**JSON 格式 — 单个凭据：**

```json
{
  "refreshToken": "aorAAAA...",
  "clientId": "f_R3uaW3...",
  "clientSecret": "eyJraWQi...",
  "region": "us-east-1",
  "startUrl": "https://d-9066017a60.awsapps.com/start/",
  "provider": "Enterprise",
  "machineId": ""
}
```

**JSON 格式 — 多个凭据（数组）：**

```json
[
  { "refreshToken": "...", "clientId": "c1", ... },
  { "refreshToken": "...", "clientId": "c2", ... }
]
```

### 模式 2：AWS SSO 缓存文件

适用于直接从 `~/.aws/sso/cache/` 读取 Kiro 的 SSO token 文件。

**自动发现 client 文件：**

```bash
cpa-client upload-kiro --token-file ~/.aws/sso/cache/kiro-auth-token.json
```

程序会从 token 文件中读取 `clientIdHash`，然后在同目录下查找 `{clientIdHash}.json` 作为 client 文件。

**手动指定 client 文件：**

```bash
cpa-client upload-kiro \
  --token-file ./token.json \
  --client-file ./client.json
```

**指定上传文件名：**

```bash
cpa-client upload-kiro \
  --token-file ~/.aws/sso/cache/kiro-auth-token.json \
  --name kiro-idc.json
```

SSO 模式默认文件名为 `kiro-idc.json`。

## 文件命名规则

上传到服务器的目标文件名按以下优先级确定：

| 优先级 | 条件 | 文件名 | 示例 |
|--------|------|--------|------|
| 1 | 指定了 `--name` | 使用指定名称 | `my-kiro.json` |
| 2 | 使用 `--file` 且未指定 `--name` | 使用源文件名 | `1.json` |
| 3 | stdin 且凭据有 email | `kiro-{provider}-{email}.json` | `kiro-enterprise-user-example.com.json` |
| 4 | stdin 且凭据有 startUrl | `kiro-{provider}-{domain}-{clientId前8位}.json` | `kiro-enterprise-d-9066017a60.awsapps.com-f-r3uaw3.json` |
| 5 | stdin 兜底 | `kiro-{provider}-{序号}.json` | `kiro-unknown-0.json` |

**多凭据文件：** 当一个文件包含多个凭据（JSON 数组）时，文件名会自动追加序号后缀以避免覆盖：

```
credential.json（含 3 个凭据）→ credential-1.json, credential-2.json, credential-3.json
```

## 完整示例

```bash
# 首先保存配置
cpa-client config --server https://ccplus.example.com --api-key MyKey123

# 上传单个凭据文件
cpa-client upload-kiro --file auths/1.json

# 上传多个文件
cpa-client upload-kiro --file auths/1.json
cpa-client upload-kiro --file auths/2.json

# 指定目标文件名
cpa-client upload-kiro --file auths/1.json --name kiro-alice.json

# 从 AWS SSO 缓存上传
cpa-client upload-kiro --token-file ~/.aws/sso/cache/kiro-auth-token.json

# 临时使用不同服务器
cpa-client upload-kiro --file auths/1.json --server https://other-server.com --api-key OtherKey
```

## 凭据转换

上传时，Kiro Account Manager 格式（camelCase）会自动转换为服务器内部格式（snake_case）：

| 输入字段 | 内部字段 | 说明 |
|----------|----------|------|
| `refreshToken` | `refresh_token` | 刷新令牌 |
| `clientId` | `client_id` | OAuth 客户端 ID |
| `clientSecret` | `client_secret` | OAuth 客户端密钥 |
| `region` | `region` | AWS 区域 |
| `startUrl` | `start_url` | SSO 起始地址 |
| `provider` | `provider` | 提供商类型 |

转换后还会自动设置：
- `type`: `"kiro"`
- `auth_method`: `"idc"`
- `expires_at`: `"2020-01-01T00:00:00Z"`（过期值，触发服务器立即刷新令牌）
