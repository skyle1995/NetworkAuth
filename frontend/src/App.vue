<template>
  <el-config-provider :locale="currentLocale">
    <MaintenancePage v-if="showMaintenance" />
    <template v-else>
      <router-view />
      <ReDialog />
    </template>
  </el-config-provider>
</template>

<script lang="ts">
import { defineAsyncComponent, defineComponent, computed } from "vue";
import { useRoute } from "vue-router";
import { ElConfigProvider } from "element-plus";
import { ReDialog } from "@/components/ReDialog";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import { getConfig } from "@/config";
const MaintenancePage = defineAsyncComponent(
  () => import("@/views/error/503.vue")
);

export default defineComponent({
  name: "app",
  components: {
    [ElConfigProvider.name]: ElConfigProvider,
    ReDialog,
    MaintenancePage
  },
  setup() {
    const route = useRoute();
    const showMaintenance = computed(() => {
      const isMaintenance = getConfig()?.MaintenanceMode;
      if (!isMaintenance) return false;

      // 检查当前路由是否是白名单（后台、错误页、安装页）
      const path = route.path;
      if (
        path.startsWith("/admin") ||
        path.startsWith("/error") ||
        path === "/install"
      ) {
        return false;
      }
      return true;
    });

    return {
      showMaintenance
    };
  },
  computed: {
    currentLocale() {
      return zhCn;
    }
  }
});
</script>
