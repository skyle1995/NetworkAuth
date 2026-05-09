# CHANGELOG

[2026-05-09] [BUGFIX] 修复前端导航菜单高亮 BUG：当将非 `/home/index` 页面设置为首页时，访问原首页路由不会再导致首页项与当前业务项同时激活
[2026-05-09] [BUGFIX] 门户首页入口纠偏：移除 `/home` 静态重定向，仅在访问 `/home` 入口页时按导航 `is_home` 跳转，显式访问 `/home/index` 不再被配置首页劫持
[2026-05-09] [UI/UX] 门户首页补全：按 ChipperCash 门户模板重构 `/home/index`，补齐 Hero 展示与 NetworkAuth 业务特性的功能卡片区块
[2026-05-09] [BUGFIX] 门户首页修复：公开门户按导航 `is_home` 动态决定 `/home` 与站点 Logo 的跳转目标，修复首页标记保存后前台不生效的问题
[2026-05-09] [BUGFIX] 门户模板收口：统一 `home/layout.vue` 与其他单后台项目的模板结构，并将维护页 `503.vue` 改为异步组件加载，消除动态/静态重复引入警告
[2026-05-09] [UI/UX] 门户模板统一：`home/layout.vue` 与 ChipperCash 单后台模板完全对齐，仅保留 NetworkAuth 自身的页脚项目名
