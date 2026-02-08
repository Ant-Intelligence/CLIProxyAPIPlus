# PostgreSQL 数据库运维指南

本文档详细介绍 CLIProxyAPI Plus 的 PostgreSQL 数据库日常运维操作、故障排查和最佳实践。

## 目录

- [数据库访问](#数据库访问)
- [数据库结构](#数据库结构)
- [日常运维操作](#日常运维操作)
- [数据备份与恢复](#数据备份与恢复)
- [数据查询与管理](#数据查询与管理)
- [性能监控与优化](#性能监控与优化)
- [故障排查](#故障排查)
- [安全管理](#安全管理)
- [数据迁移](#数据迁移)
- [维护计划](#维护计划)

---

## 数据库访问

### 连接方式

#### 1. 通过 Docker 容器连接

```bash
# 基本连接（使用配置的用户名）
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy

# 指定其他参数
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy -h localhost -p 5432
```

**常见错误排查**：

```bash
# 错误: role "postgres" does not exist
# 原因: 默认用户不是 postgres，而是配置的用户名（cliproxy）
# 解决: 使用正确的用户名 -U cliproxy

# 错误: password authentication failed
# 原因: 密码错误
# 解决: 检查 .env 文件或 compose.yml 中的 POSTGRES_PASSWORD
```

#### 2. 从宿主机连接（如果暴露了端口）

```bash
# 使用 psql 客户端
psql -h localhost -p 5432 -U cliproxy -d cliproxy

# 使用连接字符串
psql "postgresql://cliproxy:password@localhost:5432/cliproxy"

# 非交互式执行查询
psql -h localhost -U cliproxy -d cliproxy -c "SELECT version();"
```

#### 3. 使用图形化工具

**DBeaver 配置：**
- 连接类型: PostgreSQL
- 主机: localhost
- 端口: 5432
- 数据库: cliproxy
- 用户名: cliproxy
- 密码: （见 .env 文件）

**pgAdmin 配置：**
- 右键 Servers → Create → Server
- Name: CLIProxyAPI
- Connection → Host: localhost
- Port: 5432
- Maintenance database: cliproxy
- Username: cliproxy
- Password: （见 .env 文件）

### 环境变量说明

```bash
# 从 compose.yml 或 .env 文件中获取配置
POSTGRES_DB=cliproxy               # 数据库名称
POSTGRES_USER=cliproxy             # 数据库用户名
POSTGRES_PASSWORD=changeme         # 数据库密码（务必修改！）
PGSTORE_SCHEMA=cliproxy            # Schema 名称

# 查看当前配置
cat .env | grep POSTGRES
# 或
docker exec cli-proxy-api env | grep PGSTORE
```

### 常用 psql 命令速查

```sql
-- 数据库操作
\l                              -- 列出所有数据库
\c cliproxy                     -- 切换到 cliproxy 数据库
\dn                             -- 列出所有 schema

-- 表操作
\dt                             -- 列出当前 schema 的所有表
\dt cliproxy.*                  -- 列出 cliproxy schema 的表
\d cliproxy.config_store        -- 查看表结构
\dt+                            -- 查看表大小
\di                             -- 列出索引

-- 查询相关
\timing on                      -- 显示查询执行时间
\x auto                         -- 自动扩展显示（适合宽表）
\pset pager off                 -- 关闭分页器

-- 系统信息
\conninfo                       -- 查看当前连接信息
\du                             -- 列出所有用户
\password                       -- 修改当前用户密码

-- 帮助
\?                              -- psql 命令帮助
\h SELECT                       -- SQL 命令帮助

-- 退出
\q                              -- 退出 psql
```

---

## 数据库结构

### Schema 结构概览

```sql
-- 查看所有 schema
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT LIKE 'pg_%'
  AND schema_name != 'information_schema'
ORDER BY schema_name;

-- 查看 cliproxy schema 的对象统计
SELECT
    table_schema,
    table_type,
    COUNT(*) as count
FROM information_schema.tables
WHERE table_schema = 'cliproxy'
GROUP BY table_schema, table_type;
```

### 配置表 (config_store)

**用途**: 存储系统配置文件（config.yaml 的 YAML 内容）

```sql
-- 表结构
CREATE TABLE IF NOT EXISTS cliproxy.config_store (
    id TEXT PRIMARY KEY,                        -- 配置标识（默认为 "config"）
    content TEXT NOT NULL,                      -- YAML 配置内容（纯文本）
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- 创建时间
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()   -- 最后更新时间
);

-- 查看表详细信息
\d+ cliproxy.config_store

-- 基本查询
SELECT
    id,
    LENGTH(content) as content_size,
    pg_size_pretty(LENGTH(content)) as size_pretty,
    created_at,
    updated_at
FROM cliproxy.config_store;

-- 查看完整配置内容
SELECT content FROM cliproxy.config_store WHERE id = 'config';

-- 检查配置是否包含特定关键字
SELECT
    id,
    CASE
        WHEN content LIKE '%claude-api-key%' THEN '✓ 已配置 Claude'
        ELSE '✗ 未配置 Claude'
    END as claude_status,
    CASE
        WHEN content LIKE '%gemini-api-key%' THEN '✓ 已配置 Gemini'
        ELSE '✗ 未配置 Gemini'
    END as gemini_status
FROM cliproxy.config_store
WHERE id = 'config';
```

### 认证令牌表 (auth_store)

**用途**: 存储各提供商的 OAuth 令牌和认证凭证

```sql
-- 表结构
CREATE TABLE IF NOT EXISTS cliproxy.auth_store (
    id TEXT PRIMARY KEY,                        -- 令牌标识（文件路径）
    content JSONB NOT NULL,                     -- 令牌 JSON 内容
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- 创建时间
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()   -- 最后更新时间
);

-- 查看表详细信息
\d+ cliproxy.auth_store

-- 统计令牌总数
SELECT COUNT(*) as total_tokens FROM cliproxy.auth_store;

-- 按提供商分组统计
SELECT
    content->>'type' as provider,
    COUNT(*) as token_count,
    MIN(created_at) as earliest_token,
    MAX(updated_at) as latest_update
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY token_count DESC;

-- 查看令牌详细列表
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as account,
    created_at as created,
    updated_at as last_updated,
    NOW() - updated_at as age
FROM cliproxy.auth_store
ORDER BY updated_at DESC;
```

### 表空间和大小统计

```sql
-- 查看数据库总大小
SELECT pg_size_pretty(pg_database_size('cliproxy')) as database_size;

-- 查看各表占用空间
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size('cliproxy.' || tablename)) as total_size,
    pg_size_pretty(pg_relation_size('cliproxy.' || tablename)) as table_size,
    pg_size_pretty(pg_indexes_size('cliproxy.' || tablename)) as index_size
FROM pg_tables
WHERE schemaname = 'cliproxy'
ORDER BY pg_total_relation_size('cliproxy.' || tablename) DESC;

-- 查看表的行数估计
SELECT
    schemaname,
    tablename,
    n_live_tup as estimated_rows
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';
```

---

## 日常运维操作

### 健康检查

```bash
# 快速检查数据库状态
docker exec cliproxy-postgres pg_isready -U cliproxy
# 输出: /var/run/postgresql:5432 - accepting connections

# 查看数据库版本
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SELECT version();"

# 检查数据库大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy'));"

# 检查表数量
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cliproxy';"
```

### 连接管理

```sql
-- 查看所有活动连接
SELECT
    pid,
    usename as username,
    application_name,
    client_addr as client_ip,
    backend_start as connected_at,
    state,
    LEFT(query, 50) as query_preview
FROM pg_stat_activity
WHERE datname = 'cliproxy'
ORDER BY backend_start DESC;

-- 统计连接状态
SELECT
    state,
    COUNT(*) as connections
FROM pg_stat_activity
WHERE datname = 'cliproxy'
GROUP BY state
ORDER BY connections DESC;

-- 查看连接数占用率
SELECT
    COUNT(*) as current_connections,
    current_setting('max_connections')::int as max_connections,
    ROUND(100.0 * COUNT(*) / current_setting('max_connections')::int, 2) || '%' as usage_rate
FROM pg_stat_activity
WHERE datname = 'cliproxy';

-- 查看空闲连接
SELECT
    pid,
    usename,
    application_name,
    NOW() - state_change as idle_duration
FROM pg_stat_activity
WHERE datname = 'cliproxy'
  AND state = 'idle'
ORDER BY idle_duration DESC;

-- 终止指定连接（慎用）
SELECT pg_terminate_backend(12345);  -- 替换为实际的 pid

-- 终止所有空闲超过 1 小时的连接
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = 'cliproxy'
  AND state = 'idle'
  AND NOW() - state_change > INTERVAL '1 hour';
```

### 配置管理操作

```sql
-- 1. 查看当前配置
SELECT
    id,
    pg_size_pretty(LENGTH(content)) as size,
    created_at,
    updated_at,
    AGE(NOW(), updated_at) as last_modified_ago
FROM cliproxy.config_store;

-- 2. 查看配置内容（带行号）
WITH lines AS (
    SELECT
        ROW_NUMBER() OVER() as line_num,
        unnest(string_to_array(content, E'\n')) as line
    FROM cliproxy.config_store
    WHERE id = 'config'
)
SELECT
    LPAD(line_num::text, 4, '0') || ' | ' || line as content
FROM lines
LIMIT 50;  -- 只显示前 50 行

-- 3. 导出配置到文件（从容器内执行）
\o /tmp/config_export.yaml
SELECT content FROM cliproxy.config_store WHERE id = 'config';
\o

-- 4. 配置变更历史（如果有审计表）
SELECT
    action,
    timestamp,
    user_name,
    jsonb_pretty(old_data) as before,
    jsonb_pretty(new_data) as after
FROM cliproxy.audit_log
WHERE table_name = 'config_store'
ORDER BY timestamp DESC
LIMIT 10;
```

**⚠️ 注意事项**：
- 直接修改数据库中的配置后，需要重启应用服务才能生效
- 建议通过管理 API 修改配置，而不是直接操作数据库
- 修改前务必备份

### 令牌管理操作

```sql
-- 1. 查看所有令牌概览
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as account,
    content->>'label' as label,
    CASE
        WHEN content ? 'access_token' THEN '✓'
        ELSE '✗'
    END as has_access_token,
    CASE
        WHEN content ? 'refresh_token' THEN '✓'
        ELSE '✗'
    END as has_refresh_token,
    updated_at
FROM cliproxy.auth_store
ORDER BY updated_at DESC;

-- 2. 查看特定提供商的令牌
-- Claude 令牌
SELECT
    id,
    content->>'email' as email,
    content->>'sessionKey' as session_key_preview,
    updated_at
FROM cliproxy.auth_store
WHERE content->>'type' = 'claude'
ORDER BY updated_at DESC;

-- Kiro 令牌
SELECT
    id,
    content->>'email' as email,
    content->>'provider' as auth_provider,
    updated_at
FROM cliproxy.auth_store
WHERE content->>'type' = 'kiro'
ORDER BY updated_at DESC;

-- 3. 查看令牌详细信息（美化显示）
SELECT
    id,
    jsonb_pretty(content) as token_details
FROM cliproxy.auth_store
WHERE id = 'your_token_id_here';

-- 4. 查找包含特定邮箱的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email
FROM cliproxy.auth_store
WHERE content->>'email' ILIKE '%@gmail.com%';

-- 5. 检查令牌健康状态
SELECT
    content->>'type' as provider,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE content ? 'access_token') as with_access_token,
    COUNT(*) FILTER (WHERE content ? 'refresh_token') as with_refresh_token,
    AVG(EXTRACT(epoch FROM (NOW() - updated_at))/86400)::int as avg_age_days
FROM cliproxy.auth_store
GROUP BY content->>'type';

-- 6. 查找长时间未更新的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    updated_at,
    AGE(NOW(), updated_at) as age
FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '90 days'
ORDER BY updated_at;
```

---

## 数据备份与恢复

### 备份策略

#### 方案 1: 完整数据库备份（推荐）

```bash
# 创建备份目录
mkdir -p ~/cliproxy-backups

# 基本备份
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy \
    > ~/cliproxy-backups/backup_$(date +%Y%m%d_%H%M%S).sql

# 压缩备份（节省 80% 空间）
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy \
    | gzip > ~/cliproxy-backups/backup_$(date +%Y%m%d_%H%M%S).sql.gz

# 自定义格式备份（支持并行恢复，推荐用于大数据库）
docker exec cliproxy-postgres pg_dump -U cliproxy -Fc cliproxy \
    > ~/cliproxy-backups/backup_$(date +%Y%m%d_%H%M%S).dump

# 仅备份 schema（不含数据）
docker exec cliproxy-postgres pg_dump -U cliproxy --schema-only cliproxy \
    > ~/cliproxy-backups/schema_only.sql

# 仅备份数据（不含 schema）
docker exec cliproxy-postgres pg_dump -U cliproxy --data-only cliproxy \
    > ~/cliproxy-backups/data_only.sql
```

#### 方案 2: 分表备份

```bash
# 备份配置表
docker exec cliproxy-postgres pg_dump -U cliproxy \
    -t cliproxy.config_store cliproxy \
    > ~/cliproxy-backups/config_$(date +%Y%m%d).sql

# 备份令牌表
docker exec cliproxy-postgres pg_dump -U cliproxy \
    -t cliproxy.auth_store cliproxy \
    > ~/cliproxy-backups/auth_$(date +%Y%m%d).sql

# 同时备份多个表
docker exec cliproxy-postgres pg_dump -U cliproxy \
    -t cliproxy.config_store \
    -t cliproxy.auth_store \
    cliproxy > ~/cliproxy-backups/tables_$(date +%Y%m%d).sql
```

#### 方案 3: 导出为 CSV

```bash
# 导出配置（CSV 格式）
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "COPY cliproxy.config_store TO STDOUT CSV HEADER" \
    > ~/cliproxy-backups/config.csv

# 导出令牌列表（带字段解析）
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "COPY (
        SELECT
            id,
            content->>'type' as provider,
            content->>'email' as email,
            created_at,
            updated_at
        FROM cliproxy.auth_store
    ) TO STDOUT CSV HEADER" \
    > ~/cliproxy-backups/auth_list.csv

# 导出完整令牌数据（JSON 格式）
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -A -c \
    "SELECT row_to_json(t) FROM (SELECT * FROM cliproxy.auth_store) t" \
    > ~/cliproxy-backups/auth_full.json
```

#### 方案 4: Docker 卷物理备份

```bash
# 停止应用（确保数据一致性）
docker compose stop cli-proxy-api

# 备份 PostgreSQL 数据卷
docker run --rm \
    -v cliproxy_postgres_data:/data:ro \
    -v ~/cliproxy-backups:/backup \
    alpine tar czf /backup/postgres_volume_$(date +%Y%m%d).tar.gz -C /data .

# 重新启动应用
docker compose start cli-proxy-api

# 查看备份文件
ls -lh ~/cliproxy-backups/
```

### 自动备份脚本

创建 `/usr/local/bin/backup-cliproxy.sh`：

```bash
#!/bin/bash
# CLIProxyAPI 自动备份脚本

set -e

# 配置
BACKUP_DIR="/var/backups/cliproxy"
CONTAINER_NAME="cliproxy-postgres"
DB_USER="cliproxy"
DB_NAME="cliproxy"
RETENTION_DAYS=30
LOG_FILE="/var/log/cliproxy-backup.log"

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "开始备份..."

# 备份文件名
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/cliproxy_$TIMESTAMP.sql.gz"

# 执行备份
if docker exec "$CONTAINER_NAME" pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE"; then
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    log "✅ 备份成功: $BACKUP_FILE (大小: $SIZE)"

    # 删除旧备份
    DELETED=$(find "$BACKUP_DIR" -name "cliproxy_*.sql.gz" -mtime +$RETENTION_DAYS -delete -print | wc -l)
    if [ "$DELETED" -gt 0 ]; then
        log "🗑️ 已清理 $DELETED 个超过 $RETENTION_DAYS 天的旧备份"
    fi

    # 验证备份完整性
    if gunzip -t "$BACKUP_FILE" 2>/dev/null; then
        log "✓ 备份文件完整性验证通过"
    else
        log "❌ 警告: 备份文件可能已损坏"
        exit 1
    fi
else
    log "❌ 备份失败"
    exit 1
fi

log "备份完成"
```

设置定时任务：

```bash
# 赋予执行权限
chmod +x /usr/local/bin/backup-cliproxy.sh

# 添加到 crontab（每天凌晨 2 点执行）
(crontab -l 2>/dev/null; echo "0 2 * * * /usr/local/bin/backup-cliproxy.sh") | crontab -

# 查看 crontab
crontab -l

# 手动测试
/usr/local/bin/backup-cliproxy.sh
```

### 恢复操作

#### 从 SQL 备份恢复

```bash
# 1. 恢复完整数据库（会覆盖现有数据）
cat ~/cliproxy-backups/backup_20260207.sql | \
    docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 2. 恢复压缩备份
gunzip -c ~/cliproxy-backups/backup_20260207.sql.gz | \
    docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 3. 恢复自定义格式备份
docker exec -i cliproxy-postgres pg_restore \
    -U cliproxy -d cliproxy --clean --if-exists \
    < ~/cliproxy-backups/backup_20260207.dump

# 4. 并行恢复（加快速度，适合大数据库）
docker exec -i cliproxy-postgres pg_restore \
    -U cliproxy -d cliproxy --jobs=4 \
    < ~/cliproxy-backups/backup_20260207.dump
```

#### 恢复前的准备

```bash
# 1. 备份当前数据（以防万一）
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy \
    > ~/cliproxy-backups/before_restore_$(date +%Y%m%d_%H%M%S).sql

# 2. 停止应用服务（避免数据冲突）
docker compose stop cli-proxy-api

# 3. 清空现有数据（可选）
docker exec -i cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
DROP SCHEMA cliproxy CASCADE;
CREATE SCHEMA cliproxy;
EOF

# 4. 执行恢复
cat ~/cliproxy-backups/backup_20260207.sql | \
    docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 5. 验证恢复结果
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT COUNT(*) FROM cliproxy.auth_store;"

# 6. 重启应用
docker compose start cli-proxy-api
```

#### 选择性恢复（仅恢复特定表）

```bash
# 方法 1: 使用 pg_restore 的 -t 参数
docker exec -i cliproxy-postgres pg_restore \
    -U cliproxy -d cliproxy -t config_store \
    < ~/cliproxy-backups/backup.dump

# 方法 2: 从 SQL 备份中提取特定表
# 先恢复到临时表，再复制数据
docker exec -i cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
-- 恢复配置表
BEGIN;
CREATE TEMP TABLE config_temp AS SELECT * FROM cliproxy.config_store WITH NO DATA;
\copy config_temp FROM '/path/to/config.csv' CSV HEADER
TRUNCATE cliproxy.config_store;
INSERT INTO cliproxy.config_store SELECT * FROM config_temp;
COMMIT;
EOF
```

#### 从 Docker 卷恢复

```bash
# 1. 停止所有服务
docker compose down

# 2. 备份现有数据卷（可选）
docker run --rm \
    -v cliproxy_postgres_data:/data:ro \
    -v ~/cliproxy-backups:/backup \
    alpine tar czf /backup/current_volume_backup.tar.gz -C /data .

# 3. 删除现有数据卷
docker volume rm cliproxy_postgres_data

# 4. 创建新数据卷
docker volume create cliproxy_postgres_data

# 5. 恢复数据
docker run --rm \
    -v cliproxy_postgres_data:/data \
    -v ~/cliproxy-backups:/backup \
    alpine tar xzf /backup/postgres_volume_20260207.tar.gz -C /data

# 6. 重启服务
docker compose up -d

# 7. 验证
docker logs cliproxy-postgres
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SELECT version();"
```

### 灾难恢复计划

```bash
# 创建灾难恢复脚本 /usr/local/bin/cliproxy-disaster-recovery.sh
cat > /usr/local/bin/cliproxy-disaster-recovery.sh << 'EOF'
#!/bin/bash
# CLIProxyAPI 灾难恢复脚本

set -e

BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "用法: $0 <备份文件路径>"
    echo "示例: $0 ~/cliproxy-backups/backup_20260207.sql.gz"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "错误: 备份文件不存在: $BACKUP_FILE"
    exit 1
fi

echo "🚨 开始灾难恢复..."
echo "备份文件: $BACKUP_FILE"
echo ""

# 1. 停止服务
echo "📛 停止应用服务..."
docker compose stop cli-proxy-api

# 2. 备份当前数据库
echo "💾 备份当前数据库..."
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy \
    > /tmp/pre_recovery_$(date +%Y%m%d_%H%M%S).sql

# 3. 清空数据库
echo "🗑️ 清空现有数据..."
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << SQL
DROP SCHEMA IF EXISTS cliproxy CASCADE;
CREATE SCHEMA cliproxy;
SQL

# 4. 恢复数据
echo "📥 恢复数据..."
if [[ "$BACKUP_FILE" == *.gz ]]; then
    gunzip -c "$BACKUP_FILE" | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy
else
    cat "$BACKUP_FILE" | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy
fi

# 5. 验证
echo "✓ 验证数据..."
TOKEN_COUNT=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM cliproxy.auth_store;")
CONFIG_COUNT=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM cliproxy.config_store;")

echo "  - 令牌数量: $TOKEN_COUNT"
echo "  - 配置数量: $CONFIG_COUNT"

# 6. 重启服务
echo "🚀 重启应用..."
docker compose start cli-proxy-api

# 7. 等待服务就绪
echo "⏳ 等待服务启动..."
sleep 5

# 8. 健康检查
if docker exec cliproxy-postgres pg_isready -U cliproxy > /dev/null 2>&1; then
    echo "✅ 灾难恢复完成！"
else
    echo "❌ 警告: 数据库可能未正常启动"
    exit 1
fi
EOF

chmod +x /usr/local/bin/cliproxy-disaster-recovery.sh
```

---

## 数据查询与管理

### 配置查询实用案例

```sql
-- 1. 检查配置文件大小变化
SELECT
    id,
    LENGTH(content) as size_bytes,
    pg_size_pretty(LENGTH(content)) as size_pretty,
    updated_at,
    updated_at - created_at as config_age
FROM cliproxy.config_store;

-- 2. 搜索配置中的 API 密钥（不显示完整密钥）
SELECT
    id,
    CASE WHEN content ~* 'api[-_]?keys?' THEN '✓ 包含 API Keys' ELSE '✗ 无 API Keys' END as has_api_keys,
    CASE WHEN content ~* 'claude' THEN '✓ 配置了 Claude' ELSE '✗' END as has_claude,
    CASE WHEN content ~* 'gemini' THEN '✓ 配置了 Gemini' ELSE '✗' END as has_gemini,
    CASE WHEN content ~* 'kiro' THEN '✓ 配置了 Kiro' ELSE '✗' END as has_kiro,
    updated_at
FROM cliproxy.config_store;

-- 3. 分行显示配置内容（便于阅读）
SELECT
    unnest(string_to_array(content, E'\n')) as line
FROM cliproxy.config_store
WHERE id = 'config';

-- 4. 配置变更审计（需要启用审计功能）
SELECT
    timestamp,
    action,
    user_name,
    CASE
        WHEN old_data IS NULL THEN '新建配置'
        WHEN new_data IS NULL THEN '删除配置'
        ELSE '修改配置'
    END as change_type
FROM cliproxy.audit_log
WHERE table_name = 'config_store'
ORDER BY timestamp DESC
LIMIT 20;
```

### 令牌查询实用案例

```sql
-- 1. 令牌健康检查仪表板
SELECT
    content->>'type' as provider,
    COUNT(*) as total_tokens,
    COUNT(CASE WHEN content ? 'access_token' THEN 1 END) as with_access_token,
    COUNT(CASE WHEN content ? 'refresh_token' THEN 1 END) as with_refresh_token,
    ROUND(AVG(EXTRACT(epoch FROM (NOW() - updated_at))/3600)::numeric, 1) as avg_hours_since_update,
    MAX(updated_at) as most_recent_update
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY total_tokens DESC;

-- 2. 查找需要刷新的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as account,
    updated_at,
    NOW() - updated_at as age,
    CASE
        WHEN NOW() - updated_at > INTERVAL '60 days' THEN '🔴 紧急'
        WHEN NOW() - updated_at > INTERVAL '30 days' THEN '🟡 警告'
        ELSE '🟢 正常'
    END as status
FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '30 days'
ORDER BY updated_at;

-- 3. 按账号分组查看令牌
SELECT
    content->>'email' as account,
    array_agg(content->>'type') as providers,
    COUNT(*) as token_count,
    MIN(created_at) as first_token,
    MAX(updated_at) as last_update
FROM cliproxy.auth_store
WHERE content->>'email' IS NOT NULL
GROUP BY content->>'email'
ORDER BY token_count DESC;

-- 4. 查找重复的令牌
SELECT
    content->>'type' as provider,
    content->>'email' as account,
    COUNT(*) as duplicate_count,
    array_agg(id) as token_ids
FROM cliproxy.auth_store
GROUP BY content->>'type', content->>'email'
HAVING COUNT(*) > 1;

-- 5. 令牌大小分析
SELECT
    content->>'type' as provider,
    COUNT(*) as count,
    pg_size_pretty(AVG(LENGTH(content::text))::bigint) as avg_size,
    pg_size_pretty(SUM(LENGTH(content::text))::bigint) as total_size
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY SUM(LENGTH(content::text)) DESC;

-- 6. 查看最近添加的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    created_at,
    AGE(NOW(), created_at) as age
FROM cliproxy.auth_store
ORDER BY created_at DESC
LIMIT 10;

-- 7. 查找包含特定字段的令牌
SELECT
    id,
    content->>'type' as provider,
    jsonb_object_keys(content) as available_fields
FROM cliproxy.auth_store
WHERE content ? 'refresh_token'  -- 查找包含 refresh_token 的令牌
LIMIT 5;
```

### 数据清理操作

```sql
-- 1. 安全删除前的预览
-- 预览将被删除的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    updated_at,
    NOW() - updated_at as age
FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '180 days'
ORDER BY updated_at;

-- 确认无误后执行删除
BEGIN;
DELETE FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '180 days';
-- 检查删除数量
-- 如果正确，执行 COMMIT; 如果错误，执行 ROLLBACK;
SELECT '已删除 ' || COUNT(*) || ' 个令牌' FROM cliproxy.auth_store WHERE FALSE;
COMMIT;

-- 2. 删除特定提供商的令牌
-- 先查看
SELECT COUNT(*) FROM cliproxy.auth_store WHERE content->>'type' = 'test-provider';
-- 确认后删除
DELETE FROM cliproxy.auth_store WHERE content->>'type' = 'test-provider';

-- 3. 删除测试账号的令牌
DELETE FROM cliproxy.auth_store
WHERE content->>'email' LIKE '%test%'
   OR content->>'email' LIKE '%example.com%';

-- 4. 清理损坏的令牌数据
-- 查找格式异常的令牌
SELECT
    id,
    jsonb_typeof(content) as type,
    content
FROM cliproxy.auth_store
WHERE jsonb_typeof(content) != 'object'
   OR content = '{}'::jsonb
   OR NOT (content ? 'type');

-- 删除损坏的令牌
DELETE FROM cliproxy.auth_store
WHERE jsonb_typeof(content) != 'object'
   OR content = '{}'::jsonb
   OR NOT (content ? 'type');

-- 5. 归档旧令牌（移动到历史表）
-- 创建归档表
CREATE TABLE IF NOT EXISTS cliproxy.auth_store_archive (
    LIKE cliproxy.auth_store INCLUDING ALL
);

-- 移动旧令牌到归档表
WITH moved AS (
    DELETE FROM cliproxy.auth_store
    WHERE updated_at < NOW() - INTERVAL '1 year'
    RETURNING *
)
INSERT INTO cliproxy.auth_store_archive
SELECT * FROM moved;

-- 6. 去重（保留最新的令牌）
WITH duplicates AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY content->>'type', content->>'email'
            ORDER BY updated_at DESC
        ) as rn
    FROM cliproxy.auth_store
)
DELETE FROM cliproxy.auth_store
WHERE id IN (
    SELECT id FROM duplicates WHERE rn > 1
);
```

### 数据统计报表

```sql
-- 综合统计报表
SELECT
    '数据库总大小' as metric,
    pg_size_pretty(pg_database_size('cliproxy')) as value
UNION ALL
SELECT
    '令牌总数',
    COUNT(*)::text
FROM cliproxy.auth_store
UNION ALL
SELECT
    '配置数量',
    COUNT(*)::text
FROM cliproxy.config_store
UNION ALL
SELECT
    '最早令牌日期',
    MIN(created_at)::text
FROM cliproxy.auth_store
UNION ALL
SELECT
    '最新令牌更新',
    MAX(updated_at)::text
FROM cliproxy.auth_store;

-- 按周统计新增令牌
SELECT
    DATE_TRUNC('week', created_at) as week,
    content->>'type' as provider,
    COUNT(*) as new_tokens
FROM cliproxy.auth_store
WHERE created_at > NOW() - INTERVAL '3 months'
GROUP BY DATE_TRUNC('week', created_at), content->>'type'
ORDER BY week DESC, new_tokens DESC;
```

---

## 性能监控与优化

### 性能指标监控

```sql
-- 1. 数据库整体性能指标
SELECT
    numbackends as active_connections,
    xact_commit as total_commits,
    xact_rollback as total_rollbacks,
    ROUND(100.0 * xact_rollback / NULLIF(xact_commit + xact_rollback, 0), 2) as rollback_rate,
    blks_read as disk_reads,
    blks_hit as cache_hits,
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) as cache_hit_ratio,
    tup_returned as rows_returned,
    tup_fetched as rows_fetched,
    tup_inserted as rows_inserted,
    tup_updated as rows_updated,
    tup_deleted as rows_deleted
FROM pg_stat_database
WHERE datname = 'cliproxy';

-- 2. 表访问模式分析
SELECT
    schemaname,
    tablename,
    seq_scan as sequential_scans,
    seq_tup_read as seq_rows_read,
    idx_scan as index_scans,
    idx_tup_fetch as idx_rows_fetched,
    ROUND(100.0 * idx_scan / NULLIF(seq_scan + idx_scan, 0), 2) as index_usage_ratio,
    n_tup_ins + n_tup_upd + n_tup_del as total_modifications,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as dead_row_ratio,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY seq_scan + idx_scan DESC;

-- 3. 索引效率分析
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
    CASE
        WHEN idx_scan = 0 THEN '❌ 从未使用'
        WHEN idx_scan < 100 THEN '⚠️ 很少使用'
        ELSE '✅ 经常使用'
    END as usage_status
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
ORDER BY idx_scan DESC;

-- 4. 慢查询监控（需要启用 pg_stat_statements）
-- 首先启用扩展: CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SELECT
    LEFT(query, 100) as query_preview,
    calls,
    ROUND(total_exec_time::numeric, 2) as total_time_ms,
    ROUND(mean_exec_time::numeric, 2) as avg_time_ms,
    ROUND(max_exec_time::numeric, 2) as max_time_ms,
    ROUND(stddev_exec_time::numeric, 2) as stddev_time_ms
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat_statements%'
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 5. 锁等待检测
SELECT
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS blocking_statement,
    blocked_activity.wait_event_type AS blocked_wait_type,
    NOW() - blocked_activity.query_start AS blocked_duration
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;

-- 6. I/O 性能统计
SELECT
    tablename,
    heap_blks_read as heap_disk_reads,
    heap_blks_hit as heap_cache_hits,
    ROUND(100.0 * heap_blks_hit / NULLIF(heap_blks_hit + heap_blks_read, 0), 2) as heap_cache_ratio,
    idx_blks_read as index_disk_reads,
    idx_blks_hit as index_cache_hits,
    ROUND(100.0 * idx_blks_hit / NULLIF(idx_blks_hit + idx_blks_read, 0), 2) as index_cache_ratio
FROM pg_statio_user_tables
WHERE schemaname = 'cliproxy';
```

### 索引优化

```sql
-- 1. 查看现有索引
SELECT
    schemaname,
    tablename,
    indexname,
    indexdef,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_indexes
WHERE schemaname = 'cliproxy'
ORDER BY tablename, indexname;

-- 2. 识别缺失的索引（基于查询模式）
-- 查找频繁进行全表扫描的表
SELECT
    schemaname,
    tablename,
    seq_scan,
    seq_tup_read,
    idx_scan,
    CASE
        WHEN seq_scan > 100 AND idx_scan = 0 THEN '🔴 急需索引'
        WHEN seq_scan > idx_scan THEN '🟡 考虑添加索引'
        ELSE '🟢 索引使用良好'
    END as recommendation
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY seq_scan DESC;

-- 3. 查找未使用的索引（可考虑删除）
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
    indexdef
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
  AND idx_scan = 0
  AND indexrelname NOT LIKE '%_pkey'  -- 保留主键
ORDER BY pg_relation_size(indexrelid) DESC;

-- 4. 创建推荐索引
-- 为 updated_at 添加索引（提升时间范围查询）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_store_updated_at
ON cliproxy.auth_store(updated_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_store_updated_at
ON cliproxy.config_store(updated_at DESC);

-- 为 JSONB content 添加 GIN 索引（支持 JSON 查询）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_store_content_gin
ON cliproxy.auth_store USING GIN (content);

-- 为特定 JSON 字段添加表达式索引
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_store_type
ON cliproxy.auth_store ((content->>'type'));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_store_email
ON cliproxy.auth_store ((content->>'email'));

-- 组合索引（用于多条件查询）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_store_type_updated
ON cliproxy.auth_store ((content->>'type'), updated_at DESC);

-- 5. 监控索引创建进度
SELECT
    phase,
    tuples_done,
    tuples_total,
    ROUND(100.0 * tuples_done / NULLIF(tuples_total, 0), 2) as progress_percent,
    current_locker_pid
FROM pg_stat_progress_create_index;

-- 6. 重建膨胀的索引
REINDEX INDEX CONCURRENTLY cliproxy.idx_auth_store_updated_at;
REINDEX TABLE CONCURRENTLY cliproxy.auth_store;

-- 7. 分析索引使用效果（创建索引后）
EXPLAIN ANALYZE
SELECT *
FROM cliproxy.auth_store
WHERE content->>'type' = 'claude'
  AND updated_at > NOW() - INTERVAL '7 days';
```

### 数据库维护

```sql
-- 1. 分析表（更新统计信息，优化查询计划）
ANALYZE cliproxy.config_store;
ANALYZE cliproxy.auth_store;
ANALYZE;  -- 分析所有表

-- 2. 清理表（回收空间）
-- 标准 VACUUM（不锁表，在线执行）
VACUUM cliproxy.config_store;
VACUUM cliproxy.auth_store;

-- VACUUM FULL（会锁表，回收更多空间）
VACUUM FULL cliproxy.config_store;
VACUUM FULL cliproxy.auth_store;

-- VACUUM ANALYZE（清理 + 分析，推荐）
VACUUM ANALYZE cliproxy.config_store;
VACUUM ANALYZE cliproxy.auth_store;

-- 3. 查看表膨胀情况
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as bloat_percent,
    last_vacuum,
    last_autovacuum
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY n_dead_tup DESC;

-- 4. 重建表（极端情况下使用）
-- 警告：会锁表，生产环境谨慎使用
CLUSTER cliproxy.auth_store USING auth_store_pkey;

-- 5. 检查自动 VACUUM 配置
SELECT
    name,
    setting,
    unit,
    short_desc
FROM pg_settings
WHERE name LIKE '%autovacuum%'
   OR name LIKE '%vacuum%'
ORDER BY name;
```

### PostgreSQL 配置优化

```bash
# 查看当前配置
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SELECT
    name,
    setting,
    unit,
    short_desc
FROM pg_settings
WHERE name IN (
    'shared_buffers',
    'work_mem',
    'maintenance_work_mem',
    'effective_cache_size',
    'max_connections',
    'checkpoint_completion_target',
    'wal_buffers',
    'random_page_cost'
)
ORDER BY name;
EOF
```

在 `compose.yml` 中优化配置：

```yaml
postgres:
  image: postgres:18-alpine
  environment:
    POSTGRES_DB: cliproxy
    POSTGRES_USER: cliproxy
    POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}

    # 性能优化参数
    # 共享缓冲区（推荐设置为系统内存的 25%）
    POSTGRES_SHARED_BUFFERS: "512MB"

    # 最大连接数
    POSTGRES_MAX_CONNECTIONS: "200"

    # 工作内存（每个连接的排序和哈希操作）
    POSTGRES_WORK_MEM: "16MB"

    # 维护工作内存（VACUUM、CREATE INDEX 等）
    POSTGRES_MAINTENANCE_WORK_MEM: "128MB"

    # 有效缓存大小（操作系统 + PostgreSQL 缓存）
    POSTGRES_EFFECTIVE_CACHE_SIZE: "1GB"

    # WAL 缓冲区
    POSTGRES_WAL_BUFFERS: "16MB"

    # 检查点完成目标
    POSTGRES_CHECKPOINT_COMPLETION_TARGET: "0.9"
```

---

## 故障排查

### 连接问题诊断

```bash
# 1. 检查容器状态
docker ps | grep cliproxy-postgres
# 应该显示 "Up" 状态

# 2. 检查数据库是否就绪
docker exec cliproxy-postgres pg_isready -U cliproxy
# 应该显示: accepting connections

# 3. 查看容器日志
docker logs cliproxy-postgres --tail=50
# 查找错误信息

# 4. 检查网络连接
docker exec cli-proxy-api ping -c 3 postgres
# 应该能 ping 通

# 5. 测试端口连通性
docker exec cli-proxy-api nc -zv postgres 5432
# 应该显示: succeeded

# 6. 测试数据库连接
docker exec cli-proxy-api psql \
    "postgresql://cliproxy:password@postgres:5432/cliproxy" \
    -c "SELECT 1;"
# 应该返回: 1

# 7. 检查环境变量
docker exec cli-proxy-api env | grep PGSTORE
# 验证 DSN 配置是否正确

# 8. 检查防火墙和网络策略
docker network ls
docker network inspect cliproxy-network
```

### 性能问题排查

```sql
-- 1. 查找长时间运行的查询
SELECT
    pid,
    now() - query_start as duration,
    state,
    usename,
    LEFT(query, 100) as query_preview
FROM pg_stat_activity
WHERE state != 'idle'
  AND datname = 'cliproxy'
  AND now() - query_start > interval '5 seconds'
ORDER BY duration DESC;

-- 2. 终止慢查询
-- 先尝试取消（不会回滚事务）
SELECT pg_cancel_backend(12345);  -- 替换为实际 pid

-- 如果取消无效，强制终止（会回滚事务）
SELECT pg_terminate_backend(12345);

-- 3. 查找占用 CPU 的查询
SELECT
    pid,
    usename,
    application_name,
    state,
    query_start,
    LEFT(query, 100) as query
FROM pg_stat_activity
WHERE state = 'active'
  AND datname = 'cliproxy'
ORDER BY query_start
LIMIT 10;

-- 4. 检查表膨胀
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
    n_dead_tup as dead_tuples,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as bloat_percent,
    CASE
        WHEN n_dead_tup > 10000 AND n_dead_tup::float / NULLIF(n_live_tup, 0) > 0.2 THEN '🔴 需要 VACUUM'
        WHEN n_dead_tup > 5000 THEN '🟡 建议 VACUUM'
        ELSE '🟢 正常'
    END as status
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY n_dead_tup DESC;

-- 5. 检查缓存命中率
SELECT
    'Database' as type,
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) as cache_hit_ratio
FROM pg_stat_database
WHERE datname = 'cliproxy'
UNION ALL
SELECT
    'Table: ' || tablename,
    ROUND(100.0 * heap_blks_hit / NULLIF(heap_blks_hit + heap_blks_read, 0), 2)
FROM pg_statio_user_tables
WHERE schemaname = 'cliproxy';

-- 缓存命中率建议：
-- > 99%: 优秀
-- 95-99%: 良好
-- < 95%: 需要增加 shared_buffers

-- 6. 查看等待事件
SELECT
    wait_event_type,
    wait_event,
    COUNT(*) as waiting_count
FROM pg_stat_activity
WHERE wait_event IS NOT NULL
  AND datname = 'cliproxy'
GROUP BY wait_event_type, wait_event
ORDER BY waiting_count DESC;
```

### 数据一致性问题

```sql
-- 1. 检查表完整性
SELECT
    schemaname,
    tablename,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    last_vacuum,
    last_analyze
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';

-- 2. 验证主键约束
SELECT
    conname as constraint_name,
    contype as type,
    pg_get_constraintdef(oid) as definition
FROM pg_constraint
WHERE conrelid = 'cliproxy.auth_store'::regclass;

-- 3. 检查重复主键（理论上不应该存在）
SELECT id, COUNT(*)
FROM cliproxy.auth_store
GROUP BY id
HAVING COUNT(*) > 1;

-- 4. 检查 NULL 值（主键和 NOT NULL 字段）
SELECT
    COUNT(*) FILTER (WHERE id IS NULL) as null_ids,
    COUNT(*) FILTER (WHERE content IS NULL) as null_content,
    COUNT(*) as total
FROM cliproxy.auth_store;

-- 5. 验证 JSONB 格式
SELECT
    id,
    jsonb_typeof(content) as json_type,
    CASE
        WHEN jsonb_typeof(content) != 'object' THEN '❌ 不是对象'
        WHEN content = '{}'::jsonb THEN '⚠️ 空对象'
        WHEN NOT (content ? 'type') THEN '⚠️ 缺少 type 字段'
        ELSE '✅ 正常'
    END as status
FROM cliproxy.auth_store
WHERE jsonb_typeof(content) != 'object'
   OR content = '{}'::jsonb
   OR NOT (content ? 'type');

-- 6. 修复损坏的数据
-- 删除格式异常的记录
DELETE FROM cliproxy.auth_store
WHERE jsonb_typeof(content) != 'object'
   OR content = '{}'::jsonb
   OR NOT (content ? 'type');
```

### 磁盘空间问题

```bash
# 1. 查看磁盘使用情况
docker exec cliproxy-postgres df -h
# 关注 /var/lib/postgresql/data 的使用率

# 2. 查看数据库大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SELECT
    pg_size_pretty(pg_database_size('cliproxy')) as database_size,
    pg_size_pretty(pg_tablespace_size('pg_default')) as tablespace_size;
EOF

# 3. 查看各表大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SELECT
    schemaname||'.'||tablename as table,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as indexes_size
FROM pg_tables
WHERE schemaname = 'cliproxy'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
EOF

# 4. 查看 WAL 日志大小
docker exec cliproxy-postgres du -sh /var/lib/postgresql/data/pg_wal

# 5. 清理空间的方法
# 方法 1: 清理旧数据
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "DELETE FROM cliproxy.auth_store WHERE updated_at < NOW() - INTERVAL '180 days';"

# 方法 2: VACUUM FULL 回收空间（会锁表）
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "VACUUM FULL cliproxy.auth_store;"

# 方法 3: 清理 PostgreSQL 日志
docker exec cliproxy-postgres sh -c "find /var/lib/postgresql/data/log -name '*.log' -mtime +7 -delete"

# 方法 4: 清理 WAL 归档（如果启用了归档）
docker exec cliproxy-postgres sh -c "find /var/lib/postgresql/data/pg_wal -name '*.old' -delete"

# 6. 扩展 Docker 卷（如果需要）
# 参考 Docker 文档进行卷扩容
```

### 常见错误及解决方案

#### 错误 1: role "postgres" does not exist

```bash
# 原因：默认用户不是 postgres，而是配置的用户名
# 解决：使用正确的用户名
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy
```

#### 错误 2: password authentication failed

```bash
# 原因：密码错误
# 解决：检查 .env 文件或环境变量中的 POSTGRES_PASSWORD
cat .env | grep POSTGRES_PASSWORD

# 如需重置密码
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy
ALTER USER cliproxy WITH PASSWORD 'new_password';
# 然后更新 PGSTORE_DSN 中的密码
```

#### 错误 3: could not connect to server

```bash
# 原因：数据库未启动或网络问题
# 解决：
# 1. 检查容器状态
docker ps | grep cliproxy-postgres

# 2. 查看日志
docker logs cliproxy-postgres --tail=50

# 3. 重启容器
docker compose restart postgres

# 4. 检查网络
docker network inspect cliproxy-network
```

#### 错误 4: too many connections

```sql
-- 原因：连接数超过 max_connections 限制
-- 解决：
-- 1. 查看当前连接数
SELECT COUNT(*) FROM pg_stat_activity WHERE datname = 'cliproxy';

-- 2. 查看最大连接数
SHOW max_connections;

-- 3. 终止空闲连接
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = 'cliproxy'
  AND state = 'idle'
  AND NOW() - state_change > INTERVAL '10 minutes';

-- 4. 增加最大连接数（在 compose.yml 中）
-- POSTGRES_MAX_CONNECTIONS: "200"
```

#### 错误 5: disk full

```bash
# 原因：磁盘空间不足
# 解决：
# 1. 清理旧数据
# 2. 执行 VACUUM FULL
# 3. 清理日志文件
# 4. 扩展磁盘空间
# 详见上面的"磁盘空间问题"章节
```

---

## 安全管理

### 用户权限管理

```sql
-- 1. 查看所有用户
\du

-- 或者使用 SQL
SELECT
    usename as username,
    usesuper as is_superuser,
    usecreatedb as can_create_db,
    usecreaterole as can_create_role,
    useconnlimit as connection_limit,
    valuntil as password_expiry
FROM pg_user
ORDER BY usename;

-- 2. 创建只读用户
CREATE USER readonly WITH PASSWORD 'readonly_password';
GRANT CONNECT ON DATABASE cliproxy TO readonly;
GRANT USAGE ON SCHEMA cliproxy TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA cliproxy TO readonly;

-- 确保新表也自动授权
ALTER DEFAULT PRIVILEGES IN SCHEMA cliproxy
    GRANT SELECT ON TABLES TO readonly;

-- 3. 创建读写用户
CREATE USER readwrite WITH PASSWORD 'readwrite_password';
GRANT CONNECT ON DATABASE cliproxy TO readwrite;
GRANT USAGE ON SCHEMA cliproxy TO readwrite;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA cliproxy TO readwrite;

-- 4. 创建管理员用户
CREATE USER admin_user WITH PASSWORD 'admin_password';
GRANT ALL PRIVILEGES ON DATABASE cliproxy TO admin_user;
GRANT ALL PRIVILEGES ON SCHEMA cliproxy TO admin_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA cliproxy TO admin_user;

-- 5. 撤销权限
REVOKE ALL ON DATABASE cliproxy FROM some_user;
REVOKE SELECT ON cliproxy.auth_store FROM some_user;

-- 6. 删除用户
-- 先撤销所有权限
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA cliproxy FROM old_user;
REVOKE ALL PRIVILEGES ON SCHEMA cliproxy FROM old_user;
REVOKE CONNECT ON DATABASE cliproxy FROM old_user;
-- 然后删除用户
DROP USER old_user;

-- 7. 修改用户密码
ALTER USER cliproxy WITH PASSWORD 'new_secure_password';

-- 8. 设置密码过期时间
ALTER USER cliproxy VALID UNTIL '2026-12-31';

-- 9. 限制用户连接数
ALTER USER readonly CONNECTION LIMIT 5;

-- 10. 查看用户权限
SELECT
    grantee,
    table_schema,
    table_name,
    privilege_type
FROM information_schema.role_table_grants
WHERE grantee = 'readonly'
  AND table_schema = 'cliproxy'
ORDER BY table_name, privilege_type;
```

### 连接安全

```bash
# 1. 启用 SSL/TLS 连接
# 修改 DSN 使用 SSL
export PGSTORE_DSN="postgresql://cliproxy:password@postgres:5432/cliproxy?sslmode=require"

# 2. 生成 SSL 证书（自签名，用于测试）
docker exec cliproxy-postgres openssl req -new -x509 -days 365 -nodes \
    -out /var/lib/postgresql/server.crt \
    -keyout /var/lib/postgresql/server.key \
    -subj "/CN=cliproxy-postgres"

# 设置证书权限
docker exec cliproxy-postgres chown postgres:postgres /var/lib/postgresql/server.{crt,key}
docker exec cliproxy-postgres chmod 600 /var/lib/postgresql/server.key

# 3. 启用 SSL（修改 PostgreSQL 配置）
docker exec cliproxy-postgres sh -c "echo 'ssl = on' >> /var/lib/postgresql/data/postgresql.conf"
docker exec cliproxy-postgres sh -c "echo 'ssl_cert_file = \'/var/lib/postgresql/server.crt\'' >> /var/lib/postgresql/data/postgresql.conf"
docker exec cliproxy-postgres sh -c "echo 'ssl_key_file = \'/var/lib/postgresql/server.key\'' >> /var/lib/postgresql/data/postgresql.conf"

# 重启 PostgreSQL
docker compose restart postgres

# 4. 限制连接来源（修改 pg_hba.conf）
# 仅允许来自应用容器的连接
docker exec cliproxy-postgres sh -c "cat >> /var/lib/postgresql/data/pg_hba.conf << EOF
# 仅允许内部网络连接
host    cliproxy        cliproxy        172.18.0.0/16           md5
# 拒绝其他所有连接
host    all             all             0.0.0.0/0               reject
EOF"
```

### 审计日志

```sql
-- 1. 创建审计日志表
CREATE TABLE IF NOT EXISTS cliproxy.audit_log (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    user_name TEXT,
    client_addr INET,
    operation TEXT,
    table_name TEXT,
    record_id TEXT,
    old_data JSONB,
    new_data JSONB,
    query TEXT
);

-- 创建索引
CREATE INDEX idx_audit_log_timestamp ON cliproxy.audit_log(timestamp DESC);
CREATE INDEX idx_audit_log_operation ON cliproxy.audit_log(operation);
CREATE INDEX idx_audit_log_table ON cliproxy.audit_log(table_name);

-- 2. 创建审计触发器函数
CREATE OR REPLACE FUNCTION cliproxy.audit_trigger_func()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'DELETE') THEN
        INSERT INTO cliproxy.audit_log (
            user_name,
            client_addr,
            operation,
            table_name,
            record_id,
            old_data
        ) VALUES (
            current_user,
            inet_client_addr(),
            TG_OP,
            TG_TABLE_NAME,
            OLD.id,
            row_to_json(OLD)::jsonb
        );
        RETURN OLD;
    ELSIF (TG_OP = 'UPDATE') THEN
        INSERT INTO cliproxy.audit_log (
            user_name,
            client_addr,
            operation,
            table_name,
            record_id,
            old_data,
            new_data
        ) VALUES (
            current_user,
            inet_client_addr(),
            TG_OP,
            TG_TABLE_NAME,
            NEW.id,
            row_to_json(OLD)::jsonb,
            row_to_json(NEW)::jsonb
        );
        RETURN NEW;
    ELSIF (TG_OP = 'INSERT') THEN
        INSERT INTO cliproxy.audit_log (
            user_name,
            client_addr,
            operation,
            table_name,
            record_id,
            new_data
        ) VALUES (
            current_user,
            inet_client_addr(),
            TG_OP,
            TG_TABLE_NAME,
            NEW.id,
            row_to_json(NEW)::jsonb
        );
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- 3. 应用审计触发器
CREATE TRIGGER auth_store_audit
AFTER INSERT OR UPDATE OR DELETE ON cliproxy.auth_store
FOR EACH ROW EXECUTE FUNCTION cliproxy.audit_trigger_func();

CREATE TRIGGER config_store_audit
AFTER INSERT OR UPDATE OR DELETE ON cliproxy.config_store
FOR EACH ROW EXECUTE FUNCTION cliproxy.audit_trigger_func();

-- 4. 查看审计日志
SELECT
    timestamp,
    user_name,
    operation,
    table_name,
    record_id,
    CASE
        WHEN operation = 'INSERT' THEN '新增'
        WHEN operation = 'UPDATE' THEN '修改'
        WHEN operation = 'DELETE' THEN '删除'
    END as operation_cn
FROM cliproxy.audit_log
ORDER BY timestamp DESC
LIMIT 50;

-- 5. 查看特定表的变更历史
SELECT
    timestamp,
    user_name,
    operation,
    record_id,
    CASE
        WHEN old_data IS NOT NULL THEN jsonb_pretty(old_data)
        ELSE NULL
    END as before,
    CASE
        WHEN new_data IS NOT NULL THEN jsonb_pretty(new_data)
        ELSE NULL
    END as after
FROM cliproxy.audit_log
WHERE table_name = 'auth_store'
  AND record_id = 'specific_token_id'
ORDER BY timestamp DESC;

-- 6. 统计操作频率
SELECT
    table_name,
    operation,
    COUNT(*) as count,
    MAX(timestamp) as last_operation
FROM cliproxy.audit_log
WHERE timestamp > NOW() - INTERVAL '7 days'
GROUP BY table_name, operation
ORDER BY count DESC;

-- 7. 清理旧审计日志
DELETE FROM cliproxy.audit_log
WHERE timestamp < NOW() - INTERVAL '90 days';
```

### 数据加密

```sql
-- 1. 启用 pgcrypto 扩展
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 2. 加密敏感字段（示例）
-- 加密
SELECT pgp_sym_encrypt('sensitive_data', 'encryption_key');

-- 解密
SELECT pgp_sym_decrypt(
    '\x...'::bytea,  -- 加密后的数据
    'encryption_key'
);

-- 3. 存储加密数据的表结构示例
CREATE TABLE IF NOT EXISTS cliproxy.encrypted_secrets (
    id TEXT PRIMARY KEY,
    encrypted_value BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 插入加密数据
INSERT INTO cliproxy.encrypted_secrets (id, encrypted_value)
VALUES ('api_key_1', pgp_sym_encrypt('secret_value', 'master_key'));

-- 读取解密数据
SELECT
    id,
    pgp_sym_decrypt(encrypted_value, 'master_key') as decrypted_value
FROM cliproxy.encrypted_secrets;
```

### 安全检查清单

```bash
#!/bin/bash
# 安全检查脚本

echo "=== CLIProxyAPI 数据库安全检查 ==="
echo ""

# 1. 检查密码强度
echo "1. 检查数据库密码..."
if [ "$POSTGRES_PASSWORD" == "changeme" ] || [ "$POSTGRES_PASSWORD" == "password" ]; then
    echo "❌ 警告：使用了弱密码！"
else
    echo "✅ 密码已修改"
fi
echo ""

# 2. 检查 SSL 状态
echo "2. 检查 SSL 配置..."
SSL_STATUS=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c "SHOW ssl;")
if [[ "$SSL_STATUS" == *"on"* ]]; then
    echo "✅ SSL 已启用"
else
    echo "⚠️ SSL 未启用（生产环境建议启用）"
fi
echo ""

# 3. 检查端口暴露
echo "3. 检查端口暴露..."
EXPOSED=$(docker port cliproxy-postgres 5432 2>/dev/null)
if [ -z "$EXPOSED" ]; then
    echo "✅ 5432 端口未暴露到宿主机"
else
    echo "⚠️ 5432 端口已暴露: $EXPOSED（生产环境不建议暴露）"
fi
echo ""

# 4. 检查用户权限
echo "4. 检查用户权限..."
SUPERUSERS=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM pg_user WHERE usesuper = true;")
echo "超级用户数量: $SUPERUSERS"
echo ""

# 5. 检查备份
echo "5. 检查最近备份..."
LAST_BACKUP=$(find /var/backups/cliproxy -name "*.sql.gz" -mtime -1 2>/dev/null | wc -l)
if [ "$LAST_BACKUP" -gt 0 ]; then
    echo "✅ 最近 24 小时有 $LAST_BACKUP 个备份"
else
    echo "⚠️ 最近 24 小时没有备份"
fi
echo ""

# 6. 检查连接数
echo "6. 检查连接数..."
CONNECTIONS=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = 'cliproxy';")
MAX_CONN=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c "SHOW max_connections;")
echo "当前连接数: $CONNECTIONS / $MAX_CONN"
echo ""

echo "=== 检查完成 ==="
```

---

## 数据迁移

### 从文件存储迁移到 PostgreSQL

```bash
#!/bin/bash
# 从文件存储迁移到 PostgreSQL

set -e

echo "开始迁移到 PostgreSQL..."

# 1. 备份现有文件
echo "1. 备份现有令牌文件..."
BACKUP_DIR=~/cliproxy-migration-backup-$(date +%Y%m%d_%H%M%S)
mkdir -p "$BACKUP_DIR"

if [ -d ~/.cli-proxy-api ]; then
    cp -r ~/.cli-proxy-api "$BACKUP_DIR/"
    echo "✅ 已备份到: $BACKUP_DIR"
else
    echo "⚠️ 未找到 ~/.cli-proxy-api 目录"
fi

# 2. 启动 PostgreSQL 环境
echo "2. 配置 PostgreSQL 环境..."
export PGSTORE_DSN="postgresql://cliproxy:password@localhost:5432/cliproxy?sslmode=disable"
export PGSTORE_SCHEMA="cliproxy"
export PGSTORE_LOCAL_PATH="/var/lib/cliproxy/pgstore"

# 3. 启动服务
echo "3. 启动服务..."
docker compose up -d postgres
sleep 5

# 4. 等待数据库就绪
echo "4. 等待数据库就绪..."
until docker exec cliproxy-postgres pg_isready -U cliproxy > /dev/null 2>&1; do
    echo "   等待 PostgreSQL 启动..."
    sleep 2
done
echo "✅ PostgreSQL 已就绪"

# 5. 复制文件到本地缓存目录
echo "5. 复制令牌文件到 pgstore..."
if [ -d ~/.cli-proxy-api ]; then
    docker exec cli-proxy-api mkdir -p /var/lib/cliproxy/pgstore/auths
    docker cp ~/.cli-proxy-api/. cli-proxy-api:/var/lib/cliproxy/pgstore/auths/
    echo "✅ 文件已复制"
fi

# 6. 重启应用（触发同步）
echo "6. 重启应用服务..."
docker compose restart cli-proxy-api
sleep 3

# 7. 验证迁移
echo "7. 验证迁移结果..."
TOKEN_COUNT=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM cliproxy.auth_store;")
echo "✅ 数据库中的令牌数量: $TOKEN_COUNT"

# 8. 列出迁移的令牌
echo "8. 迁移的令牌列表:"
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as account,
    updated_at
FROM cliproxy.auth_store
ORDER BY content->>'type', id;
EOF

echo ""
echo "✅ 迁移完成！"
echo "备份位置: $BACKUP_DIR"
echo ""
echo "建议："
echo "1. 验证所有令牌是否正常工作"
echo "2. 确认无误后可以删除原文件: rm -rf ~/.cli-proxy-api"
echo "3. 定期备份数据库: /usr/local/bin/backup-cliproxy.sh"
```

### 从 PostgreSQL 导出到文件

```bash
#!/bin/bash
# 从 PostgreSQL 导出令牌到文件系统

set -e

EXPORT_DIR=~/cliproxy-export-$(date +%Y%m%d_%H%M%S)
mkdir -p "$EXPORT_DIR/config"
mkdir -p "$EXPORT_DIR/auths"

echo "导出数据到文件系统..."
echo "导出目录: $EXPORT_DIR"

# 1. 导出配置
echo "1. 导出配置文件..."
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT content FROM cliproxy.config_store WHERE id = 'config';" \
    > "$EXPORT_DIR/config/config.yaml"
echo "✅ 配置已导出"

# 2. 导出令牌（逐个导出为 JSON 文件）
echo "2. 导出令牌文件..."
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -A -F'|' -c \
    "SELECT id, content FROM cliproxy.auth_store;" | while IFS='|' read -r id content; do
    # 创建子目录
    dir=$(dirname "$id")
    mkdir -p "$EXPORT_DIR/auths/$dir"

    # 写入文件
    echo "$content" > "$EXPORT_DIR/auths/$id"
    echo "  导出: $id"
done
echo "✅ 令牌已导出"

# 3. 生成导出报告
cat > "$EXPORT_DIR/EXPORT_INFO.txt" << EOF
导出时间: $(date)
导出来源: PostgreSQL (cliproxy-postgres)
配置文件数: $(find "$EXPORT_DIR/config" -type f | wc -l)
令牌文件数: $(find "$EXPORT_DIR/auths" -type f | wc -l)

文件列表:
$(tree "$EXPORT_DIR" 2>/dev/null || find "$EXPORT_DIR" -type f)
EOF

echo ""
echo "✅ 导出完成！"
echo "导出位置: $EXPORT_DIR"
cat "$EXPORT_DIR/EXPORT_INFO.txt"
```

### 跨服务器迁移

```bash
#!/bin/bash
# 跨服务器数据库迁移

# === 在源服务器执行 ===
SOURCE_HOST="source.example.com"
TARGET_HOST="target.example.com"
BACKUP_FILE="cliproxy_migration_$(date +%Y%m%d_%H%M%S).dump"

# 1. 在源服务器备份
echo "在源服务器备份数据..."
ssh "$SOURCE_HOST" "docker exec cliproxy-postgres pg_dump -U cliproxy -Fc cliproxy > /tmp/$BACKUP_FILE"

# 2. 传输到目标服务器
echo "传输备份文件到目标服务器..."
scp "$SOURCE_HOST:/tmp/$BACKUP_FILE" "/tmp/$BACKUP_FILE"
scp "/tmp/$BACKUP_FILE" "$TARGET_HOST:/tmp/"

# 3. 在目标服务器恢复
echo "在目标服务器恢复数据..."
ssh "$TARGET_HOST" << 'ENDSSH'
# 停止应用
docker compose stop cli-proxy-api

# 清空现有数据
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
DROP SCHEMA IF EXISTS cliproxy CASCADE;
CREATE SCHEMA cliproxy;
EOF

# 恢复数据
cat /tmp/$BACKUP_FILE | docker exec -i cliproxy-postgres pg_restore -U cliproxy -d cliproxy

# 验证
TOKEN_COUNT=$(docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c "SELECT COUNT(*) FROM cliproxy.auth_store;")
echo "恢复的令牌数量: $TOKEN_COUNT"

# 重启应用
docker compose start cli-proxy-api
ENDSSH

echo "✅ 迁移完成！"
```

---

## 维护计划

### 日常维护任务

创建 `/usr/local/bin/cliproxy-daily.sh`：

```bash
#!/bin/bash
# CLIProxyAPI 日常维护任务

LOG_FILE="/var/log/cliproxy-maintenance.log"
CONTAINER="cliproxy-postgres"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=== 开始日常维护 ==="

# 1. 健康检查
log "1. 健康检查..."
if docker exec $CONTAINER pg_isready -U cliproxy > /dev/null 2>&1; then
    log "✅ 数据库运行正常"
else
    log "❌ 数据库异常"
    exit 1
fi

# 2. 备份数据库
log "2. 执行数据库备份..."
/usr/local/bin/backup-cliproxy.sh >> "$LOG_FILE" 2>&1

# 3. 清理过期令牌
log "3. 清理过期令牌（90天未更新）..."
DELETED=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "DELETE FROM cliproxy.auth_store WHERE updated_at < NOW() - INTERVAL '90 days'; SELECT ROW_COUNT();" 2>&1)
log "清理了 $DELETED 个过期令牌"

# 4. 分析表
log "4. 更新统计信息..."
docker exec $CONTAINER psql -U cliproxy -d cliproxy -c "ANALYZE;" >> "$LOG_FILE" 2>&1

# 5. 检查数据库大小
DB_SIZE=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy'));")
log "数据库大小: $DB_SIZE"

# 6. 检查连接数
CONN_COUNT=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cliproxy';")
log "当前连接数: $CONN_COUNT"

# 7. 检查表膨胀
DEAD_TUPLES=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT SUM(n_dead_tup) FROM pg_stat_user_tables WHERE schemaname = 'cliproxy';")
log "死元组数量: $DEAD_TUPLES"

if [ "$DEAD_TUPLES" -gt 10000 ]; then
    log "⚠️ 死元组过多，建议执行 VACUUM"
fi

log "=== 日常维护完成 ==="
log ""
```

### 周度维护任务

创建 `/usr/local/bin/cliproxy-weekly.sh`：

```bash
#!/bin/bash
# CLIProxyAPI 周度维护任务

LOG_FILE="/var/log/cliproxy-maintenance.log"
CONTAINER="cliproxy-postgres"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=== 开始周度维护 ==="

# 1. VACUUM 清理
log "1. 执行 VACUUM 清理..."
docker exec $CONTAINER psql -U cliproxy -d cliproxy << EOF >> "$LOG_FILE" 2>&1
VACUUM ANALYZE cliproxy.config_store;
VACUUM ANALYZE cliproxy.auth_store;
EOF
log "✅ VACUUM 完成"

# 2. 检查表膨胀情况
log "2. 检查表膨胀情况:"
docker exec $CONTAINER psql -U cliproxy -d cliproxy << EOF >> "$LOG_FILE" 2>&1
SELECT
    tablename,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as bloat_percent
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY n_dead_tup DESC;
EOF

# 3. 检查未使用的索引
log "3. 检查未使用的索引:"
docker exec $CONTAINER psql -U cliproxy -d cliproxy << EOF >> "$LOG_FILE" 2>&1
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
    idx_scan as scans
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
  AND idx_scan = 0
  AND indexrelname NOT LIKE '%_pkey'
ORDER BY pg_relation_size(indexrelid) DESC;
EOF

# 4. 性能统计
log "4. 性能统计:"
docker exec $CONTAINER psql -U cliproxy -d cliproxy << EOF >> "$LOG_FILE" 2>&1
SELECT
    'Cache Hit Ratio' as metric,
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) || '%' as value
FROM pg_stat_database
WHERE datname = 'cliproxy';
EOF

# 5. 清理审计日志（如果启用）
log "5. 清理旧审计日志（超过 90 天）..."
AUDIT_DELETED=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "DELETE FROM cliproxy.audit_log WHERE timestamp < NOW() - INTERVAL '90 days' RETURNING 1;" 2>&1 | wc -l)
log "清理了 $AUDIT_DELETED 条审计日志"

log "=== 周度维护完成 ==="
log ""
```

### 月度维护任务

创建 `/usr/local/bin/cliproxy-monthly.sh`：

```bash
#!/bin/bash
# CLIProxyAPI 月度维护任务

LOG_FILE="/var/log/cliproxy-maintenance.log"
CONTAINER="cliproxy-postgres"
REPORT_FILE="/var/log/cliproxy-monthly-report-$(date +%Y%m).txt"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=== 开始月度维护 ==="

# 1. 生成月度报告
log "1. 生成月度报告..."
cat > "$REPORT_FILE" << EOF
CLIProxyAPI 月度维护报告
生成时间: $(date)

=== 数据库统计 ===
EOF

docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> "$REPORT_FILE"
-- 数据库大小
SELECT '数据库大小' as 指标, pg_size_pretty(pg_database_size('cliproxy')) as 值
UNION ALL
-- 表数量和大小
SELECT
    '表: ' || tablename,
    pg_size_pretty(pg_total_relation_size('cliproxy.' || tablename))
FROM pg_tables
WHERE schemaname = 'cliproxy'
UNION ALL
-- 令牌统计
SELECT '令牌总数', COUNT(*)::text FROM cliproxy.auth_store
UNION ALL
SELECT '配置数量', COUNT(*)::text FROM cliproxy.config_store;

-- 按提供商统计令牌
\echo ''
\echo '=== 令牌分布 ==='
SELECT
    content->>'type' as 提供商,
    COUNT(*) as 数量,
    MIN(created_at)::date as 最早,
    MAX(updated_at)::date as 最新
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY 数量 DESC;

-- 性能指标
\echo ''
\echo '=== 性能指标 ==='
SELECT
    '缓存命中率' as 指标,
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) || '%' as 值
FROM pg_stat_database
WHERE datname = 'cliproxy'
UNION ALL
SELECT
    '连接数',
    COUNT(*)::text
FROM pg_stat_activity
WHERE datname = 'cliproxy';
SQL

log "✅ 报告已生成: $REPORT_FILE"

# 2. 重建索引
log "2. 重建索引..."
docker exec $CONTAINER psql -U cliproxy -d cliproxy -c \
    "REINDEX SCHEMA cliproxy;" >> "$LOG_FILE" 2>&1
log "✅ 索引重建完成"

# 3. 完整备份
log "3. 执行完整备份..."
BACKUP_DIR="/var/backups/cliproxy/monthly"
mkdir -p "$BACKUP_DIR"
BACKUP_FILE="$BACKUP_DIR/monthly_backup_$(date +%Y%m).dump"
docker exec $CONTAINER pg_dump -U cliproxy -Fc cliproxy > "$BACKUP_FILE"
log "✅ 完整备份: $BACKUP_FILE"

# 4. 清理旧备份（保留 12 个月）
log "4. 清理旧备份（保留 12 个月）..."
find "$BACKUP_DIR" -name "monthly_backup_*.dump" -mtime +365 -delete
log "✅ 旧备份已清理"

# 5. 检查长期未更新的令牌
log "5. 检查长期未更新的令牌（180天）..."
OLD_TOKENS=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT COUNT(*) FROM cliproxy.auth_store WHERE updated_at < NOW() - INTERVAL '180 days';")
log "发现 $OLD_TOKENS 个长期未更新的令牌"

if [ "$OLD_TOKENS" -gt 0 ]; then
    log "⚠️ 建议检查这些令牌是否仍在使用"
fi

log "=== 月度维护完成 ==="
log "月度报告: $REPORT_FILE"
log ""

# 发送报告邮件（可选）
# mail -s "CLIProxyAPI 月度报告" admin@example.com < "$REPORT_FILE"
```

### 配置 Cron 定时任务

```bash
# 编辑 crontab
crontab -e

# 添加以下内容
# CLIProxyAPI 维护任务

# 每天凌晨 2 点 - 日常维护
0 2 * * * /usr/local/bin/cliproxy-daily.sh

# 每周日凌晨 3 点 - 周度维护
0 3 * * 0 /usr/local/bin/cliproxy-weekly.sh

# 每月 1 号凌晨 4 点 - 月度维护
0 4 1 * * /usr/local/bin/cliproxy-monthly.sh

# 每 5 分钟 - 监控检查（可选）
*/5 * * * * /usr/local/bin/cliproxy-monitoring.sh
```

### 维护任务权限设置

```bash
# 创建维护脚本目录
sudo mkdir -p /usr/local/bin

# 设置脚本权限
sudo chmod +x /usr/local/bin/cliproxy-*.sh

# 创建日志目录
sudo mkdir -p /var/log
sudo touch /var/log/cliproxy-maintenance.log
sudo chmod 644 /var/log/cliproxy-maintenance.log

# 创建备份目录
sudo mkdir -p /var/backups/cliproxy/{daily,weekly,monthly}
sudo chmod 700 /var/backups/cliproxy
```

---

## 附录

### 常用命令速查表

```bash
# === 连接相关 ===
# 连接数据库
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy

# 非交互式查询
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SELECT version();"

# === 备份恢复 ===
# 备份
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy > backup.sql

# 恢复
cat backup.sql | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# === 维护操作 ===
# 分析表
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "ANALYZE;"

# VACUUM
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "VACUUM ANALYZE;"

# === 监控查询 ===
# 查看连接数
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = 'cliproxy';"

# 查看数据库大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy'));"

# 查看表大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT tablename, pg_size_pretty(pg_total_relation_size('cliproxy.'||tablename)) FROM pg_tables WHERE schemaname = 'cliproxy';"
```

### 有用的 SQL 片段

```sql
-- 查看所有表的记录数
SELECT
    schemaname,
    tablename,
    n_live_tup
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY n_live_tup DESC;

-- 查看最近的数据库活动
SELECT
    datname,
    usename,
    application_name,
    client_addr,
    state,
    query_start
FROM pg_stat_activity
ORDER BY query_start DESC
LIMIT 10;

-- 查找包含特定文本的令牌
SELECT
    id,
    content
FROM cliproxy.auth_store
WHERE content::text ILIKE '%search_text%';

-- 批量更新令牌的某个字段
UPDATE cliproxy.auth_store
SET content = jsonb_set(content, '{updated_field}', '"new_value"')
WHERE content->>'type' = 'provider_name';
```

### 资源链接

- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [pgx - Go PostgreSQL 驱动](https://github.com/jackc/pgx)
- [Docker Compose 文档](https://docs.docker.com/compose/)
- [PostgreSQL Token Store](postgresql.md)
- [Docker 部署指南](docker-postgres-deployment.md)
- [快速开始](../POSTGRES_QUICKSTART.md)

---

**最后更新**: 2026-02-07
