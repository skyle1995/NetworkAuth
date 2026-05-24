export default {
  path: "/error",
  redirect: "/error/403",
  meta: {
    icon: "ri/information-line",
    showLink: false,
    title: "异常页面",
    rank: 9
  },
  children: [
    {
      path: "/error/403",
      name: "403",
      component: () => import("@/views/error/403.vue"),
      meta: {
        title: "403"
      }
    },
    {
      path: "/error/404",
      name: "404",
      component: () => import("@/views/error/404.vue"),
      meta: {
        title: "404"
      }
    },
    {
      path: "/error/500",
      name: "500",
      component: () => import("@/views/error/500.vue"),
      meta: {
        title: "500"
      }
    },
    {
      path: "/error/503",
      name: "503",
      component: () => import("@/views/error/503.vue"),
      meta: {
        title: "维护中"
      }
    }
  ]
} satisfies RouteConfigsTable;
