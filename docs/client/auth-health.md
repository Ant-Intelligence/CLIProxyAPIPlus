# auth-health

检查服务器上 OAuth 帐号的健康状态，查看帐号是否正常、已禁用或出现错误。

## 用法

```bash
cpa-client auth-health [flags]
```

## 参数

| 参数 | 短写 | 说明 |
|------|------|------|
| `--provider` | `-p` | 按提供商过滤（默认 `antigravity`） |
| `--json` | | 以 JSON 格式输出（默认为表格） |

## 提供商过滤

`--provider` 支持前缀匹配。例如 `--provider gemini` 会匹配 `gemini`、`gemini-cli`、`gemini-api-key` 等所有以 `gemini` 开头的提供商。

常用提供商值：

| 值 | 匹配 |
|----|------|
| `antigravity` | Antigravity 帐号（默认） |
| `gemini` | 所有 Gemini 类型帐号 |
| `kiro` | Kiro (AWS CodeWhisperer) 帐号 |
| `claude` | Claude 帐号 |
| `codex` | Codex (OpenAI) 帐号 |
| `copilot` | GitHub Copilot 帐号 |
| `""` | 所有帐号（传空字符串） |

## 示例

**默认查看 Antigravity 帐号：**

```bash
cpa-client auth-health
```

**查看所有 Gemini 类型帐号：**

```bash
cpa-client auth-health --provider gemini
```

**查看 Kiro 帐号：**

```bash
cpa-client auth-health --provider kiro
```

**查看所有帐号：**

```bash
cpa-client auth-health --provider ""
```

**JSON 输出：**

```bash
cpa-client auth-health --provider kiro --json
```

**输出示例（表格）：**

```
Auth Account Health (gemini)
============================

Account                        Provider      Status      Message                   Last Refresh
──────────────────────────     ──────────    ────────    ────────────────────────   ──────────────
alice@gmail.com                gemini-cli    ACTIVE                                 5m ago
bob@gmail.com                  gemini-cli    ACTIVE                                 12m ago
service-account                gemini-api    DISABLED    manually disabled          -
carol@gmail.com                gemini-cli    ERROR       token refresh failed       2h ago

Total: 4 accounts | 2 healthy, 1 error, 1 disabled
```

## 状态说明

| 状态 | 颜色 | 说明 |
|------|------|------|
| ACTIVE | 绿色 | 帐号正常运行 |
| DISABLED | 黄色 | 帐号已被手动禁用 |
| ERROR | 红色 | 帐号出现错误（如 token 刷新失败） |
| UNAVAIL | 红色 | 帐号不可用 |

"Last Refresh" 列显示最近一次 token 刷新的时间，格式为相对时间（如 `5m ago`、`2h ago`、`3d ago`）。
