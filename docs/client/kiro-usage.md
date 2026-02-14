# kiro-usage

查看服务器上所有 Kiro 帐号的额度使用情况，包括已用额度、剩余额度、使用百分比和重置时间。

## 用法

```bash
cpa-client kiro-usage [flags]
```

## 参数

| 参数 | 说明 |
|------|------|
| `--json` | 以 JSON 格式输出（默认为表格） |

## 示例

**表格输出（默认）：**

```bash
cpa-client kiro-usage
```

输出示例：

```
Kiro Account Credit Usage
=========================

Account           Subscription  Used / Total     Remaining   Usage%   Reset In   Status
──────────────    ──────────    ──────────────   ─────────   ──────   ────────   ──────
alice@example.com Pro           750 / 5,000      4,250       15.00%   22 days    OK
bob@example.com   Free          980 / 1,000      20          98.00%   5 days     LOW
carol@example.com Pro           5,000 / 5,000    0           100.00%  1 day      EXHAUSTED

Total: 3 accounts | 1 healthy, 1 exhausted, 1 low
```

**JSON 输出：**

```bash
cpa-client kiro-usage --json
```

```json
{
  "accounts": [
    {
      "name": "kiro-enterprise-alice.json",
      "email": "alice@example.com",
      "total_limit": 5000,
      "current_usage": 750,
      "remaining_quota": 4250,
      "usage_percent": 15.0,
      "is_exhausted": false,
      "subscription_title": "Pro",
      "days_until_reset": 22,
      "next_reset": "2026-03-01T00:00:00Z"
    }
  ]
}
```

## 状态说明

| 状态 | 颜色 | 条件 |
|------|------|------|
| OK | 绿色 | 使用率 < 80% |
| LOW | 黄色 | 使用率 >= 80% |
| EXHAUSTED | 红色 | 额度已耗尽 |
| ERR | 红色 | 查询帐号出错 |
