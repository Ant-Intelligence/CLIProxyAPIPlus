# Kiro 订阅账号实现机制

## 概述

CLIProxyAPI Plus 支持 Kiro (AWS CodeWhisperer) 订阅账号的缓存创建、读取分配、自动刷新和负载均衡。本文档详细记录了其内部实现机制和架构设计。

## 核心架构

### 1. Token 持久化与缓存

#### 文件结构
- **位置**: `~/.cli-proxy-api/` (默认) 或配置的 `auth-dir`
- **命名格式**:
  - Google OAuth: `kiro-google-{email}.json`
  - GitHub OAuth: `kiro-github-{email}.json`
  - AWS Builder ID: `kiro-aws-{email|profileArn}.json`
  - AWS IDC: `kiro-idc-{email|identifier}.json`

#### Token 文件格式
```json
{
  "type": "kiro",
  "access_token": "aoaAAAAA...",
  "refresh_token": "aorAAAAA...",
  "profile_arn": "arn:aws:codewhisperer:us-east-1:...",
  "expires_at": "2026-02-08T12:00:00Z",
  "last_refresh": "2026-02-08T10:00:00Z",
  "auth_method": "builder-id",  // or "idc", "google", "github"
  "provider": "aws-builder-id",
  "client_id": "...",
  "client_secret": "...",
  "email": "user@example.com",
  "region": "us-east-1",          // IDC only
  "start_url": "https://..."      // IDC only
}
```

### 2. Token Repository 接口

**位置**: `internal/auth/kiro/token_repository.go`

```go
type TokenRepository interface {
    FindOldestUnverified(limit int) []*Token
    UpdateToken(token *Token) error
}
```

#### FileTokenRepository 实现
- **扫描逻辑**: 遍历 auth-dir 目录，查找 `kiro-*.json` 文件
- **筛选条件**:
  - 文件名以 `kiro-` 开头
  - 包含有效的 `refresh_token`
  - `auth_method` 为 `idc` 或 `builder-id`
  - 过期时间在 5 分钟内或已过期
- **排序**: 按 `last_refresh` 时间排序（最旧的优先）
- **原子更新**: 使用临时文件 + rename 保证写入原子性

### 3. 后台自动刷新机制

**位置**: `internal/auth/kiro/background_refresh.go` 和 `refresh_manager.go`

#### BackgroundRefresher
- **检查间隔**: 每 1 分钟扫描一次
- **批量处理**: 每批最多处理 50 个 token
- **并发控制**: 最多 10 个并发刷新
- **节流控制**: 每个 token 之间间隔 100ms

#### 刷新策略
```go
// 按 auth_method 选择刷新方式
switch authMethod {
case "idc":
    // IDC: 使用 SSO OIDC RefreshTokenWithRegion (支持区域化)
    tokenData, err = ssoClient.RefreshTokenWithRegion(
        ctx, clientID, clientSecret, refreshToken, region, startURL)

case "builder-id":
    // Builder ID: 使用 SSO OIDC RefreshToken
    tokenData, err = ssoClient.RefreshToken(
        ctx, clientID, clientSecret, refreshToken)

default:
    // Google/GitHub: 使用 Kiro OAuth RefreshToken
    tokenData, err = oauth.RefreshTokenWithFingerprint(
        ctx, refreshToken, tokenID)
}
```

#### 优雅降级 (Graceful Degradation)
- 如果刷新失败但 token 仍然有效（未过期），继续使用现有 token
- 只更新 `last_refresh` 时间戳，避免频繁重试
- 防止因网络抖动或临时故障导致的服务中断

### 4. Token 分配与负载均衡

**位置**: `sdk/cliproxy/auth/conductor.go` 和 `selector.go`

#### Auth 对象结构
```go
type Auth struct {
    ID               string           // 唯一标识符 (文件名)
    Provider         string           // "kiro"
    Prefix           string           // 可选的命名空间前缀
    Status           Status           // Active/Expired/Failed
    Disabled         bool             // 是否被禁用
    Unavailable      bool             // 临时不可用 (quota/cooldown)
    Metadata         map[string]any   // 包含 access_token, refresh_token 等
    Quota            QuotaState       // 配额状态
    NextRefreshAfter time.Time        // 下次刷新时间 (过期前 20 分钟)
    NextRetryAfter   time.Time        // 下次重试时间
    ModelStates      map[string]*ModelState  // 每个模型的状态
}
```

#### Selector 选择策略

##### 1. Round-Robin (轮询)
```go
type RoundRobinSelector struct{}

func (s *RoundRobinSelector) Pick(ctx, provider, model string, opts Options, auths []*Auth) (*Auth, error) {
    // 1. 过滤可用的 auth
    available := filterAvailable(auths)

    // 2. 按照 providerOffsets 轮询选择
    offset := manager.providerOffsets[provider+model]
    selected := available[offset % len(available)]

    // 3. 更新 offset
    manager.providerOffsets[provider+model]++

    return selected
}
```

##### 2. Fill-First (填充优先)
```go
type FillFirstSelector struct{}

func (s *FillFirstSelector) Pick(ctx, provider, model string, opts Options, auths []*Auth) (*Auth, error) {
    // 优先选择可用配额最多的 auth
    // 实现逻辑类似，但按配额剩余量排序
    return selectByQuotaRemaining(auths)
}
```

#### 可用性过滤

```go
func filterAvailable(auths []*Auth) []*Auth {
    var result []*Auth
    for _, auth := range auths {
        if auth.Disabled {
            continue  // 跳过被禁用的
        }
        if auth.Unavailable {
            continue  // 跳过临时不可用的
        }
        if time.Now().Before(auth.NextRetryAfter) {
            continue  // 跳过在 cooldown 中的
        }
        if auth.Status != StatusActive {
            continue  // 跳过非激活状态的
        }
        result = append(result, auth)
    }
    return result
}
```

### 5. 速率限制与冷却机制

**位置**: `internal/auth/kiro/rate_limiter.go` 和 `cooldown.go`

#### RateLimiter 配置
```go
const (
    DefaultMinTokenInterval  = 1 * time.Second   // 最小请求间隔
    DefaultMaxTokenInterval  = 2 * time.Second   // 最大请求间隔
    DefaultDailyMaxRequests  = 500               // 每日最大请求数
    DefaultJitterPercent     = 0.3               // 抖动百分比
    DefaultBackoffBase       = 30 * time.Second  // 退避基础时间
    DefaultBackoffMax        = 5 * time.Minute   // 最大退避时间
    DefaultBackoffMultiplier = 1.5               // 退避倍数
    DefaultSuspendCooldown   = 1 * time.Hour     // 暂停冷却时间
)
```

#### TokenState 状态跟踪
```go
type TokenState struct {
    LastRequest    time.Time  // 上次请求时间
    RequestCount   int        // 总请求计数
    CooldownEnd    time.Time  // 冷却结束时间
    FailCount      int        // 失败计数
    DailyRequests  int        // 每日请求计数
    DailyResetTime time.Time  // 每日重置时间
    IsSuspended    bool       // 是否被暂停
    SuspendedAt    time.Time  // 暂停时间
    SuspendReason  string     // 暂停原因
}
```

#### 请求流程
```go
// 1. 检查并等待速率限制
rl.WaitForToken(tokenKey)

// 2. 执行请求
resp, err := executeRequest(...)

// 3. 根据结果更新状态
if err != nil {
    rl.MarkTokenFailed(tokenKey)  // 设置退避冷却
    rl.CheckAndMarkSuspended(tokenKey, err.Error())  // 检查是否暂停
} else {
    rl.MarkTokenSuccess(tokenKey)  // 清除失败状态
}
```

#### 退避算法
```go
func calculateBackoff(failCount int) time.Duration {
    // 指数退避: base * multiplier^(failCount-1)
    backoff := backoffBase * math.Pow(backoffMultiplier, float64(failCount-1))

    // 添加抖动 (±30%)
    jitter := backoff * jitterPercent * (random() * 2 - 1)
    backoff += jitter

    // 限制最大值
    if backoff > backoffMax {
        return backoffMax
    }
    return backoff
}
```

#### CooldownManager
```go
const (
    CooldownReason429          = "rate_limit_exceeded"
    CooldownReasonSuspended    = "account_suspended"
    CooldownReasonQuotaExhausted = "quota_exhausted"

    DefaultShortCooldown = 1 * time.Minute
    MaxShortCooldown     = 5 * time.Minute
    LongCooldown         = 24 * time.Hour
)

// 设置冷却
cm.SetCooldown(tokenKey, duration, reason)

// 检查是否在冷却期
if cm.IsInCooldown(tokenKey) {
    // 等待或跳过此 token
    remaining := cm.GetRemainingCooldown(tokenKey)
}
```

### 6. 配额管理

#### QuotaState 结构
```go
type QuotaState struct {
    Exceeded       bool       // 是否超过配额
    Reason         string     // 配额超限原因
    NextRecoverAt  time.Time  // 预计恢复时间
    BackoffLevel   int        // 退避等级
}
```

#### 配额超限处理
```go
func handleQuotaExceeded(auth *Auth, retryAfter *time.Duration) {
    auth.Quota.Exceeded = true
    auth.Quota.Reason = "daily_quota_exceeded"

    if retryAfter != nil {
        // 使用服务器提供的重试时间
        auth.Quota.NextRecoverAt = time.Now().Add(*retryAfter)
    } else {
        // 计算到下一天的时间
        auth.Quota.NextRecoverAt = nextDayMidnight()
    }

    // 增加退避等级
    auth.Quota.BackoffLevel++

    // 标记为临时不可用
    auth.Unavailable = true
}
```

### 7. Token 刷新回调机制

**位置**: `internal/auth/kiro/background_refresh.go`

#### 回调接口
```go
type BackgroundRefresher struct {
    onTokenRefreshed func(tokenID string, tokenData *KiroTokenData)
    callbackMu       sync.RWMutex  // 保护回调并发访问
}

// 设置回调
WithOnTokenRefreshed(func(tokenID string, tokenData *KiroTokenData) {
    // 通知 Watcher 更新内存中的 Auth 对象
    watcher.OnTokenRefreshed(tokenID, tokenData)
})
```

#### 刷新成功后触发回调
```go
func (r *BackgroundRefresher) refreshSingle(ctx context.Context, token *Token) {
    // ... 刷新逻辑 ...

    if err := r.tokenRepo.UpdateToken(token); err != nil {
        log.Printf("failed to update token %s: %v", token.ID, err)
        return
    }

    // 触发回调，通知外部组件
    r.callbackMu.RLock()
    callback := r.onTokenRefreshed
    r.callbackMu.RUnlock()

    if callback != nil {
        defer func() {
            if rec := recover(); rec != nil {
                log.Printf("callback panic for token %s: %v", token.ID, rec)
            }
        }()
        callback(token.ID, newTokenData)
    }
}
```

### 8. 多账号读取与加载

**位置**: `sdk/auth/kiro.go`

#### ImportFromKiroIDE
```go
func (a *KiroAuthenticator) ImportFromKiroIDE(ctx, cfg) (*Auth, error) {
    // 1. 从 ~/.kiro/kiro-auth-token.json 读取
    tokenData, err := kiroauth.LoadKiroIDEToken()

    // 2. 提取 email (从 JWT 解析)
    if tokenData.Email == "" {
        tokenData.Email = kiroauth.ExtractEmailFromJWT(tokenData.AccessToken)
    }

    // 3. 生成唯一文件名
    idPart := extractKiroIdentifier(tokenData.Email, tokenData.ProfileArn, tokenData.ClientID)
    provider := sanitizeProvider(tokenData.Provider)  // "google", "github", etc.
    fileName := fmt.Sprintf("kiro-%s-%s.json", provider, idPart)

    // 4. 创建 Auth 对象
    return &Auth{
        ID:               fileName,
        Provider:         "kiro",
        Metadata:         buildMetadata(tokenData),
        NextRefreshAfter: expiresAt.Add(-20 * time.Minute),
    }
}
```

#### 多账号支持
- 支持同时登录多个 Google/GitHub/AWS 账号
- 每个账号生成独立的 token 文件
- 通过 email 或 profileArn 区分不同账号
- 文件名去重：同一账号只保留最新的 token

### 9. 设备指纹与安全

**位置**: `internal/auth/kiro/fingerprint.go`

#### 设备指纹生成
```go
func GenerateDeviceFingerprint(tokenID string) (string, error) {
    // 1. 收集系统信息
    hostname, _ := os.Hostname()
    username, _ := user.Current()

    // 2. 生成唯一标识
    data := fmt.Sprintf("%s:%s:%s:%d",
        hostname,
        username.Username,
        tokenID,
        time.Now().UnixNano())

    // 3. 计算 SHA256 哈希
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:]), nil
}
```

#### 用途
- AWS Builder ID 和 Google OAuth 需要设备指纹
- 用于关联设备注册信息
- 防止 token 被跨设备滥用

### 10. 请求执行与重试

**位置**: `internal/runtime/executor/kiro_executor.go`

#### Execute 流程
```go
func (e *KiroExecutor) Execute(ctx, auth, req, opts) (Response, error) {
    // 1. 获取 rate limiter
    limiter := kiro.GetRateLimiterSingleton()
    tokenKey := auth.ID

    // 2. 等待速率限制
    limiter.WaitForToken(tokenKey)

    // 3. 检查是否可用
    if !limiter.IsTokenAvailable(tokenKey) {
        return nil, errors.New("token unavailable due to quota or cooldown")
    }

    // 4. 提取 access token
    accessToken, ok := auth.Metadata["access_token"].(string)
    if !ok || accessToken == "" {
        return nil, errors.New("access token not found")
    }

    // 5. 构建请求
    kiroReq := buildKiroRequest(req, accessToken)

    // 6. 执行请求
    resp, err := httpClient.Do(kiroReq)
    if err != nil {
        limiter.MarkTokenFailed(tokenKey)
        return nil, err
    }

    // 7. 处理响应
    if resp.StatusCode == 429 {
        // Rate limit exceeded
        limiter.MarkTokenFailed(tokenKey)
        limiter.CheckAndMarkSuspended(tokenKey, "rate limit exceeded")
        return nil, ErrRateLimitExceeded
    }

    if resp.StatusCode >= 500 {
        // Server error
        limiter.MarkTokenFailed(tokenKey)
        return nil, ErrServerError
    }

    // 8. 成功
    limiter.MarkTokenSuccess(tokenKey)
    return parseResponse(resp)
}
```

#### 重试机制
```go
func executeWithRetry(ctx, auth, req, opts) (Response, error) {
    var lastErr error
    retryCount := getRetryCount(auth, opts)  // 默认 3 次

    for attempt := 0; attempt <= retryCount; attempt++ {
        resp, err := execute(ctx, auth, req, opts)
        if err == nil {
            return resp, nil
        }

        lastErr = err

        // 判断是否可重试
        if !isRetryable(err) {
            break
        }

        // 检查是否超过最大重试间隔
        if isInCooldown(auth) {
            cooldown := getCooldownRemaining(auth)
            maxWait := getMaxRetryInterval(opts)  // 默认 30 秒
            if cooldown > maxWait {
                break  // 冷却时间太长，放弃重试
            }
            time.Sleep(cooldown)
        }

        // 指数退避
        backoff := calculateBackoff(attempt)
        time.Sleep(backoff)
    }

    return nil, lastErr
}
```

## 配置示例

### config.yaml
```yaml
# Kiro 订阅账号配置
kiro:
  # 方式 1: 指定 token 文件路径
  - token-file: "~/.aws/sso/cache/kiro-auth-token.json"
    agent-task-type: "vibe"  # 可选: "vibe" 或空

  # 方式 2: 直接提供 token
  - access-token: "aoaAAAAA..."
    refresh-token: "aorAAAAA..."
    profile-arn: "arn:aws:codewhisperer:us-east-1:..."

  # 方式 3: AWS IDC 账号
  - access-token: "aoaAAAAA..."
    refresh-token: "aorAAAAA..."
    client-id: "..."
    client-secret: "..."
    region: "us-east-1"
    start-url: "https://my-sso-portal.awsapps.com/start"
    auth-method: "idc"

# 路由策略
routing:
  strategy: "round-robin"  # 或 "fill-first"

# 请求重试配置
request-retry: 3
max-retry-interval: 30  # 秒
```

### 登录命令

```bash
# AWS Builder ID (设备码流程)
./CLIProxyAPI -kiro-aws-login

# AWS Builder ID (授权码流程，更好的 UX)
./CLIProxyAPI -kiro-aws-authcode

# AWS IDC (带区域选择)
./CLIProxyAPI -kiro-login

# 从 Kiro IDE 导入
./CLIProxyAPI -kiro-import

# 高级选项
./CLIProxyAPI -kiro-login -no-browser      # 不自动打开浏览器
./CLIProxyAPI -kiro-login -incognito       # 使用无痕模式
```

## 技术要点

### 1. 并发安全
- 所有状态修改都使用 `sync.Mutex` 或 `sync.RWMutex` 保护
- 使用 `atomic.Value` 存储配置快照
- 使用 `semaphore` 控制并发刷新数量

### 2. 性能优化
- Token 扫描只在启动和定时任务时进行
- 使用内存缓存避免频繁读取文件
- 批量处理 + 并发刷新提升吞吐量
- 节流控制避免刷新风暴

### 3. 容错设计
- 优雅降级：刷新失败时继续使用旧 token
- 回调隔离：使用 defer+recover 防止 panic 传播
- 原子更新：使用临时文件保证写入一致性
- 多级重试：请求级重试 + 配额级重试

### 4. 可观测性
- 详细的日志记录（刷新、冷却、配额）
- 状态跟踪（可用性、配额、错误）
- Metrics 集成（请求计数、失败率）

### 5. 扩展性
- 支持多种认证方式（Google/GitHub/AWS）
- 可插拔的 Selector 策略
- 灵活的配置覆盖机制
- 支持多 token 存储后端（File/Git/Postgres/S3）

## 调试建议

### 启用调试日志
```yaml
debug: true
```

### 检查 Token 状态
```bash
# 查看所有 token 文件
ls -lh ~/.cli-proxy-api/kiro-*.json

# 查看 token 内容
cat ~/.cli-proxy-api/kiro-aws-user@example.com.json | jq .

# 检查过期时间
cat ~/.cli-proxy-api/kiro-*.json | jq -r '"\(.email // .profile_arn): \(.expires_at)"'
```

### 监控后台刷新
```bash
# 查看日志中的刷新记录
tail -f logs/app.log | grep "background refresh"
tail -f logs/app.log | grep "token repository"
```

### 排查速率限制
```bash
# 查看 rate limiter 状态
tail -f logs/app.log | grep "rate limit"
tail -f logs/app.log | grep "cooldown"
```

## 常见问题

### Q1: Token 刷新失败怎么办？
A:
1. 检查网络连接和代理配置
2. 验证 `client_id` 和 `client_secret` 是否正确
3. 对于 IDC 账号，确认 `region` 和 `start_url` 配置正确
4. 查看日志中的详细错误信息

### Q2: 如何避免配额超限？
A:
1. 合理配置 `daily_max_requests`
2. 使用多个账号进行负载均衡
3. 启用配额冷却机制（默认开启）
4. 监控 `QuotaState` 状态

### Q3: 如何切换路由策略？
A:
```yaml
routing:
  strategy: "fill-first"  # 改为 fill-first 策略
```

### Q4: 如何禁用某个账号？
A:
在 token 文件中添加：
```json
{
  "disabled": true
}
```

或者删除对应的 token 文件。

## 相关文件

### 核心实现
- `internal/auth/kiro/token_repository.go` - Token 持久化
- `internal/auth/kiro/background_refresh.go` - 后台刷新
- `internal/auth/kiro/refresh_manager.go` - 刷新管理器
- `internal/auth/kiro/rate_limiter.go` - 速率限制
- `internal/auth/kiro/cooldown.go` - 冷却管理

### SDK 接口
- `sdk/auth/kiro.go` - Kiro 认证器
- `sdk/cliproxy/auth/conductor.go` - Auth 管理器
- `sdk/cliproxy/auth/selector.go` - 选择器策略
- `sdk/cliproxy/auth/types.go` - 类型定义

### 执行器
- `internal/runtime/executor/kiro_executor.go` - Kiro 请求执行器
- `internal/translator/kiro/` - 协议转换器

### OAuth 实现
- `internal/auth/kiro/oauth.go` - Google OAuth
- `internal/auth/kiro/sso_oidc.go` - AWS SSO OIDC
- `internal/auth/kiro/aws_auth.go` - AWS Builder ID

## 版本历史

- v6.0: 初始实现，支持 Google/GitHub OAuth
- v6.1: 添加 AWS Builder ID 支持
- v6.2: 添加 AWS IDC 支持和区域化刷新
- v6.3: 优化后台刷新性能和容错机制
- v6.4: 添加配额管理和冷却机制
