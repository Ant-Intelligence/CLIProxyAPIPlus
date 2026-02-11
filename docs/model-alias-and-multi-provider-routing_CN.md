# 模型别名与多 Provider 路由文档

## 概述

CLIProxyAPI Plus 支持通过 `oauth-model-alias` 和 per-credential 别名配置为模型设置客户端可见的别名。同时，当多个 Provider（如 kiro 和 antigravity）提供相同模型时，系统会将它们的凭证混合在一起，通过统一的路由策略进行选择和故障转移。

本文档详细说明两个场景的行为：
1. **模型别名冲突** — 多个模型配置了相同别名时的处理逻辑
2. **多 Provider 同模型路由** — 不同 Provider 提供相同模型时的请求路由与故障转移

## 场景一：模型别名冲突

### 别名配置方式

系统存在两套独立的别名机制：

| 机制 | 配置位置 | 适用范围 |
|------|----------|----------|
| **OAuth 模型别名** | `config.yaml` → `oauth-model-alias` | OAuth/文件凭证 channel |
| **API Key 模型别名** | `config.yaml` → per-credential `models` | API Key 凭证 |

#### OAuth 模型别名

在 `config.yaml` 中按 channel 配置：

```yaml
oauth-model-alias:
  antigravity:
    - name: "rev19-uic3-1p"                    # 上游真实模型名
      alias: "gemini-2.5-computer-use-preview"  # 客户端可见的别名
    - name: "gemini-3-pro-image"
      alias: "gemini-3-pro-image-preview"
  gemini-cli:
    - name: "gemini-2.5-pro"
      alias: "g2.5p"
      fork: true   # fork=true 时保留原始模型名，同时添加别名作为额外模型
  kiro:
    - name: "claude-opus-4-6"
      alias: "kiro-opus"
```

**支持的 channel：** `gemini-cli`、`vertex`、`aistudio`、`antigravity`、`claude`、`codex`、`qwen`、`iflow`、`kiro`、`github-copilot`

> **注意：** OAuth 模型别名**不适用于** API Key 类凭证（`gemini-api-key`、`codex-api-key`、`claude-api-key`、`openai-compatibility`、`vertex-api-key`、`ampcode`）。这些凭证使用各自的 per-credential 别名机制。

#### API Key 模型别名

在各 API Key 凭证配置的 `models` 字段中定义别名：

```yaml
gemini-api-key:
  - api-key: "AIza..."
    models:
      - name: "gemini-2.5-pro-exp-03-25"
        alias: "gemini-2.5-pro"
```

### 同一 Channel 内的别名冲突：First-Win 策略

当同一 channel 内多个不同的上游模型被映射到**相同的别名**时，系统采用 **first-win（先到先得）** 策略：

- YAML 中**先出现**的别名定义生效
- 后续重复的别名定义被**静默丢弃**
- **不会产生日志警告**

```yaml
oauth-model-alias:
  antigravity:
    - name: "model-A"
      alias: "my-model"    # ✅ 生效：my-model → model-A
    - name: "model-B"
      alias: "my-model"    # ❌ 静默丢弃：alias "my-model" 已被 model-A 占用
    - name: "model-C"
      alias: "MY-MODEL"    # ❌ 静默丢弃：大小写不敏感，等同于 "my-model"
```

**编译逻辑**（`sdk/cliproxy/auth/oauth_model_alias.go:20-56`）：

```go
aliasKey := strings.ToLower(alias)
if _, exists := rev[aliasKey]; exists {
    continue  // first-win: 后续重复别名直接跳过
}
rev[aliasKey] = name
```

#### 关键行为

1. **大小写不敏感**：别名在存储和查找时统一转为小写（`strings.ToLower`），因此 `"My-Model"` 和 `"my-model"` 视为相同别名
2. **自身别名跳过**：当 `name` 与 `alias` 相同时（大小写不敏感），该条目被跳过
3. **无冲突警告**：被丢弃的别名不会产生任何日志输出，需要开发者自行注意 YAML 中的顺序

### 跨 Channel 的相同别名

不同 channel 之间的别名**完全隔离**，互不影响：

```yaml
oauth-model-alias:
  antigravity:
    - name: "upstream-model-x"
      alias: "fast-model"       # antigravity channel 的别名
  kiro:
    - name: "upstream-model-y"
      alias: "fast-model"       # kiro channel 的别名，与 antigravity 互不冲突
```

别名表的内部结构按 channel 分隔（`map[channel]map[alias]name`），因此相同别名可以在不同 channel 中指向不同的上游模型。

### API Key 凭证的别名

API Key 凭证的别名同样采用 **first-win** 策略（`sdk/cliproxy/auth/conductor.go:326-370`）：

```go
// Config priority: first alias wins.
if _, exists := out[aliasKey]; exists {
    continue
}
out[aliasKey] = name
```

行为与 OAuth 别名一致：先定义的生效，后续重复静默丢弃，大小写不敏感。

### Fork 模式

当 `fork: true` 时，别名和原始模型名**同时存在**于模型列表中。客户端可以使用任一名称发送请求：

```yaml
oauth-model-alias:
  gemini-cli:
    - name: "gemini-2.5-pro"
      alias: "g2.5p"
      fork: true
```

上述配置后，模型列表中同时包含 `gemini-2.5-pro` 和 `g2.5p`，两者都路由到 `gemini-2.5-pro`。

`fork: false`（默认）时，别名**替换**原始模型名，客户端只能看到并使用别名。

### Thinking Suffix 处理

别名系统与 thinking suffix（如 `model-name(thinking=1024)`）协作：

1. 查找别名时，先提取 base model name（去掉 suffix）进行匹配
2. 如果配置中的 `name` 本身带有 suffix，则配置的 suffix **优先**
3. 如果用户请求带有 suffix 而配置不带，则用户的 suffix 被**保留**到解析后的模型名上

**源码位置：** `sdk/cliproxy/auth/oauth_model_alias.go:141-198`

### 场景一小结

| 情况 | 行为 |
|------|------|
| 同 channel 内重复别名 | First-win，静默丢弃后续，无日志警告 |
| 跨 channel 相同别名 | 各 channel 独立，互不影响 |
| 大小写差异 | 不敏感，统一转小写 |
| name == alias | 跳过该条目 |
| API Key 别名冲突 | 同样 first-win |
| fork: true | 别名和原名同时存在 |

## 场景二：多 Provider 提供相同模型

### 场景描述

多个 Provider 可以提供相同的模型。例如：

- **kiro**（通过 AWS CodeWhisperer）提供 `claude-opus-4-6`
- **antigravity**（通过 Antigravity CLI）也提供 `claude-opus-4-6`

当客户端请求 `claude-opus-4-6` 时，系统会将所有 Provider 的可用凭证混合在一起，形成统一的候选池。

### 请求路由流程

```
客户端请求 (model: claude-opus-4-6)
        │
        ▼
┌─────────────────────────┐
│  1. 解析模型名           │  提取 base model（去除 thinking suffix）
│     确定目标 provider(s) │  根据模型名查找所有可提供此模型的 provider
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  2. 构建混合候选池       │  遍历所有 auth，过滤条件：
│     (pickNextMixed)      │  - 未禁用
│                          │  - Provider 在目标列表中
│                          │  - 未被本次请求尝试过
│                          │  - 有对应的 executor
│                          │  - Registry 确认支持该模型
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  3. Selector 选择凭证    │  按优先级分组 → 取最高优先级组
│     (Pick)               │  同优先级内按 ID 字母序排序
│                          │  按路由策略（round-robin/fill-first）选择
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  4. 应用别名            │  applyOAuthModelAlias → applyAPIKeyModelAlias
│     (模型名重写)         │  将客户端模型名转为 Provider 期望的上游模型名
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  5. Executor 执行请求    │  由 Provider 对应的 executor 处理
│     (协议转换)           │  kiro executor / antigravity executor 各自
│                          │  独立完成协议转换（→ OpenAI/Claude/Gemini）
└───────────┬─────────────┘
            │
         成功？
        ╱    ╲
      是       否
      │         │
      ▼         ▼
   返回响应   记录错误，标记该 auth 为 "tried"
              回到步骤 2，选择下一个候选
              （可能跨 Provider 故障转移）
```

### 混合候选池

`pickNextMixed()` 函数构建候选池时，**不区分 Provider**，将所有满足条件的凭证混合在一起：

**候选条件**（`sdk/cliproxy/auth/conductor.go:1610-1631`）：

| 检查项 | 说明 |
|--------|------|
| `candidate.Disabled` | 跳过已禁用的凭证 |
| Provider 匹配 | 凭证的 Provider 必须在目标 Provider 列表中 |
| 未尝试过 | 本次请求链中未使用过该凭证 |
| Executor 已注册 | 该 Provider 存在对应的 executor |
| Registry 模型支持 | 全局 Registry 确认该凭证的客户端支持请求的模型 |

### 候选排序规则

候选池构建后，由 `getAvailableAuths()` 进行排序（`sdk/cliproxy/auth/selector.go:142-177`）：

1. **按优先级分组**：每个凭证根据 `attributes.priority` 属性值分组（默认为 0）
2. **选择最高优先级组**：只有最高优先级组的凭证参与选择
3. **组内按 ID 字母序排序**：`sort.Slice(available, func(i, j int) bool { return available[i].ID < available[j].ID })`

**重要：** 排序仅依据优先级和 ID 字母序，不依据"可用客户端数"。如果 kiro 有 3 个凭证、antigravity 有 2 个凭证，它们都在同一优先级组内，将按 ID 字母序混合排列。

**示例**：

假设有以下凭证（优先级均为 0）：

```
kiro-account-1      (provider: kiro)
kiro-account-2      (provider: kiro)
antigravity-user-1  (provider: antigravity)
antigravity-user-2  (provider: antigravity)
```

排序后的候选顺序（按 ID 字母序）：
```
antigravity-user-1 → antigravity-user-2 → kiro-account-1 → kiro-account-2
```

### 自动故障转移

当选中的凭证执行失败时，系统自动尝试下一个候选（`sdk/cliproxy/auth/conductor.go:565-616`）：

```
executeMixedOnce() 循环流程：

1. pickNextMixed() → 获取下一个未尝试过的凭证
2. executor.Execute() → 执行请求
3. 如果成功 → 返回响应
4. 如果失败：
   a. 记录错误到 lastErr
   b. 将该凭证 ID 加入 tried 集合
   c. 如有 RetryAfter → 记录冷却信息 (MarkResult)
   d. 回到步骤 1，尝试下一个凭证
5. 所有候选都失败 → 返回最后一个错误
```

**关键特性**：
- **跨 Provider 故障转移**：kiro 凭证失败后，可以自动切换到 antigravity 凭证
- **无额外配置**：故障转移是内置行为，不需要额外配置
- **冷却感知**：冷却中的凭证在候选池构建阶段就会被排除

### 协议转换透明

不同 Provider 使用各自的 executor 进行协议转换，对客户端完全透明：

| Provider | Executor | 协议转换 |
|----------|----------|----------|
| kiro | Kiro Executor | Kiro ↔ OpenAI（AWS CodeWhisperer 协议） |
| antigravity | Antigravity Executor | Antigravity ↔ Gemini ↔ OpenAI |
| claude | Claude Executor | Claude ↔ OpenAI（Anthropic 原生协议） |
| codex | Codex Executor | OpenAI 原生 |

客户端只需使用标准的 OpenAI 或 Claude API 格式发送请求，系统自动根据选中的 Provider 完成协议转换。

### 配置示例

#### 多 Provider 提供 claude-opus-4-6

```yaml
# config.yaml

api-keys:
  - "sk-my-api-key"

routing:
  strategy: "round-robin"

# kiro 和 antigravity 同时提供 claude-opus-4-6
# 无需额外配置，系统自动发现两个 Provider 都支持该模型
# 登录命令：
#   ./CLIProxyAPI -kiro-aws-login
#   ./CLIProxyAPI -antigravity-login
```

#### 使用优先级区分 Provider

通过凭证的 `priority` 属性，可以控制 Provider 之间的优先级：

```
凭证文件中设置 attributes：
  kiro-account-1:      priority=10   (优先使用 kiro)
  kiro-account-2:      priority=10
  antigravity-user-1:  priority=0    (kiro 不可用时使用 antigravity)
  antigravity-user-2:  priority=0
```

行为：
- 正常情况下只在 `kiro-account-1` 和 `kiro-account-2` 之间轮询
- 当所有 kiro 凭证冷却或不可用时，自动切换到 antigravity 凭证

## 最佳实践

1. **注意 YAML 顺序**：OAuth 模型别名使用 first-win 策略，确保优先级更高的别名定义在前面。由于冲突不会产生警告，建议定期审查配置避免意外覆盖

2. **跨 channel 别名隔离**：如果多个 channel 需要不同的别名映射，可以放心使用相同的别名名称，channel 之间完全隔离

3. **多 Provider 冗余部署**：为关键模型配置多个 Provider 的凭证，利用自动故障转移提高可用性。例如同时配置 kiro 和 antigravity 提供 `claude-opus-4-6`

4. **合理设置优先级**：通过 `priority` 属性区分主备 Provider。将低延迟/高配额的 Provider 设置更高优先级

5. **避免别名与上游模型名冲突**：不要将别名设置为另一个已存在的上游模型名，这可能导致路由混乱

6. **善用 fork 模式**：当希望同时保留原始模型名和别名时，使用 `fork: true`，避免原始模型名从列表中消失

## 关键源码位置

| 组件 | 文件 | 行号 | 说明 |
|------|------|------|------|
| OAuthModelAlias 配置结构 | `internal/config/config.go` | 114-120, 181-189 | 别名配置定义 |
| OAuth 别名编译 | `sdk/cliproxy/auth/oauth_model_alias.go` | 20-56 | first-win 去重逻辑 |
| OAuth 别名解析 | `sdk/cliproxy/auth/oauth_model_alias.go` | 141-198 | 运行时别名查找（含 thinking suffix） |
| OAuth channel 判定 | `sdk/cliproxy/auth/oauth_model_alias.go` | 225-253 | 判定凭证所属 channel |
| API Key 别名编译 | `sdk/cliproxy/auth/conductor.go` | 326-370 | API Key 凭证的别名去重 |
| 混合候选池构建 | `sdk/cliproxy/auth/conductor.go` | 1586-1662 | pickNextMixed() |
| 混合执行与故障转移 | `sdk/cliproxy/auth/conductor.go` | 565-616 | executeMixedOnce() |
| 优先级排序与选择 | `sdk/cliproxy/auth/selector.go` | 142-177 | getAvailableAuths() |
| 路由策略实现 | `sdk/cliproxy/auth/selector.go` | 19-27 | RoundRobinSelector / FillFirstSelector |

## 参考资料

- [路由策略文档](./routing-strategy_CN.md) - Round-Robin 与 Fill-First 策略详解
- [SDK 访问控制文档](./sdk-access_CN.md) - 访问控制详细说明
- [配置示例](../config.example.yaml) - 完整配置示例（含 oauth-model-alias 示例）
- [CLAUDE.md](../CLAUDE.md) - 项目架构文档

## 更新日志

- 2025-02: 初始版本，覆盖别名冲突与多 Provider 路由两个场景
