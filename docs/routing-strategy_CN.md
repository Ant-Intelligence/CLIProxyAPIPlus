# 路由策略配置文档

## 概述

CLIProxyAPI Plus 支持灵活的凭证路由策略，用于在多个可用凭证（账号）之间选择执行请求。路由策略决定了当存在多个可用凭证时，系统如何选择使用哪一个凭证来处理 API 请求。

## 配置位置

在 `config.yaml` 中配置路由策略：

```yaml
routing:
  strategy: "round-robin"  # round-robin (默认), fill-first
```

也可以通过管理 API 动态修改：

```bash
# 查询当前策略
GET /v0/management/routing/strategy

# 修改策略
PUT /v0/management/routing/strategy
Content-Type: application/json

{
  "value": "fill-first"
}
```

## 支持的路由策略

### 1. Round-Robin（轮询，默认）

**别名**: `round-robin`, `roundrobin`, `rr`

#### 工作原理

轮询策略会在所有可用凭证之间循环分配请求。系统为每个 `provider:model` 组合维护一个游标（cursor），每次请求时选择下一个可用凭证，然后递增游标。

**执行流程**：
1. 收集所有可用的凭证（排除禁用和冷却中的凭证）
2. 按优先级分组，选择最高优先级的凭证组
3. 在该组内按 ID 排序，确保顺序确定性
4. 使用游标选择当前索引位置的凭证：`credentials[cursor % len(credentials)]`
5. 递增游标，下次请求将使用下一个凭证

**示例**：

假设有 3 个可用凭证 `[A, B, C]`：

```
请求1 -> 凭证 A (cursor=0, 0%3=0)
请求2 -> 凭证 B (cursor=1, 1%3=1)
请求3 -> 凭证 C (cursor=2, 2%3=2)
请求4 -> 凭证 A (cursor=3, 3%3=0)
请求5 -> 凭证 B (cursor=4, 4%3=1)
...
```

#### 优点

- **负载均衡**: 所有凭证使用均匀，避免单个账号过载
- **延长使用时间**: 分散请求可以延长每个账号达到配额限制的时间
- **降低封禁风险**: 避免频繁使用单个账号触发速率限制

#### 适用场景

- 多个凭证配额相近，希望均匀使用
- 希望最大化总体吞吐量
- 适合持续高负载的生产环境

### 2. Fill-First（填充优先）

**别名**: `fill-first`, `fillfirst`, `ff`

#### 工作原理

填充优先策略总是选择第一个可用的凭证（按 ID 排序）。这会"用完"一个账号的配额后才会使用下一个账号。

**执行流程**：
1. 收集所有可用的凭证（排除禁用和冷却中的凭证）
2. 按优先级分组，选择最高优先级的凭证组
3. 在该组内按 ID 排序
4. 总是返回列表中的第一个凭证：`credentials[0]`

**示例**：

假设有 3 个可用凭证 `[A, B, C]`：

```
请求1 -> 凭证 A
请求2 -> 凭证 A
请求3 -> 凭证 A
...
请求N -> 凭证 A (直到 A 触发配额限制进入冷却)
请求N+1 -> 凭证 B (A 冷却中)
请求N+2 -> 凭证 B
...
```

#### 优点

- **错开配额窗口**: 有助于错开滚动窗口的订阅上限（如每小时/每天的消息限制）
- **预测性强**: 行为确定，易于理解和调试
- **保留备用**: 备用凭证保持"新鲜"状态，适合紧急使用

#### 适用场景

- API 提供商使用滚动时间窗口限制（如"每小时 X 次请求"）
- 希望最大化单个凭证的配额利用
- 有明确的主备凭证概念
- 流量不均匀，有明显的高峰和低谷

## 高级特性

### 优先级（Priority）

两种策略都支持凭证优先级。优先级高的凭证会优先被选择，只有在高优先级凭证不可用时才会使用低优先级凭证。

**配置方式**：

凭证的优先级通过 `priority` 属性设置（在凭证文件的 `attributes` 字段中）：

```json
{
  "id": "credential-1",
  "attributes": {
    "priority": "10"
  }
}
```

- 优先级值越大越优先（默认为 0）
- 相同优先级的凭证按配置的策略（round-robin 或 fill-first）选择
- 优先级可以用于区分生产/测试凭证，或高速/经济凭证

**示例**：

```
凭证 A (priority=10)
凭证 B (priority=10)
凭证 C (priority=0)

Round-Robin: A -> B -> A -> B -> ... (只在 C 不可用时才使用 C)
Fill-First:  A -> A -> A -> ... (只在 A 不可用时使用 B)
```

### 自动冷却管理

两种策略都会自动跳过处于冷却期的凭证：

- **配额超限**: 当凭证触发 API 配额限制时，自动进入冷却期
- **错误恢复**: 发生特定错误后，按退避策略暂时禁用
- **模型级冷却**: 可以针对特定模型设置冷却，不影响其他模型

当所有凭证都处于冷却期时，系统会返回 `429 Too Many Requests` 错误，并在响应头中包含 `Retry-After`，指示最早可用凭证的恢复时间。

### 禁用状态

可以手动禁用特定凭证或凭证的特定模型，禁用的凭证不会被任何策略选择。

## 策略对比

| 特性 | Round-Robin | Fill-First |
|------|-------------|------------|
| **负载分布** | 均匀分布到所有凭证 | 集中在第一个可用凭证 |
| **配额利用** | 缓慢消耗每个凭证 | 快速消耗当前凭证 |
| **切换频率** | 每次请求可能切换 | 仅在冷却时切换 |
| **预测性** | 中等 | 高 |
| **备用保护** | 无特殊保护 | 自动保留备用凭证 |
| **适合场景** | 持续高负载 | 波动负载，有滚动窗口限制 |
| **复杂度** | 需维护游标状态 | 无状态，简单 |

## 配置示例

### 示例 1: 生产环境，高吞吐量

```yaml
# 均匀分配负载到所有账号
routing:
  strategy: "round-robin"

# 多个等价凭证
# 每个账号配额：1000 请求/小时
# 预期总负载：2500 请求/小时
# 配置 3 个账号，round-robin 分配
```

### 示例 2: 有主备凭证，滚动窗口限制

```yaml
# 优先使用主凭证，备用凭证待命
routing:
  strategy: "fill-first"

# 主凭证：高速账号（优先使用）
# 备用凭证：经济账号（主账号冷却时使用）
# API 限制：100 请求/小时（滚动窗口）
```

### 示例 3: 混合优先级，round-robin

```yaml
routing:
  strategy: "round-robin"

# 凭证配置（在认证文件中）：
# - premium-1 (priority: 10)
# - premium-2 (priority: 10)
# - standard-1 (priority: 0)
# - standard-2 (priority: 0)

# 行为：在 premium-1 和 premium-2 之间轮询
# 仅当两个 premium 凭证都不可用时，才使用 standard 凭证
```

## 动态修改策略

可以在运行时通过管理 API 修改路由策略，无需重启服务：

```bash
# 查询当前策略
curl http://localhost:8317/v0/management/routing/strategy \
  -H "X-Management-Key: your-secret-key"

# 响应
{
  "strategy": "round-robin"
}

# 修改为 fill-first
curl -X PUT http://localhost:8317/v0/management/routing/strategy \
  -H "X-Management-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"value": "fill-first"}'
```

策略修改会立即生效，影响所有后续请求。

## 实现细节

### 源码位置

- **配置定义**: `internal/config/config.go` - `RoutingConfig` 结构
- **策略实现**: `sdk/cliproxy/auth/selector.go`
  - `RoundRobinSelector` - 轮询选择器
  - `FillFirstSelector` - 填充优先选择器
- **策略创建**: `sdk/cliproxy/builder.go` - 根据配置创建对应的选择器
- **管理 API**: `internal/api/handlers/management/config_basic.go` - 策略查询和修改接口

### 策略接口

所有路由策略都实现 `Selector` 接口：

```go
type Selector interface {
    Pick(ctx context.Context, provider, model string, opts Options, auths []*Auth) (*Auth, error)
}
```

### 线程安全

- `RoundRobinSelector` 使用互斥锁保护游标状态，支持并发访问
- `FillFirstSelector` 无状态，天然线程安全

## 故障排查

### 问题：所有凭证都在冷却中

**现象**: 收到 `429 Too Many Requests` 错误，提示所有凭证都在冷却

**原因**: 请求速率超过所有凭证的总配额

**解决方案**:
1. 增加更多凭证
2. 降低请求速率
3. 等待 `Retry-After` 指示的时间后重试
4. 检查是否有凭证被错误禁用

### 问题：Round-Robin 未均匀分配

**现象**: 某些凭证使用次数明显多于其他凭证

**可能原因**:
1. 凭证优先级不同（预期行为）
2. 某些凭证频繁进入冷却期
3. 多个 provider:model 组合有各自独立的游标

**排查方法**:
- 检查凭证配置的 `priority` 属性
- 查看管理 API 的凭证状态，确认冷却情况
- 注意不同模型的游标是独立的

### 问题：Fill-First 不切换到下一个凭证

**现象**: 即使当前凭证已达配额，仍在使用同一个凭证

**可能原因**:
1. 冷却机制未正确触发
2. 配额检测延迟
3. 凭证状态未正确更新

**排查方法**:
- 检查 `disable-cooling` 配置是否意外启用
- 查看错误日志，确认是否正确识别配额超限
- 通过管理 API 查看凭证状态，确认 `NextRetryAfter` 时间

## 最佳实践

1. **生产环境推荐使用 Round-Robin**：除非有特殊需求，否则 round-robin 通常是更稳健的选择

2. **合理设置优先级**：使用优先级区分不同用途的凭证，例如：
   - priority=10: 生产凭证
   - priority=5: 测试凭证
   - priority=0: 应急备用凭证

3. **监控凭证状态**：定期通过管理 API 检查凭证状态，及时发现配额问题

4. **配额规划**：根据实际负载和 API 限制合理规划凭证数量

5. **测试切换**：在上线前测试凭证失效时的自动切换行为

## 参考资料

- [CLAUDE.md](../CLAUDE.md) - 项目架构文档
- [配置示例](../config.example.yaml) - 完整配置示例
- [SDK 访问控制文档](./sdk-access_CN.md) - 访问控制详细说明

## 更新日志

- 2025-02: 初始版本，支持 round-robin 和 fill-first 策略
