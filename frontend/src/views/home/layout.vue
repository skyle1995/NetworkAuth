<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { isUrl, storageLocal } from "@pureadmin/utils";
import { type DataInfo } from "@/utils/auth";
import { getConfig } from "@/config";
import {
  getPublicPortalNavigationList,
  type PortalNavigationItem
} from "@/api/admin/portalNavigation";
import MenuIcon from "~icons/ri/menu-line";
import { useDataThemeChange } from "@/layout/hooks/useDataThemeChange";

import dayIcon from "@/assets/svg/day.svg?component";
import darkIcon from "@/assets/svg/dark.svg?component";

defineOptions({
  name: "HomeLayout"
});

const router = useRouter();
const route = useRoute();
const navMenus = ref<PortalNavigationItem[]>([]);
const adminMenu = ref<PortalNavigationItem | null>(null);

type HomeMenuItem = PortalNavigationItem & {
  children?: PortalNavigationItem[];
};

const { dataTheme, overallStyle, dataThemeChange } = useDataThemeChange();
// 页面加载时根据本地存储初始化主题
dataThemeChange(overallStyle.value);

// 当前激活的菜单项
const activeMenu = ref(route.path);

// 监听路由变化更新菜单高亮
watch(
  () => route.path,
  newPath => {
    activeMenu.value = newPath;
  }
);

// 检查管理员是否已登录
const isLogged = computed(() => {
  const userInfo = storageLocal().getItem<DataInfo<number>>("admin-info");
  return !!userInfo;
});

// 跳转逻辑：如果已登录去控制台，未登录去登录页
const goAdmin = () => {
  if (isLogged.value) {
    router.push("/admin/dashboard");
  } else {
    router.push("/admin/login");
  }
};

/**
 * 获取指定路径对应的导航项
 * @param key 导航路径
 * @returns 匹配到的导航项
 */
const getMatchedMenu = (key: string) => {
  return (
    navMenus.value.find(item => item.path === key) ||
    (adminMenu.value?.path === key ? adminMenu.value : null)
  );
};

/**
 * 获取当前配置的首页路径
 * @param menus 门户导航列表
 * @returns 首页跳转路径
 */
const getConfiguredHomePath = (menus: PortalNavigationItem[]) => {
  return menus.find(item => item.is_home && item.path)?.path || "/home/index";
};

/**
 * 按配置的首页路径执行跳转
 * @param path 首页路径
 * @returns void
 */
const navigateToConfiguredHome = (path: string) => {
  switch (path) {
    case "/home":
      router.push("/home/index");
      return;
    case "admin":
      goAdmin();
      return;
    default:
      if (isUrl(path) || getMatchedMenu(path)?.is_external) {
        window.location.assign(path);
        return;
      }
      router.push(path);
      return;
  }
};

/**
 * 处理站点 Logo 点击
 * @returns void
 */
const handleLogoClick = () => {
  navigateToConfiguredHome(getConfiguredHomePath(navMenus.value));
};

/**
 * 同步首页配置到当前路由
 * 仅当访问 /home 入口页时，自动跳到 is_home 对应的导航
 * @param menus 门户导航列表
 * @returns void
 */
const syncConfiguredHomeRoute = (menus: PortalNavigationItem[]) => {
  if (route.path !== "/home") {
    return;
  }

  const homePath = getConfiguredHomePath(menus);
  switch (homePath) {
    case "/home":
      router.replace("/home/index");
      return;
    case "/home/index":
      router.replace(homePath);
      return;
    case "admin":
      goAdmin();
      return;
    default:
      if (isUrl(homePath) || getMatchedMenu(homePath)?.is_external) {
        window.location.replace(homePath);
        return;
      }
      router.replace(homePath);
      return;
  }
};

const handleSelect = (key: string) => {
  const matchedMenu = getMatchedMenu(key);

  switch (key) {
    case "admin":
      goAdmin();
      break;
    default:
      if (matchedMenu?.is_external || isUrl(key)) {
        window.open(key, "_blank");
        return;
      }
      router.push(key);
      break;
  }
};

/**
 * 获取门户公开导航
 * 从后台读取当前可见导航项并用于顶部菜单展示
 */
const fetchPortalNavigationList = async () => {
  try {
    const res = await getPublicPortalNavigationList();
    switch (res.code) {
      case 0:
        navMenus.value = (res.data || []).filter(item => item.path !== "admin");
        adminMenu.value =
          (res.data || []).find(item => item.path === "admin") || null;
        syncConfiguredHomeRoute(res.data || []);
        break;
      default:
        navMenus.value = [];
        adminMenu.value = null;
        break;
    }
  } catch {
    navMenus.value = [];
    adminMenu.value = null;
  }
};

const topMenus = computed<HomeMenuItem[]>(() => {
  const topLevelItems = navMenus.value
    .filter(item => !item.parent_id)
    .sort((a, b) => a.sort - b.sort || a.id - b.id);

  return topLevelItems.map(item => {
    if (item.type !== "group") {
      return item;
    }
    return {
      ...item,
      children: navMenus.value
        .filter(child => child.parent_id === item.id)
        .sort((a, b) => a.sort - b.sort || a.id - b.id)
    };
  });
});

const mobileMenus = computed(() => {
  return topMenus.value.flatMap(item => {
    if (item.type !== "group") {
      return [{ ...item, is_child: false, group_name: "" }];
    }
    const groupHeader = {
      ...item,
      is_child: false,
      group_name: item.name,
      is_group_header: true
    };
    const children = (item.children || []).map(child => ({
      ...child,
      is_child: true,
      group_name: item.name
    }));
    return [groupHeader, ...children];
  });
});

/**
 * 获取导航显示名称
 * 管理员入口在登录后统一展示为进入控制台
 */
const getMenuName = (menu: PortalNavigationItem) => {
  switch (menu.path) {
    case "admin":
      return isLogged.value ? "进入控制台" : menu.name;
    default:
      return menu.name;
  }
};

/**
 * 判断菜单是否激活
 * 对首页链接额外兼容 /home 与 /home/index 两种路径
 */
const isMenuActive = (menu: PortalNavigationItem) => {
  if (menu.is_home) {
    return (
      activeMenu.value === menu.path ||
      activeMenu.value === "/home" ||
      (activeMenu.value === "/home/index" && menu.path === "/home")
    );
  }
  return activeMenu.value === menu.path;
};

const isGroupActive = (menu: HomeMenuItem) => {
  return (menu.children || []).some(child => isMenuActive(child));
};

onMounted(() => {
  fetchPortalNavigationList();
});

// 动态动作菜单配置方案 (控制按钮/下拉显隐)
const actionMenus = computed(() => {
  return {
    adminMenu: adminMenu.value
  };
});
</script>

<template>
  <div class="front-home-layout">
    <!-- 导航栏 -->
    <header class="header">
      <div class="header-content">
        <!-- Logo -->
        <div class="logo" style="cursor: pointer" @click="handleLogoClick">
          <img
            v-if="getConfig()?.Logo"
            :src="getConfig()?.Logo"
            alt="logo"
            class="logo-img"
          />
          <span class="logo-text">{{ getConfig()?.Title || "加载中..." }}</span>
        </div>

        <!-- 移动端下拉菜单 -->
        <div class="mobile-nav">
          <el-switch
            v-model="dataTheme"
            inline-prompt
            :active-icon="dayIcon"
            :inactive-icon="darkIcon"
            class="theme-switch"
            @change="dataThemeChange"
          />
          <el-dropdown trigger="click" @command="handleSelect">
            <el-button class="mobile-menu-btn" text>
              <el-icon :size="24"><MenuIcon /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <!-- 动态渲染中间菜单项 -->
                <el-dropdown-item
                  v-for="menu in mobileMenus"
                  :key="menu.id"
                  :command="menu.type === 'group' ? undefined : menu.path"
                  :disabled="menu.type === 'group'"
                  :style="
                    menu.type === 'group'
                      ? 'font-weight: 600; color: var(--el-text-color-primary)'
                      : isMenuActive(menu)
                        ? 'color: var(--el-color-primary); background-color: var(--el-color-primary-light-9)'
                        : ''
                  "
                >
                  {{
                    menu.is_child ? `  ${getMenuName(menu)}` : getMenuName(menu)
                  }}
                </el-dropdown-item>

                <el-dropdown-item
                  v-if="actionMenus.adminMenu"
                  divided
                  :command="actionMenus.adminMenu.path"
                >
                  {{ getMenuName(actionMenus.adminMenu) }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>

        <!-- 桌面端中间导航菜单 -->
        <div class="nav-menu desktop-nav">
          <div class="desktop-menu-list">
            <template v-for="menu in topMenus" :key="menu.id">
              <el-dropdown
                v-if="menu.type === 'group'"
                trigger="hover"
                placement="bottom"
                popper-class="home-nav-dropdown"
                @command="handleSelect"
              >
                <div
                  class="nav-item nav-group"
                  :class="{ 'is-active': isGroupActive(menu) }"
                >
                  <span>{{ getMenuName(menu) }}</span>
                </div>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="child in menu.children || []"
                      :key="child.id"
                      :command="child.path"
                      :class="{ 'is-active': isMenuActive(child) }"
                    >
                      {{ getMenuName(child) }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <div
                v-else
                class="nav-item"
                :class="{ 'is-active': isMenuActive(menu) }"
                @click="handleSelect(menu.path)"
              >
                {{ getMenuName(menu) }}
              </div>
            </template>
          </div>
        </div>

        <!-- 右侧操作 -->
        <div class="actions desktop-nav">
          <el-switch
            v-model="dataTheme"
            inline-prompt
            :active-icon="dayIcon"
            :inactive-icon="darkIcon"
            class="theme-switch"
            @change="dataThemeChange"
          />
          <el-button
            v-if="actionMenus.adminMenu"
            type="primary"
            @click="handleSelect(actionMenus.adminMenu.path)"
          >
            {{ getMenuName(actionMenus.adminMenu) }}
          </el-button>
        </div>
      </div>
    </header>

    <!-- 主体内容 -->
    <main class="main-content">
      <router-view v-slot="{ Component }">
        <transition name="el-fade-in" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>

    <!-- 页脚 -->
    <footer class="footer">
      <div class="footer-content">
        <p>
          {{
            getConfig()?.FooterText ??
            "Copyright © 2026 NetworkAuth. All Rights Reserved."
          }}
        </p>
        <div
          v-if="getConfig()?.IcpRecord || getConfig()?.PsbRecord"
          class="record-info"
        >
          <a
            v-if="getConfig()?.IcpRecord"
            :href="getConfig()?.IcpRecordLink || 'https://beian.miit.gov.cn'"
            target="_blank"
            class="record-link"
          >
            {{ getConfig()?.IcpRecord }}
          </a>
          <span
            v-if="getConfig()?.IcpRecord && getConfig()?.PsbRecord"
            class="divider"
            >|</span
          >
          <a
            v-if="getConfig()?.PsbRecord"
            :href="
              getConfig()?.PsbRecordLink ||
              'http://www.beian.gov.cn/portal/registerSystemInfo'
            "
            target="_blank"
            class="record-link"
          >
            {{ getConfig()?.PsbRecord }}
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped lang="scss">
.front-home-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  @supports (min-height: 100dvh) {
    min-height: 100dvh;
  }
  font-family:
    -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue",
    Arial, sans-serif;
  background-color: var(--el-bg-color-page);
  color: var(--el-text-color-primary);
  transition:
    background-color 0.3s,
    color 0.3s;
}

.header {
  position: sticky;
  top: 0;
  z-index: 10;
  background-color: var(--el-bg-color-overlay);
  backdrop-filter: blur(10px);
  box-shadow: var(--el-box-shadow-light);
  transition:
    background-color 0.3s,
    box-shadow 0.3s;

  .header-content {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 64px;
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 20px;
  }
}

.logo {
  display: flex;
  gap: 10px;
  align-items: center;
  min-width: 200px;
}

.logo-img {
  width: auto;
  height: 32px;
}

.logo-text {
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  transition: color 0.3s;
}

.mobile-nav {
  display: none;
}

.theme-switch {
  margin-right: 15px;
}

.nav-menu {
  flex: 1;
  display: flex;
  justify-content: flex-start;
  margin-left: 20px;
}

.desktop-menu-list {
  display: flex;
  align-items: stretch;
  gap: 4px;
}

.nav-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  height: 64px;
  padding: 0 18px;
  color: var(--el-text-color-regular);
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  user-select: none;
  transition:
    color 0.2s ease,
    background-color 0.2s ease;

  &::after {
    content: "";
    position: absolute;
    left: 14px;
    right: 14px;
    bottom: 0;
    height: 2px;
    border-radius: 999px;
    background: transparent;
    transition: background-color 0.2s ease;
  }

  &:hover {
    color: var(--el-color-primary);
  }

  &.is-active {
    color: var(--el-color-primary);

    &::after {
      background: var(--el-color-primary);
    }
  }
}

.nav-group {
  gap: 4px;
}

:deep(.el-dropdown-menu__item) {
  min-width: 120px;
}

:global(.home-nav-dropdown .el-dropdown-menu__item.is-active) {
  color: var(--el-color-primary);
  background-color: var(--el-color-primary-light-9);
  font-weight: 600;
}

.actions {
  min-width: 200px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.main-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  padding: 20px;
  width: 100%;
  box-sizing: border-box;

  &::before,
  &::after {
    content: "";
    flex: 1;
  }

  > * {
    flex-shrink: 0;
  }
}

.footer {
  padding: 20px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
  text-align: center;
  transition: color 0.3s;

  .footer-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .record-info {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    margin-top: 4px;
    flex-wrap: wrap;

    .record-link {
      color: var(--el-text-color-secondary);
      text-decoration: none;
      transition: color 0.3s;

      &:hover {
        color: var(--el-color-primary);
      }
    }

    .divider {
      color: var(--el-border-color);
      font-size: 12px;
      transition: color 0.3s;
    }
  }
}

@media screen and (max-width: 768px) {
  .header-content {
    padding: 0 10px;
  }

  .logo {
    min-width: auto;
  }

  .logo-text {
    display: inline-block;
    font-size: 18px;
  }

  .desktop-nav {
    display: none !important;
  }

  .mobile-nav {
    display: flex;
    flex: 1;
    align-items: center;
    justify-content: flex-end;
    margin-left: 10px;

    .mobile-menu-btn {
      padding: 8px;
    }
  }

  .actions {
    min-width: auto;

    .el-button {
      padding: 8px 10px;
      font-size: 13px;
    }
  }
}
</style>
