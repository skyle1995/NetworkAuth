const Layout = () => import("@/layout/index.vue");

export default {
  path: "/admin/system",
  name: "System",
  component: Layout,
  redirect: "/admin/system/profile",
  meta: {
    icon: "ep/setting",
    title: "系统管理",
    rank: 10
  },
  children: [
    {
      path: "/admin/system/profile",
      name: "ProfileIndex",
      component: () => import("@/views/admin/system/profile/index.vue"),
      meta: {
        icon: "ep/user",
        title: "个人资料"
      }
    },
    {
      path: "/admin/system/settings",
      name: "SettingsIndex",
      component: () => import("@/views/admin/system/settings/index.vue"),
      meta: {
        icon: "ep/setting",
        title: "系统设置"
      }
    },
    {
      path: "/admin/system/portal-navigation",
      name: "PortalNavigationIndex",
      component: () => import("@/views/admin/system/portal-navigation/index.vue"),
      meta: {
        icon: "ep:operation",
        title: "导航设置"
      }
    }
  ]
} satisfies RouteConfigsTable;
