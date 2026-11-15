const Layout = () => import("@/layout/index.vue");

// ============================================================================
// 后台业务菜单分组（一号通改造后的信息架构）
// ============================================================================
//
// 分组依据是「资源归属」，不是「历史文件名」：
//
//   应用管理    —— 只放**单个应用**自己的东西（应用本体、它的运行实例）
//   账号中心  —— 全站唯一的账号、钱包、等级、注册/试用策略
//   卡密中心  —— 制卡与售卖模板
//   开发者      —— 全站唯一的接口定义，以及应用可引用的变量/函数
//
// 历史包袱说明：
//   · 「接口设置」原在应用管理下，但接口在全站是唯一的一套（不按应用区分），
//     放应用管理里会让人误以为要逐应用配置 → 已移入「开发者」。
//   · 「会员等级」原在卡密管理下，但等级属于会员体系本身，卡密只是升级途径之一
//     → 已移入「账号中心」。
//   · 「账号策略（全站）」原藏在应用列表的「更多」下拉里（点开某个应用才能改全站策略）
//     → 已独立成页，位于「账号中心 → 账号策略」。
//   · 「在线管理」现在是**应用激活实例**的视图（二级令牌后登录不再建会话）
//     → 改名为「应用运行」，归入应用管理。

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
        path: "/admin/online/index",
        name: "OnlineManageIndex",
        component: () => import("@/views/admin/online/index.vue"),
        meta: {
          title: "应用运行"
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
      title: "账号中心",
      rank: 2
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
        path: "/admin/cards/levels",
        name: "LevelsIndex",
        component: () => import("@/views/admin/application/levels/index.vue"),
        meta: {
          title: "会员等级"
        }
      },
      {
        path: "/admin/account/policy",
        name: "AccountPolicyIndex",
        component: () => import("@/views/admin/account/policy/index.vue"),
        meta: {
          title: "账号策略"
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
  },
  {
    path: "/admin/card",
    name: "CardManage",
    component: Layout,
    redirect: "/admin/cards/index",
    meta: {
      icon: "ep:tickets",
    title: "卡密中心",
      rank: 3
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
      }
    ]
  },
  {
    path: "/admin/dev",
    name: "DevManage",
    component: Layout,
    redirect: "/admin/apis/index",
    meta: {
      icon: "ep:set-up",
      title: "开发配置",
      rank: 4
    },
    children: [
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
  }
] satisfies Array<RouteConfigsTable>;
