# NetworkAuth（开发中）

一个基于 Go 语言开发的网络应用管理系统，提供应用程序管理、API接口管理、变量管理、用户认证等功能的 Web 管理平台。

## 功能特性

### 🚀 核心功能
- **应用管理**: 支持应用的增删改查、版本管理、状态控制、密钥管理
- **API接口管理**: 支持多种加密算法的API接口配置（RC4、RSA、易加密等）
- **变量管理**: 独立的变量系统，支持变量的增删改查和别名管理
- **函数管理**: 支持自定义函数代码管理，可绑定特定应用或全局使用
- **用户管理**: 完整的用户认证和权限管理系统
- **系统设置**: 灵活的系统配置和参数管理
- **系统安装**: 提供可视化的安装向导，轻松完成数据库和管理员配置
- **仪表盘**: 实时系统状态监控和统计数据展示
- **日志审计**: 详细的登录日志和操作日志记录，保障系统安全

### 🔧 技术特性
- **RESTful API**: 标准的 REST API 接口设计
- **JWT 认证**: 基于 JWT 的安全认证机制
- **多种加密算法**: 支持 RC4、RSA、RSA动态、易加密等多种加密方式
- **数据库支持**: 支持 MySQL 和 SQLite 数据库
- **Redis 缓存**: 集成 Redis 缓存提升性能（可选）
- **Excel 导出**: 支持数据导出为 Excel 文件
- **日志系统**: 完整的日志记录和管理，支持日志切割
- **配置管理**: 基于 Viper 的灵活配置系统

### 🎨 界面特性
- **响应式设计**: 支持多种设备和屏幕尺寸
- **现代化 UI**: 基于 LayUI 的现代化管理界面
- **主题支持**: 支持明暗主题切换
- **实时更新**: 支持数据的实时刷新和更新
- **片段化加载**: 采用 AJAX 片段加载提升用户体验

## 技术栈

- **后端**: Go 1.25.0
- **Web 框架**: Gin + 自定义路由
- **数据库**: GORM + MySQL/SQLite
- **缓存**: Redis（可选）
- **认证**: JWT + 验证码
- **日志**: Logrus + Lumberjack
- **配置**: Viper
- **前端**: LayUI + JavaScript
- **工具**: Excelize (Excel导出)
- **加密**: 自定义加密工具包

## 项目结构

```
networkDev/
├── cmd/                    # 命令行工具
│   ├── root.go            # 根命令定义
│   └── server.go          # 服务器启动命令
├── config/                 # 配置文件和配置管理
│   ├── config.go          # 配置加载和验证
│   ├── security.go        # 安全配置
│   └── validator.go       # 配置验证器
├── constants/              # 常量定义
│   └── status.go          # 状态常量
├── controllers/            # 控制器层
│   ├── admin/             # 管理后台控制器
│   │   ├── api.go         # API接口管理
│   │   ├── app.go         # 应用管理
│   │   ├── auth.go        # 认证管理
│   │   ├── captcha.go     # 验证码管理
│   │   ├── function.go    # 函数管理
│   │   ├── handlers.go    # 通用处理器
│   │   ├── login_log.go   # 登录日志
│   │   ├── operation_log.go # 操作日志
│   │   ├── profile.go     # 个人资料
│   │   ├── settings.go    # 系统设置
│   │   ├── user.go        # 用户管理
│   │   └── variable.go    # 变量管理
│   ├── default/           # 默认控制器
│   ├── install/           # 安装向导控制器
│   └── base.go            # 基础控制器
├── database/              # 数据库相关
│   ├── database.go        # 数据库连接
│   ├── migrate.go         # 数据库迁移
│   └── settings.go        # 默认设置初始化
├── middleware/            # 中间件
│   ├── devmode.go         # 开发模式中间件
│   ├── install.go         # 安装检查中间件
│   ├── logging.go         # 日志中间件
│   └── maintenance.go     # 维护模式中间件
├── models/                # 数据模型
│   ├── api.go             # API接口模型
│   ├── app.go             # 应用模型
│   ├── function.go        # 函数模型
│   ├── login_log.go       # 登录日志模型
│   ├── operation_log.go   # 操作日志模型
│   ├── settings.go        # 系统设置模型
│   ├── user.go            # 用户模型
│   └── variable.go        # 变量模型
├── server/                # 服务器路由配置
│   ├── admin.go           # 管理后台路由
│   ├── default.go         # 默认路由
│   ├── install.go         # 安装路由
│   └── routes.go          # 路由注册
├── services/              # 业务逻辑层
│   ├── log_cleanup.go     # 日志清理服务
│   ├── operation_log.go   # 操作日志服务
│   ├── query.go           # 查询服务
│   └── settings.go        # 设置服务
├── utils/                 # 工具函数
│   ├── encrypt/           # 加密工具包
│   ├── excel/             # Excel工具
│   ├── logger/            # 日志工具
│   ├── timeutil/          # 时间工具
│   ├── cookie.go          # Cookie工具
│   ├── crypto.go          # 加密工具
│   ├── csrf.go            # CSRF防护
│   ├── database.go        # 数据库工具
│   └── errors.go          # 错误处理
└── web/                   # Web 资源
    ├── assets/            # 资源文件
    ├── static/            # 静态资源
    └── template/          # 模板文件
        ├── admin/         # 管理后台模板
        ├── default/       # 默认模板
        └── install/       # 安装向导模板
```

## 快速开始

### 环境要求

- Go 1.25.0 或更高版本
- MySQL 5.7+ 或 SQLite 3
- Redis (可选，用于缓存)

### 安装步骤

1. **克隆项目**
   ```bash
   git clone <repository-url>
   cd networkDev
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **运行项目**
   ```bash
   # 直接运行
   ./networkDev server
   
   # 或使用 go run
   go run main.go server
   ```

4. **系统初始化**
   
   打开浏览器访问: `http://localhost:8080/install`
   
   根据安装向导提示，配置数据库连接和管理员账号即可完成初始化。

### 命令行工具

项目基于 Cobra CLI 框架，提供了丰富的命令行工具支持：

```bash
# 查看帮助信息
./networkDev --help

# 启动服务器
./networkDev server

# 指定配置文件启动
./networkDev --config ./config.json server

# 指定端口启动 (覆盖配置文件)
./networkDev server -p 8080
```

## API 文档

### 认证接口
- `POST /admin/api/auth/login` - 用户登录
- `POST /admin/api/auth/logout` - 用户登出
- `GET /admin/api/auth/captcha` - 获取验证码

### 应用管理接口
- `GET /admin/api/apps/list` - 获取应用列表
- `POST /admin/api/apps/create` - 创建应用
- `POST /admin/api/apps/update` - 更新应用
- `POST /admin/api/apps/delete` - 删除应用
- `POST /admin/api/apps/batch_delete` - 批量删除应用

### 变量管理接口
- `GET /admin/variable/list` - 获取变量列表
- `POST /admin/variable/create` - 创建变量
- `POST /admin/variable/update` - 更新变量
- `POST /admin/variable/delete` - 删除变量
- `POST /admin/variable/batch_delete` - 批量删除变量

### 函数管理接口
- `GET /admin/function/list` - 获取函数列表
- `POST /admin/function/create` - 创建函数
- `POST /admin/function/update` - 更新函数
- `POST /admin/function/delete` - 删除函数
- `POST /admin/function/batch_delete` - 批量删除函数

### 系统管理接口
- `GET /admin/api/settings` - 获取系统设置
- `POST /admin/api/settings/update` - 更新系统设置
- `GET /admin/api/logs` - 获取操作日志
- `GET /admin/api/login_logs` - 获取登录日志

## 部署

### Docker 部署

```bash
# 构建镜像
docker build -t networkdev .

# 运行容器
docker run -d -p 8080:8080 networkdev
```

### 生产环境部署

1. 编译生产版本
   ```bash
   go build -o networkdev main.go
   ```

2. 配置生产环境配置文件

3. 使用进程管理工具（如 systemd）管理服务

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。
