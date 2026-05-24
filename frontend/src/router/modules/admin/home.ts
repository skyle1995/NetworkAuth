const { VITE_HIDE_HOME } = import.meta.env;
const Layout = () => import("@/layout/index.vue");

export default {
  path: "/admin",
  name: "Home",
  component: Layout,
  redirect: "/admin/dashboard",
  meta: {
    icon: "ep:home-filled",
    title: "仪表盘",
    rank: 0
  },
  children: [
    {
      path: "/admin/dashboard",
      name: "Dashboard",
      component: () => import("@/views/admin/dashboard/index.vue"),
      meta: {
        title: "仪表盘",
        showLink: VITE_HIDE_HOME === "true" ? false : true
      }
    }
  ]
} satisfies RouteConfigsTable;
