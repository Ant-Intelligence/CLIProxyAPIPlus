# 上游 HTTP 错误码处理逻辑

本文档说明 CLIProxyAPIPlus 在收到上游 API（以 Kiro 为主）返回的各类 HTTP 错误码时的处理策略。

## 错误码处理总览

| HTTP 状态码 | 含义 | 处理策略 | 是否重试 | 是否切换端点 |
|-------------|------|----------|----------|--------------|
| **400** | 请求格式错误 / 参数校验失败 | 立即返回错误 | 否 | 否 |
| **401** | 未授权 / Token 过期 | 刷新 Token 后重试 | 是（刷新成功时） | 否 |
| **402** | 月度配额用尽 | 立即返回错误 | 否 | 否 |
| **429** | 请求频率限制 | Token 进入冷却期，切换下一个端点 | 否（当前端点） | 是 |
| **5xx** | 服务端错误 | 指数退避重试（最多 3 次） | 是 | 否 |

## 详细说明

### 400 — 请求格式错误

**代码位置：** `internal/runtime/executor/kiro_executor.go:1261-1272`

**处理逻辑：**
1. 读取响应体
2. 记录警告日志
3. **立即返回错误，不做任何重试**

**设计理由：** 400 错误表示请求本身存在问题（如参数缺失、格式不合法），重试相同的请求必然还会失败，因此没有重试的意义。同时也不切换端点，因为问题出在请求内容而非端点本身。

**典型错误示例：**
```json
{"message": "Improperly formed request.", "reason": null}
```

**日志示例：**
```
[warn] [kiro_executor.go:1268] kiro: received 400 error (attempt 1/3), body: {"message":"Improperly formed request.","reason":null}
```

> **关于日志中的 `attempt 1/3`：** 这个数字具有误导性。`1/3` 表示"第 1 次尝试，共 3 次机会"（重试循环设定 `maxRetries=2`，即 attempt 0/1/2 共 3 次），但 **400 错误在第 1 次尝试后就直接 `return` 退出了，不会进行第 2、3 次重试**。这里只是复用了循环变量打印日志，对 400 而言永远只会看到 `attempt 1/3`。真正会用到多次重试的是 401（Token 刷新后重试）和 5xx（指数退避重试）场景。

---

### 401 — Token 过期或无效

**代码位置：** `internal/runtime/executor/kiro_executor.go:1274-1307`

**处理逻辑：**
1. 读取响应体
2. 调用 `e.Refresh(ctx, auth)` 尝试刷新 Token
3. 如果刷新成功：
   - 持久化新 Token 到文件
   - 使用新凭据重建请求
   - 重试请求
4. 如果刷新失败：立即返回 401 错误

**设计理由：** Token 过期是可恢复的临时性错误，通过刷新 Token 即可解决，值得重试。

---

### 402 — 月度配额用尽

**代码位置：** `internal/runtime/executor/kiro_executor.go:1309-1319`

**处理逻辑：**
1. 读取响应体
2. 直接将上游错误体透传返回

**设计理由：** 月度配额限制是账户级别的限制，无法通过重试或切换端点解决。

---

### 429 — 请求频率限制

**代码位置：** `internal/runtime/executor/kiro_executor.go:1208-1231`

**处理逻辑：**
1. 读取响应体
2. 将当前 Token 设置为冷却状态（`SetCooldown`）
3. 记录最后一次 429 错误（`last429Err`）
4. **跳出内层重试循环，尝试下一个端点**

**设计理由：** 429 表示当前端点/Token 的配额已耗尽，但其他端点可能仍有剩余配额。通过切换端点可以继续服务请求。被限流的 Token 会进入冷却期，在冷却时间结束前不会再被选用。

---

### 5xx — 服务端错误

**代码位置：** `internal/runtime/executor/kiro_executor.go:1233-1259`

**处理逻辑：**
1. 读取响应体
2. 判断是否为可重试状态码（502、503、504）：
   - 是：使用 `retryConfig` 计算延迟，进行重试
   - 否（500、501 等）：使用指数退避重试（`2^attempt` 秒，上限 30 秒）
3. 所有重试耗尽后返回错误

**设计理由：** 服务端错误通常是暂时性的，通过等待后重试大概率可以恢复。502/503/504 尤其常见于上游服务临时不可用的场景。

---

## 错误传播链路

```
上游 API 返回错误
    ↓
Kiro Executor: 构造 statusErr{code, msg}
    ↓
Handler 层 (sdk/api/handlers/handlers.go):
    通过 StatusCode() 接口提取状态码
    ↓
HTTP 响应: 将上游状态码和错误体透传给客户端
```

### statusErr 结构体

**定义位置：** `internal/runtime/executor/openai_compat_executor.go:386-390`

```go
type statusErr struct {
    code       int
    msg        string
    retryAfter *time.Duration
}
```

实现了以下接口方法：
- `Error() string` — 返回上游错误消息体
- `StatusCode() int` — 返回 HTTP 状态码
- `RetryAfter() *time.Duration` — 返回重试等待时间（如有）

### Handler 层处理

**代码位置：** `sdk/api/handlers/handlers.go:397-410`

Handler 通过 Go 的接口类型断言提取 `StatusCode()`，将上游的 HTTP 状态码**原样透传**给客户端。客户端收到的状态码与上游返回的状态码一致。

---

## 常见排查指南

| 客户端收到的错误 | 可能原因 | 排查方向 |
|------------------|----------|----------|
| 400 Improperly formed request | 请求体格式不符合上游 API 要求 | 检查请求中的 model 名称、消息格式、参数是否正确 |
| 401 Unauthorized | Token 过期且刷新失败 | 检查 Token 文件是否存在、refresh token 是否有效，尝试重新登录 |
| 402 Monthly limit | 账户月度配额已用完 | 等待配额重置或更换账户 |
| 429 Too Many Requests | 所有端点/Token 均被限流 | 等待冷却期结束，或增加更多 Token |
| 502/503/504 | 上游服务暂时不可用，重试仍失败 | 等待上游恢复，检查上游服务状态 |
