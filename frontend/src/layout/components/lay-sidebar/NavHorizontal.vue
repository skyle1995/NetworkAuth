<script setup lang="ts">
import { emitter } from "@/utils/mitt";
import { useNav } from "@/layout/hooks/useNav";
import LaySearch from "../lay-search/index.vue";
import { responsiveStorageNameSpace } from "@/config";
import { ref, nextTick, computed, onMounted } from "vue";
import { storageLocal, isAllEmpty } from "@pureadmin/utils";
import { usePermissionStoreHook } from "@/store/modules/permission";
import LaySidebarItem from "../lay-sidebar/components/SidebarItem.vue";
import LaySidebarFullScreen from "../lay-sidebar/components/SidebarFullScreen.vue";

import LogoutCircleRLine from "~icons/ri/logout-circle-r-line";
import UserLine from "~icons/ri/user-3-line";
import Setting from "~icons/ri/settings-3-line";
import UserAvatar from "~icons/ep/user-filled";

const menuRef = ref();
const showLogo = ref(
  storageLocal().getItem<StorageConfigs>(
    `${responsiveStorageNameSpace()}configure`
  )?.showLogo ?? true
);

const {
  route,
  title,
  logout,
  onPanel,
  getLogo,
  username,
  userAvatar,
  backTopMenu,
  avatarsStyle
} = useNav();

const defaultActive = computed(() =>
  !isAllEmpty(route.meta?.activePath) ? route.meta.activePath : route.path
);

const menuData = computed(() => {
  const currentPath = route.path;
  const match = currentPath.match(/^\/([^\/]+)/);
  const rootPath = match ? match[0] : "/admin";
  return usePermissionStoreHook().wholeMenus.filter(m =>
    m.path.startsWith(rootPath)
  );
});

nextTick(() => {
  menuRef.value?.handleResize();
});

onMounted(() => {
  emitter.on("logoChange", key => {
    showLogo.value = key;
  });
});
</script>

<template>
  <div v-loading="menuData.length === 0" class="horizontal-header">
    <div v-if="showLogo" class="horizontal-header-left" @click="backTopMenu">
      <img :src="getLogo()" alt="logo" />
      <span>{{ title }}</span>
    </div>
    <el-menu
      ref="menuRef"
      mode="horizontal"
      popper-class="pure-scrollbar"
      class="horizontal-header-menu"
      :default-active="defaultActive"
    >
      <LaySidebarItem
        v-for="route in menuData"
        :key="route.path"
        :item="route"
        :base-path="route.path"
      />
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
