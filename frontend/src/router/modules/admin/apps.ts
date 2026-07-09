const Layout = () => import("@/layout/index.vue");

// 原「应用管理」分组下的页面已拆为顶级菜单，紧随仪表盘(rank 0)之后，顺序保持不变。
export default [
  {
    path: "/admin/apps",
    name: "Apps",
    component: Layout,
    redirect: "/admin/apps/index",
    meta: {
      icon: "ep:grid",
      title: "应用程序",
      rank: 1
    },
    children: [
      {
        path: "/admin/apps/index",
        name: "AppsIndex",
        component: () => import("@/views/admin/application/apps/index.vue"),
        meta: {
          title: "应用程序"
        }
      }
    ]
  },
  {
    path: "/admin/apis",
    name: "Apis",
    component: Layout,
    redirect: "/admin/apis/index",
    meta: {
      icon: "ep:connection",
      title: "接口设置",
      rank: 2
    },
    children: [
      {
        path: "/admin/apis/index",
        name: "ApisIndex",
        component: () => import("@/views/admin/application/apis/index.vue"),
        meta: {
          title: "接口设置"
        }
      }
    ]
  },
  {
    path: "/admin/variables",
    name: "Variables",
    component: Layout,
    redirect: "/admin/variables/index",
    meta: {
      icon: "ep:document",
      title: "公共变量",
      rank: 3
    },
    children: [
      {
        path: "/admin/variables/index",
        name: "VariablesIndex",
        component: () =>
          import("@/views/admin/application/variables/index.vue"),
        meta: {
          title: "公共变量"
        }
      }
    ]
  },
  {
    path: "/admin/functions",
    name: "Functions",
    component: Layout,
    redirect: "/admin/functions/index",
    meta: {
      icon: "ep:setting",
      title: "公共函数",
      rank: 4
    },
    children: [
      {
        path: "/admin/functions/index",
        name: "FunctionsIndex",
        component: () =>
          import("@/views/admin/application/functions/index.vue"),
        meta: {
          title: "公共函数"
        }
      }
    ]
  },
  {
    path: "/admin/cards",
    name: "Cards",
    component: Layout,
    redirect: "/admin/cards/index",
    meta: {
      icon: "ep:tickets",
      title: "卡密管理",
      rank: 5
    },
    children: [
      {
        path: "/admin/cards/index",
        name: "CardsIndex",
        component: () => import("@/views/admin/application/cards/index.vue"),
        meta: {
          title: "卡密管理"
        }
      }
    ]
  },
  {
    path: "/admin/members",
    name: "Members",
    component: Layout,
    redirect: "/admin/members/index",
    meta: {
      icon: "ep:user",
      title: "终端账号",
      rank: 6
    },
    children: [
      {
        path: "/admin/members/index",
        name: "MembersIndex",
        component: () => import("@/views/admin/application/members/index.vue"),
        meta: {
          title: "终端账号"
        }
      }
    ]
  },
  {
    path: "/admin/online",
    name: "OnlineManage",
    component: Layout,
    redirect: "/admin/online/index",
    meta: {
      icon: "ep:monitor",
      title: "在线管理",
      rank: 7
    },
    children: [
      {
        path: "/admin/online/index",
        name: "OnlineManageIndex",
        component: () => import("@/views/admin/online/index.vue"),
        meta: {
          title: "在线管理"
        }
      }
    ]
  },
  {
    path: "/admin/blacklist",
    name: "Blacklist",
    component: Layout,
    redirect: "/admin/blacklist/index",
    meta: {
      icon: "ep:circle-close",
      title: "黑名单",
      rank: 8
    },
    children: [
      {
        path: "/admin/blacklist/index",
        name: "BlacklistIndex",
        component: () => import("@/views/admin/blacklist/index.vue"),
        meta: {
          title: "黑名单"
        }
      }
    ]
  }
] satisfies Array<RouteConfigsTable>;
