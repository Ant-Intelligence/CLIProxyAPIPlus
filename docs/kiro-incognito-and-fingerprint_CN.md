# Kiro 隐私模式与设备指纹

## 概述

CLIProxyAPI Plus 为 Kiro 登录和 API 请求提供了两项重要的辅助功能：

- **隐私模式 (Incognito Mode)**: OAuth 登录时以隐私/无痕窗口打开浏览器，支持多帐号切换
- **设备指纹 (Device Fingerprint)**: 为每个 token 生成独立的虚拟设备标识，模拟真实 Kiro IDE 客户端

## 目录

- [隐私模式](#隐私模式)
  - [为什么需要隐私模式](#为什么需要隐私模式)
  - [使用方式](#使用方式)
  - [支持的浏览器](#支持的浏览器)
  - [工作原理](#工作原理)
- [设备指纹](#设备指纹)
  - [为什么需要设备指纹](#为什么需要设备指纹)
  - [指纹维度](#指纹维度)
  - [工作原理](#工作原理-1)
  - [生命周期](#生命周期)

---

## 隐私模式

### 为什么需要隐私模式

普通浏览器窗口会复用已有的登录 session（cookies），这意味着：

- 如果你已在浏览器中登录了 AWS 帐号 A，OAuth 流程会自动使用帐号 A
- 你无法在不手动退出的情况下选择登录帐号 B
- 对于需要管理多个 Kiro 帐号的场景，这会带来不便

隐私窗口会忽略已有 cookies，每次 OAuth 登录都会弹出全新的登录页面，让你自由选择帐号。

### 使用方式

Kiro 登录**默认启用**隐私模式，无需额外参数：

```bash
# 默认启用隐私模式
./CLIProxyAPI -kiro-login
./CLIProxyAPI -kiro-aws-login
./CLIProxyAPI -kiro-aws-authcode

# 显式关闭隐私模式（使用已有浏览器 session）
./CLIProxyAPI -kiro-login --no-incognito

# 显式开启隐私模式（适用于其他非 Kiro 登录）
./CLIProxyAPI -login --incognito
```

**参数说明**：

| 参数 | 说明 |
|------|------|
| `--incognito` | 强制开启隐私模式 |
| `--no-incognito` | 强制关闭隐私模式，使用已有浏览器 session |
| (默认) | Kiro 登录默认开启，其他登录默认关闭 |

### 支持的浏览器

隐私模式支持以下主流浏览器，并自动适配各浏览器的隐私窗口参数：

| 浏览器 | 隐私参数 | macOS | Windows | Linux |
|--------|----------|:-----:|:-------:|:-----:|
| Google Chrome | `--incognito` | ✅ | ✅ | ✅ |
| Chromium | `--incognito` | - | - | ✅ |
| Firefox | `--private-window` | ✅ | ✅ | ✅ |
| Microsoft Edge | `--inprivate` | ✅ | ✅ | ✅ |
| Brave | `--incognito` | ✅ | ✅ | ✅ |
| Safari | AppleScript | ✅ | - | - |

### 工作原理

隐私模式的启动采用三层降级策略，确保在各种环境下都能正常工作：

```
1. 检测系统默认浏览器
   ├── macOS:   读取 LaunchServices 配置
   ├── Windows: 查询注册表 HKCU\...\UrlAssociations
   └── Linux:   调用 xdg-settings get default-web-browser

2. 用默认浏览器的隐私参数打开
   └── 失败？进入下一步

3. 按优先级尝试已安装的浏览器
   Chrome → Firefox → Brave → Edge → (Safari AppleScript)
   └── 全部失败？回退到普通模式打开
```

**相关代码**: `internal/browser/browser.go`

---

## 设备指纹

### 为什么需要设备指纹

AWS CodeWhisperer 后端会检查请求中的设备信息（User-Agent、自定义 HTTP 头等）来识别客户端。CLIProxyAPI Plus 作为代理服务器，需要模拟真实的 Kiro IDE 客户端行为。

设备指纹系统解决以下问题：

- **身份模拟**: 让代理请求看起来像来自真实的 Kiro IDE
- **多帐号隔离**: 每个 token 使用独立的设备指纹，避免多帐号共享同一指纹被识别为异常
- **一致性**: 同一 token 在运行期间保持相同的指纹，避免频繁变化引起检测

### 指纹维度

每个指纹包含 11 个维度的设备信息，全部从预定义的真实值池中随机选取：

| 维度 | HTTP 头 | 取值范围 |
|------|---------|----------|
| SDK 版本 | `X-Kiro-SDK-Version` | 1.0.20 ~ 1.0.27 |
| 操作系统 | `X-Kiro-OS-Type` | darwin / windows / linux |
| 系统版本 | `X-Kiro-OS-Version` | 与 OS 类型匹配的版本号 |
| Node 版本 | `X-Kiro-Node-Version` | 18.x / 20.x / 22.x |
| Kiro 版本 | `X-Kiro-Version` | 0.3.x ~ 0.8.x |
| Kiro 哈希 | `X-Kiro-Hash` | SHA256 (64 位十六进制) |
| 语言偏好 | `Accept-Language` | en-US / zh-CN / ja-JP 等 |
| 屏幕分辨率 | `X-Screen-Resolution` | 1920x1080 ~ 3840x2160 |
| 色深 | `X-Color-Depth` | 24 / 32 |
| CPU 核心数 | `X-Hardware-Concurrency` | 4 ~ 32 |
| 时区偏移 | `X-Timezone-Offset` | -480 ~ +540 |

**OS 版本联动**: 系统版本号会与操作系统类型匹配，不会出现 `darwin` + `10.0.22621`（Windows 版本号）的矛盾组合。

### User-Agent 格式

指纹系统会构建与真实 Kiro IDE 一致的 User-Agent 字符串：

```
aws-sdk-js/1.0.24 ua/2.1 os/darwin#14.3 lang/js md/nodejs#20.11.0 api/codewhispererstreaming#1.0.24 m/E KiroIDE-0.7.0-{hash}
```

以及 `X-Amz-User-Agent` 头：

```
aws-sdk-js/1.0.24 KiroIDE-0.7.0-{hash}
```

### 工作原理

```
                    ┌───────────────────────────┐
                    │    FingerprintManager      │
                    │  (内存缓存, 并发安全)       │
                    └─────────┬─────────────────┘
                              │
              GetFingerprint(tokenKey)
                              │
                    ┌─────────▼─────────────────┐
                    │ 缓存中已有该 token 的指纹？  │
                    └─────────┬─────────────────┘
                         ╱          ╲
                       是             否
                       │              │
                  返回已有指纹     随机生成新指纹
                                      │
                              ┌───────▼───────────┐
                              │ 随机选取 11 个维度  │
                              │ OS 版本联动匹配     │
                              │ SHA256 生成 Hash   │
                              └───────┬───────────┘
                                      │
                              存入缓存并返回
```

**核心特性**：

1. **Token 级绑定**: 同一个 token 始终返回相同指纹，保持请求一致性
2. **自动隔离**: 不同 token 自动获得不同指纹
3. **并发安全**: 使用 `sync.RWMutex` 保护，支持多 goroutine 并发访问
4. **内存缓存**: 指纹存储在内存中，服务重启后自动生成新指纹

### 生命周期

| 事件 | 指纹行为 |
|------|----------|
| 首次使用 token 发送请求 | 自动生成新指纹并缓存 |
| 同一 token 再次请求 | 返回已缓存的相同指纹 |
| Token 被移除或过期 | 调用 `RemoveFingerprint` 清除对应指纹 |
| 服务重启 | 所有指纹重新生成（内存缓存不持久化） |

**相关代码**: `internal/auth/kiro/fingerprint.go`
