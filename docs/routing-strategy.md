# Routing Strategy Configuration

## Overview

CLIProxyAPI Plus supports flexible credential routing strategies for selecting among multiple available credentials (accounts) to execute requests. The routing strategy determines how the system chooses which credential to use when multiple valid options are available for handling an API request.

## Configuration Location

Configure the routing strategy in `config.yaml`:

```yaml
routing:
  strategy: "round-robin"  # round-robin (default), fill-first
```

You can also modify the strategy dynamically via the Management API:

```bash
# Query current strategy
GET /v0/management/routing/strategy

# Update strategy
PUT /v0/management/routing/strategy
Content-Type: application/json

{
  "value": "fill-first"
}
```

## Supported Routing Strategies

### 1. Round-Robin (Default)

**Aliases**: `round-robin`, `roundrobin`, `rr`

#### How It Works

The round-robin strategy cycles through all available credentials. The system maintains a cursor for each `provider:model` combination, selecting the next available credential on each request and incrementing the cursor.

**Execution Flow**:
1. Collect all available credentials (excluding disabled and cooling credentials)
2. Group by priority, select the highest priority group
3. Sort by ID within the group for deterministic ordering
4. Select credential at cursor position: `credentials[cursor % len(credentials)]`
5. Increment cursor; next request uses the next credential

**Example**:

With 3 available credentials `[A, B, C]`:

```
Request 1 -> Credential A (cursor=0, 0%3=0)
Request 2 -> Credential B (cursor=1, 1%3=1)
Request 3 -> Credential C (cursor=2, 2%3=2)
Request 4 -> Credential A (cursor=3, 3%3=0)
Request 5 -> Credential B (cursor=4, 4%3=1)
...
```

#### Advantages

- **Load Balancing**: Even usage across all credentials, avoiding single account overload
- **Extended Availability**: Distributing requests delays quota exhaustion on any single account
- **Reduced Ban Risk**: Avoids triggering rate limits from frequent use of a single account

#### Use Cases

- Multiple credentials with similar quotas, aiming for even usage
- Maximizing total throughput
- Suitable for sustained high-load production environments

### 2. Fill-First

**Aliases**: `fill-first`, `fillfirst`, `ff`

#### How It Works

The fill-first strategy always selects the first available credential (sorted by ID). This "burns through" one account's quota before moving to the next.

**Execution Flow**:
1. Collect all available credentials (excluding disabled and cooling credentials)
2. Group by priority, select the highest priority group
3. Sort by ID within the group
4. Always return the first credential in the list: `credentials[0]`

**Example**:

With 3 available credentials `[A, B, C]`:

```
Request 1 -> Credential A
Request 2 -> Credential A
Request 3 -> Credential A
...
Request N -> Credential A (until A hits quota limit and enters cooldown)
Request N+1 -> Credential B (A is cooling)
Request N+2 -> Credential B
...
```

#### Advantages

- **Staggered Quota Windows**: Helps stagger rolling-window subscription caps (e.g., hourly/daily message limits)
- **Predictable**: Deterministic behavior, easy to understand and debug
- **Reserve Protection**: Backup credentials stay "fresh" for emergency use

#### Use Cases

- API providers use rolling time window limits (e.g., "X requests per hour")
- Maximizing single credential quota utilization
- Clear primary/backup credential concept
- Uneven traffic with distinct peaks and valleys

## Advanced Features

### Priority

Both strategies support credential priority. Higher priority credentials are selected first; lower priority credentials are only used when higher priority ones are unavailable.

**Configuration**:

Credential priority is set via the `priority` attribute (in the credential file's `attributes` field):

```json
{
  "id": "credential-1",
  "attributes": {
    "priority": "10"
  }
}
```

- Higher priority value = higher precedence (default is 0)
- Credentials with the same priority are selected according to the configured strategy (round-robin or fill-first)
- Priority can differentiate production/test credentials or high-speed/economy credentials

**Example**:

```
Credential A (priority=10)
Credential B (priority=10)
Credential C (priority=0)

Round-Robin: A -> B -> A -> B -> ... (only use C when A and B unavailable)
Fill-First:  A -> A -> A -> ... (only use B when A unavailable)
```

### Automatic Cooldown Management

Both strategies automatically skip credentials in cooldown:

- **Quota Exceeded**: Credential automatically enters cooldown when API quota limits are hit
- **Error Recovery**: Temporarily disabled after specific errors using backoff strategy
- **Model-Level Cooldown**: Can set cooldown for specific models without affecting others

When all credentials are in cooldown, the system returns a `429 Too Many Requests` error with a `Retry-After` header indicating when the earliest credential becomes available.

### Disabled State

You can manually disable specific credentials or specific models on a credential. Disabled credentials are never selected by any strategy.

## Strategy Comparison

| Feature | Round-Robin | Fill-First |
|---------|-------------|------------|
| **Load Distribution** | Even across all credentials | Concentrated on first available |
| **Quota Utilization** | Slowly consume each credential | Quickly consume current credential |
| **Switch Frequency** | May switch every request | Only switches during cooldown |
| **Predictability** | Medium | High |
| **Backup Protection** | No special protection | Automatically reserves backup credentials |
| **Best For** | Sustained high load | Variable load with rolling window limits |
| **Complexity** | Maintains cursor state | Stateless, simple |

## Configuration Examples

### Example 1: Production Environment, High Throughput

```yaml
# Even load distribution across all accounts
routing:
  strategy: "round-robin"

# Multiple equivalent credentials
# Each account quota: 1000 requests/hour
# Expected total load: 2500 requests/hour
# Configure 3 accounts with round-robin distribution
```

### Example 2: Primary/Backup Credentials, Rolling Window Limits

```yaml
# Prioritize primary credential, backup on standby
routing:
  strategy: "fill-first"

# Primary credential: High-speed account (preferred)
# Backup credential: Economy account (used when primary is cooling)
# API limit: 100 requests/hour (rolling window)
```

### Example 3: Mixed Priority, Round-Robin

```yaml
routing:
  strategy: "round-robin"

# Credential configuration (in auth files):
# - premium-1 (priority: 10)
# - premium-2 (priority: 10)
# - standard-1 (priority: 0)
# - standard-2 (priority: 0)

# Behavior: Round-robin between premium-1 and premium-2
# Only use standard credentials when both premium credentials are unavailable
```

## Dynamic Strategy Changes

You can modify the routing strategy at runtime via the Management API without restarting the service:

```bash
# Query current strategy
curl http://localhost:8317/v0/management/routing/strategy \
  -H "X-Management-Key: your-secret-key"

# Response
{
  "strategy": "round-robin"
}

# Change to fill-first
curl -X PUT http://localhost:8317/v0/management/routing/strategy \
  -H "X-Management-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"value": "fill-first"}'
```

Strategy changes take effect immediately for all subsequent requests.

## Implementation Details

### Source Code Locations

- **Configuration Definition**: `internal/config/config.go` - `RoutingConfig` struct
- **Strategy Implementation**: `sdk/cliproxy/auth/selector.go`
  - `RoundRobinSelector` - Round-robin selector
  - `FillFirstSelector` - Fill-first selector
- **Strategy Creation**: `sdk/cliproxy/builder.go` - Creates selector based on config
- **Management API**: `internal/api/handlers/management/config_basic.go` - Strategy query and modification endpoints

### Strategy Interface

All routing strategies implement the `Selector` interface:

```go
type Selector interface {
    Pick(ctx context.Context, provider, model string, opts Options, auths []*Auth) (*Auth, error)
}
```

### Thread Safety

- `RoundRobinSelector` uses mutex to protect cursor state, supports concurrent access
- `FillFirstSelector` is stateless and inherently thread-safe

## Troubleshooting

### Issue: All Credentials in Cooldown

**Symptoms**: Receiving `429 Too Many Requests` error, message indicates all credentials are cooling

**Cause**: Request rate exceeds total quota across all credentials

**Solutions**:
1. Add more credentials
2. Reduce request rate
3. Wait for the time indicated in `Retry-After` header before retrying
4. Check if credentials are incorrectly disabled

### Issue: Round-Robin Not Evenly Distributing

**Symptoms**: Some credentials used significantly more than others

**Possible Causes**:
1. Different credential priorities (expected behavior)
2. Some credentials frequently entering cooldown
3. Multiple provider:model combinations have independent cursors

**Troubleshooting**:
- Check `priority` attribute in credential configuration
- View credential status via Management API to confirm cooldown situations
- Note that different models have independent cursors

### Issue: Fill-First Not Switching to Next Credential

**Symptoms**: Still using the same credential even after reaching quota

**Possible Causes**:
1. Cooldown mechanism not triggering correctly
2. Quota detection delay
3. Credential state not updating properly

**Troubleshooting**:
- Check if `disable-cooling` config is inadvertently enabled
- Review error logs to confirm quota exceeded errors are properly identified
- Check credential status via Management API, confirm `NextRetryAfter` time

## Best Practices

1. **Production: Use Round-Robin**: Unless you have specific requirements, round-robin is typically the more robust choice

2. **Set Priorities Wisely**: Use priority to differentiate credentials by purpose, for example:
   - priority=10: Production credentials
   - priority=5: Test credentials
   - priority=0: Emergency backup credentials

3. **Monitor Credential Status**: Regularly check credential status via Management API to identify quota issues early

4. **Quota Planning**: Plan credential count based on actual load and API limits

5. **Test Failover**: Test automatic switching behavior during credential failures before going live

## References

- [CLAUDE.md](../CLAUDE.md) - Project architecture documentation
- [Configuration Example](../config.example.yaml) - Complete configuration example
- [SDK Access Control Documentation](./sdk-access.md) - Detailed access control documentation

## Changelog

- 2025-02: Initial version, supporting round-robin and fill-first strategies
