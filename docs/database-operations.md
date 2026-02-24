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

#### 2. 从宿主机连接（如果暴露了端口）

```bash
# 使用 psql 客户端
psql -h localhost -p 5432 -U cliproxy -d cliproxy

# 使用连接字符串
psql "postgresql://cliproxy:password@localhost:5432/cliproxy"
```

#### 3. 使用其他数据库工具

```bash
# DBeaver
# 连接类型: PostgreSQL
# 主机: localhost
# 端口: 5432
# 数据库: cliproxy
# 用户: cliproxy

# pgAdmin
# 添加服务器，填写相同的连接信息
```

### 环境变量说明

```bash
# 从 compose.yml 或 .env 文件中获取
POSTGRES_DB=cliproxy               # 数据库名
POSTGRES_USER=cliproxy             # 用户名
POSTGRES_PASSWORD=changeme         # 密码（请修改为强密码）
PGSTORE_SCHEMA=cliproxy            # Schema 名称
```

### 常用 psql 命令

```sql
-- 列出所有数据库
\l

-- 切换到指定数据库
\c cliproxy

-- 列出当前数据库的所有表
\dt

-- 列出指定 schema 的表
\dt cliproxy.*

-- 查看表结构
\d cliproxy.config_store
\d cliproxy.auth_store

-- 查看索引
\di cliproxy.*

-- 查看表大小
\dt+ cliproxy.*

-- 查看当前连接信息
\conninfo

-- 退出 psql
\q

-- 显示查询执行时间
\timing on

-- 设置显示格式
\x auto                  -- 自动扩展显示
\pset pager off          -- 关闭分页
```

---

## 数据库结构

### Schema 结构

```sql
-- 查看所有 schema
SELECT schema_name
FROM information_schema.schemata
ORDER BY schema_name;

-- 查看 cliproxy schema 下的对象
SELECT
    table_name,
    table_type
FROM information_schema.tables
WHERE table_schema = 'cliproxy'
ORDER BY table_name;
```

### 配置表 (config_store)

存储系统配置文件（config.yaml 的内容）。

```sql
-- 表结构
CREATE TABLE cliproxy.config_store (
    id TEXT PRIMARY KEY,              -- 配置标识（默认为 "config"）
    content TEXT NOT NULL,            -- YAML 配置内容（纯文本）
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- 创建时间
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()   -- 更新时间
);

-- 查看表结构
\d cliproxy.config_store

-- 查询配置
SELECT id, created_at, updated_at, LENGTH(content) as content_size
FROM cliproxy.config_store;

-- 查看完整配置内容
SELECT content FROM cliproxy.config_store WHERE id = 'config';
```

### 认证令牌表 (auth_store)

存储各个提供商的 OAuth 令牌和认证凭证。

```sql
-- 表结构
CREATE TABLE cliproxy.auth_store (
    id TEXT PRIMARY KEY,              -- 令牌标识（通常是文件路径）
    content JSONB NOT NULL,           -- 令牌 JSON 内容
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- 创建时间
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()   -- 更新时间
);

-- 查看表结构
\d cliproxy.auth_store

-- 统计令牌数量
SELECT COUNT(*) as total_tokens FROM cliproxy.auth_store;

-- 按提供商统计令牌
SELECT
    content->>'type' as provider,
    COUNT(*) as count
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY count DESC;
```

---

## 日常运维操作

### 查看数据库状态

```bash
# 检查数据库是否在线
docker exec cliproxy-postgres pg_isready -U cliproxy

# 查看数据库版本
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SELECT version();"

# 查看数据库大小
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy')) as database_size;"
```

### 查看连接信息

```sql
-- 查看当前所有连接
SELECT
    pid,
    usename,
    application_name,
    client_addr,
    backend_start,
    state,
    query
FROM pg_stat_activity
WHERE datname = 'cliproxy'
ORDER BY backend_start;

-- 统计连接数
SELECT
    state,
    COUNT(*) as count
FROM pg_stat_activity
WHERE datname = 'cliproxy'
GROUP BY state;

-- 查看最大连接数配置
SHOW max_connections;

-- 查看当前连接数占比
SELECT
    COUNT(*) as current_connections,
    current_setting('max_connections')::int as max_connections,
    ROUND(100.0 * COUNT(*) / current_setting('max_connections')::int, 2) as usage_percent
FROM pg_stat_activity
WHERE datname = 'cliproxy';
```

### 查看表统计信息

```sql
-- 查看表记录数和大小
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) as index_size
FROM pg_tables
WHERE schemaname = 'cliproxy'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- 查看表的详细统计
SELECT
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    last_vacuum,
    last_autovacuum,
    last_analyze
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';
```

### 配置管理

```sql
-- 查看当前配置
SELECT content FROM cliproxy.config_store WHERE id = 'config';

-- 查看配置更新历史（需要启用审计）
SELECT id, created_at, updated_at
FROM cliproxy.config_store
ORDER BY updated_at DESC;

-- 更新配置（不推荐直接修改，建议通过管理 API）
-- 警告：直接修改需要重启服务才能生效
UPDATE cliproxy.config_store
SET content = '你的新配置内容',
    updated_at = NOW()
WHERE id = 'config';

-- 验证配置是否为有效 YAML（在应用层验证）
-- 建议先备份再修改
```

### 令牌管理

```sql
-- 查看所有令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    created_at,
    updated_at
FROM cliproxy.auth_store
ORDER BY updated_at DESC;

-- 查看特定提供商的令牌
SELECT
    id,
    content->>'email' as email,
    content->>'expires_at' as expires_at,
    updated_at
FROM cliproxy.auth_store
WHERE content->>'type' = 'claude'
ORDER BY updated_at DESC;

-- 查看即将过期的令牌（假设有 expires_at 字段）
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    (content->>'expires_at')::timestamp as expires_at
FROM cliproxy.auth_store
WHERE (content->>'expires_at')::timestamp < NOW() + INTERVAL '7 days'
ORDER BY (content->>'expires_at')::timestamp;

-- 删除特定令牌
DELETE FROM cliproxy.auth_store WHERE id = 'path/to/token.json';

-- 删除所有 Claude 令牌（危险操作，请谨慎）
DELETE FROM cliproxy.auth_store WHERE content->>'type' = 'claude';
```

---

## 数据备份与恢复

### 备份策略

#### 1. 完整数据库备份（推荐）

```bash
# 基本备份
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy > backup_$(date +%Y%m%d_%H%M%S).sql

# 压缩备份（节省空间）
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz

# 自定义格式备份（支持并行恢复）
docker exec cliproxy-postgres pg_dump -U cliproxy -Fc cliproxy > backup_$(date +%Y%m%d_%H%M%S).dump

# 仅备份特定 schema
docker exec cliproxy-postgres pg_dump -U cliproxy -n cliproxy cliproxy > backup_schema_$(date +%Y%m%d).sql
```

#### 2. 仅备份数据表

```bash
# 备份配置表
docker exec cliproxy-postgres pg_dump -U cliproxy -t cliproxy.config_store cliproxy > config_backup.sql

# 备份令牌表
docker exec cliproxy-postgres pg_dump -U cliproxy -t cliproxy.auth_store cliproxy > auth_backup.sql

# 备份所有表
docker exec cliproxy-postgres pg_dump -U cliproxy -t cliproxy.config_store -t cliproxy.auth_store cliproxy > tables_backup.sql
```

#### 3. 导出为 CSV 格式

```bash
# 导出配置（CSV）
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "COPY cliproxy.config_store TO STDOUT CSV HEADER" > config_backup.csv

# 导出令牌列表
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "COPY (SELECT id, content->>'type' as type, content->>'email' as email, created_at, updated_at FROM cliproxy.auth_store) TO STDOUT CSV HEADER" \
    > auth_list.csv
```

#### 4. 物理备份（Docker 卷）

```bash
# 停止容器（确保数据一致性）
docker compose -f compose.yml stop cli-proxy-api postgres

# 备份 Docker 卷
docker run --rm \
    -v cliproxy_postgres_data:/data \
    -v $(pwd):/backup \
    alpine tar czf /backup/postgres_volume_$(date +%Y%m%d).tar.gz /data

# 重新启动容器
docker compose -f compose.yml start postgres cli-proxy-api
```

### 恢复操作

#### 1. 从 SQL 备份恢复

```bash
# 恢复完整数据库
cat backup_20260207.sql | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 恢复压缩备份
gunzip -c backup_20260207.sql.gz | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 恢复自定义格式备份
docker exec -i cliproxy-postgres pg_restore -U cliproxy -d cliproxy < backup_20260207.dump

# 恢复特定表
cat config_backup.sql | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy
```

#### 2. 从 Docker 卷恢复

```bash
# 停止容器
docker compose -f compose.yml stop cli-proxy-api postgres

# 删除现有数据卷
docker volume rm cliproxy_postgres_data

# 创建新卷
docker volume create cliproxy_postgres_data

# 恢复数据
docker run --rm \
    -v cliproxy_postgres_data:/data \
    -v $(pwd):/backup \
    alpine tar xzf /backup/postgres_volume_20260207.tar.gz -C /

# 重新启动
docker compose -f compose.yml start postgres cli-proxy-api
```

#### 3. 选择性恢复

```bash
# 仅恢复特定表（会覆盖现有数据）
docker exec -i cliproxy-postgres psql -U cliproxy cliproxy << EOF
BEGIN;
TRUNCATE cliproxy.config_store CASCADE;
\copy cliproxy.config_store FROM '/path/to/config_backup.csv' CSV HEADER;
COMMIT;
EOF
```

### 自动备份脚本

创建自动备份脚本 `/usr/local/bin/backup-cliproxy-db.sh`：

```bash
#!/bin/bash
# CLIProxyAPI PostgreSQL 自动备份脚本

BACKUP_DIR="/var/backups/cliproxy"
CONTAINER_NAME="cliproxy-postgres"
DB_USER="cliproxy"
DB_NAME="cliproxy"
RETENTION_DAYS=30

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 备份文件名
BACKUP_FILE="$BACKUP_DIR/cliproxy_$(date +%Y%m%d_%H%M%S).sql.gz"

# 执行备份
docker exec "$CONTAINER_NAME" pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE"

# 检查备份是否成功
if [ $? -eq 0 ]; then
    echo "✅ 备份成功: $BACKUP_FILE"
    # 删除旧备份
    find "$BACKUP_DIR" -name "cliproxy_*.sql.gz" -mtime +$RETENTION_DAYS -delete
    echo "🗑️ 已清理 $RETENTION_DAYS 天前的旧备份"
else
    echo "❌ 备份失败"
    exit 1
fi
```

添加到 crontab：

```bash
# 每天凌晨 2 点自动备份
0 2 * * * /usr/local/bin/backup-cliproxy-db.sh >> /var/log/cliproxy-backup.log 2>&1
```

---

## 数据查询与管理

### 配置查询

```sql
-- 查看配置是否存在
SELECT EXISTS(SELECT 1 FROM cliproxy.config_store WHERE id = 'config');

-- 查看配置大小
SELECT
    id,
    pg_size_pretty(LENGTH(content)) as size,
    updated_at
FROM cliproxy.config_store;

-- 搜索配置中的关键字（示例：查找是否配置了 Claude API）
SELECT
    id,
    POSITION('claude-api-key' IN content) > 0 as has_claude_key
FROM cliproxy.config_store;

-- 提取配置的前几行（预览）
SELECT
    id,
    SUBSTRING(content FROM 1 FOR 200) as preview,
    updated_at
FROM cliproxy.config_store;
```

### 令牌查询

```sql
-- 查看令牌数量统计
SELECT
    content->>'type' as provider,
    COUNT(*) as count,
    MIN(created_at) as earliest,
    MAX(updated_at) as latest
FROM cliproxy.auth_store
GROUP BY content->>'type'
ORDER BY count DESC;

-- 查看最近更新的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    updated_at,
    NOW() - updated_at as age
FROM cliproxy.auth_store
ORDER BY updated_at DESC
LIMIT 10;

-- 查看超过 N 天未更新的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    updated_at,
    NOW() - updated_at as age
FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '90 days'
ORDER BY updated_at;

-- 搜索包含特定 email 的令牌
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    updated_at
FROM cliproxy.auth_store
WHERE content->>'email' LIKE '%example.com%';

-- 查看令牌的完整内容（美化输出）
SELECT
    id,
    jsonb_pretty(content) as token_content
FROM cliproxy.auth_store
WHERE id = 'specific_token_id';

-- 检查令牌中的特定字段
SELECT
    id,
    content->>'type' as provider,
    content->>'email' as email,
    content->'access_token' IS NOT NULL as has_access_token,
    content->'refresh_token' IS NOT NULL as has_refresh_token,
    updated_at
FROM cliproxy.auth_store;
```

### 数据清理

```sql
-- 删除测试令牌（示例：ID 包含 'test' 的令牌）
DELETE FROM cliproxy.auth_store
WHERE id LIKE '%test%';

-- 删除超过 180 天未更新的令牌（归档前先备份）
-- 先查看要删除的数据
SELECT id, content->>'type' as provider, updated_at
FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '180 days';

-- 确认后执行删除
DELETE FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '180 days';

-- 清理特定提供商的所有令牌
DELETE FROM cliproxy.auth_store
WHERE content->>'type' = 'provider_name';

-- 清空所有令牌（危险操作！）
TRUNCATE TABLE cliproxy.auth_store;
```

---

## 性能监控与优化

### 性能指标监控

```sql
-- 查看数据库性能统计
SELECT
    datname as database,
    numbackends as connections,
    xact_commit as commits,
    xact_rollback as rollbacks,
    blks_read as disk_reads,
    blks_hit as cache_hits,
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) as cache_hit_ratio
FROM pg_stat_database
WHERE datname = 'cliproxy';

-- 查看表访问统计
SELECT
    schemaname,
    tablename,
    seq_scan as sequential_scans,
    seq_tup_read as seq_rows_read,
    idx_scan as index_scans,
    idx_tup_fetch as idx_rows_fetched,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';

-- 查看慢查询（需要启用 pg_stat_statements 扩展）
-- CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SELECT
    query,
    calls,
    total_time,
    mean_time,
    max_time
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat_statements%'
ORDER BY mean_time DESC
LIMIT 10;

-- 查看锁等待
SELECT
    pid,
    usename,
    pg_blocking_pids(pid) as blocked_by,
    query as blocked_query
FROM pg_stat_activity
WHERE cardinality(pg_blocking_pids(pid)) > 0;
```

### 索引管理

```sql
-- 查看现有索引
SELECT
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE schemaname = 'cliproxy'
ORDER BY tablename, indexname;

-- 查看索引使用情况
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
ORDER BY idx_scan;

-- 查看未使用的索引（可能可以删除）
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
  AND idx_scan = 0
  AND indexrelname NOT LIKE '%_pkey'  -- 保留主键
ORDER BY pg_relation_size(indexrelid) DESC;
```

### 推荐索引

```sql
-- 为 updated_at 添加索引（提升按时间查询的性能）
CREATE INDEX IF NOT EXISTS idx_auth_store_updated_at
ON cliproxy.auth_store(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_config_store_updated_at
ON cliproxy.config_store(updated_at DESC);

-- 为 JSONB content 添加 GIN 索引（支持 JSON 查询）
CREATE INDEX IF NOT EXISTS idx_auth_store_content
ON cliproxy.auth_store USING GIN (content);

-- 为特定 JSON 字段添加表达式索引
CREATE INDEX IF NOT EXISTS idx_auth_store_type
ON cliproxy.auth_store((content->>'type'));

CREATE INDEX IF NOT EXISTS idx_auth_store_email
ON cliproxy.auth_store((content->>'email'));

-- 查看索引创建进度
SELECT
    pid,
    phase,
    tuples_done,
    tuples_total,
    ROUND(100.0 * tuples_done / NULLIF(tuples_total, 0), 2) as progress_percent
FROM pg_stat_progress_create_index;
```

### 维护操作

```sql
-- 分析表（更新统计信息）
ANALYZE cliproxy.config_store;
ANALYZE cliproxy.auth_store;

-- 分析整个 schema
ANALYZE;

-- 清理表（回收空间，不锁表）
VACUUM cliproxy.config_store;
VACUUM cliproxy.auth_store;

-- 完全清理（回收更多空间，会锁表）
VACUUM FULL cliproxy.config_store;
VACUUM FULL cliproxy.auth_store;

-- 清理并分析（推荐）
VACUUM ANALYZE cliproxy.config_store;
VACUUM ANALYZE cliproxy.auth_store;

-- 重建索引（解决索引膨胀）
REINDEX TABLE cliproxy.config_store;
REINDEX TABLE cliproxy.auth_store;

-- 重建整个 schema 的索引
REINDEX SCHEMA cliproxy;
```

### 配置优化

```bash
# 查看当前配置
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SHOW ALL;"

# 查看关键配置
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SHOW shared_buffers;
SHOW work_mem;
SHOW maintenance_work_mem;
SHOW effective_cache_size;
SHOW max_connections;
EOF
```

在 `compose.yml` 中优化 PostgreSQL 配置：

```yaml
postgres:
  environment:
    # 增加共享缓冲区（推荐设置为系统内存的 25%）
    POSTGRES_SHARED_BUFFERS: 512MB

    # 增加最大连接数
    POSTGRES_MAX_CONNECTIONS: 200

    # 增加工作内存
    POSTGRES_WORK_MEM: 16MB

    # 增加维护工作内存
    POSTGRES_MAINTENANCE_WORK_MEM: 128MB
```

---

## 故障排查

### 连接问题

```bash
# 检查容器是否运行
docker ps | grep cliproxy-postgres

# 检查容器日志
docker logs cliproxy-postgres --tail=100

# 检查端口是否监听
docker exec cliproxy-postgres netstat -tlnp | grep 5432

# 测试网络连接
docker exec cli-proxy-api nc -zv postgres 5432

# 测试数据库连接
docker exec cli-proxy-api psql "postgresql://cliproxy:password@postgres:5432/cliproxy" -c "SELECT 1;"
```

### 性能问题

```sql
-- 查看长时间运行的查询
SELECT
    pid,
    now() - query_start as duration,
    state,
    query
FROM pg_stat_activity
WHERE state != 'idle'
  AND now() - query_start > interval '5 seconds'
ORDER BY duration DESC;

-- 终止长时间运行的查询
SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE pid = 12345;

-- 强制终止（慎用）
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid = 12345;

-- 查看表膨胀
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    n_dead_tup as dead_tuples,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as dead_percent
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
  AND n_dead_tup > 0
ORDER BY dead_percent DESC;
```

### 数据一致性问题

```sql
-- 检查表完整性
SELECT
    schemaname,
    tablename,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';

-- 验证主键约束
SELECT
    conname as constraint_name,
    contype as constraint_type,
    pg_get_constraintdef(oid) as definition
FROM pg_constraint
WHERE conrelid = 'cliproxy.auth_store'::regclass;

-- 检查重复数据（理论上不应该存在，因为有主键）
SELECT id, COUNT(*)
FROM cliproxy.auth_store
GROUP BY id
HAVING COUNT(*) > 1;

-- 验证 JSONB 数据格式
SELECT
    id,
    jsonb_typeof(content) as json_type,
    content IS NOT NULL as has_content
FROM cliproxy.auth_store
WHERE jsonb_typeof(content) != 'object'
   OR content IS NULL;
```

### 空间不足

```bash
# 查看磁盘使用
docker exec cliproxy-postgres df -h

# 查看数据库占用空间
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy'));"

# 查看各表占用空间
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy << EOF
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as indexes_size
FROM pg_tables
WHERE schemaname = 'cliproxy'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
EOF

# 清理方案
# 1. 执行 VACUUM FULL（需要锁表，谨慎使用）
# 2. 删除旧数据
# 3. 清理 PostgreSQL 日志
# 4. 扩展 Docker 卷大小
```

---

## 安全管理

### 用户权限管理

```sql
-- 查看当前用户权限
\du

-- 创建只读用户
CREATE USER readonly_user WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE cliproxy TO readonly_user;
GRANT USAGE ON SCHEMA cliproxy TO readonly_user;
GRANT SELECT ON ALL TABLES IN SCHEMA cliproxy TO readonly_user;

-- 创建管理用户（可读写）
CREATE USER admin_user WITH PASSWORD 'admin_password';
GRANT CONNECT ON DATABASE cliproxy TO admin_user;
GRANT USAGE ON SCHEMA cliproxy TO admin_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA cliproxy TO admin_user;

-- 撤销权限
REVOKE ALL ON DATABASE cliproxy FROM some_user;

-- 修改用户密码
ALTER USER cliproxy WITH PASSWORD 'new_secure_password';
```

### 审计日志

```sql
-- 启用审计日志（需要 pgaudit 扩展）
-- CREATE EXTENSION IF NOT EXISTS pgaudit;

-- 查看连接历史
SELECT
    usename,
    application_name,
    client_addr,
    backend_start,
    state_change
FROM pg_stat_activity
ORDER BY backend_start DESC;

-- 自定义审计表
CREATE TABLE IF NOT EXISTS cliproxy.audit_log (
    id SERIAL PRIMARY KEY,
    action TEXT NOT NULL,
    table_name TEXT,
    user_name TEXT,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    old_data JSONB,
    new_data JSONB
);

-- 创建触发器记录变更（示例）
CREATE OR REPLACE FUNCTION cliproxy.audit_trigger_func()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'DELETE') THEN
        INSERT INTO cliproxy.audit_log (action, table_name, user_name, old_data)
        VALUES (TG_OP, TG_TABLE_NAME, current_user, row_to_json(OLD));
        RETURN OLD;
    ELSIF (TG_OP = 'UPDATE') THEN
        INSERT INTO cliproxy.audit_log (action, table_name, user_name, old_data, new_data)
        VALUES (TG_OP, TG_TABLE_NAME, current_user, row_to_json(OLD), row_to_json(NEW));
        RETURN NEW;
    ELSIF (TG_OP = 'INSERT') THEN
        INSERT INTO cliproxy.audit_log (action, table_name, user_name, new_data)
        VALUES (TG_OP, TG_TABLE_NAME, current_user, row_to_json(NEW));
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- 应用触发器到表
CREATE TRIGGER auth_store_audit_trigger
AFTER INSERT OR UPDATE OR DELETE ON cliproxy.auth_store
FOR EACH ROW EXECUTE FUNCTION cliproxy.audit_trigger_func();
```

### SSL/TLS 配置

在生产环境中启用 SSL 连接：

```bash
# 修改 DSN 使用 SSL
PGSTORE_DSN="postgresql://cliproxy:password@postgres:5432/cliproxy?sslmode=require"

# 生成自签名证书（测试用）
docker exec cliproxy-postgres openssl req -new -x509 -days 365 -nodes -text \
    -out /var/lib/postgresql/server.crt \
    -keyout /var/lib/postgresql/server.key \
    -subj "/CN=cliproxy-postgres"

# 修改 PostgreSQL 配置启用 SSL
docker exec cliproxy-postgres sh -c "echo 'ssl = on' >> /var/lib/postgresql/data/postgresql.conf"
docker compose restart postgres
```

### 数据加密

```sql
-- 使用 pgcrypto 扩展加密敏感数据
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 加密字段（示例）
SELECT pgp_sym_encrypt('sensitive_data', 'encryption_key');

-- 解密字段
SELECT pgp_sym_decrypt(encrypted_column::bytea, 'encryption_key') FROM table_name;
```

---

## 数据迁移

### 从文件存储迁移到 PostgreSQL

```bash
# 1. 备份现有文件令牌
cp -r ~/.cli-proxy-api ~/cliproxy-backup
# 或
cp -r /path/to/auths ~/cliproxy-backup

# 2. 启动 PostgreSQL 环境
export PGSTORE_DSN="postgresql://cliproxy:password@localhost:5432/cliproxy"
export PGSTORE_SCHEMA="cliproxy"

# 3. 将文件复制到本地缓存目录
# PostgreSQL 启动时会自动同步到数据库
cp -r ~/cliproxy-backup/* /var/lib/cliproxy/pgstore/auths/

# 4. 重启服务，自动同步
docker compose restart cli-proxy-api

# 5. 验证迁移
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c \
    "SELECT COUNT(*) FROM cliproxy.auth_store;"
```

### 从 PostgreSQL 导出到文件

```bash
# 导出配置到文件
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -c \
    "SELECT content FROM cliproxy.config_store WHERE id = 'config';" \
    > exported_config.yaml

# 导出所有令牌到 JSON 文件
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -t -A -c \
    "SELECT json_build_object('id', id, 'content', content) FROM cliproxy.auth_store;" \
    > exported_tokens.json
```

### 数据库迁移（跨服务器）

```bash
# 方法 1: 使用 pg_dump 和 pg_restore
# 在源服务器
pg_dump -U cliproxy -h source-host cliproxy -Fc -f cliproxy_export.dump

# 传输到目标服务器
scp cliproxy_export.dump target-server:/tmp/

# 在目标服务器
pg_restore -U cliproxy -h target-host -d cliproxy /tmp/cliproxy_export.dump

# 方法 2: 使用管道直接传输
pg_dump -U cliproxy -h source-host cliproxy | psql -U cliproxy -h target-host cliproxy

# 方法 3: 使用 Docker 卷迁移
# 备份源卷
docker run --rm -v source_postgres_data:/data -v $(pwd):/backup alpine \
    tar czf /backup/postgres_migration.tar.gz /data

# 在目标机器恢复
docker run --rm -v target_postgres_data:/data -v $(pwd):/backup alpine \
    tar xzf /backup/postgres_migration.tar.gz -C /
```

---

## 维护计划

### 日常维护任务

```bash
# 每日任务脚本
cat > /usr/local/bin/cliproxy-daily-maintenance.sh << 'EOF'
#!/bin/bash
# CLIProxyAPI 日常维护脚本

CONTAINER="cliproxy-postgres"
LOG_FILE="/var/log/cliproxy-maintenance.log"

echo "=== $(date) - 开始日常维护 ===" >> $LOG_FILE

# 1. 备份数据库
echo "执行数据库备份..." >> $LOG_FILE
/usr/local/bin/backup-cliproxy-db.sh >> $LOG_FILE 2>&1

# 2. 清理过期令牌（超过 90 天未更新）
echo "清理过期令牌..." >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
DELETE FROM cliproxy.auth_store
WHERE updated_at < NOW() - INTERVAL '90 days';
SQL

# 3. 分析表
echo "分析数据库表..." >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy -c "ANALYZE;" >> $LOG_FILE 2>&1

# 4. 检查数据库大小
echo "数据库大小:" >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT pg_size_pretty(pg_database_size('cliproxy'));" >> $LOG_FILE 2>&1

# 5. 检查连接数
echo "当前连接数:" >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cliproxy';" >> $LOG_FILE 2>&1

echo "=== 日常维护完成 ===" >> $LOG_FILE
echo "" >> $LOG_FILE
EOF

chmod +x /usr/local/bin/cliproxy-daily-maintenance.sh

# 添加到 crontab（每天凌晨 3 点）
echo "0 3 * * * /usr/local/bin/cliproxy-daily-maintenance.sh" | crontab -
```

### 周度维护任务

```bash
# 每周任务脚本
cat > /usr/local/bin/cliproxy-weekly-maintenance.sh << 'EOF'
#!/bin/bash
# CLIProxyAPI 周度维护脚本

CONTAINER="cliproxy-postgres"
LOG_FILE="/var/log/cliproxy-maintenance.log"

echo "=== $(date) - 开始周度维护 ===" >> $LOG_FILE

# 1. VACUUM 清理
echo "执行 VACUUM 清理..." >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
VACUUM ANALYZE cliproxy.config_store;
VACUUM ANALYZE cliproxy.auth_store;
SQL

# 2. 检查表膨胀
echo "检查表膨胀情况:" >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
SELECT
    schemaname,
    tablename,
    n_dead_tup as dead_tuples,
    ROUND(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as dead_percent
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy'
ORDER BY dead_percent DESC;
SQL

# 3. 检查未使用的索引
echo "检查未使用的索引:" >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes
WHERE schemaname = 'cliproxy'
  AND idx_scan = 0
  AND indexrelname NOT LIKE '%_pkey';
SQL

echo "=== 周度维护完成 ===" >> $LOG_FILE
echo "" >> $LOG_FILE
EOF

chmod +x /usr/local/bin/cliproxy-weekly-maintenance.sh

# 添加到 crontab（每周日凌晨 4 点）
echo "0 4 * * 0 /usr/local/bin/cliproxy-weekly-maintenance.sh" | crontab -
```

### 月度维护任务

```bash
# 每月任务脚本
cat > /usr/local/bin/cliproxy-monthly-maintenance.sh << 'EOF'
#!/bin/bash
# CLIProxyAPI 月度维护脚本

CONTAINER="cliproxy-postgres"
LOG_FILE="/var/log/cliproxy-maintenance.log"

echo "=== $(date) - 开始月度维护 ===" >> $LOG_FILE

# 1. 重建索引
echo "重建索引..." >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
REINDEX SCHEMA cliproxy;
SQL

# 2. 生成性能报告
echo "生成性能报告:" >> $LOG_FILE
docker exec $CONTAINER psql -U cliproxy -d cliproxy << SQL >> $LOG_FILE 2>&1
-- 表统计
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes
FROM pg_stat_user_tables
WHERE schemaname = 'cliproxy';

-- 缓存命中率
SELECT
    ROUND(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0), 2) as cache_hit_ratio
FROM pg_stat_database
WHERE datname = 'cliproxy';
SQL

echo "=== 月度维护完成 ===" >> $LOG_FILE
echo "" >> $LOG_FILE
EOF

chmod +x /usr/local/bin/cliproxy-monthly-maintenance.sh

# 添加到 crontab（每月 1 号凌晨 5 点）
echo "0 5 1 * * /usr/local/bin/cliproxy-monthly-maintenance.sh" | crontab -
```

### 监控告警

```bash
# 监控脚本
cat > /usr/local/bin/cliproxy-monitoring.sh << 'EOF'
#!/bin/bash
# CLIProxyAPI 监控脚本

CONTAINER="cliproxy-postgres"
ALERT_EMAIL="admin@example.com"

# 检查数据库连接
if ! docker exec $CONTAINER pg_isready -U cliproxy > /dev/null 2>&1; then
    echo "❌ 数据库连接失败" | mail -s "CLIProxyAPI 告警: 数据库不可用" $ALERT_EMAIL
    exit 1
fi

# 检查磁盘空间
DISK_USAGE=$(docker exec $CONTAINER df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 85 ]; then
    echo "⚠️ 磁盘使用率超过 85%: ${DISK_USAGE}%" | mail -s "CLIProxyAPI 告警: 磁盘空间不足" $ALERT_EMAIL
fi

# 检查连接数
CONN_COUNT=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cliproxy';")
MAX_CONN=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SHOW max_connections;")
CONN_PERCENT=$((100 * CONN_COUNT / MAX_CONN))
if [ $CONN_PERCENT -gt 80 ]; then
    echo "⚠️ 连接数超过 80%: ${CONN_COUNT}/${MAX_CONN}" | mail -s "CLIProxyAPI 告警: 连接数过高" $ALERT_EMAIL
fi

# 检查长时间运行的查询
LONG_QUERY=$(docker exec $CONTAINER psql -U cliproxy -d cliproxy -t -c \
    "SELECT count(*) FROM pg_stat_activity WHERE state != 'idle' AND now() - query_start > interval '5 minutes';")
if [ $LONG_QUERY -gt 0 ]; then
    echo "⚠️ 发现 ${LONG_QUERY} 个长时间运行的查询" | mail -s "CLIProxyAPI 告警: 慢查询" $ALERT_EMAIL
fi

echo "✅ 所有检查通过"
EOF

chmod +x /usr/local/bin/cliproxy-monitoring.sh

# 添加到 crontab（每 5 分钟检查一次）
echo "*/5 * * * * /usr/local/bin/cliproxy-monitoring.sh" | crontab -
```

---

## 参考资料

### 官方文档
- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [pgx - PostgreSQL Driver](https://github.com/jackc/pgx)
- [Docker Compose](https://docs.docker.com/compose/)

### 项目文档
- [PostgreSQL Token Store](postgresql.md)
- [Docker 部署指南](docker-postgres-deployment.md)
- [快速开始](../POSTGRES_QUICKSTART.md)

### 性能调优
- [PostgreSQL Tuning](https://wiki.postgresql.org/wiki/Tuning_Your_PostgreSQL_Server)
- [PgTune](https://pgtune.leopard.in.ua/) - 配置生成器

### 监控工具
- [pg_stat_statements](https://www.postgresql.org/docs/current/pgstatstatements.html)
- [pgBadger](https://pgbadger.darold.net/) - 日志分析工具
- [pgAdmin](https://www.pgadmin.org/) - 图形化管理工具

---

## 常见问题

### Q: 如何重置数据库密码？

```bash
# 方法 1: 通过环境变量（需要重建容器）
# 修改 .env 文件中的 POSTGRES_PASSWORD
# 然后重建容器
docker compose down
docker volume rm cliproxy_postgres_data  # 警告：会删除数据
docker compose up -d

# 方法 2: 在数据库内修改
docker exec -it cliproxy-postgres psql -U cliproxy -d cliproxy
ALTER USER cliproxy WITH PASSWORD 'new_password';
\q

# 然后更新应用的 DSN
```

### Q: 数据库日志在哪里？

```bash
# 查看容器日志
docker logs cliproxy-postgres

# 进入容器查看 PostgreSQL 日志
docker exec -it cliproxy-postgres sh
cat /var/lib/postgresql/data/log/postgresql-*.log

# 实时查看日志
docker logs -f cliproxy-postgres
```

### Q: 如何升级 PostgreSQL 版本？

```bash
# 1. 备份数据
./backup-cliproxy-db.sh

# 2. 停止服务
docker compose down

# 3. 修改 compose.yml 中的镜像版本
# image: postgres:17-alpine -> postgres:18-alpine

# 4. 启动新版本（可能需要数据迁移）
docker compose up -d

# 5. 验证版本
docker exec cliproxy-postgres psql -U cliproxy -d cliproxy -c "SELECT version();"
```

### Q: 数据库响应慢怎么办？

1. 检查慢查询
2. 分析表和索引
3. 执行 VACUUM
4. 增加共享缓冲区
5. 检查磁盘 I/O
6. 查看是否有锁等待

详见 [性能监控与优化](#性能监控与优化) 章节。

---

**提示**: 定期查看此文档并根据实际运维经验更新最佳实践。
