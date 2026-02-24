# Docker + PostgreSQL 部署指南

本指南介绍如何使用 Docker Compose 和 PostgreSQL 部署 CLIProxyAPI Plus。

## 快速开始

### 单实例部署（推荐入门）

```bash
# 1. 复制环境变量配置文件
cp .env.postgres.example .env

# 2. 修改 .env 文件，设置安全的数据库密码
vim .env  # 修改 POSTGRES_PASSWORD

# 3. 准备配置文件
cp config.example.yaml config.yaml
vim config.yaml  # 按需配置 API keys 等

# 4. 启动服务
docker compose -f docker-compose.postgres.yml up -d

# 5. 查看日志
docker compose -f docker-compose.postgres.yml logs -f

# 6. 检查服务状态
docker compose -f docker-compose.postgres.yml ps
```

服务启动后：
- CLIProxyAPI: http://localhost:8317
- OAuth 回调端口: 8085
- PostgreSQL: 仅内部访问（未暴露端口）

### 多实例部署（负载均衡）

```bash
# 1. 编辑 docker-compose.postgres.yml
vim docker-compose.postgres.yml

# 取消注释以下部分：
# - cli-proxy-api-2 服务
# - cli_proxy_cache_2 卷
# - nginx 服务（可选）

# 2. 如果使用 Nginx 负载均衡，确保 nginx.conf 已配置

# 3. 启动所有服务
docker compose -f docker-compose.postgres.yml up -d

# 4. 检查所有实例
docker compose -f docker-compose.postgres.yml ps
```

服务地址：
- Nginx 负载均衡器: http://localhost:80
- 实例 1 直连: http://localhost:8317
- 实例 2 直连: http://localhost:8318

## 配置说明

### 环境变量

在 `.env` 文件中配置：

```bash
# 数据库配置（必需）
POSTGRES_DB=cliproxy               # 数据库名称
POSTGRES_USER=cliproxy             # 数据库用户
POSTGRES_PASSWORD=your_password    # 数据库密码（务必修改！）

# Schema 配置
PGSTORE_SCHEMA=cliproxy            # PostgreSQL schema

# 服务端口
CLI_PROXY_PORT_1=8317              # 实例1 API端口
CLI_PROXY_PORT_2=8318              # 实例2 API端口
CLI_PROXY_OAUTH_PORT_1=8085        # 实例1 OAuth端口
CLI_PROXY_OAUTH_PORT_2=8086        # 实例2 OAuth端口

# Nginx 端口（负载均衡）
NGINX_PORT=80                      # Nginx 监听端口
```

### config.yaml 配置

确保 `config.yaml` 包含基本配置：

```yaml
# 服务端口（容器内端口）
port: 8317

# API 密钥
api-keys:
  - "your-api-key-1"
  - "your-api-key-2"

# 其他提供商配置（按需添加）
# claude-api-key: ...
# gemini-api-key: ...
# kiro: ...
```

**注意**：使用 PostgreSQL 后，`auth-dir` 配置将被忽略，认证文件存储在数据库中。

## 服务管理

### 启动服务

```bash
# 启动所有服务
docker compose -f docker-compose.postgres.yml up -d

# 仅启动特定服务
docker compose -f docker-compose.postgres.yml up -d postgres
docker compose -f docker-compose.postgres.yml up -d cli-proxy-api-1
```

### 停止服务

```bash
# 停止所有服务
docker compose -f docker-compose.postgres.yml down

# 停止并删除数据卷（警告：会删除所有数据！）
docker compose -f docker-compose.postgres.yml down -v
```

### 重启服务

```bash
# 重启所有服务
docker compose -f docker-compose.postgres.yml restart

# 重启特定服务
docker compose -f docker-compose.postgres.yml restart cli-proxy-api-1
```

### 查看日志

```bash
# 查看所有服务日志
docker compose -f docker-compose.postgres.yml logs -f

# 查看特定服务日志
docker compose -f docker-compose.postgres.yml logs -f cli-proxy-api-1
docker compose -f docker-compose.postgres.yml logs -f postgres

# 查看最近 100 行日志
docker compose -f docker-compose.postgres.yml logs --tail=100 cli-proxy-api-1
```

### 更新服务

```bash
# 拉取最新镜像
docker compose -f docker-compose.postgres.yml pull

# 重新创建容器（不影响数据卷）
docker compose -f docker-compose.postgres.yml up -d --force-recreate
```

## OAuth 登录

使用 PostgreSQL 后，OAuth 登录生成的令牌会自动保存到数据库，多个实例自动共享。

### 方法 1: 在容器中执行登录

```bash
# 进入容器
docker exec -it cli-proxy-api-1 /bin/sh

# 执行登录命令（例如 Claude）
./CLIProxyAPI -claude-login

# 按提示完成浏览器 OAuth 流程
```

### 方法 2: 使用本地二进制登录

如果你已经有本地安装的 CLIProxyAPI：

```bash
# 本地登录
./CLIProxyAPI -claude-login

# 将生成的令牌文件导入数据库
# 方法：复制令牌文件到容器，重启服务会自动同步
docker cp ~/.cli-proxy-api/claude_session_xxx.json cli-proxy-api-1:/var/lib/cliproxy/pgstore/auths/
docker compose -f docker-compose.postgres.yml restart cli-proxy-api-1
```

### 方法 3: Web OAuth（适用于 Kiro）

Kiro 支持 Web OAuth 界面：

```bash
# 访问 Web OAuth 页面
http://localhost:8317/v0/oauth/kiro

# 在浏览器中完成登录流程
# 令牌自动保存到 PostgreSQL
```

## 数据管理

### 备份数据

```bash
# 备份 PostgreSQL 数据库
docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy > backup.sql

# 备份配置文件
cp config.yaml config.backup.yaml

# 备份整个 Docker 卷（推荐）
docker run --rm -v cliproxyapiplus_postgres_data:/data -v $(pwd):/backup \
    alpine tar czf /backup/postgres_backup.tar.gz /data
```

### 恢复数据

```bash
# 恢复 PostgreSQL 数据库
cat backup.sql | docker exec -i cliproxy-postgres psql -U cliproxy cliproxy

# 恢复 Docker 卷
docker run --rm -v cliproxyapiplus_postgres_data:/data -v $(pwd):/backup \
    alpine tar xzf /backup/postgres_backup.tar.gz -C /
```

### 查看数据库内容

```bash
# 连接到 PostgreSQL
docker exec -it cliproxy-postgres psql -U cliproxy cliproxy

# 在 psql 中执行查询
\dt                              # 列出所有表
SELECT * FROM cliproxy.config_store;    # 查看配置
SELECT id, updated_at FROM cliproxy.auth_store;  # 查看令牌列表
\q                               # 退出
```

### 清理数据

```bash
# 清理未使用的 Docker 资源
docker system prune -a

# 清理特定服务的数据卷（警告：数据会丢失！）
docker compose -f docker-compose.postgres.yml down
docker volume rm cliproxyapiplus_postgres_data
docker volume rm cliproxyapiplus_cli_proxy_cache_1
```

## 监控和调试

### 健康检查

```bash
# 检查服务健康状态
docker compose -f docker-compose.postgres.yml ps

# 检查 API 是否正常
curl http://localhost:8317/v1/models

# 检查 PostgreSQL 连接
docker exec cliproxy-postgres pg_isready -U cliproxy
```

### 性能监控

```bash
# 查看容器资源使用
docker stats

# 查看 PostgreSQL 连接数
docker exec cliproxy-postgres psql -U cliproxy cliproxy \
    -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cliproxy';"

# 查看数据库大小
docker exec cliproxy-postgres psql -U cliproxy cliproxy \
    -c "SELECT pg_size_pretty(pg_database_size('cliproxy'));"
```

### 调试问题

```bash
# 查看详细日志
docker compose -f docker-compose.postgres.yml logs -f --tail=200

# 进入容器调试
docker exec -it cli-proxy-api-1 /bin/sh

# 检查环境变量
docker exec cli-proxy-api-1 env | grep PGSTORE

# 测试 PostgreSQL 连接
docker exec cli-proxy-api-1 nc -zv postgres 5432
```

## 负载均衡配置

### 使用 Nginx（推荐）

已提供 `nginx.conf` 配置文件，支持：
- 最少连接负载均衡
- 健康检查
- 请求限流
- WebSocket 支持
- 流式响应支持

启用步骤：
1. 在 `docker-compose.postgres.yml` 中取消注释 nginx 服务
2. 取消注释 `cli-proxy-api-2` 服务
3. 修改 `nginx.conf` 中的 `cli-proxy-api-2` 行（去掉注释）
4. 重启服务

### 使用外部负载均衡器

如果使用 HAProxy、Traefik 等外部负载均衡器：

```yaml
# 在 docker-compose.postgres.yml 中暴露更多实例
cli-proxy-api-3:
  # ... 复制 cli-proxy-api-1 的配置
  ports:
    - "8319:8317"
```

然后在负载均衡器中配置后端：
- http://host:8317
- http://host:8318
- http://host:8319

## 生产环境建议

### 安全性

1. **修改默认密码**
   ```bash
   # 使用强密码
   POSTGRES_PASSWORD=$(openssl rand -base64 32)
   ```

2. **不要暴露 PostgreSQL 端口**
   - 保持 `docker-compose.postgres.yml` 中的 PostgreSQL 端口注释
   - 只允许容器内部访问

3. **使用 SSL/TLS**
   ```bash
   # 修改 DSN 使用 SSL
   PGSTORE_DSN=postgresql://user:pass@postgres:5432/db?sslmode=require
   ```

4. **限制文件权限**
   ```bash
   chmod 600 .env
   chmod 600 config.yaml
   ```

### 高可用性

1. **使用多个实例**
   - 启用 `cli-proxy-api-2` 或更多实例
   - 配置 Nginx 负载均衡

2. **数据库备份**
   ```bash
   # 设置定时备份
   0 2 * * * docker exec cliproxy-postgres pg_dump -U cliproxy cliproxy > /backup/cliproxy_$(date +\%Y\%m\%d).sql
   ```

3. **监控和告警**
   - 使用 Prometheus + Grafana 监控
   - 配置健康检查告警

4. **日志管理**
   - 使用日志收集工具（ELK、Loki）
   - 定期清理旧日志

### 性能优化

1. **PostgreSQL 调优**
   ```bash
   # 在 .env 中调整
   POSTGRES_SHARED_BUFFERS=512MB
   POSTGRES_MAX_CONNECTIONS=200
   ```

2. **容器资源限制**
   ```yaml
   # 在 docker-compose.postgres.yml 中添加
   deploy:
     resources:
       limits:
         cpus: '2'
         memory: 2G
       reservations:
         cpus: '1'
         memory: 1G
   ```

3. **使用本地卷而非网络卷**
   - 本地 Docker 卷性能更好
   - 网络卷适用于分布式部署

## 故障排查

### 问题：容器无法启动

```bash
# 检查日志
docker compose -f docker-compose.postgres.yml logs

# 检查配置文件语法
docker compose -f docker-compose.postgres.yml config

# 检查端口占用
netstat -tuln | grep 8317
```

### 问题：无法连接 PostgreSQL

```bash
# 检查 PostgreSQL 是否运行
docker compose -f docker-compose.postgres.yml ps postgres

# 检查网络连接
docker exec cli-proxy-api-1 ping postgres

# 检查密码是否正确
docker exec -it cliproxy-postgres psql -U cliproxy cliproxy
```

### 问题：令牌未同步

```bash
# 检查令牌是否写入数据库
docker exec cliproxy-postgres psql -U cliproxy cliproxy \
    -c "SELECT id, updated_at FROM cliproxy.auth_store;"

# 重启服务强制同步
docker compose -f docker-compose.postgres.yml restart cli-proxy-api-1

# 检查本地缓存目录
docker exec cli-proxy-api-1 ls -la /var/lib/cliproxy/pgstore/auths/
```

### 问题：负载均衡不工作

```bash
# 检查 Nginx 配置
docker exec cliproxy-nginx nginx -t

# 检查 Nginx 日志
docker logs cliproxy-nginx

# 测试后端连接
docker exec cliproxy-nginx wget -O- http://cli-proxy-api-1:8317/v1/models
```

## 参考资料

- [PostgreSQL Token Store 文档](postgresql.md)
- [Docker Compose 官方文档](https://docs.docker.com/compose/)
- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [Nginx 官方文档](https://nginx.org/en/docs/)
