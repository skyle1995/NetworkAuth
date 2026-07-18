const Layout = () => import("@/layout/index.vue");

// 后台业务菜单分为三组：应用管理 / 卡密管理 / 账号管理，紧随仪表盘(rank 0)之后。
export default [
  {
    path: "/admin/app",
    name: "AppManage",
    component: Layout,
    redirect: "/admin/apps/index",
    meta: {
      icon: "ep:grid",
      title: "应用管理",
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
      },
      {
        path: "/admin/apis/index",
        name: "ApisIndex",
        component: () => import("@/views/admin/application/apis/index.vue"),
        meta: {
          title: "接口设置"
        }
      },
      {
        path: "/admin/variables/index",
        name: "VariablesIndex",
        component: () =>
          import("@/views/admin/application/variables/index.vue"),
        meta: {
          title: "公共变量"
        }
      },
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
    path: "/admin/card",
    name: "CardManage",
    component: Layout,
    redirect: "/admin/cards/index",
    meta: {
      icon: "ep:tickets",
      title: "卡密管理",
      rank: 2
    },
    children: [
      {
        path: "/admin/cards/index",
        name: "CardsIndex",
        component: () => import("@/views/admin/application/cards/index.vue"),
        meta: {
          title: "卡密管理"
        }
      },
      {
        path: "/admin/cards/packages",
        name: "PackagesIndex",
        component: () => import("@/views/admin/application/packages/index.vue"),
        meta: {
          title: "卡密套餐"
        }
      },
      {
        path: "/admin/cards/levels",
        name: "LevelsIndex",
        component: () => import("@/views/admin/application/levels/index.vue"),
        meta: {
          title: "会员等级"
        }
      }
    ]
  },
  {
    path: "/admin/account",
    name: "AccountManage",
    component: Layout,
    redirect: "/admin/members/index",
    meta: {
      icon: "ep:user",
      title: "账号管理",
      rank: 3
    },
    children: [
      {
        path: "/admin/members/index",
        name: "MembersIndex",
        component: () => import("@/views/admin/application/members/index.vue"),
        meta: {
          title: "终端账号"
        }
      },
      {
        path: "/admin/online/index",
        name: "OnlineManageIndex",
        component: () => import("@/views/admin/online/index.vue"),
        meta: {
          title: "在线管理"
        }
      },
      {
        path: "/admin/blacklist/index",
        name: "BlacklistIndex",
        component: () => import("@/views/admin/blacklist/index.vue"),
        meta: {
          title: "拉黑管理"
        }
      }
    ]
  }
] satisfies Array<RouteConfigsTable>;
