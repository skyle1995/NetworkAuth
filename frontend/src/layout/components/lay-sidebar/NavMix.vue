<script setup lang="ts">
import { isAllEmpty } from "@pureadmin/utils";
import { useNav } from "@/layout/hooks/useNav";
import LaySearch from "../lay-search/index.vue";
import { ref, toRaw, watch, onMounted, nextTick, computed } from "vue";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { getParentPaths, findRouteByPath } from "@/router/utils";
import { usePermissionStoreHook } from "@/store/modules/permission";
import LaySidebarExtraIcon from "../lay-sidebar/components/SidebarExtraIcon.vue";
import LaySidebarFullScreen from "../lay-sidebar/components/SidebarFullScreen.vue";

import LogoutCircleRLine from "~icons/ri/logout-circle-r-line";
import UserLine from "~icons/ri/user-3-line";
import Setting from "~icons/ri/settings-3-line";
import UserAvatar from "~icons/ep/user-filled";

const menuRef = ref();
const defaultActive = ref(null);

const {
  route,
  device,
  logout,
  onPanel,
  resolvePath,
  username,
  userAvatar,
  getDivStyle,
  avatarsStyle
} = useNav();

const menuData = computed(() => {
  const currentPath = route.path;
  const match = currentPath.match(/^\/([^\/]+)/);
  const rootPath = match ? match[0] : "/admin";
  return usePermissionStoreHook().wholeMenus.filter(m =>
    m.path.startsWith(rootPath)
  );
});

function getDefaultActive(routePath) {
  const wholeMenus = menuData.value;
  /** 当前路由的父级路径 */
  const parentRoutes = getParentPaths(routePath, wholeMenus)[0];
  defaultActive.value = !isAllEmpty(route.meta?.activePath)
    ? route.meta.activePath
    : findRouteByPath(parentRoutes, wholeMenus)?.children[0]?.path;
}

onMounted(() => {
  getDefaultActive(route.path);
});

nextTick(() => {
  menuRef.value?.handleResize();
});

watch(
  () => [route.path, usePermissionStoreHook().wholeMenus],
  () => {
    getDefaultActive(route.path);
  }
);
</script>

<template>
  <div
    v-if="device !== 'mobile'"
    v-loading="menuData.length === 0"
    class="horizontal-header"
  >
    <el-menu
      ref="menuRef"
      router
      mode="horizontal"
      popper-class="pure-scrollbar"
      class="horizontal-header-menu"
      :default-active="defaultActive"
    >
      <el-menu-item
        v-for="route in menuData"
        :key="route.path"
        :index="resolvePath(route) || route.redirect"
      >
        <template #title>
          <div
            v-if="toRaw(route.meta.icon)"
            :class="['sub-menu-icon', route.meta.icon]"
          >
            <component
              :is="useRenderIcon(route.meta && toRaw(route.meta.icon))"
            />
          </div>
          <div :style="getDivStyle">
            <span class="select-none">
              {{ route.meta.title }}
            </span>
            <LaySidebarExtraIcon :extraIcon="route.meta.extraIcon" />
          </div>
        </template>
      </el-menu-item>
    </el-menu>
    <div class="horizontal-header-right">
      <!-- 菜单搜索 -->
      <LaySearch id="header-search" />
      <!-- 全屏 -->
      <LaySidebarFullScreen id="full-screen" />
      <!-- 退出登录 -->
      <el-dropdown trigger="click">
        <span class="el-dropdown-link navbar-bg-hover">
          <img v-if="userAvatar" :src="userAvatar" :style="avatarsStyle" />
          <el-avatar
            v-else
            :size="24"
            :style="avatarsStyle"
            style="
              color: inherit;
              background: transparent;
              border: 1px solid var(--el-border-color);
            "
          >
            <IconifyIconOffline :icon="UserAvatar" />
          </el-avatar>
          <p v-if="username" class="dark:text-white">{{ username }}</p>
        </span>
        <template #dropdown>
          <el-dropdown-menu class="logout">
            <el-dropdown-item @click="$router.push('/admin/system/profile')">
              <IconifyIconOffline :icon="UserLine" style="margin: 5px" />
              个人资料
            </el-dropdown-item>
            <el-dropdown-item @click="logout">
              <IconifyIconOffline
                :icon="LogoutCircleRLine"
                style="margin: 5px"
              />
              退出系统
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <span
        class="set-icon navbar-bg-hover"
        title="打开系统配置"
        @click="onPanel"
      >
        <IconifyIconOffline :icon="Setting" />
      </span>
    </div>
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-loading-mask) {
  opacity: 0.45;
}

.logout {
  width: 120px;

  ::v-deep(.el-dropdown-menu__item) {
    display: inline-flex;
    flex-wrap: wrap;
    min-width: 100%;
  }
}
</style>
