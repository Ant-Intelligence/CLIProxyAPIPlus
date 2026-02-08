# Antigravity 账号配置指南

## 概述

本文档详细说明如何在 CLIProxyAPI Plus 中添加和使用 Antigravity 账号。Antigravity 是 Google Deepmind 团队开发的强大 AI 编码助手，通过 Google OAuth 认证提供访问。

**关键特性**:
- 支持 Gemini 系列模型（包括 Gemini 2.5/3.0 系列）
- 支持 Claude 系列模型（通过 Antigravity 代理）
- 兼容 OpenAI、Claude、Gemini 三种 API 格式
- 支持流式和非流式响应
- 支持思维链（Thinking）功能

## 目录

- [认证方式](#认证方式)
- [快速开始](#快速开始)
- [API 接口说明](#api-接口说明)
- [支持的模型](#支持的模型)
- [API 格式支持](#api-格式支持)
- [使用示例](#使用示例)
- [配置管理](#配置管理)
- [常见问题](#常见问题)

---

## 认证方式

Antigravity 使用 Google OAuth 2.0 进行认证，需要以下权限：
- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`
- `https://www.googleapis.com/auth/userinfo.profile`
- `https://www.googleapis.com/auth/cclog`
- `https://www.googleapis.com/auth/experimentsandconfigs`

---

## 快速开始

### 1. 执行 OAuth 登录

在运行 CLIProxyAPI 的机器上执行：

```bash
./CLIProxyAPI -antigravity-login
```

**可选参数**:
```bash
# 不自动打开浏览器
./CLIProxyAPI -antigravity-login -no-browser

# 使用无痕模式（多账号切换）
./CLIProxyAPI -antigravity-login -incognito

# 自定义回调端口（默认: 51121）
./CLIProxyAPI -antigravity-login -oauth-callback-port 8888
```

### 2. 完成浏览器授权

命令执行后会：
1. 自动打开浏览器访问 Google OAuth 授权页面
2. 使用您的 Google 账号登录并授权
3. 授权成功后，认证信息自动保存到 `~/.cli-proxy-api/antigravity_*.json`

### 3. 启动服务器

```bash
./CLIProxyAPI -config config.yaml
```

服务器启动后会自动加载 Antigravity 认证信息。

---

## API 接口说明

### 基础信息

- **服务器地址**: `http://your-server:8317` (默认端口)
- **认证方式**: API Key (在请求头中添加 `Authorization: Bearer YOUR_API_KEY`)
- **支持的 API 格式**: OpenAI、Claude、Gemini

### 主要端点

#### 1. 获取模型列表

```http
GET /v1/models
Authorization: Bearer YOUR_API_KEY
```

**响应示例**:
```json
{
  "object": "list",
  "data": [
    {
      "id": "gemini-2.5-flash",
      "object": "model",
      "created": 1704067200,
      "owned_by": "antigravity",
      "type": "antigravity"
    },
    {
      "id": "gemini-3-pro-high",
      "object": "model",
      "created": 1704067200,
      "owned_by": "antigravity",
      "type": "antigravity"
    }
  ]
}
```

#### 2. 聊天补全（OpenAI 格式）

```http
POST /v1/chat/completions
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

#### 3. 聊天补全（Claude 格式）

```http
POST /v1/messages
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
```

#### 4. 聊天补全（Gemini 格式）

```http
POST /v1beta/models/{model}:generateContent
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

---

## 支持的模型

### Gemini 系列

| 模型名称 | 说明 | 上下文长度 | 思维链支持 |
|---------|------|-----------|-----------|
| `gemini-2.5-flash` | Gemini 2.5 Flash | 1,048,576 | ✅ (0-24576) |
| `gemini-2.5-flash-lite` | Gemini 2.5 Flash Lite | 1,048,576 | ✅ (0-24576) |
| `gemini-3-pro-high` | Gemini 3 Pro High | 2,097,152 | ✅ (128-32768) |
| `gemini-3-pro-image` | Gemini 3 Pro Image | 2,097,152 | ✅ (128-32768) |
| `gemini-3-flash` | Gemini 3 Flash | 1,048,576 | ✅ (128-32768) |

### Claude 系列（通过 Antigravity）

| 模型名称 | 说明 | 上下文长度 | 最大输出 |
|---------|------|-----------|---------|
| `claude-sonnet-4-5` | Claude 4.5 Sonnet | 200,000 | 64,000 |
| `claude-sonnet-4-5-thinking` | Claude 4.5 Sonnet (思维链) | 200,000 | 64,000 |
| `claude-opus-4-5-thinking` | Claude 4.5 Opus (思维链) | 200,000 | 64,000 |
| `claude-opus-4-6-thinking` | Claude 4.6 Opus (思维链) | 1,000,000 | 128,000 |

### 其他模型

- `gpt-oss-120b-medium`: GPT 开源模型
- `tab_flash_lite_preview`: Tab Flash Lite 预览版

**注意**: 实际可用模型取决于您的 Antigravity 账号权限。使用 `/v1/models` 接口查看当前可用模型。

---

## API 格式支持

### 1. OpenAI 格式

CLIProxyAPI 完全兼容 OpenAI API 格式，可直接替换 OpenAI 的 base URL。

**支持的端点**:
- `POST /v1/chat/completions` - 聊天补全
- `GET /v1/models` - 获取模型列表

**特性支持**:
- ✅ 流式响应 (`stream: true`)
- ✅ 多轮对话
- ✅ 系统提示词
- ✅ 函数调用（部分模型）
- ✅ 思维链（通过扩展参数）

### 2. Claude 格式

支持 Anthropic Claude API 格式。

**支持的端点**:
- `POST /v1/messages` - 消息 API
- `GET /v1/models` - 获取模型列表

**特性支持**:
- ✅ 流式响应
- ✅ 系统提示词
- ✅ 多轮对话
- ✅ 思维预算（thinking budget）

### 3. Gemini 格式

支持 Google Gemini API 格式。

**支持的端点**:
- `POST /v1beta/models/{model}:generateContent` - 生成内容
- `POST /v1beta/models/{model}:streamGenerateContent` - 流式生成
- `GET /v1beta/models` - 获取模型列表

**特性支持**:
- ✅ 流式响应
- ✅ 系统指令
- ✅ 多轮对话
- ✅ 思维配置（thinkingConfig）

---

## 使用示例

### 示例 1: OpenAI 格式 - 基础对话

```bash
curl -X POST http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [
      {
        "role": "user",
        "content": "用 Python 写一个快速排序算法"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 2000
  }'
```

### 示例 2: OpenAI 格式 - 流式响应

```bash
curl -X POST http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer AI_club2026" \
  -d '{
    "model": "gemini-3-pro-high",
    "messages": [
      {
        "role": "system",
        "content": "你是一个专业的代码审查助手"
      },
      {
        "role": "user",
        "content": "审查这段代码并给出改进建议：\n\ndef add(a, b):\n    return a + b"
      }
    ],
    "stream": true,
    "temperature": 0.5
  }'
```

### 示例 3: Claude 格式 - 思维链模型

```bash
curl -X POST http://localhost:8317/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: AI_club2026" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gemini-2.5-flash-lite",
    "max_tokens": 4096,
    "thinking": {
      "type": "enabled",
      "budget_tokens": 10000
    },
    "messages": [
      {
        "role": "user",
        "content": "解释量子计算的基本原理"
      }
    ]
  }'
```

### 示例 4: Gemini 格式 - 原生 API

```bash
curl -X POST http://localhost:8317/v1beta/models/gemini-2.5-flash:generateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {
            "text": "介绍一下机器学习的基本概念"
          }
        ]
      }
    ],
    "generationConfig": {
      "temperature": 0.7,
      "maxOutputTokens": 2048,
      "thinkingConfig": {
        "thinkingBudget": 8192
      }
    }
  }'
```

### 示例 5: Python SDK (OpenAI)

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="http://localhost:8317/v1"
)

response = client.chat.completions.create(
    model="gemini-3-pro-high",
    messages=[
        {"role": "system", "content": "你是一个有帮助的助手"},
        {"role": "user", "content": "什么是深度学习？"}
    ],
    temperature=0.7,
    max_tokens=1500
)

print(response.choices[0].message.content)
```

### 示例 6: Python SDK (Anthropic)

```python
from anthropic import Anthropic

client = Anthropic(
    api_key="YOUR_API_KEY",
    base_url="http://localhost:8317"
)

message = client.messages.create(
    model="claude-sonnet-4-5-thinking",
    max_tokens=4096,
    thinking={
        "type": "enabled",
        "budget_tokens": 10000
    },
    messages=[
        {"role": "user", "content": "解释相对论的基本概念"}
    ]
)

print(message.content)
```

### 示例 7: 使用模型别名

在 `config.yaml` 中配置别名：

```yaml
oauth-model-alias:
  antigravity:
    - name: "gemini-3-pro-high"
      alias: "g3p"
    - name: "claude-sonnet-4-5-thinking"
      alias: "cs45t"
```

然后使用别名调用：

```bash
curl -X POST http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "g3p",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## 配置管理

### 配置文件位置

认证信息存储在：
```
~/.cli-proxy-api/antigravity_<email>.json
```

### 配置文件结构

```json
{
  "access_token": "ya29.a0...",
  "refresh_token": "1//0g...",
  "expires_at": "2024-01-15T10:30:00Z",
  "project_id": "cloudaicompanion-...",
  "email": "user@gmail.com"
}
```

### 高级配置选项

在 `config.yaml` 中配置：

```yaml
# 模型别名配置
oauth-model-alias:
  antigravity:
    - name: "gemini-2.5-flash"
      alias: "flash"
    - name: "gemini-3-pro-high"
      alias: "pro"
      fork: true  # 保留原名称，同时添加别名

# 排除特定模型
oauth-excluded-models:
  antigravity:
    - "gemini-2.5-*"        # 排除所有 2.5 系列
    - "*-preview"           # 排除所有预览版
    - "claude-opus-*"       # 排除所有 Opus 模型

# 请求负载配置
payload:
  default:
    - models:
        - name: "gemini-3-*"
          protocol: "antigravity"
      params:
        "generationConfig.thinkingConfig.thinkingBudget": 16384

  override:
    - models:
        - name: "claude-*-thinking"
          protocol: "antigravity"
      params:
        "thinking.budget_tokens": 20000
```

### 多账号支持

支持添加多个 Antigravity 账号：

```bash
# 使用无痕模式登录第二个账号
./CLIProxyAPI -antigravity-login -incognito
```

系统会自动轮询使用多个账号（round-robin 策略）。

### 配置路由策略

在 `config.yaml` 中：

```yaml
routing:
  strategy: "round-robin"  # 或 "fill-first"
```

- `round-robin`: 轮询使用所有账号
- `fill-first`: 优先使用第一个账号，配额用尽后切换

---

## 常见问题

### 1. 如何查看当前可用的模型？

```bash
curl http://localhost:8317/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 2. Token 过期怎么办？

CLIProxyAPI 会自动刷新 token。如果自动刷新失败，重新执行登录：

```bash
./CLIProxyAPI -antigravity-login
```

### 3. 如何使用思维链功能？

**OpenAI 格式**（通过扩展参数）:
```json
{
  "model": "gemini-3-pro-high",
  "messages": [...],
  "thinking_budget": 16384
}
```

**Claude 格式**:
```json
{
  "model": "claude-sonnet-4-5-thinking",
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  },
  "messages": [...]
}
```

**Gemini 格式**:
```json
{
  "contents": [...],
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 16384
    }
  }
}
```

### 4. 支持哪些编程语言的 SDK？

所有支持 OpenAI API 的 SDK 都可以使用，只需修改 `base_url`：

- **Python**: `openai`, `anthropic`
- **JavaScript/TypeScript**: `openai`, `@anthropic-ai/sdk`
- **Go**: `github.com/sashabaranov/go-openai`
- **Java**: OpenAI Java SDK
- **Ruby**: `ruby-openai`

### 5. 如何处理速率限制？

Antigravity 有速率限制。建议：
- 使用多个账号
- 实现请求重试逻辑
- 监控响应头中的速率限制信息

### 6. 如何备份认证信息？

```bash
# 备份认证目录
cp -r ~/.cli-proxy-api ~/.cli-proxy-api.backup

# 或使用 Git 存储（需配置环境变量）
export GITSTORE_GIT_URL="https://github.com/user/tokens.git"
export GITSTORE_GIT_USERNAME="user"
export GITSTORE_GIT_TOKEN="ghp_..."
```

### 7. 如何在 Docker 中使用？

```yaml
# docker-compose.yml
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api  # 挂载认证目录
    restart: unless-stopped
```

先在宿主机登录，然后启动容器：
```bash
./CLIProxyAPI -antigravity-login
docker compose up -d
```

### 8. 如何调试请求？

启用调试日志：

```yaml
# config.yaml
debug: true
```

或查看请求日志：
```bash
tail -f logs/request.log
```

### 9. 支持流式响应吗？

是的，所有三种 API 格式都支持流式响应：

- OpenAI: `"stream": true`
- Claude: `"stream": true`
- Gemini: 使用 `:streamGenerateContent` 端点

### 10. 如何切换到不同的 Antigravity 环境？

默认使用生产环境。如需使用其他环境，需要修改源代码中的 `antigravityBaseURLProd` 常量。

---

## 技术支持

- **项目主页**: https://github.com/router-for-me/CLIProxyAPIPlus
- **问题反馈**: https://github.com/router-for-me/CLIProxyAPIPlus/issues
- **文档**: https://github.com/router-for-me/CLIProxyAPIPlus/tree/main/docs

---

## 更新日志

- **v6.0**: 初始 Antigravity 支持
  - Google OAuth 认证
  - 多模型支持
  - 三种 API 格式兼容
  - 自动 token 刷新
