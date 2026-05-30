# 更新日志

## 2026-05-27

### 安全

- [2026-05-27] [安全] 图形验证码缓存改为带硬上限的内存存储（TTL 10 分钟、最大 20000 条），降低恶意刷验证码导致的内存风险。

## 2026-05-25

### 仓库维护

- [2026-05-25] [调整] 前后端改为同仓维护：取消忽略 frontend 源码，仅忽略前端依赖/环境/产物，并移除 frontend 内嵌 Git 仓库关联。
- [2026-05-25] [调整] 放行前端 .env/.env.development/.env.production/.env.staging 推送，改为仅忽略 .env.local 与 .env.*.local。

## 2026-05-24

### 登录

- [2026-05-24] [修正] 登录态判定改为仅依赖 localStorage 中 token + expires，不再依赖 multiple-tabs cookie，浏览器重启不再误清登录态。

### 前端资源

- [2026-05-24] [调整] 后端静态资源嵌入改为 go:embed frontend/dist，由 main.go 注入 fs.FS 至 server 挂载；移除 public/public.go 依赖，构建不再需要拷贝到 public/dist。

## 2026-05-17

### 前端布局

- [修正] 操作日志/登录日志页面底部 margin 为 0 导致页脚高度被覆盖的问题，统一改为 `margin: 24px !important`
- [修正] 操作日志/登录日志页面 PureTableBar 组件 slot 冲突警告，将 `#buttons` 移至组件外部
- [修正] API 管理页面底部 margin 为 0 的问题

## 2026-05-09

### 门户首页与模板收口

- [修正] 修复前端导航菜单高亮 BUG：当将非 `/home/index` 页面设置为首页时，访问原首页路由不会再导致首页项与当前业务项同时激活
- [修正] 门户首页入口纠偏：移除 `/home` 静态重定向，仅在访问 `/home` 入口页时按导航 `is_home` 跳转，显式访问 `/home/index` 不再被配置首页劫持
- [功能] 门户首页补全：按 ChipperCash 门户模板重构 `/home/index`，补齐 Hero 展示与 NetworkAuth 业务特性的功能卡片区块
- [修正] 门户首页修复：公开门户按导航 `is_home` 动态决定 `/home` 与站点 Logo 的跳转目标，修复首页标记保存后前台不生效的问题
- [修正] 门户模板收口：统一 `home/layout.vue` 与其他单后台项目的模板结构，并将维护页 `503.vue` 改为异步组件加载，消除动态/静态重复引入警告
- [样式] 门户模板统一：`home/layout.vue` 与 ChipperCash 单后台模板完全对齐，仅保留 NetworkAuth 自身的页脚项目名
