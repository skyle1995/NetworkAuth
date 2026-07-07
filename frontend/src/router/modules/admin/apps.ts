const Layout = () => import("@/layout/index.vue");

export default {
  path: "/admin/apps",
  name: "Apps",
  component: Layout,
  redirect: "/admin/apps/index",
  meta: {
    icon: "ep:menu",
    title: "应用管理",
    rank: 2
  },
  children: [
    {
      path: "/admin/apps/index",
      name: "AppsIndex",
      component: () => import("@/views/admin/application/apps/index.vue"),
      meta: {
        title: "应用程序",
        icon: "ep:grid",
        showParent: true
      }
    },
    {
      path: "/admin/apis/index",
      name: "ApisIndex",
      component: () => import("@/views/admin/application/apis/index.vue"),
      meta: {
        title: "接口设置",
        icon: "ep:connection",
        showParent: true
      }
    },
    {
      path: "/admin/variables/index",
      name: "VariablesIndex",
      component: () => import("@/views/admin/application/variables/index.vue"),
      meta: {
        title: "公共变量",
        icon: "ep:document",
        showParent: true
      }
    },
    {
      path: "/admin/functions/index",
      name: "FunctionsIndex",
      component: () => import("@/views/admin/application/functions/index.vue"),
      meta: {
        title: "公共函数",
        icon: "ep:setting",
        showParent: true
      }
    },
    {
      path: "/admin/cards/index",
      name: "CardsIndex",
      component: () => import("@/views/admin/application/cards/index.vue"),
      meta: {
        title: "卡密管理",
        icon: "ep:tickets",
        showParent: true
      }
    },
    {
      path: "/admin/members/index",
      name: "MembersIndex",
      component: () => import("@/views/admin/application/members/index.vue"),
      meta: {
        title: "终端用户",
        icon: "ep:user",
        showParent: true
      }
    }
  ]
} satisfies RouteConfigsTable;
