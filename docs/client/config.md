# config

保存或查看服务器连接配置。保存后，后续命令不再需要每次传入 `--server` 和 `--api-key`。

## 用法

```bash
# 保存配置
cpa-client config --server https://your-server.example.com --api-key YOUR_KEY

# 只更新其中一项
cpa-client config --server https://new-server.example.com
cpa-client config --api-key NEW_KEY

# 查看当前配置
cpa-client config
```

## 输出示例

```
Server:  https://your-server.example.com
API Key: YOUR_KEY
```

保存时输出：

```
Config saved to /root/.cli-proxy-api/client.yaml
```

## 说明

- 配置文件路径为 `~/.cli-proxy-api/client.yaml`
- 目录不存在时会自动创建
- 命令行的 `--server` / `--api-key` 参数始终优先于配置文件中的值
