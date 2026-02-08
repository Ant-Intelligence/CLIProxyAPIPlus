# Kiro (AWS CodeWhisperer) 账号配置指南

## 概述

本文档详细说明如何在 CLIProxyAPI Plus 中添加和配置 Kiro (AWS CodeWhisperer) 账号。Kiro 提供了灵活的认证方式,支持本地 OAuth 登录和远程管理两种配置模式。

**关键概念**:
- **本地登录模式**: 在运行 CLIProxyAPI 的机器上直接执行 OAuth 登录流程
- **远程管理模式**: 通过管理 API 将已有的认证配置上传到远程服务器

## 目录

- [认证方式对比](#认证方式对比)
- [方式一: 本地 OAuth 登录 (推荐)](#方式一-本地-oauth-登录-推荐)
  - [Google OAuth 登录](#1-google-oauth-登录)
  - [AWS Builder ID 设备流程登录](#2-aws-builder-id-设备流程登录)
  - [AWS Builder ID 授权码登录 (推荐)](#3-aws-builder-id-授权码登录-推荐)
  - [从 Kiro IDE 导入](#4-从-kiro-ide-导入)
- [方式二: 远程管理 API 配置](#方式二-远程管理-api-配置)
  - [上传本地已有配置](#上传本地已有配置)
  - [使用管理界面](#使用管理界面)
- [验证配置](#验证配置)
- [配置管理](#配置管理)
- [常见问题](#常见问题)

## 认证方式对比

| 特性 | 本地 OAuth 登录 | 远程管理 API |
|------|----------------|--------------|
| **适用场景** | 服务器首次配置 | 已有配置迁移、批量管理 |
| **需要浏览器** | ✅ 是 | ❌ 否 |
| **自动刷新** | ✅ 自动设置 | ⚠️ 需确保配置完整 |
| **操作复杂度** | ⭐ 简单 | ⭐⭐ 中等 |
| **推荐度** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |

**选择建议**:
- 🎯 **首次配置**: 使用本地 OAuth 登录,一键完成所有配置
- 🔄 **配置迁移**: 使用远程管理 API 上传已有配置文件
- 👥 **多服务器**: 在一台机器上登录后,通过管理 API 分发到其他服务器

---

## 方式一: 本地 OAuth 登录 (推荐)

本地 OAuth 登录会在运行 CLIProxyAPI 的机器上启动 OAuth 流程,完成后自动将认证信息保存到 `auth-dir` 目录(默认: `~/.cli-proxy-api/`)。

### 前置要求

1. **已构建 CLIProxyAPI**:
   ```bash
   go build -o CLIProxyAPI ./cmd/server
   ```

2. **浏览器环境**:
   - 本地机器能够打开浏览器
   - 服务器环境可使用 `-no-browser` 参数手动复制链接

3. **网络连接**:
   - 能够访问 Google OAuth 或 AWS SSO 服务

### 1. Google OAuth 登录

使用 Google 账号通过 OAuth2 登录 Kiro。

```bash
# 基本用法
./CLIProxyAPI -kiro-login

# 不自动打开浏览器 (服务器环境推荐)
./CLIProxyAPI -kiro-login -no-browser

# 使用隐私模式
./CLIProxyAPI -kiro-login -incognito
```

**执行流程**:
1. 启动本地 HTTP 服务器 (默认端口 9876)
2. 打开浏览器访问 Google OAuth 授权页面
3. 用户登录 Google 账号并授权
4. 回调本地服务器获取授权码
5. 交换授权码获取 access token 和 refresh token
6. 保存到 `~/.cli-proxy-api/kiro-google-{timestamp}.json`

**优点**:
- ✅ 简单快速,一键完成
- ✅ 支持大多数用户

**限制**:
- ⚠️ 需要 Google 账号
- ⚠️ 可能受企业防火墙限制

### 2. AWS Builder ID 设备流程登录

使用 AWS Builder ID 通过设备授权流程登录。

```bash
./CLIProxyAPI -kiro-aws-login
```

**执行流程**:
1. 注册客户端获取 `client_id` 和 `client_secret`
2. 启动设备授权流程,获取用户码和验证 URL
3. 在终端显示用户码和验证链接
4. 用户在浏览器中访问链接并输入用户码
5. 轮询等待用户完成授权
6. 获取 access token 和 refresh token
7. 保存到 `~/.cli-proxy-api/kiro-aws-{timestamp}.json`

**终端输出示例**:
```
🔐 AWS Builder ID Device Flow Login
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Please visit: https://device.sso.us-east-1.amazonaws.com/

And enter code: ABCD-1234

⏳ Waiting for authorization...
```

**优点**:
- ✅ 适合无图形界面的服务器环境
- ✅ 使用 AWS 官方认证体系

**限制**:
- ⚠️ 需要手动输入验证码
- ⚠️ 用户体验相对繁琐

### 3. AWS Builder ID 授权码登录 (推荐)

使用 AWS Builder ID 通过授权码流程登录,提供更好的用户体验。

```bash
# 基本用法
./CLIProxyAPI -kiro-aws-authcode

# 不自动打开浏览器
./CLIProxyAPI -kiro-aws-authcode -no-browser

# 使用隐私模式
./CLIProxyAPI -kiro-aws-authcode -incognito
```

**执行流程**:
1. 注册客户端获取 `client_id` 和 `client_secret`
2. 启动本地 HTTP 服务器接收回调
3. 打开浏览器访问 AWS SSO 授权页面
4. 用户登录 AWS Builder ID 并授权
5. AWS 回调本地服务器传递授权码
6. 使用授权码交换 access token 和 refresh token
7. 保存到 `~/.cli-proxy-api/kiro-aws-{timestamp}.json`

**优点**:
- ✅ 用户体验最佳,无需手动输入验证码
- ✅ 使用 AWS 官方认证体系
- ✅ 支持浏览器自动跳转

**推荐理由**:
- 🎯 最接近原生 AWS CLI 的体验
- 🎯 适合团队成员频繁登录的场景
- 🎯 支持企业 SSO 集成

### 4. 从 Kiro IDE 导入

如果你已经在 Kiro IDE 或 AWS Toolkit 中登录过,可以直接导入现有配置。

```bash
./CLIProxyAPI -kiro-import
```

**执行流程**:
1. 查找本地 Kiro 配置文件:
   - `~/.aws/sso/cache/kiro-auth-token.json`
   - `~/.aws/sso/cache/{client_id_hash}.json`
2. 读取并合并配置
3. 验证配置有效性
4. 复制到 `~/.cli-proxy-api/kiro-import-{timestamp}.json`

**优点**:
- ✅ 无需重新登录
- ✅ 直接使用现有配置

**限制**:
- ⚠️ 需要已有的 Kiro 配置
- ⚠️ 配置必须仍然有效

### 登录参数说明

所有登录命令都支持以下通用参数:

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-no-browser` | 不自动打开浏览器,手动复制链接访问 | `false` |
| `-incognito` | 使用浏览器隐私模式 | `false` |
| `-config` | 指定配置文件路径 | `config.yaml` |

**示例**:

```bash
# 服务器环境: 不打开浏览器,手动复制链接
./CLIProxyAPI -kiro-aws-authcode -no-browser

# 多账号切换: 使用隐私模式避免自动登录
./CLIProxyAPI -kiro-login -incognito

# 自定义配置: 指定配置文件
./CLIProxyAPI -kiro-login -config /etc/cliproxy/config.yaml
```

### 保存的配置文件格式

本地 OAuth 登录完成后,会在 `auth-dir` 目录下生成 JSON 配置文件:

```json
{
  "type": "kiro",
  "access_token": "eyJraWQ...",
  "refresh_token": "eyJjdHk...",
  "expires_at": "2026-02-07T15:30:00Z",
  "auth_method": "IdC",
  "provider": "Enterprise",
  "region": "us-east-1",
  "client_id": "amzn1.application-oa2-client...",
  "client_secret": "amzn1.oa2-cs...",
  "client_id_hash": "a1b2c3d4e5f6..."
}
```

**字段说明**:
- `type`: 固定值 `"kiro"`,用于提供商识别
- `access_token`: 访问令牌,用于 API 请求
- `refresh_token`: 刷新令牌,用于自动续期
- `expires_at`: 令牌过期时间 (ISO 8601 格式)
- `auth_method`: 认证方法 (`IdC` = AWS Identity Center)
- `provider`: 提供商类型 (如 `Enterprise`, `Google`)
- `region`: AWS 区域
- `client_id`: OAuth 客户端 ID
- `client_secret`: OAuth 客户端密钥
- `client_id_hash`: 客户端 ID 哈希值

---

## 方式二: 远程管理 API 配置

如果你已经部署了 CLIProxyAPI 服务器 (如 `https://accdev3.ai-code.club`),可以通过管理 API 远程上传配置,而不需要在服务器上执行登录命令。

### 前置要求

1. **服务器配置**:
   - 已启用远程管理: `config.yaml` 中设置 `allow-remote: true`
   - 配置管理密钥: `remote-management.password`

2. **管理密钥**:
   ```yaml
   # config.yaml
   remote-management:
     allow-remote: true
     password: "your-management-key"
   ```

3. **工具依赖**:
   - `curl`: HTTP 请求工具
   - `jq`: JSON 处理工具 (可选,用于格式转换)

### 上传本地已有配置

如果你在本地机器上已经通过 `-kiro-login` 等命令登录过,可以将配置上传到远程服务器。

#### 方法 1: 直接上传完整配置 (推荐)

假设你的本地配置文件是 `~/.cli-proxy-api/kiro-aws-xxx.json`:

```bash
# 上传配置文件
curl -X POST "https://accdev3.ai-code.club/v0/management/auth-files?name=kiro-aws-smoky-doozy-device@duck.com.json" \
  -H "Authorization: Bearer AiCode_202668" \
  -H "Content-Type: application/json" \
  -d @/root/.cli-proxy-api/kiro-aws-smoky-doozy-device@duck.com.json
```

#### 方法 2: 从 AWS SSO 缓存上传 (需要格式转换)

如果从 `~/.aws/sso/cache/` 导入配置,需要转换格式:

**⚠️ 重要**: AWS SSO 使用 camelCase 命名,CLIProxyAPI 使用 snake_case 命名,必须转换!

**使用转换脚本** (推荐):

```bash
# 下载转换脚本
chmod +x convert-and-upload-kiro.sh

# 执行上传
./convert-and-upload-kiro.sh https://accdev3.ai-code.club your-management-key kiro-production.json
```

**手动使用 jq 转换**:

```bash
curl -X POST "https://accdev3.ai-code.club/v0/management/auth-files?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key" \
  -H "Content-Type: application/json" \
  -d "$(
    CLIENT_HASH=$(jq -r .clientIdHash ~/.aws/sso/cache/kiro-auth-token.json)
    jq -n \
      --arg type 'kiro' \
      --slurpfile token ~/.aws/sso/cache/kiro-auth-token.json \
      --slurpfile client ~/.aws/sso/cache/\${CLIENT_HASH}.json \
      '{
        type: \$type,
        access_token: \$token[0].accessToken,
        refresh_token: \$token[0].refreshToken,
        expires_at: \$token[0].expiresAt,
        auth_method: \$token[0].authMethod,
        provider: \$token[0].provider,
        region: \$token[0].region,
        client_id: \$client[0].clientId,
        client_secret: \$client[0].clientSecret,
        client_id_hash: \$token[0].clientIdHash
      }'
  )"
```

**格式转换对照表**:

| AWS SSO 格式 (camelCase) | CLIProxyAPI 格式 (snake_case) |
|-------------------------|------------------------------|
| `accessToken` | `access_token` |
| `refreshToken` | `refresh_token` |
| `expiresAt` | `expires_at` |
| `authMethod` | `auth_method` |
| `clientId` | `client_id` |
| `clientSecret` | `client_secret` |
| `clientIdHash` | `client_id_hash` |
| ❌ 缺少 | ✅ `"type": "kiro"` (必须) |

### 使用管理界面

CLIProxyAPI Plus 还提供了 Web 管理界面 (如果启用):

1. **访问管理界面**:
   ```
   https://accdev3.ai-code.club/v0/management/ui
   ```

2. **输入管理密钥**:
   ```
   输入密钥: your-management-key
   ```

3. **上传配置文件**:
   - 点击 "Auth Files" 标签
   - 点击 "Upload New File"
   - 选择本地配置文件
   - 点击 "Upload"

### 管理 API 参考

#### 列出所有认证文件

```bash
curl -X GET "https://accdev3.ai-code.club/v0/management/auth-files" \
  -H "Authorization: Bearer your-management-key"
```

**响应示例**:
```json
{
  "files": [
    {
      "name": "kiro-production.json",
      "provider": "kiro",
      "type": "kiro",
      "models": ["amazon.nova-pro-v1:0", "amazon.nova-lite-v1:0"],
      "disabled": false
    }
  ]
}
```

#### 下载认证文件

```bash
curl -X GET "https://accdev3.ai-code.club/v0/management/auth-files/download?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key" \
  > kiro-backup.json
```

#### 删除认证文件

```bash
curl -X DELETE "https://accdev3.ai-code.club/v0/management/auth-files?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key"
```

#### 禁用/启用认证文件

```bash
# 禁用
curl -X PATCH "https://accdev3.ai-code.club/v0/management/auth-files/status?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key" \
  -H "Content-Type: application/json" \
  -d '{"disabled": true}'

# 启用
curl -X PATCH "https://accdev3.ai-code.club/v0/management/auth-files/status?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key" \
  -H "Content-Type: application/json" \
  -d '{"disabled": false}'
```

---

## 验证配置

无论使用哪种方式添加配置,都应该验证配置是否成功。

### 1. 检查配置文件是否存在

**本地登录方式**:
```bash
ls -la ~/.cli-proxy-api/
```

**远程管理方式**:
```bash
curl -s "https://accdev3.ai-code.club/v0/management/auth-files" \
  -H "Authorization: Bearer your-management-key" | jq '.files[] | select(.provider == "kiro")'
```

### 2. 查看支持的模型

```bash
curl -s "https://accdev3.ai-code.club/v0/management/auth-files/models?name=kiro-production.json" \
  -H "Authorization: Bearer your-management-key" | jq .
```

**响应示例**:
```json
{
  "name": "kiro-production.json",
  "provider": "kiro",
  "models": [
    "amazon.nova-pro-v1:0",
    "amazon.nova-lite-v1:0",
    "amazon.nova-micro-v1:0"
  ]
}
```

### 3. 测试 API 请求

使用 OpenAI 兼容的 API 格式测试:

```bash
curl -X POST "https://accdev3.ai-code.club/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "kiro:production/amazon.nova-lite-v1:0",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

**模型名称格式**:
```
kiro:{prefix}/{model-id}
```

其中:
- `kiro`: 固定的提供商前缀
- `{prefix}`: 配置文件名去掉 `.json` 后缀 (如 `production`)
- `{model-id}`: AWS CodeWhisperer 模型 ID (如 `amazon.nova-lite-v1:0`)

### 4. 查看服务器日志

如果配置成功,服务器日志会显示:

```
INFO[2026-02-07T10:30:00Z] Loading credentials from auth files...
INFO[2026-02-07T10:30:00Z] Loaded kiro credential: production (provider: Enterprise, region: us-east-1)
INFO[2026-02-07T10:30:00Z] Available models: [amazon.nova-pro-v1:0 amazon.nova-lite-v1:0]
```

---

## 配置管理

### 自动刷新令牌

CLIProxyAPI Plus 会自动管理 Kiro token 的刷新:

- **后台刷新管理器**: 监控所有 token 的过期时间
- **提前刷新**: 在过期前 5 分钟自动刷新
- **失败重试**: 使用指数退避策略重试
- **持久化**: 刷新后的 token 自动保存到文件

**查看刷新状态**:
服务器日志会显示刷新活动:
```
INFO[2026-02-07T10:25:00Z] kiro: token will expire in 4m30s, refreshing...
INFO[2026-02-07T10:25:01Z] kiro: token refreshed successfully, new expiry: 2026-02-07T11:25:00Z
```

### 多账号管理

你可以添加多个 Kiro 配置文件:

```bash
# 添加生产环境配置
./CLIProxyAPI -kiro-aws-authcode
# 保存为: kiro-aws-20260207-1.json

# 添加测试环境配置
./CLIProxyAPI -kiro-login
# 保存为: kiro-google-20260207-2.json
```

**重命名配置文件** (建议):
```bash
cd ~/.cli-proxy-api/
mv kiro-aws-20260207-1.json kiro-production.json
mv kiro-google-20260207-2.json kiro-staging.json
```

**使用不同配置**:
```bash
# 使用生产环境配置
curl -X POST "http://localhost:19000/v1/chat/completions" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "kiro:production/amazon.nova-pro-v1:0",
    ...
  }'

# 使用测试环境配置
curl -X POST "http://localhost:19000/v1/chat/completions" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "kiro:staging/amazon.nova-lite-v1:0",
    ...
  }'
```

### 配置文件优先级

当启用 `routing.strategy: round-robin` 时,多个配置会轮流使用。你可以为配置设置优先级:

**方法 1: 文件名排序**
配置按文件名字母顺序排序:
```
kiro-1-high-priority.json   # 优先级最高
kiro-2-medium-priority.json
kiro-3-low-priority.json    # 优先级最低
```

**方法 2: 禁用低优先级配置**
临时禁用某些配置:
```bash
curl -X PATCH "http://localhost:19000/v0/management/auth-files/status?name=kiro-low-priority.json" \
  -H "Authorization: Bearer your-management-key" \
  -d '{"disabled": true}'
```

### 配置热重载

CLIProxyAPI Plus 支持配置文件的热重载,无需重启服务器:

- **自动监控**: 文件系统监控 `auth-dir` 目录变化
- **即时生效**: 新增、修改、删除配置立即生效
- **无中断**: 不影响正在进行的请求

**手动触发重载** (如果自动监控未启用):
```bash
# 发送 HUP 信号
kill -HUP $(pgrep CLIProxyAPI)

# 或使用管理 API
curl -X POST "http://localhost:19000/v0/management/reload" \
  -H "Authorization: Bearer your-management-key"
```

---

## 常见问题

### Q1: OAuth 登录时提示端口被占用

**错误**:
```
failed to start callback server: address already in use
```

**原因**: 默认端口 9876 已被其他程序占用

**解决**:
系统会自动尝试使用动态端口,如果仍然失败:
1. 检查占用进程: `lsof -i :9876`
2. 停止占用进程或使用 `-no-browser` 参数

### Q2: 浏览器未自动打开

**现象**: 执行登录命令后浏览器没有打开

**解决**:
```bash
# 使用 -no-browser 参数,手动复制链接
./CLIProxyAPI -kiro-login -no-browser

# 控制台会显示授权链接,手动复制到浏览器打开
Please visit the following URL to authorize:
https://accounts.google.com/o/oauth2/v2/auth?client_id=...
```

### Q3: Token 刷新失败

**错误**:
```
failed to refresh kiro token: invalid_grant
```

**原因**:
- Refresh token 已过期或被撤销
- 客户端密钥已更改
- 网络连接问题

**解决**:
重新执行登录流程:
```bash
./CLIProxyAPI -kiro-aws-authcode
```

### Q4: 上传配置后提示 "provider: unknown"

**原因**: 配置文件格式不正确,缺少 `"type": "kiro"` 字段或使用了错误的命名格式

**解决**:
使用转换脚本确保格式正确:
```bash
./convert-and-upload-kiro.sh
```

或手动检查配置:
```bash
# 下载配置文件
curl -s "https://accdev3.ai-code.club/v0/management/auth-files/download?name=your-file.json" \
  -H "Authorization: Bearer your-key" | jq .

# 确保包含以下字段 (snake_case):
# - type: "kiro"
# - access_token
# - refresh_token
# - client_id
# - client_secret
```

### Q5: API 请求返回 401 Unauthorized

**可能原因**:

1. **API Key 错误**:
   - 检查请求头: `Authorization: Bearer your-api-key`
   - 确认 API key 在 `config.yaml` 的 `api-keys` 列表中

2. **Token 已过期**:
   - 检查日志是否有刷新失败的消息
   - 重新登录获取新 token

3. **模型名称错误**:
   - 格式: `kiro:{prefix}/{model-id}`
   - 确认 prefix 和 model-id 正确

### Q6: 导入 Kiro IDE 配置失败

**错误**:
```
failed to read token file: no such file or directory
```

**原因**: `~/.aws/sso/cache/kiro-auth-token.json` 不存在

**解决**:
1. 确认已在 Kiro IDE 或 AWS Toolkit 中登录
2. 检查文件是否存在:
   ```bash
   ls -la ~/.aws/sso/cache/
   ```
3. 如果不存在,使用其他登录方式

### Q7: 企业防火墙阻止 OAuth 请求

**现象**: OAuth 流程卡在授权步骤,浏览器无法访问授权 URL

**解决**:
1. **使用 AWS Builder ID** (可能有更好的企业支持):
   ```bash
   ./CLIProxyAPI -kiro-aws-authcode
   ```

2. **配置代理**:
   ```yaml
   # config.yaml
   sdk:
     http-proxy: "http://proxy.company.com:8080"
     https-proxy: "http://proxy.company.com:8080"
   ```

3. **联系 IT 部门**:
   - 请求白名单: `*.amazonaws.com`, `*.google.com`
   - 或使用已登录的配置通过管理 API 上传

### Q8: 多个配置如何选择?

**问题**: 添加了多个 Kiro 配置,如何控制使用哪一个?

**解决**:

1. **使用模型名称指定** (推荐):
   ```bash
   # 使用 production 配置
   "model": "kiro:production/amazon.nova-pro-v1:0"

   # 使用 staging 配置
   "model": "kiro:staging/amazon.nova-lite-v1:0"
   ```

2. **配置路由策略**:
   ```yaml
   # config.yaml
   routing:
     strategy: "round-robin"  # 轮流使用所有配置
     # 或
     strategy: "fill-first"   # 优先使用第一个配置,失败才用下一个
   ```

3. **禁用不需要的配置**:
   ```bash
   curl -X PATCH "http://localhost:19000/v0/management/auth-files/status?name=kiro-old.json" \
     -H "Authorization: Bearer your-key" \
     -d '{"disabled": true}'
   ```

---

## 安全建议

1. **保护管理密钥**:
   ```bash
   # 不要在命令历史中暴露密钥
   export MANAGEMENT_KEY="your-key"
   curl ... -H "Authorization: Bearer $MANAGEMENT_KEY"

   # 使用配置文件
   chmod 600 config.yaml
   ```

2. **使用 HTTPS**:
   ```bash
   # 生产环境必须使用 HTTPS
   https://your-server.com/v0/management/...

   # 不要在不安全的网络上使用 HTTP
   ```

3. **限制访问**:
   ```yaml
   # config.yaml
   remote-management:
     allow-remote: true
     allowed-ips:
       - "192.168.1.0/24"  # 仅允许内网访问
   ```

4. **定期轮换凭证**:
   ```bash
   # 每 90 天重新登录
   ./CLIProxyAPI -kiro-aws-authcode

   # 删除旧配置
   curl -X DELETE "http://localhost:19000/v0/management/auth-files?name=kiro-old.json" \
     -H "Authorization: Bearer your-key"
   ```

5. **备份配置**:
   ```bash
   # 定期备份认证配置
   tar -czf kiro-config-backup-$(date +%Y%m%d).tar.gz ~/.cli-proxy-api/
   ```

---

## 总结

### 推荐工作流

**场景 1: 新部署服务器**
```bash
# 1. 构建服务器
go build -o CLIProxyAPI ./cmd/server

# 2. 执行 OAuth 登录
./CLIProxyAPI -kiro-aws-authcode

# 3. 启动服务器
./CLIProxyAPI

# 4. 测试 API
curl -X POST "http://localhost:19000/v1/chat/completions" ...
```

**场景 2: 多服务器部署**
```bash
# 在第一台服务器上登录
./CLIProxyAPI -kiro-aws-authcode

# 上传到其他服务器
curl -X POST "https://server2.com/v0/management/auth-files?name=kiro.json" \
  -H "Authorization: Bearer management-key" \
  -d @~/.cli-proxy-api/kiro-aws-*.json

curl -X POST "https://server3.com/v0/management/auth-files?name=kiro.json" \
  -H "Authorization: Bearer management-key" \
  -d @~/.cli-proxy-api/kiro-aws-*.json
```

**场景 3: 配置迁移**
```bash
# 从本地 AWS SSO 缓存迁移
./convert-and-upload-kiro.sh https://your-server.com your-key kiro.json

# 验证
curl "https://your-server.com/v0/management/auth-files" \
  -H "Authorization: Bearer your-key" | jq '.files[] | select(.provider == "kiro")'
```

### 快速参考

```bash
# 本地登录 (推荐)
./CLIProxyAPI -kiro-aws-authcode

# 远程上传
curl -X POST "https://server/v0/management/auth-files?name=kiro.json" \
  -H "Authorization: Bearer key" \
  -d @config.json

# 验证配置
curl "https://server/v0/management/auth-files" \
  -H "Authorization: Bearer key"

# 测试 API
curl -X POST "https://server/v1/chat/completions" \
  -H "Authorization: Bearer api-key" \
  -d '{"model": "kiro:prefix/model", ...}'
```

### 相关文档

- [Kiro 配置上传指南](../KIRO_UPLOAD.md) - 配置上传详细说明
- [路由策略配置](./routing-strategy_CN.md) - 多配置路由策略
- [管理 API 文档](./CLAUDE.md) - 完整的管理 API 参考
- [Docker 部署指南](./docker-postgres-deployment.md) - Docker 环境配置

---

**文档版本**: v1.0
**最后更新**: 2026-02-07
**适用版本**: CLIProxyAPI Plus v6.x
