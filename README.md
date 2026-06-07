# NetworkAuth（网络授权服务）

网络授权服务 (NetworkAuth) 是一个专注于应用鉴权、接口管理和动态逻辑分发的后端系统。它基于 Go 语言开发，提供应用程序管理、API接口管理、变量管理、用户认证等核心服务。

## 功能特性

### 🚀 核心功能
- **应用管理**: 支持应用的增删改查、版本管理、状态控制、密钥管理
- **API接口管理**: 支持多种加密算法的API接口配置（RC4、RSA、易加密等）
- **变量管理**: 独立的变量系统，支持变量的增删改查和别名管理
- **函数管理**: 支持自定义函数代码管理，可绑定特定应用或全局使用
- **用户管理**: 完整的用户认证和权限管理系统
- **系统设置**: 灵活的系统配置和参数管理
- **系统初始化**: 提供引导式的数据表初始化和默认设置注入
- **日志审计**: 详细的登录日志和操作日志记录，保障系统安全

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

- **语言**: Go 1.25.0
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
├── middleware/             # Gin 中间件 (日志、认证、维护模式等)
├── models/                 # GORM 数据模型定义
├── server/                 # HTTP 服务器路由注册
├── services/               # 核心业务逻辑层
├── utils/                  # 通用工具函数 (加密、日志、时间等)
└── main.go                 # 项目入口
```

## 快速开始

### 环境要求

- Go 1.25.0 或更高版本
- MySQL 5.7+ 或 SQLite 3
- Redis (可选)

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