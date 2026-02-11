# Kiro 缓存 Token 分配策略总结

## 概述

CLIProxyAPI Plus 的 Kiro 集成使用了一个 **1:2:25 固定比例分配算法**，将输入 token 分配到三个类别，以模拟 Claude API 的 prompt caching 行为。

## 核心算法

### 分配比例: 1:2:25

```
总份数 = 1 + 2 + 25 = 28 份

input_tokens              = floor(total_tokens * 1/28)   // 3.57%
cache_creation_input_tokens = floor(total_tokens * 2/28)   // 7.14%
cache_read_input_tokens   = total_tokens - input - creation  // 89.29% (余数)
```

### 分配阈值

- **最小阈值**: `100 tokens`
- **低于 100**: 全部分配给 `input_tokens`，不启用缓存
- **≥ 100**: 应用 1:2:25 分配比例

## 实现代码

**位置**: `internal/usage/token_distribution.go`

```go
type CacheTokenDistribution struct {
    InputTokens              int64  // 普通输入 tokens
    CacheCreationInputTokens int64  // 缓存创建 tokens
    CacheReadInputTokens     int64  // 缓存读取 tokens
}

func DistributeCacheTokens(totalInputTokens int64) CacheTokenDistribution {
    // 低于阈值：不分配
    if totalInputTokens < 100 {
        return CacheTokenDistribution{
            InputTokens: totalInputTokens,
        }
    }

    // 应用 1:2:25 比例
    input := totalInputTokens * 1 / 28
    creation := totalInputTokens * 2 / 28
    read := totalInputTokens - input - creation  // 余数全部给 read

    return CacheTokenDistribution{
        InputTokens:              input,
        CacheCreationInputTokens: creation,
        CacheReadInputTokens:     read,
    }
}
```

## 示例计算

### 示例 1: 1000 tokens

```go
DistributeCacheTokens(1000)

返回:
  InputTokens:              35    // 1000 * 1/28 = 35.71 → 35
  CacheCreationInputTokens: 71    // 1000 * 2/28 = 71.42 → 71
  CacheReadInputTokens:     894   // 1000 - 35 - 71 = 894

验证: 35 + 71 + 894 = 1000 ✓
```

### 示例 2: 2800 tokens (完美分割)

```go
DistributeCacheTokens(2800)

返回:
  InputTokens:              100   // 2800 * 1/28 = 100
  CacheCreationInputTokens: 200   // 2800 * 2/28 = 200
  CacheReadInputTokens:     2500  // 2800 * 25/28 = 2500

验证: 100 + 200 + 2500 = 2800 ✓
```

### 示例 3: 50 tokens (低于阈值)

```go
DistributeCacheTokens(50)

返回:
  InputTokens:              50    // 低于 100，不分配
  CacheCreationInputTokens: 0
  CacheReadInputTokens:     0
```

## 应用场景

### 1. 非流式响应

**文件**: `internal/translator/kiro/claude/kiro_claude_response.go:117-129`

```go
func BuildClaudeResponse(content, tools, model, usageInfo, stopReason) {
    // 应用缓存分配
    distributed := internalusage.DistributeCacheTokens(usageInfo.InputTokens)

    // 构建 usage map
    usageMap := map[string]interface{}{
        "input_tokens":  distributed.InputTokens,
        "output_tokens": usageInfo.OutputTokens,
    }

    // 只在有缓存 tokens 时添加这些字段
    if distributed.CacheCreationInputTokens > 0 {
        usageMap["cache_creation_input_tokens"] = distributed.CacheCreationInputTokens
    }
    if distributed.CacheReadInputTokens > 0 {
        usageMap["cache_read_input_tokens"] = distributed.CacheReadInputTokens
    }

    // 返回 Claude 格式响应
    response := map[string]interface{}{
        "usage": usageMap,
        // ... 其他字段
    }
}
```

### 2. 流式响应

**文件**: `internal/translator/kiro/claude/kiro_claude_stream.go`

应用于两个关键事件:

#### message_start 事件 (第 16 行)
```go
func BuildClaudeMessageStartEvent(model, inputTokens) {
    distributed := internalusage.DistributeCacheTokens(inputTokens)

    usageMap := map[string]interface{}{
        "input_tokens": distributed.InputTokens,
        "output_tokens": 0,
    }
    if distributed.CacheCreationInputTokens > 0 {
        usageMap["cache_creation_input_tokens"] = distributed.CacheCreationInputTokens
    }
    if distributed.CacheReadInputTokens > 0 {
        usageMap["cache_read_input_tokens"] = distributed.CacheReadInputTokens
    }
    // ... 生成 SSE 事件
}
```

#### message_delta 事件 (第 127 行)
```go
func BuildClaudeMessageDeltaEvent(stopReason, usageInfo) {
    distributed := internalusage.DistributeCacheTokens(usageInfo.InputTokens)

    // 与 message_start 相同的分配逻辑
    // 确保客户端看到一致的缓存 token 分布
}
```

## 设计目的

### 1. API 兼容性
模拟 Claude API 的 prompt caching 响应格式，让客户端能够正确解析 usage 字段。

### 2. 成本估算
区分不同类型的 token，帮助客户端：
- 普通输入 tokens (最贵)
- 缓存创建 tokens (次贵)
- 缓存读取 tokens (最便宜)

### 3. 透明性
Kiro 原生 API 不区分这些 token 类型，通过固定比例分配提供统一的客户端体验。

### 4. 保守估算
大部分 token（89.29%）分配给 `cache_read_input_tokens`（最便宜），避免成本高估，对用户友好。

## 实现特点

| 特点 | 说明 |
|------|------|
| **确定性** | 相同输入总是产生相同的分配结果 |
| **完整性** | 三个分类的总和始终等于原始 `totalInputTokens` |
| **零开销** | 纯数学计算，无 I/O、无状态、无副作用 |
| **向后兼容** | 总 token 数不变，只是细分类别 |
| **线程安全** | 无状态函数，天然并发安全 |

## 工具方法

```go
// 验证分配正确性：计算总输入 tokens
func (d CacheTokenDistribution) TotalInputTokens() int64 {
    return d.InputTokens +
           d.CacheCreationInputTokens +
           d.CacheReadInputTokens
}

// 检查是否启用了缓存分配
func (d CacheTokenDistribution) HasCacheTokens() bool {
    return d.CacheCreationInputTokens > 0 ||
           d.CacheReadInputTokens > 0
}
```

## 与真实 Claude Prompt Caching 的区别

| 特性 | Kiro 模拟策略 | 真实 Claude Caching |
|------|--------------|-------------------|
| **分配方式** | 固定 1:2:25 比例 | 基于实际缓存行为 |
| **阈值** | 100 tokens | 1024 tokens (最小可缓存) |
| **动态性** | 静态分配 | 动态检测缓存命中/未命中 |
| **准确性** | 估算 | 精确计量 |
| **用途** | API 格式兼容 | 实际成本节省 |

## 调试技巧

### 查看分配结果

```go
import "github.com/router-for-me/CLIProxyAPI/v6/internal/usage"

// 测试分配
dist := usage.DistributeCacheTokens(1000)
fmt.Printf("Input: %d, Creation: %d, Read: %d\n",
    dist.InputTokens,
    dist.CacheCreationInputTokens,
    dist.CacheReadInputTokens)

// 验证总和
total := dist.TotalInputTokens()
fmt.Printf("Total: %d (should be 1000)\n", total)

// 检查是否有缓存
if dist.HasCacheTokens() {
    fmt.Println("Cache distribution is active")
}
```

### 响应示例

**非流式 (JSON)**:
```json
{
  "usage": {
    "input_tokens": 35,
    "cache_creation_input_tokens": 71,
    "cache_read_input_tokens": 894,
    "output_tokens": 256
  }
}
```

**流式 (SSE message_start)**:
```
event: message_start
data: {
  "type": "message_start",
  "message": {
    "usage": {
      "input_tokens": 35,
      "cache_creation_input_tokens": 71,
      "cache_read_input_tokens": 894,
      "output_tokens": 0
    }
  }
}
```

## 相关文件

- `internal/usage/token_distribution.go` - 核心算法实现
- `internal/translator/kiro/claude/kiro_claude_response.go` - 非流式响应应用
- `internal/translator/kiro/claude/kiro_claude_stream.go` - 流式响应应用

## 版本历史

- **v6.x**: 初始实现 (feature/cache-token-distribution 分支)
- 基于 AIClient-2-API 项目的 `RatioTokenDistribution.js` Node.js 实现移植

## 总结

Kiro 的缓存 token 分配策略是一个简单而优雅的解决方案：

✅ **简单**: 固定 1:2:25 比例，易于理解和维护
✅ **兼容**: 完全符合 Claude API 格式要求
✅ **保守**: 89% 分配给最便宜的 `cache_read`
✅ **可靠**: 确定性算法，无副作用
✅ **高效**: 纯计算，零开销
