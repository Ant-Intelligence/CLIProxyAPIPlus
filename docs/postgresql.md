# PostgreSQL Token Store

## 概述

CLIProxyAPI Plus 支持使用 PostgreSQL 数据库作为令牌和配置的集中存储后端。这对于多实例部署、集群环境或容器化场景特别有用。

PostgreSQL Token Store 提供以下功能：
- 集中存储 OAuth 令牌和认证凭证
- 集中管理配置文件（config.yaml）
- 多实例间自动同步
- 本地缓存提升性能
- 数据持久化，防止容器重启导致数据丢失

## 使用场景

### ✅ 适合使用 PostgreSQL 的场景

1. **多实例部署**
   - 运行多个 CLIProxyAPI 实例实现负载均衡
   - 所有实例共享同一套认证凭证
   - 配置统一管理，一处修改全部生效

2. **容器化部署**
   - Docker/Kubernetes 等容器环境
   - 容器重启后数据不丢失
   - 无需配置持久化卷来存储认证文件

3. **云部署**
   - 云函数、Serverless 环境
   - 无状态实例需要共享状态
   - 多区域部署时的数据同步

4. **集群部署**
   - 多台服务器组成的集群
   - 自动故障转移和高可用
   - 统一的认证凭证管理

5. **团队协作**
   - 多人/多团队共享 API 凭证
   - 集中管理避免凭证分散
   - 权限控制和审计

### ❌ 不需要 PostgreSQL 的场景

1. **单机部署**
   - 只运行一个实例
   - 使用默认的文件存储即可（`~/.cli-proxy-api/`）
   - 配置和维护更简单

2. **开发测试环境**
   - 本地开发调试
   - 快速测试验证
   - 文件存储更方便

3. **个人使用**
   - 个人电脑运行
   - 不需要多实例共享
   - 简化部署流程

## 配置方法

### 环境变量配置

通过设置以下环境变量启用 PostgreSQL Token Store：

```bash
# 必需：PostgreSQL 数据库连接字符串（DSN）
PGSTORE_DSN="postgresql://username:password@localhost:5432/dbname"

# 可选：数据库 schema（默认为空，使用 public schema）
PGSTORE_SCHEMA="cliproxy"

# 可选：本地缓存目录路径（默认为工作目录下的 pgstore/）
PGSTORE_LOCAL_PATH="/var/lib/cliproxy/pgstore"
```

### DSN 连接字符串格式

```
postgresql://[user[:password]@][host][:port][/database][?param1=value1&...]
```

示例：
```bash
# 标准连接
PGSTORE_DSN="postgresql://cliproxy:mypassword@localhost:5432/cliproxy_db"

# 使用 SSL
PGSTORE_DSN="postgresql://user:pass@db.example.com:5432/dbname?sslmode=require"

# 连接池配置
PGSTORE_DSN="postgresql://user:pass@localhost:5432/db?pool_max_conns=10"
```

### Docker Compose 配置示例

```yaml
services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: cliproxy
      POSTGRES_USER: cliproxy
      POSTGRES_PASSWORD: your_secure_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  cli-proxy-api-1:
    image: eceasy/cli-proxy-api-plus:latest
    environment:
      PGSTORE_DSN: "postgresql://cliproxy:your_secure_password@postgres:5432/cliproxy"
      PGSTORE_SCHEMA: "cliproxy"
    ports:
      - "8317:8317"
    depends_on:
      - postgres
    restart: unless-stopped

  cli-proxy-api-2:
    image: eceasy/cli-proxy-api-plus:latest
    environment:
      PGSTORE_DSN: "postgresql://cliproxy:your_secure_password@postgres:5432/cliproxy"
      PGSTORE_SCHEMA: "cliproxy"
    ports:
      - "8318:8317"
    depends_on:
      - postgres
    restart: unless-stopped

volumes:
  postgres_data:
```

### .env 文件配置

在项目根目录创建 `.env` 文件：

```bash
PGSTORE_DSN=postgresql://cliproxy:password@localhost:5432/cliproxy
PGSTORE_SCHEMA=cliproxy
PGSTORE_LOCAL_PATH=/var/lib/cliproxy/pgstore
```

应用会自动加载 `.env` 文件中的配置。

## 工作原理

### 数据流程

1. **启动初始化**
   ```
   应用启动 → 连接 PostgreSQL → 创建表结构
   → 从数据库同步到本地 → 应用读取本地文件
   ```

2. **运行时同步**
   ```
   OAuth 登录 → 令牌保存到本地文件 → 自动同步到 PostgreSQL
   → 其他实例从 PostgreSQL 同步 → 所有实例获得新令牌
   ```

3. **配置更新**
   ```
   修改 config.yaml → 保存到本地 → 同步到 PostgreSQL
   → 其他实例热加载新配置 → 全部实例配置统一
   ```

### 本地缓存机制

PostgreSQL Token Store 采用**数据库 + 本地缓存**的混合模式：

- **写入流程**：本地文件 → 同时写入 PostgreSQL
- **读取流程**：直接读取本地文件（性能优化）
- **同步时机**：
  - 应用启动时从数据库拉取最新数据
  - 令牌/配置变更时自动推送到数据库
  - 文件监控检测到变化时自动同步

这种设计保证了：
- ✅ 性能：读取本地文件，无数据库查询延迟
- ✅ 可靠性：数据持久化到数据库，不丢失
- ✅ 一致性：多实例通过数据库共享状态

### 目录结构

启用 PostgreSQL 后，本地目录结构：

```
{PGSTORE_LOCAL_PATH}/
├── config/
│   └── config.yaml          # 从数据库同步的配置文件
└── auths/
    ├── claude_session_*.json     # Claude OAuth 令牌
    ├── codex_session_*.json      # Codex OAuth 令牌
    ├── gemini_*.json             # Gemini 凭证
    ├── kiro_*.json               # Kiro 令牌
    ├── copilot_session_*.json    # GitHub Copilot 令牌
    └── ...                       # 其他提供商的令牌文件
```

## 数据库表结构

PostgreSQL Token Store 自动创建以下表：

### 配置表（config_store）

```sql
CREATE TABLE config_store (
    id TEXT PRIMARY KEY,              -- 配置ID（默认为 "config"）
    content TEXT NOT NULL,            -- config.yaml 的文本内容
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 认证令牌表（auth_store）

```sql
CREATE TABLE auth_store (
    id TEXT PRIMARY KEY,              -- 令牌文件的唯一标识
    content JSONB NOT NULL,           -- 令牌的 JSON 内容
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 自定义表名

可以通过配置使用自定义的表名（默认为 `config_store` 和 `auth_store`）。表名会根据 `PGSTORE_SCHEMA` 设置自动加上 schema 前缀。

示例：
- 未设置 schema：表名为 `config_store`, `auth_store`
- 设置 `PGSTORE_SCHEMA=cliproxy`：表名为 `cliproxy.config_store`, `cliproxy.auth_store`

## 与其他存储方式对比

CLIProxyAPI Plus 支持多种令牌存储后端：

| 存储方式 | 适用场景 | 优点 | 缺点 |
|---------|---------|------|------|
| **文件存储**<br>(默认) | 单机部署 | • 简单易用<br>• 无需额外依赖<br>• 配置快速 | • 不支持多实例共享<br>• 容器重启数据丢失 |
| **PostgreSQL** | 多实例/集群 | • 多实例共享<br>• 数据持久化<br>• 支持高可用 | • 需要数据库<br>• 配置稍复杂 |
| **Git 存储** | 团队协作 | • 版本控制<br>• 审计历史<br>• 跨机器同步 | • 需要 Git 仓库<br>• 同步有延迟 |
| **对象存储** | 云原生/大规模 | • S3 兼容<br>• 无限扩展<br>• 云服务托管 | • 需要对象存储服务<br>• 可能有费用 |

### 存储方式优先级

当配置多种存储方式时，优先级为：
```
PostgreSQL > 对象存储 > Git 存储 > 文件存储（默认）
```

即：如果设置了 `PGSTORE_DSN`，将自动使用 PostgreSQL，忽略其他存储配置。

## 常见问题

### Q1: PostgreSQL 连接失败怎么办？

**检查项：**
1. 确认 PostgreSQL 服务正在运行
2. 验证连接字符串格式正确
3. 检查用户名/密码是否正确
4. 确认数据库已创建
5. 检查网络连接和防火墙规则
6. 查看日志：`docker logs <container_name>`

**调试命令：**
```bash
# 测试 PostgreSQL 连接
psql "postgresql://user:password@host:5432/dbname"

# 检查日志
journalctl -u cliproxyapi -f
```

### Q2: 如何迁移现有的文件令牌到 PostgreSQL？

**步骤：**
1. 备份现有的令牌文件（`~/.cli-proxy-api/` 或 `auths/` 目录）
2. 配置 PostgreSQL 环境变量
3. 将备份的文件复制到 `pgstore/auths/` 目录
4. 重启应用，会自动同步到数据库

**脚本示例：**
```bash
# 备份现有令牌
cp -r ~/.cli-proxy-api ~/cli-proxy-backup

# 配置 PostgreSQL
export PGSTORE_DSN="postgresql://user:pass@localhost:5432/db"

# 启动应用（会自动同步）
./CLIProxyAPI
```

### Q3: 多个实例如何同步？

PostgreSQL Token Store 会自动处理同步：
- 任一实例保存令牌 → 写入数据库
- 其他实例通过文件监控机制自动检测变化
- 无需手动操作

**建议：**
- 使用负载均衡器（如 Nginx）分发请求
- 所有实例指向同一数据库
- 配置健康检查确保实例正常运行

### Q4: 数据库性能会影响 API 响应速度吗？

**不会。** PostgreSQL Token Store 使用本地缓存：
- API 请求读取本地文件，无数据库查询
- 只有保存令牌时才写入数据库（异步操作）
- 数据库仅用于实例间同步，不在请求路径上

### Q5: 如何备份 PostgreSQL 中的令牌？

```bash
# 备份整个数据库
pg_dump -U cliproxy -d cliproxy > backup.sql

# 只备份令牌表
pg_dump -U cliproxy -d cliproxy -t auth_store -t config_store > tokens_backup.sql

# 导出为 JSON 格式
psql -U cliproxy -d cliproxy -c "SELECT * FROM auth_store" -A -F, -o tokens.csv
```

### Q6: 如何清理过期的令牌？

PostgreSQL Token Store 不会自动删除过期令牌。可以手动清理：

```sql
-- 查看所有令牌
SELECT id, created_at, updated_at FROM auth_store;

-- 删除指定令牌
DELETE FROM auth_store WHERE id = 'old_token_id';

-- 删除超过 90 天未更新的令牌
DELETE FROM auth_store WHERE updated_at < NOW() - INTERVAL '90 days';
```

### Q7: PostgreSQL 需要什么版本？

**推荐版本（2026）：**
- **PostgreSQL 17** - 最新稳定版（推荐用于生产环境）
- **PostgreSQL 18** - 最新特性版本（2025年11月发布）
- **PostgreSQL 16** - 长期支持版（支持至2028年）

**最低要求**：PostgreSQL 10+ （需要 JSONB 数据类型支持）

**版本选择建议**：
- 生产环境：PostgreSQL 17（成熟稳定，活跃维护）
- 追求新特性：PostgreSQL 18（最新版本）
- 保守选择：PostgreSQL 16（LTS，支持时间更长）

当前 docker-compose.postgres.yml 默认使用 PostgreSQL 17，可通过 `POSTGRES_VERSION` 环境变量更改。

## 安全建议

1. **使用强密码**
   - PostgreSQL 数据库密码应足够复杂
   - 避免在 docker-compose.yml 中硬编码密码

2. **网络隔离**
   - PostgreSQL 不应暴露到公网
   - 使用内部网络或 VPN 连接

3. **SSL/TLS 加密**
   ```bash
   PGSTORE_DSN="postgresql://user:pass@host:5432/db?sslmode=require"
   ```

4. **最小权限原则**
   - 为 CLIProxyAPI 创建专用数据库用户
   - 只授予必要的表权限（SELECT, INSERT, UPDATE）

   ```sql
   CREATE USER cliproxy WITH PASSWORD 'secure_password';
   GRANT CONNECT ON DATABASE cliproxy TO cliproxy;
   GRANT SELECT, INSERT, UPDATE ON config_store, auth_store TO cliproxy;
   ```

5. **定期备份**
   - 设置自动备份计划
   - 测试备份恢复流程

6. **审计日志**
   - 启用 PostgreSQL 审计日志
   - 监控异常访问模式

## 监控和维护

### 健康检查

```sql
-- 检查连接数
SELECT count(*) FROM pg_stat_activity WHERE datname = 'cliproxy';

-- 检查表大小
SELECT
    pg_size_pretty(pg_total_relation_size('auth_store')) as auth_size,
    pg_size_pretty(pg_total_relation_size('config_store')) as config_size;

-- 检查最近更新
SELECT id, updated_at FROM auth_store ORDER BY updated_at DESC LIMIT 10;
```

### 性能优化

对于大量令牌的场景，可以添加索引：

```sql
-- 为更新时间添加索引
CREATE INDEX idx_auth_store_updated_at ON auth_store(updated_at);

-- 为 JSONB 内容添加 GIN 索引（支持 JSON 查询）
CREATE INDEX idx_auth_store_content ON auth_store USING GIN (content);
```

### 故障排查

查看详细日志：
```bash
# 设置 debug 模式
debug: true

# 查看 PostgreSQL 日志
tail -f /var/log/postgresql/postgresql-16-main.log
```

## 参考资料

- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [pgx - PostgreSQL Driver for Go](https://github.com/jackc/pgx)
- [Docker Compose 文档](https://docs.docker.com/compose/)
- [CLIProxyAPI 主项目](https://github.com/router-for-me/CLIProxyAPI)
