const Layout = () => import("@/layout/index.vue");

export default {
  path: "/admin/logs",
  name: "Logs",
  component: Layout,
  redirect: "/admin/logs/operation",
  meta: {
    icon: "ep:document",
    title: "日志管理",
    rank: 11
  },
  children: [
    {
      path: "/admin/logs/operation",
      name: "OperationLog",
      component: () => import("@/views/admin/logs/operation/index.vue"),
      meta: {
        icon: "ep:operation",
        title: "操作日志"
      }
    },
    {
      path: "/admin/logs/login",
      name: "LoginLog",
      component: () => import("@/views/admin/logs/login/index.vue"),
      meta: {
        icon: "ep:avatar",
        title: "登录日志"
      }
    },
    {
      path: "/admin/logs/member",
      name: "MemberLog",
      component: () => import("@/views/admin/logs/member/index.vue"),
      meta: {
        icon: "ep:tickets",
        title: "调用审计"
      }
    }
  ]
} satisfies RouteConfigsTable;
