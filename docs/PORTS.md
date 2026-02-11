# CLIProxyAPI Plus 端口说明

本文档详细说明了 CLIProxyAPI Plus 中使用的各个端口及其用途。

## 端口总览

| 端口 | 用途 | 协议 | 访问范围 | 必需性 |
|------|------|------|----------|--------|
| 8317 | 主API服务 | HTTP/HTTPS | 公开 | **必需** |
| 8085 | Gemini OAuth回调 | HTTP | 本地 | OAuth时需要 |
| 1455 | Codex/OpenAI OAuth回调 | HTTP | 本地 | OAuth时需要 |
| 54545 | Claude OAuth回调 | HTTP | 本地 | OAuth时需要 |
| 51121 | Antigravity OAuth回调 | HTTP | 本地 | OAuth时需要 |
| 11451 | iFlow OAuth回调 | HTTP | 本地 | OAuth时需要 |

---

## 详细说明

### 8317 - 主API服务端口

**用途**: CLIProxyAPI Plus 的主要API服务端口

**功能**:
- 接收所有客户端的API请求
- 提供 OpenAI/Claude/Gemini 兼容的API接口
- 处理模型推理请求和流式响应
- 提供管理API端点 (`/v0/management/*`)
- 提供模型列表端点 (`/v1/models`)

**配置方式**:
```yaml
# config.yaml
port: 8317
```

**环境变量**:
```bash
CLI_PROXY_PORT_1=8317  # Docker Compose中可覆盖
```

**访问示例**:
```bash
# 获取模型列表
curl -H "Authorization: Bearer your-api-key" http://localhost:8317/v1/models

# 发送聊天请求
curl -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     -d '{"model":"gemini-2.0-flash","messages":[{"role":"user","content":"Hello"}]}' \
     http://localhost:8317/v1/chat/completions
```

---

### 8085 - Gemini OAuth回调端口

**用途**: Google OAuth 认证流程的本地回调服务器端口

**支持的认证类型**:
- Gemini CLI OAuth (通过 Google 账号登录)
- Vertex AI OAuth
- AI Studio OAuth
- Qwen OAuth (某些配置下)

**工作流程**:
1. 用户执行 `./CLIProxyAPI -login` 启动OAuth流程
2. 本地启动临时HTTP服务器监听 `localhost:8085`
3. 浏览器打开Google OAuth授权页面
4. 用户授权后，Google重定向到 `http://localhost:8085/callback`
5. 本地服务器接收授权码并交换访问令牌
6. 令牌保存到 `~/.cli-proxy-api/` 目录

**相关代码**:
```go
// internal/auth/gemini/gemini_auth.go
const DefaultCallbackPort = 8085
```

**配置方式**:
```bash
CLI_PROXY_OAUTH_PORT_1=8085  # Docker Compose中可覆盖
```

**注意事项**:
- 仅在OAuth登录过程中使用，登录完成后自动关闭
- 必须保持端口可用，否则OAuth流程会失败
- 不需要对外网暴露，仅本地访问

---

### 1455 - Codex/OpenAI OAuth回调端口

**用途**: OpenAI CLI OAuth 认证流程的本地回调服务器端口

**支持的认证类型**:
- OpenAI Codex CLI OAuth
- ChatGPT CLI OAuth (如果使用OAuth认证)

**工作流程**:
1. 用户执行 `./CLIProxyAPI -codex-login` 启动OAuth流程
2. 本地启动临时HTTP服务器监听 `localhost:1455`
3. 浏览器打开OpenAI授权页面
4. 用户授权后，OpenAI重定向到 `http://localhost:1455/auth/callback`
5. 本地服务器接收授权并保存令牌

**相关代码**:
```go
// internal/auth/codex/openai_auth.go
const RedirectURI = "http://localhost:1455/auth/callback"
```

**注意事项**:
- 仅在OAuth登录过程中使用
- 端口固定为1455，需要确保可用
- 登录完成后自动关闭服务器

---

### 54545 - Claude OAuth回调端口

**用途**: Anthropic Claude CLI OAuth 认证流程的本地回调服务器端口

**支持的认证类型**:
- Claude CLI OAuth (通过 claude.ai 账号登录)

**工作流程**:
1. 用户执行 `./CLIProxyAPI -claude-login` 启动OAuth流程
2. 本地启动临时HTTP服务器监听 `localhost:54545`
3. 浏览器打开 claude.ai 授权页面
4. 用户授权后，Anthropic重定向到 `http://localhost:54545/callback`
5. 本地服务器接收会话令牌并保存

**相关代码**:
```go
// internal/auth/claude/anthropic_auth.go
const RedirectURI = "http://localhost:54545/callback"
```

**注意事项**:
- 仅在OAuth登录过程中使用
- 端口固定为54545，需要确保可用
- Claude OAuth使用会话cookie而非标准OAuth2令牌

---

### 51121 - Antigravity OAuth回调端口

**用途**: Antigravity CLI OAuth 认证流程的本地回调服务器端口

**支持的认证类型**:
- Antigravity CLI OAuth (Google Gemini 的第三方CLI工具)

**工作流程**:
1. 用户执行 `./CLIProxyAPI -antigravity-login` 启动OAuth流程
2. 本地启动临时HTTP服务器监听 `localhost:51121`
3. 浏览器打开 Google OAuth 授权页面
4. 用户授权后重定向到 `http://localhost:51121/callback`
5. 本地服务器接收授权并保存令牌

**相关代码**:
```go
// internal/auth/antigravity/constants.go
const CallbackPort = 51121
```

**注意事项**:
- Antigravity是一个第三方Gemini CLI客户端
- 使用与官方Gemini CLI不同的回调端口以避免冲突
- 可以与Gemini CLI OAuth同时使用

---

### 11451 - iFlow OAuth回调端口

**用途**: iFlow (智谱AI) OAuth 认证流程的本地回调服务器端口

**支持的认证类型**:
- iFlow CLI OAuth (智谱AI的GLM模型)

**工作流程**:
1. 用户执行 `./CLIProxyAPI -iflow-login` 启动OAuth流程
2. 本地启动临时HTTP服务器监听 `localhost:11451`
3. 浏览器打开 iFlow 授权页面
4. 用户授权后重定向到 `http://localhost:11451/callback`
5. 本地服务器接收授权并保存令牌

**相关代码**:
```go
// internal/auth/iflow/iflow_auth.go
const CallbackPort = 11451
```

**注意事项**:
- iFlow是智谱AI的对话式编程助手
- 支持GLM-4系列模型
- 使用独立的回调端口避免与其他OAuth流程冲突

---

## Docker Compose 端口映射

### 默认映射

```yaml
services:
  cli-proxy-api:
    ports:
      - "${CLI_PROXY_PORT_1:-8317}:8317"       # 主API服务
      - "${CLI_PROXY_OAUTH_PORT_1:-8085}:8085" # Gemini OAuth回调
      - "1455:1455"                             # Codex OAuth回调
      - "54545:54545"                           # Claude OAuth回调
      - "51121:51121"                           # Antigravity OAuth回调
      - "11451:11451"                           # iFlow OAuth回调
```

### 自定义端口映射

如果默认端口已被占用，可以通过环境变量修改主API端口和Gemini OAuth端口：

```bash
# .env 文件
CLI_PROXY_PORT_1=9000      # 修改主API端口为9000
CLI_PROXY_OAUTH_PORT_1=9001  # 修改Gemini OAuth端口为9001
```

其他OAuth回调端口是硬编码的，如需修改需要：
1. 修改 `compose.yml` 中的端口映射
2. 修改相应认证模块的源代码中的端口常量
3. 重新编译程序

---

## 安全建议

### 生产环境部署

1. **主API端口 (8317)**
   - 可以对外暴露，通过API key保护
   - 建议配置反向代理 (Nginx/Caddy) 并启用HTTPS
   - 配置速率限制防止滥用

2. **OAuth回调端口 (8085, 1455, 54545, 51121, 11451)**
   - **不应对外网暴露**
   - 仅在本地执行OAuth登录时需要
   - Docker部署时，这些端口应该绑定到 `127.0.0.1` 而非 `0.0.0.0`

### 推荐的安全配置

**本地开发环境**: 使用默认配置即可

**Docker生产环境**: 修改 `compose.yml` 限制OAuth端口仅本地访问：

```yaml
services:
  cli-proxy-api:
    ports:
      - "0.0.0.0:8317:8317"           # 主API - 可公开访问
      - "127.0.0.1:8085:8085"         # Gemini OAuth - 仅本地
      - "127.0.0.1:1455:1455"         # Codex OAuth - 仅本地
      - "127.0.0.1:54545:54545"       # Claude OAuth - 仅本地
      - "127.0.0.1:51121:51121"       # Antigravity OAuth - 仅本地
      - "127.0.0.1:11451:11451"       # iFlow OAuth - 仅本地
```

**Nginx反向代理配置示例**:

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 仅代理主API端口
    location / {
        proxy_pass http://localhost:8317;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;

        # 支持服务器发送事件 (SSE)
        proxy_buffering off;
        proxy_read_timeout 86400;
    }

    # 阻止对OAuth回调端点的外部访问
    location ~ ^/(callback|oauth|auth/callback) {
        return 403;
    }
}
```

---

## 防火墙配置

### UFW (Ubuntu/Debian)

```bash
# 允许主API端口
sudo ufw allow 8317/tcp

# 阻止OAuth回调端口 (如果已开放)
sudo ufw deny 8085/tcp
sudo ufw deny 1455/tcp
sudo ufw deny 54545/tcp
sudo ufw deny 51121/tcp
sudo ufw deny 11451/tcp
```

### iptables

```bash
# 允许主API端口
sudo iptables -A INPUT -p tcp --dport 8317 -j ACCEPT

# 仅允许本地访问OAuth回调端口
sudo iptables -A INPUT -p tcp --dport 8085 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 1455 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 54545 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 51121 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 11451 -s 127.0.0.1 -j ACCEPT

# 阻止外部访问OAuth回调端口
sudo iptables -A INPUT -p tcp --dport 8085 -j DROP
sudo iptables -A INPUT -p tcp --dport 1455 -j DROP
sudo iptables -A INPUT -p tcp --dport 54545 -j DROP
sudo iptables -A INPUT -p tcp --dport 51121 -j DROP
sudo iptables -A INPUT -p tcp --dport 11451 -j DROP
```

---

## 故障排查

### 端口被占用

**检查端口占用**:
```bash
# Linux/macOS
lsof -i :8317
netstat -tulpn | grep 8317

# Windows
netstat -ano | findstr :8317
```

**Docker容器端口冲突**:
```bash
# 查看Docker端口映射
docker ps --format "table {{.Names}}\t{{.Ports}}"

# 停止冲突的容器
docker stop <container-name>
```

### OAuth登录失败

**症状**: 浏览器无法连接到回调URL

**可能原因**:
1. 回调端口被占用
2. 防火墙阻止了端口访问
3. Docker端口映射配置错误

**解决方案**:
```bash
# 1. 检查端口是否被占用
lsof -i :8085

# 2. 检查防火墙规则
sudo ufw status

# 3. 验证Docker端口映射
docker port cli-proxy-api

# 4. 测试回调端口连接
curl http://localhost:8085
```

### 容器内OAuth登录

**问题**: 在Docker容器内执行OAuth登录时，浏览器打不开

**解决方案**:
1. 使用 `-no-browser` 参数获取授权URL
2. 在宿主机浏览器中手动打开URL
3. 确保回调端口已正确映射

```bash
# 方法1: 使用no-browser参数
docker exec -it cli-proxy-api ./CLIProxyAPIPlus -login -no-browser

# 方法2: 在宿主机执行登录
# 将认证目录挂载到容器
docker run -v ~/.cli-proxy-api:/root/.cli-proxy-api ...
./CLIProxyAPI -login  # 在宿主机执行
```

---

## 参考链接

- [主配置文件说明](../config.example.yaml)
- [OAuth认证说明](../README.md#oauth-login)
- [Docker部署指南](../POSTGRES_QUICKSTART.md)
- [API使用文档](./API.md)

---

## 更新历史

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-02-07 | 1.0.0 | 初始版本，添加所有端口说明 |
