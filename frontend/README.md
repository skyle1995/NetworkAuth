# NetworkAuth-Web

本项目是基于 [vue-pure-admin](https://github.com/pure-admin/vue-pure-admin) 开发的前端管理系统模版。

## 相关项目

- **前端框架**：[vue-pure-admin](https://github.com/pure-admin/vue-pure-admin)
- **后端项目**：[NetworkAuth](https://github.com/skyle1995/NetworkAuth)

## 简介

NetworkAuth-Web 是一款中后台管理系统模版，配套 NetworkAuth 后端服务使用，提供应用程序管理、API接口管理、变量管理、用户认证等功能的可视化操作。

## 目录结构 (Views 页面归类)

当前前端页面的主要视图均存放在 `src/views` 目录下，并按照功能模块进行了如下分组归类：

- **`admin/`**：后台管理模块（需要登录授权）
  - `dashboard/`：控制台仪表盘
  - `application/`：应用相关模块分组
    - `apps/`：应用管理模块（应用列表、各类配置等）
    - `apis/`：API 接口管理模块
    - `functions/`：函数代码管理模块
    - `variables/`：变量配置管理模块
  - `system/`：系统相关模块分组
    - `settings/`：系统基础设置
    - `profile/`：个人中心
  - `logs/`：系统日志（包含登录日志、操作日志等）
  - `login/`：后台登录页面
- **`home/`**：前台首页（面向普通用户的展示页）
- **`install/`**：系统安装向导（初次部署时的配置指引）
- **`error/`**：系统错误页（403、404、500、503等）

## 快速开始

### 安装依赖

```bash
pnpm install
```

### 启动项目

```bash
pnpm dev
```

### 项目打包

```bash
pnpm build
```
