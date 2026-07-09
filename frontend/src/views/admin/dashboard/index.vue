<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import {
  getSystemInfo,
  getLoginLogs,
  getSystemStats
} from "@/api/admin/dashboard";
import {
  getSelfUpdateStatus,
  checkSelfUpdate,
  type SelfUpdateStatus
} from "@/api/admin/selfUpdate";
import dayjs from "dayjs";
import { ElMessage } from "element-plus";
import { useRouter } from "vue-router";

defineOptions({
  name: "Index"
});

const router = useRouter();

// 软件更新状态（用于基本信息里的“有新版本”提示）
const updateStatus = ref<SelfUpdateStatus | null>(null);
const hasUpdate = computed(() => {
  const latest = updateStatus.value?.latest_version?.replace(/^v/i, "") || "";
  const current = systemInfo.value.version?.replace(/^v/i, "") || "";
  if (!latest || !current) return false;
  const a = current.split(".").map(n => parseInt(n, 10) || 0);
  const b = latest.split(".").map(n => parseInt(n, 10) || 0);
  const len = Math.max(a.length, b.length);
  for (let i = 0; i < len; i++) {
    const x = a[i] || 0;
    const y = b[i] || 0;
    if (y > x) return true;
    if (y < x) return false;
  }
  return false;
});

const fetchUpdateStatus = async () => {
  try {
    const res = await getSelfUpdateStatus();
    if (res.ok && res.data) {
      updateStatus.value = res.data;
      // 状态过期（>10 分钟）则触发一次后台检查（仅超管有权限，无权时静默忽略）
      const now = Math.floor(Date.now() / 1000);
      const checkedAt = res.data.checked_at || 0;
      if (checkedAt <= 0 || now - checkedAt >= 600) {
        const chk = await checkSelfUpdate();
        if (chk.ok && chk.data) updateStatus.value = chk.data;
      }
    }
  } catch {
    // 忽略（如无权限/未配置）
  }
};

// 快捷入口
const quickLinks = [
  { label: "应用管理", type: "primary", path: "/admin/apps/index" },
  { label: "终端账号", type: "success", path: "/admin/members/index" },
  { label: "卡密管理", type: "warning", path: "/admin/cards/index" },
  { label: "接口设置", type: "info", path: "/admin/apis/index" },
  { label: "变量管理", type: "primary", path: "/admin/variables/index" },
  { label: "函数管理", type: "success", path: "/admin/functions/index" },
  { label: "系统设置", type: "info", path: "/admin/system/settings" },
  { label: "操作日志", type: "warning", path: "/admin/logs/operation" }
] as const;

const systemInfo = ref({
  version: "",
  mode: false,
  db_type: "",
  uptime: "",
  uptime_seconds: 0
});

const systemStats = ref({
  total_apps: 0,
  enabled_apps: 0,
  total_members: 0,
  normal_members: 0,
  disabled_members: 0,
  black_members: 0,
  today_new_members: 0,
  total_cards: 0,
  unused_cards: 0,
  used_cards: 0,
  frozen_cards: 0,
  total_apis: 0,
  total_functions: 0,
  total_variables: 0,
  online_sessions: 0
});

// 核心数据概览（彩色数字卡）
const overviewItems = computed(() => [
  {
    label: "应用总数",
    value: systemStats.value.total_apps,
    cls: "stat-primary"
  },
  {
    label: "启用应用",
    value: systemStats.value.enabled_apps,
    cls: "stat-success"
  },
  {
    label: "终端账号",
    value: systemStats.value.total_members,
    cls: "stat-primary"
  },
  {
    label: "今日新增",
    value: systemStats.value.today_new_members,
    cls: "stat-success"
  },
  {
    label: "在线会话",
    value: systemStats.value.online_sessions,
    cls: "stat-warning"
  },
  {
    label: "卡密总数",
    value: systemStats.value.total_cards,
    cls: "stat-primary"
  }
]);

// 账号状态分布
const memberItems = computed(() => [
  {
    label: "正常",
    value: systemStats.value.normal_members,
    cls: "stat-success"
  },
  {
    label: "封停",
    value: systemStats.value.disabled_members,
    cls: "stat-warning"
  },
  {
    label: "黑名单",
    value: systemStats.value.black_members,
    cls: "stat-danger"
  }
]);

// 卡密状态分布
const cardItems = computed(() => [
  {
    label: "未使用",
    value: systemStats.value.unused_cards,
    cls: "stat-primary"
  },
  { label: "已使用", value: systemStats.value.used_cards, cls: "stat-success" },
  { label: "冻结", value: systemStats.value.frozen_cards, cls: "stat-info" }
]);

const loginLogs = ref([]);
const totalLogs = ref(0);
const loading = ref(false);
const pagination = ref({
  currentPage: 1,
  pageSize: 30
});

const fetchSystemInfo = async () => {
  try {
    const res = await getSystemInfo();
    if (res.code === 0) {
      systemInfo.value = res.data;
    }
  } catch (error: any) {
    console.error(
      "Failed to fetch system info",
      error.response?.status,
      error.message,
      error
    );
    ElMessage.error(error.response?.data?.message || "获取系统信息失败");
  }
};

const fetchSystemStats = async () => {
  try {
    const res = await getSystemStats();
    if (res.code === 0) {
      systemStats.value = res.data;
    }
  } catch (error: any) {
    console.error(
      "Failed to fetch system stats",
      error.response?.status,
      error.message,
      error
    );
    ElMessage.error(error.response?.data?.message || "获取系统统计失败");
  }
};

const fetchLoginLogs = async () => {
  loading.value = true;
  try {
    const res = await getLoginLogs({
      page: pagination.value.currentPage,
      limit: pagination.value.pageSize
    });
    if (res.code === 0) {
      loginLogs.value = res.data.list || [];
      totalLogs.value = res.data.total || 0;
    }
  } catch (error: any) {
    console.error(
      "Failed to fetch login logs",
      error.response?.status,
      error.message,
      error
    );
    ElMessage.error(error.response?.data?.message || "获取登录日志失败");
  } finally {
    loading.value = false;
  }
};

const handleSizeChange = (val: number) => {
  pagination.value.pageSize = val;
  fetchLoginLogs();
};

const handleCurrentChange = (val: number) => {
  pagination.value.currentPage = val;
  fetchLoginLogs();
};

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format("YYYY-MM-DD HH:mm:ss");
};

const emptyFormatter = (row: any, column: any, cellValue: any) => {
  return cellValue === "" || cellValue === null || cellValue === undefined
    ? "-"
    : cellValue;
};

let timer: any = null;

const formattedUptime = computed(() => {
  const totalSeconds = systemInfo.value.uptime_seconds;
  if (!totalSeconds) return systemInfo.value.uptime || "未知";

  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (days > 0) {
    return `${days}天 ${hours}小时 ${minutes}分钟 ${seconds}秒`;
  } else if (hours > 0) {
    return `${hours}小时 ${minutes}分钟 ${seconds}秒`;
  } else if (minutes > 0) {
    return `${minutes}分钟 ${seconds}秒`;
  } else {
    return `${seconds}秒`;
  }
});

onMounted(() => {
  fetchSystemInfo();
  fetchSystemStats();
  fetchLoginLogs();
  fetchUpdateStatus();

  timer = setInterval(() => {
    if (systemInfo.value.uptime_seconds > 0) {
      systemInfo.value.uptime_seconds++;
    }
  }, 1000);
});

onUnmounted(() => {
  if (timer) {
    clearInterval(timer);
  }
});
</script>

<template>
  <div>
    <el-row :gutter="20" class="mt-4">
      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>基本信息</span>
            </div>
          </template>
          <div class="flex justify-around items-center h-[120px]">
            <div
              class="text-center cursor-pointer hover:opacity-80 transition-opacity"
              @click="router.push('/admin/system/system-update')"
            >
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                项目版本
              </div>
              <div class="text-2xl font-bold text-[var(--el-color-primary)]">
                {{ systemInfo.version || "-" }}
              </div>
              <div v-if="hasUpdate" class="mt-1">
                <el-tag type="danger" size="small" round>
                  最新 {{ updateStatus?.latest_version }}
                </el-tag>
              </div>
            </div>
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                运行环境
              </div>
              <div
                class="text-2xl font-bold"
                :class="
                  systemInfo.mode
                    ? 'text-[var(--el-color-warning)]'
                    : 'text-[var(--el-color-success)]'
                "
              >
                {{ systemInfo.mode ? "开发环境" : "生产环境" }}
              </div>
            </div>
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                数据库
              </div>
              <div class="text-2xl font-bold text-[var(--el-color-info)]">
                {{ systemInfo.db_type || "-" }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>运行状态</span>
            </div>
          </template>
          <div
            class="uptime-display flex justify-center items-center h-[120px]"
          >
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                系统已运行时间
              </div>
              <div
                class="text-3xl font-bold text-[var(--el-text-color-primary)]"
              >
                {{ formattedUptime }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>资源统计</span>
            </div>
          </template>
          <div class="flex justify-around items-center h-[120px]">
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                接口总数
              </div>
              <div class="text-2xl font-bold text-[var(--el-color-info)]">
                {{ systemStats.total_apis }}
              </div>
            </div>
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                函数总数
              </div>
              <div class="text-2xl font-bold text-[var(--el-color-warning)]">
                {{ systemStats.total_functions }}
              </div>
            </div>
            <div class="text-center">
              <div class="text-[var(--el-text-color-secondary)] text-sm mb-2">
                变量总数
              </div>
              <div class="text-2xl font-bold text-[var(--el-color-primary)]">
                {{ systemStats.total_variables }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 快捷入口 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header"><span>快捷入口</span></div>
          </template>
          <div class="flex flex-wrap gap-3">
            <el-button
              v-for="link in quickLinks"
              :key="link.path"
              :type="link.type"
              plain
              @click="router.push(link.path)"
            >
              {{ link.label }}
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 数据概览 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header"><span>数据概览</span></div>
          </template>
          <el-row :gutter="16" class="stat-overview">
            <el-col
              v-for="item in overviewItems"
              :key="item.label"
              :xs="12"
              :sm="8"
              :md="4"
            >
              <div class="stat-item">
                <div class="stat-num" :class="item.cls">{{ item.value }}</div>
                <div class="stat-label">{{ item.label }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>

    <!-- 账号 / 卡密 状态分布 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header"><span>终端账号状态</span></div>
          </template>
          <el-row :gutter="16" class="stat-overview">
            <el-col v-for="item in memberItems" :key="item.label" :span="8">
              <div class="stat-item">
                <div class="stat-num" :class="item.cls">{{ item.value }}</div>
                <div class="stat-label">{{ item.label }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header"><span>卡密状态</span></div>
          </template>
          <el-row :gutter="16" class="stat-overview">
            <el-col v-for="item in cardItems" :key="item.label" :span="8">
              <div class="stat-item">
                <div class="stat-num" :class="item.cls">{{ item.value }}</div>
                <div class="stat-label">{{ item.label }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>最近登录日志</span>
            </div>
          </template>
          <el-table
            v-loading="loading"
            :data="loginLogs"
            style="width: 100%"
            table-layout="auto"
            show-overflow-tooltip
            border
          >
            <el-table-column prop="created_at" label="登录时间" width="180">
              <template #default="scope">
                {{ formatDate(scope.row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column
              prop="username"
              label="用户名"
              width="150"
              :formatter="emptyFormatter"
            />
            <el-table-column
              prop="ip"
              label="登录IP"
              width="200"
              :formatter="emptyFormatter"
            />
            <el-table-column
              prop="status"
              label="登录状态"
              width="100"
              align="center"
            >
              <template #default="scope">
                <el-tag :type="scope.row.status === 1 ? 'success' : 'danger'">
                  {{ scope.row.status === 1 ? "成功" : "失败" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column
              prop="message"
              label="登录信息"
              width="240"
              :formatter="emptyFormatter"
            />
            <el-table-column
              prop="user_agent"
              label="浏览器类型"
              min-width="320"
              show-overflow-tooltip
              :formatter="emptyFormatter"
            />
          </el-table>
          <div class="flex mt-4 w-full overflow-x-auto">
            <div class="ml-auto shrink-0">
              <el-pagination
                v-model:current-page="pagination.currentPage"
                v-model:page-size="pagination.pageSize"
                :page-sizes="[10, 20, 30, 50, 100, 200, 500, 1000]"
                :background="true"
                layout="total, sizes, prev, pager, next, jumper"
                :total="totalLogs"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.card-header {
  font-weight: bold;
}

.uptime-display {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 120px;
}

.stat-overview .stat-item {
  padding: 12px 0;
  text-align: center;
}

.stat-overview .stat-num {
  font-size: 26px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--el-text-color-primary);
}

.stat-overview .stat-label {
  margin-top: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.stat-overview .stat-primary {
  color: var(--el-color-primary);
}

.stat-overview .stat-success {
  color: var(--el-color-success);
}

.stat-overview .stat-warning {
  color: var(--el-color-warning);
}

.stat-overview .stat-danger {
  color: var(--el-color-danger);
}

.stat-overview .stat-info {
  color: var(--el-color-info);
}
</style>
