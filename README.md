# NetworkAuth（网络授权服务）

网络授权服务 (NetworkAuth) 是一个专注于应用鉴权、接口管理和动态逻辑分发的后端系统。它基于 Go 语言开发，提供应用程序管理、API接口管理、变量管理、用户认证等核心服务。

## 功能特性

### 🚀 核心功能

- **一号通账号体系**: 全站一套账号与钱包——一个账号在所有应用共享同一份到期时间、点数与会员等级；封禁、黑名单、转绑计数同样全站生效
- **应用管理**: 应用增删改查、状态控制、密钥管理、插件标识（`plugin_id`）映射
- **版本列表**: 一个应用可维护多条版本记录，逐条启用/停用；更新检查取「已启用中的最高版本」，每条版本独立配置强制/自由更新、下载地址与更新公告
- **应用级计费策略（三开关）**: 每个应用三个**彼此独立、可任意组合**的开关——**计时模式**（要求账号有效期）、**点数模式**（按次或按周期扣点）、**会员限制**（要求会员等级达标），另有整体计费豁免
- **全局 API 接口**: 30 条接口定义全站唯一、安装即启用；应用身份由 `app_uuid` + `app_secret` 签名区分，无需为每个应用重复配置
- **全局账号策略**: 注册、邮箱验证、卡密注册/登录/充值、领取试用、注册限制、设备与 IP 绑定/转绑额度全站统一配置
- **卡密与会员**: 全站卡池 + 卡密套餐（面值快照）+ 会员等级（累充自动升级、充值返利、额外多开、赠送换绑、按等级点数折扣）
- **变量管理**: 变量可按应用隔离（`app_uuid`）或留空作为全站变量，别名在应用内唯一
- **函数管理**: 自定义函数代码全站共用，别名全局唯一，在 goja 沙箱执行
- **对外 API（API 密钥）**: `/api/openapi` 提供建应用、调计费、变量/函数维护、终端账号增删改与充值扣减，按 scope 最小授权，写操作进操作日志
- **用户管理**: 完整的用户认证和权限管理系统
- **系统设置**: 灵活的系统配置和参数管理
- **系统初始化**: 提供引导式的数据表初始化和默认设置注入
- **日志审计**: 详细的登录日志和操作日志记录，保障系统安全

> **文档入口**：
> - [docs/api-guide.md](docs/api-guide.md) — **客户端对接协议**（含两级授权、错误码、算法实现）
> - [docs/architecture.md](docs/architecture.md) — **架构总览，唯一入口**（数据归属 / 计费模型 / 会话模型 / 不变量 / 代码地图）
> - [docs/flows.md](docs/flows.md) — **各场景工作流程表**（请求流水线 / 登录 / 激活 / 心跳 / 清理）
> - [docs/app-settings.md](docs/app-settings.md) — 后台每个开关的逐项说明（运维向）
> - [docs/openapi.md](docs/openapi.md) — **对外 API（API 密钥）**：建应用 / 调计费 / 变量 / 函数 / 终端账号
> - [docs/plan-unified-account.md](docs/plan-unified-account.md) — 一号通改造的设计与实施记录
> - [docs/plan-host-plugin-session.md](docs/plan-host-plugin-session.md) — 外壳 + 插件方案（设计过程记录，字段以 api-guide 为准）

## 重大架构变更（一号通）

从「每个应用一套账号/计费/卡密」重构为「全站一套账号与钱包，应用只决定怎么用」。
以下是与改造前差异最大的几处，改造细节见 [docs/architecture.md](docs/architecture.md)。

| 领域 | 改造前 | 现在 |
|---|---|---|
| **账号** | 每个应用各自注册一套账号 | 全站唯一账号，到期时间 / 点数 / 会员等级一份钱包所有应用共享 |
| **授权** | 登录即拿令牌，直接代表"可用了" | **两级令牌**：账号令牌只证明"你是谁"（不计费）；再用它调 `70` 换应用激活令牌（才计费、才占并发） |
| **计费** | `billing_mode` 三选一（计时/点数/高级） | 三个**正交开关**：计时、点数、会员限制可任意组合；`billing_mode` 仅作兼容展示 |
| **版本** | 版本号/下载地址/更新方式挤在 `apps` 三列 | 独立 `app_versions` 表，一应用多版本，逐版本启用并各带更新公告 |
| **接口** | 每个应用复制一份接口配置 | 30 条接口全站唯一，应用身份靠签名里的 `app_uuid` 区分 |
| **卡密** | 按应用发行、按应用核销 | 全站卡池，任意卡在任意应用核销，无发行渠道字段 |
| **变量/函数** | 均绑定应用 | 变量可绑定应用或留空作全站变量；函数全站共用 |
| **绑定/并发** | 应用级各算一份 | 账号级统一：设备白名单、并发实例数、转绑额度全站一份 |
| **插件身份** | 靠应用名模糊匹配 | `apps.plugin_id` 显式映射本地插件，72 权益清单直接按插件返回 |
| **租约防伪** | 仅靠令牌 | 激活返回 `lease_sig` / `lease_sig_plugin`（Ed25519），客户端内嵌公钥可离线验签 |

> ⚠️ **事务约定**：SQLite 连接池为单连接（`SetMaxOpenConns(1)`），
> **事务内部禁止调用 `database.GetDB()`**。所有在事务内取计费的入口必须先
> `billing = billing.WithDB(tx)`，否则会与事务抢同一条连接互等死锁
> （表现为请求挂死、事务锁不释放）。回归用例：`services/single_conn_regression_test.go`。

### 🔧 技术特性
- **RESTful API**: 标准的 REST API 接口设计
- **JWT 认证**: 基于 JWT 的安全认证机制
- **多种加密算法**: 支持 RC4、RSA、RSA动态、易加密等多种加密方式
- **数据库支持**: 兼容 MySQL 和 SQLite 数据库 (通过 GORM)
- **Redis 缓存**: 集成 Redis 缓存提升性能（可选）
- **日志系统**: 完整的日志记录和管理，支持日志切割 (Logrus + Lumberjack)
- **配置管理**: 基于 Viper 的灵活配置系统
- **命令行工具**: 基于 Cobra 的强悍 CLI 管理工具

## 技术栈

- **语言**: Go 1.26.0
- **Web 框架**: Gin 
- **数据库 ORM**: GORM
- **缓存**: Redis（可选）
- **认证**: JWT + 验证码
- **日志**: Logrus + Lumberjack
- **配置管理**: Viper
- **命令行**: Cobra
- **加密**: 自定义加密工具包

## 项目结构

```
NetworkAuth/
├── cmd/                    # Cobra 命令行工具定义
├── config/                 # 配置文件模型与校验逻辑
├── constants/              # 全局常量定义 (版本号、状态码等)
├── controllers/            # 控制器层 (处理 HTTP 请求)
├── database/               # 数据库连接、迁移与默认数据填充
├── docs/                   # 架构、后台设置与客户端对接文档
├── frontend/               # Vue 3 管理后台（构建产物嵌入二进制）
├── middleware/             # Gin 中间件 (日志、认证、维护模式等)
├── models/                 # GORM 数据模型定义
├── server/                 # HTTP 服务器路由注册
├── services/               # 核心业务逻辑层
├── utils/                  # 通用工具函数 (加密、日志、时间等)
└── main.go                 # 项目入口
```

一号通改造涉及的核心文件：

| 文件 | 职责 |
|---|---|
| `models/billing.go` | 三个计费开关的推导（`DeriveBillingMode`）、30 项全局账号策略键名 |
| `models/app_level_point.go` | 等级点数：`(应用, 会员等级) → 点数消耗` |
| `models/app_version.go` | 版本列表：一个应用多版本，各带更新方式/下载地址/更新公告 |
| `services/billing.go` | `EffectiveBilling` 解析、三开关门禁判定、`WithDB` 事务句柄绑定、`AccountPolicy` 读写 |
| `services/activation.go` | 70 激活 / 71 释放 / 心跳续约；**所有取价必须在事务句柄上** |
| `services/entitlements.go` | 72 权益清单：把门禁预判结果提前给客户端 |
| `services/api_seed.go` | `EnsureGlobalAPIs()` 幂等补齐 30 条全局接口 |
| `services/member_auth.go` | 鉴权、注册/试用/充值、卡密登录 |
| `services/single_conn_regression_test.go` | 单连接池下的死锁回归（事务内禁止走全局连接池） |
| `services/unified_account_test.go` | 一号通不变量回归测试 |

## 快速开始

### 环境要求

- Go 1.26.0 或更高版本
- MySQL 5.7+ 或 SQLite 3
- Redis (可选)
- Node.js 18+ 与 pnpm（仅修改前端时需要；仓库内已含构建好的 `frontend/dist`）

### 安装与运行

1. **克隆项目**
   ```bash
   git clone https://github.com/skyle1995/NetworkAuth.git
   cd NetworkAuth
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **运行服务器**
   ```bash
   # 直接运行
   go run main.go server

   # 或者编译后运行
   go build -o networkauth main.go
   ./networkauth server
   ```

   > 首次启动会进入引导式安装：建表 → 写入 14 项账号策略默认值 → 写入 27 条全局接口条目 → 创建超级管理员。
   > 安装完成后 `apps` 为空，先到「接口设置」确认 27 条已就绪，再到「应用管理」添加应用。

4. **修改前端后重建静态资源**
   ```bash
   cd frontend
   pnpm install     # 首次或依赖变更时
   pnpm build       # 输出到 frontend/dist，会被 go:embed 打进二进制
   cd ..
   go build -o networkauth main.go   # 必须重新编译才会切换到新前端
   ```

   > ⚠️ `frontend/dist` 是构建产物（已被 gitignore）。若为生产构建二进制时该目录不存在，`go build` 会因 `//go:embed all:frontend/dist` 找不到文件而失败。

### 测试

```bash
go test ./...                                    # 全部
go test ./services/ -run TestWalletShared -v     # 一号通不变量：跨应用钱包共享
go test ./services/ -run TestHybridMode -v       # 混合模式按等级取价
```

### 命令行工具

项目基于 Cobra CLI 框架，提供了丰富的命令行工具：

```bash
# 查看帮助信息
./networkauth --help

# 启动服务器
./networkauth server

# 指定配置文件启动
./networkauth --config ./config.json server

# 指定端口启动 (覆盖配置文件)
./networkauth server -p 8080
```

## 部署

### Docker 部署

```bash
# 构建镜像
docker build -t networkauth .

# 运行容器
docker run -d -p 8080:8080 networkauth
```

### 生产环境部署

1. 编译生产版本
   ```bash
   go build -o networkauth main.go
   ```

2. 准备配置文件（可参考默认配置）。
3. 使用进程管理工具（如 systemd 或 supervisor）管理后端服务进程。

### 反向代理与客户端 IP 识别

客户端 IP 用于 `/api/open` 限流（120 次/分钟）、IP 绑定与验证（`ip_verify`）、注册限流、IP 黑名单等场景，识别规则由 `config.json` 的 `server.trusted_proxies` 决定：

- **默认（空数组）= 不信任任何代理**，一律取 TCP 直连对端地址。这是安全默认值——攻击者伪造的 `X-Forwarded-For` 不会被采信。
- **挂载 Nginx 等反向代理时必须显式配置代理机 IP/CIDR 白名单**，否则所有客户端都会被识别为代理 IP：限流被共享误触发、IP 绑定/验证与地区校验全部失真。

```json
{
  "server": {
    "trusted_proxies": ["127.0.0.1/8", "10.0.0.0/8"]
  }
}
```

仅来自白名单地址的请求才会解析 `X-Forwarded-For`（从右向左跳过可信节点，取最近的不可信节点作为真实客户端 IP）。配置项非法时服务会直接启动失败，避免带着错误的信任策略上线。

---

## 后台登录安全加固

后台登录已内置多层防护（本次安全修复），上线前请逐项确认：

- **验证码与 `dev_mode` 解耦**：`server.dev_mode: true` 不再跳过登录验证码；本地调试确需跳过时，显式设置 `security.skip_captcha: true`（启动会打印严重安全警告），生产环境必须为 `false`。
- **客户端 IP 不可伪造**：未配置 `server.trusted_proxies` 时禁用 `X-Forwarded-For`/`X-Real-IP` 解析，客户端 IP 取 TCP 对端地址。部署在 Nginx/CDN 之后时必须把代理出口地址加入白名单（如 `["127.0.0.1/32"]`），并让代理覆盖客户端可伪造的转发头，否则 IP 级限流会退化为全局共享额度、登录日志 IP 失真。
- **验证码令牌绑定客户端与类型**：滑块/点击验证码通过后签发的一次性令牌绑定签发时的 IP、User-Agent 与验证码类型，5 分钟内有效且只能使用一次；登录只接受与当前配置类型一致的令牌，未启用的验证码接口会直接拒绝（修复「配置 click 后仍可调用 slide 接口刷令牌」的绕过）。
- **限流 fail-secure**：Redis 不可用时自动降级为进程内限流，不再静默放行。
- **登录失败锁定**：同一账号连续失败达到阈值后临时锁定，重复触发按指数退避（最高 6 小时）；同一 IP 的失败（含验证码错误）也会触发临时锁定。锁定状态保存在进程内存，重启服务即清除。
- **建议**：在 Nginx/WAF 层对 `/api/admin/login`、`/api/admin/captcha/` 再叠加一层基于 `$binary_remote_addr` 的 `limit_req`。

对应配置（`config.json`，均可省略、使用安全默认值）：

```json
{
  "server": {
    "dev_mode": false,
    "trusted_proxies": []
  },
  "security": {
    "skip_captcha": false,
    "login_max_failures": 10,
    "login_window_minutes": 15,
    "login_lock_minutes": 15,
    "login_ip_max_failures": 30,
    "login_ip_lock_minutes": 10
  }
}
```

---

## Git 版本发布（Tag）

本项目的发布通常以 Tag 为触发点（例如在 CI/CD 中自动构建并创建 Release）。建议使用语义化版本号（SemVer），并统一以 `v` 前缀命名，例如 `v1.2.3`。

### 发布一个 Tag 版本

```bash
# 0) 同步远端 Tag（推荐）
#    目的：避免本地残留 Tag（远端已删除）被误推回去并触发工作流
#    Git 版本需支持 --prune-tags（不支持时看下方兼容方案）
git fetch origin --tags --prune --prune-tags

# 1) 确保代码已提交且工作区干净
git status

# 2) 确认远端不存在同名 Tag（避免误复用）
git ls-remote --tags origin v1.2.3

# 3) 创建“附注（annotated）”Tag（推荐）
git tag -a v1.2.3 -m "release: v1.2.3"

# 4) 推送 Tag 到远端（推荐：仅推送本次发布的 Tag）
git push origin v1.2.3
```

注意：不建议直接使用 `git push origin --tags`。如果远端曾删除某些 Tag，但本地仍保留，这条命令会把这些 Tag 重新推送回远端，从而可能再次触发 Release/工作流。

如你的 Git 不支持 `--prune-tags`，可用“清空本地 Tag → 重新从远端拉取 Tag”的方式确保对齐后再发布：

```bash
git tag -l | xargs -n 1 git tag -d
git fetch origin --tags
```

```powershell
git tag | ForEach-Object { git tag -d $_ }
git fetch origin --tags
```

### 重新发布同名 Tag 版本（修正打错的 Tag）

同名 Tag 一般不建议复用；如确需修正（例如 Tag 指向了错误提交），请先删除远端 Tag，再重新创建并推送。

```bash
# 假设要修正的 Tag 为 v1.2.3

# 1) 删除本地 Tag
git tag -d v1.2.3

# 2) 删除远端 Tag
git push origin --delete v1.2.3

# 3) 在正确的提交上重建 Tag（<commit> 可省略，默认当前 HEAD）
git tag -a v1.2.3 <commit> -m "release: v1.2.3"

# 4) 重新推送到远端
git push origin v1.2.3
```

如果远端已基于该 Tag 生成了 Release（例如 Gitea/GitHub Release），通常也需要同步删除并重新创建对应 Release，避免附件与版本信息不一致。

---

## CI/CD Secrets 配置

以下变量需在仓库 **Settings → Actions → Secrets** 中配置。

### 必选（缺一不可）

| Secret 名称 | 说明 |
|---|---|
| `RELEASE_TOKEN` | Gitea 访问令牌，权限需包含 `repo`。用于自动创建 Release 并上传附件 |

### 对象存储上传（可选，未配置则跳过）

以下凭据需在 **Settings → Actions → Secrets** 中配置，其余配置项在 **Settings → Actions → Variables** 中配置。

#### 腾讯云 COS

**Secrets（凭据）**

| Secret 名称 | 必填 | 说明 |
|---|---|---|
| `COS_SECRET_ID` | 是 | API 密钥 SecretId |
| `COS_SECRET_KEY` | 是 | API 密钥 SecretKey |

**Variables（配置）**

| Variable 名称 | 必填 | 说明 |
|---|---|---|
| `COS_BUCKET` | 是 | 存储桶名称（不含 AppId 后缀） |
| `COS_REGION` | 是 | 地域，如 `ap-guangzhou` |
| `COS_UPLOAD_PATH` | 否 | 上传路径前缀，如 `releases`；留空则放桶根目录 |
| `COS_CLEAR_BEFORE_UPLOAD` | 否 | 上传前清空目标路径；默认 `false`，设为 `true` 启用 |

#### 阿里云 OSS

**Secrets（凭据）**

| Secret 名称 | 必填 | 说明 |
|---|---|---|
| `OSS_ACCESS_KEY_ID` | 是 | RAM 用户 AccessKey ID |
| `OSS_ACCESS_KEY_SECRET` | 是 | RAM 用户 AccessKey Secret |

**Variables（配置）**

| Variable 名称 | 必填 | 说明 |
|---|---|---|
| `OSS_BUCKET` | 是 | Bucket 名称 |
| `OSS_ENDPOINT` | 是 | Endpoint，如 `oss-cn-hangzhou.aliyuncs.com` |
| `OSS_UPLOAD_PATH` | 否 | 上传路径前缀，同上 |
| `OSS_CLEAR_BEFORE_UPLOAD` | 否 | 上传前清空目标路径；默认 `false`，设为 `true` 启用 |

> **优先级**：COS > OSS。两类同时配置时仅使用 COS。全部未配置则跳过存储上传（Release 仍正常创建）。

---

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。
