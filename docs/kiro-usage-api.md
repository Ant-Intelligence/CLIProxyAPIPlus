# Kiro Credit Usage API

Query Kiro account credit consumption (total limit, current usage, remaining quota) via the management API.

## Endpoint

```
GET /v0/management/kiro-usage
```

## Authentication

Requires a management key, provided via one of:

- `Authorization: Bearer <management-key>` header
- `X-Management-Key: <management-key>` header

The management key is configured through either:
- `MANAGEMENT_PASSWORD` environment variable
- `remote-management.secret-key` in `config.yaml`

## Usage

### Basic request

```bash
curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY"
```

### With `jq` formatting

```bash
curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY" | jq .
```

### Using `X-Management-Key` header

```bash
curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "X-Management-Key: YOUR_MANAGEMENT_KEY"
```

### Remote server with HTTPS

```bash
curl -s https://your-server.example.com/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY" | jq .
```

## Response

### Success

```json
{
  "accounts": [
    {
      "name": "kiro-google-user@example.com.json",
      "email": "user@example.com",
      "total_limit": 200,
      "current_usage": 45.5,
      "remaining_quota": 154.5,
      "usage_percent": 22.75,
      "is_exhausted": false,
      "subscription_title": "KIRO FREE",
      "next_reset": "2026-03-01T00:00:00Z",
      "resource_type": "AGENTIC_REQUEST"
    },
    {
      "name": "kiro-aws-another@example.com.json",
      "email": "another@example.com",
      "total_limit": 200,
      "current_usage": 200,
      "remaining_quota": 0,
      "usage_percent": 100,
      "is_exhausted": true,
      "subscription_title": "KIRO FREE",
      "next_reset": "2026-03-01T00:00:00Z",
      "resource_type": "AGENTIC_REQUEST"
    }
  ]
}
```

### Response fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Auth file name or ID |
| `email` | string | Account email (if available) |
| `total_limit` | float | Total credit quota (including free trial) |
| `current_usage` | float | Credits already consumed |
| `remaining_quota` | float | Credits remaining (`total_limit - current_usage`) |
| `usage_percent` | float | Usage percentage (0-100) |
| `is_exhausted` | bool | Whether quota is fully consumed |
| `subscription_title` | string | Subscription plan (e.g. "KIRO FREE") |
| `next_reset` | string | Next quota reset time (RFC3339) |
| `resource_type` | string | Resource type (e.g. "AGENTIC_REQUEST") |
| `error` | string | Error message if query failed for this account |

### Error responses

Missing management key:
```json
{"error": "missing management key"}
```

Invalid management key:
```json
{"error": "invalid management key"}
```

Auth manager unavailable:
```json
{"error": "auth manager unavailable"}
```

### Per-account errors

If a specific account fails to query (e.g. expired token), the account entry will include an `error` field while other accounts return normally:

```json
{
  "accounts": [
    {
      "name": "kiro-expired@example.com.json",
      "email": "expired@example.com",
      "error": "API error (status 401): Unauthorized"
    },
    {
      "name": "kiro-google-active@example.com.json",
      "email": "active@example.com",
      "total_limit": 200,
      "current_usage": 10,
      "remaining_quota": 190,
      "usage_percent": 5,
      "is_exhausted": false,
      "subscription_title": "KIRO FREE",
      "next_reset": "2026-03-01T00:00:00Z",
      "resource_type": "AGENTIC_REQUEST"
    }
  ]
}
```

## Examples

### Check if any account is exhausted

```bash
curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY" | \
  jq '.accounts[] | select(.is_exhausted == true) | {name, email, next_reset}'
```

### Show summary for all accounts

```bash
curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY" | \
  jq '.accounts[] | "\(.email // .name): \(.current_usage)/\(.total_limit) (\(.usage_percent | round)%)"'
```

### Monitor remaining quota (watch every 60s)

```bash
watch -n 60 'curl -s http://127.0.0.1:8080/v0/management/kiro-usage \
  -H "Authorization: Bearer YOUR_MANAGEMENT_KEY" | \
  jq -r ".accounts[] | \"\(.email): \(.remaining_quota) remaining\""'
```
