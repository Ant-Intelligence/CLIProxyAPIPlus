# Antigravity Account Setup Guide

## Overview

This document provides detailed instructions on how to add and use Antigravity accounts in CLIProxyAPI Plus. Antigravity is a powerful agentic AI coding assistant developed by Google Deepmind, accessible through Google OAuth authentication.

**Key Features**:
- Support for Gemini series models (including Gemini 2.5/3.0 series)
- Support for Claude series models (via Antigravity proxy)
- Compatible with OpenAI, Claude, and Gemini API formats
- Support for streaming and non-streaming responses
- Support for thinking/reasoning capabilities

## Table of Contents

- [Authentication Method](#authentication-method)
- [Quick Start](#quick-start)
- [API Endpoints](#api-endpoints)
- [Supported Models](#supported-models)
- [API Format Support](#api-format-support)
- [Usage Examples](#usage-examples)
- [Configuration Management](#configuration-management)
- [FAQ](#faq)

---

## Authentication Method

Antigravity uses Google OAuth 2.0 for authentication, requiring the following scopes:
- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`
- `https://www.googleapis.com/auth/userinfo.profile`
- `https://www.googleapis.com/auth/cclog`
- `https://www.googleapis.com/auth/experimentsandconfigs`

---

## Quick Start

### 1. Execute OAuth Login

On the machine running CLIProxyAPI, execute:

```bash
./CLIProxyAPI -antigravity-login
```

**Optional Parameters**:
```bash
# Don't automatically open browser
./CLIProxyAPI -antigravity-login -no-browser

# Use incognito mode (for multi-account switching)
./CLIProxyAPI -antigravity-login -incognito

# Custom callback port (default: 51121)
./CLIProxyAPI -antigravity-login -oauth-callback-port 8888
```

### 2. Complete Browser Authorization

After executing the command:
1. Browser automatically opens to Google OAuth authorization page
2. Login with your Google account and authorize
3. Upon successful authorization, credentials are automatically saved to `~/.cli-proxy-api/antigravity_*.json`

### 3. Start the Server

```bash
./CLIProxyAPI -config config.yaml
```

The server will automatically load Antigravity authentication information on startup.

---

## API Endpoints

### Basic Information

- **Server Address**: `http://your-server:8317` (default port)
- **Authentication**: API Key (add `Authorization: Bearer YOUR_API_KEY` in request header)
- **Supported API Formats**: OpenAI, Claude, Gemini

### Main Endpoints

#### 1. List Models

```http
GET /v1/models
Authorization: Bearer YOUR_API_KEY
```

**Response Example**:
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

#### 2. Chat Completions (OpenAI Format)

```http
POST /v1/chat/completions
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

#### 3. Chat Completions (Claude Format)

```http
POST /v1/messages
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
```

#### 4. Chat Completions (Gemini Format)

```http
POST /v1beta/models/{model}:generateContent
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

---

## Supported Models

### Gemini Series

| Model Name | Description | Context Length | Thinking Support |
|-----------|-------------|----------------|------------------|
| `gemini-2.5-flash` | Gemini 2.5 Flash | 1,048,576 | ✅ (0-24576) |
| `gemini-2.5-flash-lite` | Gemini 2.5 Flash Lite | 1,048,576 | ✅ (0-24576) |
| `gemini-3-pro-high` | Gemini 3 Pro High | 2,097,152 | ✅ (128-32768) |
| `gemini-3-pro-image` | Gemini 3 Pro Image | 2,097,152 | ✅ (128-32768) |
| `gemini-3-flash` | Gemini 3 Flash | 1,048,576 | ✅ (128-32768) |

### Claude Series (via Antigravity)

| Model Name | Description | Context Length | Max Output |
|-----------|-------------|----------------|------------|
| `claude-sonnet-4-5` | Claude 4.5 Sonnet | 200,000 | 64,000 |
| `claude-sonnet-4-5-thinking` | Claude 4.5 Sonnet (Thinking) | 200,000 | 64,000 |
| `claude-opus-4-5-thinking` | Claude 4.5 Opus (Thinking) | 200,000 | 64,000 |
| `claude-opus-4-6-thinking` | Claude 4.6 Opus (Thinking) | 1,000,000 | 128,000 |

### Other Models

- `gpt-oss-120b-medium`: GPT Open Source Model
- `tab_flash_lite_preview`: Tab Flash Lite Preview

**Note**: Available models depend on your Antigravity account permissions. Use the `/v1/models` endpoint to view currently available models.

---

## API Format Support

### 1. OpenAI Format

CLIProxyAPI is fully compatible with OpenAI API format and can directly replace OpenAI's base URL.

**Supported Endpoints**:
- `POST /v1/chat/completions` - Chat completions
- `GET /v1/models` - List models

**Feature Support**:
- ✅ Streaming responses (`stream: true`)
- ✅ Multi-turn conversations
- ✅ System prompts
- ✅ Function calling (select models)
- ✅ Thinking/reasoning (via extended parameters)

### 2. Claude Format

Supports Anthropic Claude API format.

**Supported Endpoints**:
- `POST /v1/messages` - Messages API
- `GET /v1/models` - List models

**Feature Support**:
- ✅ Streaming responses
- ✅ System prompts
- ✅ Multi-turn conversations
- ✅ Thinking budget

### 3. Gemini Format

Supports Google Gemini API format.

**Supported Endpoints**:
- `POST /v1beta/models/{model}:generateContent` - Generate content
- `POST /v1beta/models/{model}:streamGenerateContent` - Stream generate
- `GET /v1beta/models` - List models

**Feature Support**:
- ✅ Streaming responses
- ✅ System instructions
- ✅ Multi-turn conversations
- ✅ Thinking configuration (thinkingConfig)

---

## Usage Examples

### Example 1: OpenAI Format - Basic Conversation

```bash
curl -X POST http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [
      {
        "role": "user",
        "content": "Write a quicksort algorithm in Python"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 2000
  }'
```

### Example 2: OpenAI Format - Streaming Response

```bash
curl -X POST http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gemini-3-pro-high",
    "messages": [
      {
        "role": "system",
        "content": "You are a professional code review assistant"
      },
      {
        "role": "user",
        "content": "Review this code and provide improvement suggestions:\n\ndef add(a, b):\n    return a + b"
      }
    ],
    "stream": true,
    "temperature": 0.5
  }'
```

### Example 3: Claude Format - Thinking Model

```bash
curl -X POST http://localhost:8317/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-thinking",
    "max_tokens": 4096,
    "thinking": {
      "type": "enabled",
      "budget_tokens": 10000
    },
    "messages": [
      {
        "role": "user",
        "content": "Explain the basic principles of quantum computing"
      }
    ]
  }'
```

### Example 4: Gemini Format - Native API

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
            "text": "Introduce the basic concepts of machine learning"
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

### Example 5: Python SDK (OpenAI)

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="http://localhost:8317/v1"
)

response = client.chat.completions.create(
    model="gemini-3-pro-high",
    messages=[
        {"role": "system", "content": "You are a helpful assistant"},
        {"role": "user", "content": "What is deep learning?"}
    ],
    temperature=0.7,
    max_tokens=1500
)

print(response.choices[0].message.content)
```

### Example 6: Python SDK (Anthropic)

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
        {"role": "user", "content": "Explain the basic concepts of relativity"}
    ]
)

print(message.content)
```

### Example 7: Using Model Aliases

Configure aliases in `config.yaml`:

```yaml
oauth-model-alias:
  antigravity:
    - name: "gemini-3-pro-high"
      alias: "g3p"
    - name: "claude-sonnet-4-5-thinking"
      alias: "cs45t"
```

Then use the alias:

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

## Configuration Management

### Configuration File Location

Authentication information is stored at:
```
~/.cli-proxy-api/antigravity_<email>.json
```

### Configuration File Structure

```json
{
  "access_token": "ya29.a0...",
  "refresh_token": "1//0g...",
  "expires_at": "2024-01-15T10:30:00Z",
  "project_id": "cloudaicompanion-...",
  "email": "user@gmail.com"
}
```

### Advanced Configuration Options

Configure in `config.yaml`:

```yaml
# Model alias configuration
oauth-model-alias:
  antigravity:
    - name: "gemini-2.5-flash"
      alias: "flash"
    - name: "gemini-3-pro-high"
      alias: "pro"
      fork: true  # Keep original name and add alias

# Exclude specific models
oauth-excluded-models:
  antigravity:
    - "gemini-2.5-*"        # Exclude all 2.5 series
    - "*-preview"           # Exclude all preview versions
    - "claude-opus-*"       # Exclude all Opus models

# Request payload configuration
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

### Multi-Account Support

Support for adding multiple Antigravity accounts:

```bash
# Login with second account using incognito mode
./CLIProxyAPI -antigravity-login -incognito
```

The system will automatically rotate through multiple accounts (round-robin strategy).

### Configure Routing Strategy

In `config.yaml`:

```yaml
routing:
  strategy: "round-robin"  # or "fill-first"
```

- `round-robin`: Rotate through all accounts
- `fill-first`: Use first account until quota exhausted, then switch

---

## FAQ

### 1. How to view currently available models?

```bash
curl http://localhost:8317/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 2. What if the token expires?

CLIProxyAPI automatically refreshes tokens. If automatic refresh fails, re-execute login:

```bash
./CLIProxyAPI -antigravity-login
```

### 3. How to use thinking/reasoning features?

**OpenAI Format** (via extended parameters):
```json
{
  "model": "gemini-3-pro-high",
  "messages": [...],
  "thinking_budget": 16384
}
```

**Claude Format**:
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

**Gemini Format**:
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

### 4. Which programming language SDKs are supported?

All SDKs that support OpenAI API can be used, just modify the `base_url`:

- **Python**: `openai`, `anthropic`
- **JavaScript/TypeScript**: `openai`, `@anthropic-ai/sdk`
- **Go**: `github.com/sashabaranov/go-openai`
- **Java**: OpenAI Java SDK
- **Ruby**: `ruby-openai`

### 5. How to handle rate limits?

Antigravity has rate limits. Recommendations:
- Use multiple accounts
- Implement request retry logic
- Monitor rate limit information in response headers

### 6. How to backup authentication information?

```bash
# Backup auth directory
cp -r ~/.cli-proxy-api ~/.cli-proxy-api.backup

# Or use Git storage (requires environment variables)
export GITSTORE_GIT_URL="https://github.com/user/tokens.git"
export GITSTORE_GIT_USERNAME="user"
export GITSTORE_GIT_TOKEN="ghp_..."
```

### 7. How to use in Docker?

```yaml
# docker-compose.yml
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api  # Mount auth directory
    restart: unless-stopped
```

Login on host machine first, then start container:
```bash
./CLIProxyAPI -antigravity-login
docker compose up -d
```

### 8. How to debug requests?

Enable debug logging:

```yaml
# config.yaml
debug: true
```

Or view request logs:
```bash
tail -f logs/request.log
```

### 9. Does it support streaming responses?

Yes, all three API formats support streaming responses:

- OpenAI: `"stream": true`
- Claude: `"stream": true`
- Gemini: Use `:streamGenerateContent` endpoint

### 10. How to switch to different Antigravity environments?

Production environment is used by default. To use other environments, modify the `antigravityBaseURLProd` constant in the source code.

---

## Technical Support

- **Project Homepage**: https://github.com/router-for-me/CLIProxyAPIPlus
- **Issue Reporting**: https://github.com/router-for-me/CLIProxyAPIPlus/issues
- **Documentation**: https://github.com/router-for-me/CLIProxyAPIPlus/tree/main/docs

---

## Changelog

- **v6.0**: Initial Antigravity support
  - Google OAuth authentication
  - Multi-model support
  - Three API format compatibility
  - Automatic token refresh
