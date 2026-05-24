export default {
  path: "/install",
  name: "Install",
  component: () => import("@/views/install/index.vue"),
  meta: {
    title: "系统安装",
    showLink: false
  }
} satisfies RouteConfigsTable;
