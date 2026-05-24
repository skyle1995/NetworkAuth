export default {
  path: "/home",
  name: "HomeIndex",
  component: () => import("@/views/home/layout.vue"),
  meta: {
    title: "首页",
    showLink: false
  },
  children: [
    {
      path: "/home/index",
      name: "HomeMain",
      component: () => import("@/views/home/index.vue"),
      meta: {
        title: "首页",
        showLink: false
      }
    }
  ]
} satisfies RouteConfigsTable;
