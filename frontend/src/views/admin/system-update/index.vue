<script setup lang="ts">
import { onMounted, ref, computed } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getSelfUpdateStatus,
  getSelfUpdateVersions,
  prepareSelfUpdate,
  checkSelfUpdate,
  checkSelfUpdateForce,
  restartSelfUpdate,
  getSelfUpdateConfig,
  updateSelfUpdateConfig,
  testSelfUpdateConfig,
  type SelfUpdateStatus,
  type SelfUpdateVersionItem,
  type SelfUpdateConfig
} from "@/api/admin/selfUpdate";

defineOptions({
  name: "SystemUpdateIndex"
});

const activeTab = ref("update");
const loading = ref(false);
const testLoading = ref(false);
const restarting = ref(false);

const status = ref<SelfUpdateStatus>({
  running: false,
  checked_at: 0,
  checked_at_str: "",
  last_error: "",
  current_version: "",
  latest_version: "",
  versions_count: 0,
  prepared: false,
  prepared_version: "",
  prepare_error: "",
  auto_replace_tried: false,
  auto_replace_ok: false,
  auto_replace_error: "",
  script_shell_path: "",
  script_powershell_path: "",
  download_progress: 0
});

const versions = ref<SelfUpdateVersionItem[]>([]);

// 版本列表按版本降序，第一个 is_newer 即可更新到的最新版
const latestNewer = computed(() => versions.value.find(v => v.is_newer));

const preparing = ref(false); // 仅"下载/准备"期间为真，用于区分"检查更新(扫描)"，避免检查时误显示下载进度
const isRunning = computed(() => status.value.running === true);
const isPrepared = computed(() => status.value.prepared === true);
const hasPrepareError = computed(
  () =>
    typeof status.value.prepare_error === "string" &&
    status.value.prepare_error.trim() !== ""
);
const autoReplaceTried = computed(
  () => status.value.auto_replace_tried === true
);
const autoReplaceOK = computed(() => status.value.auto_replace_ok === true);
const downloadProgress = computed(() =>
  Number(status.value.download_progress || 0)
);
const showDownloadCard = computed(
  () =>
    preparing.value ||
    isPrepared.value ||
    hasPrepareError.value ||
    autoReplaceTried.value ||
    downloadProgress.value > 0
);

const config = ref<SelfUpdateConfig>({
  type: 0,
  secret_id: "",
  secret_key: "",
  region: "",
  bucket: "",
  prefix: "NetworkAuth/",
  base_url: ""
});

const refreshStatus = async () => {
  try {
    const res = await getSelfUpdateStatus();
    if (res.ok && res.data) {
      status.value = res.data;
    }
  } catch {}
};

const refreshVersions = async () => {
  loading.value = true;
  try {
    const res = await getSelfUpdateVersions();
    if (res.ok && res.data) {
      versions.value = res.data;
    } else {
      versions.value = [];
    }
  } catch {
    versions.value = [];
  } finally {
    loading.value = false;
  }
};

const refreshConfig = async () => {
  try {
    const res = await getSelfUpdateConfig();
    if (res.ok && res.data) {
      config.value = { ...config.value, ...res.data };
    }
  } catch {}
};

const pollUntilDone = async () => {
  for (let i = 0; i < 300; i++) {
    await new Promise(resolve => setTimeout(resolve, 1000));
    await refreshStatus();
    if (!status.value.running) return;
  }
};

const handleScan = async () => {
  loading.value = true;
  try {
    // 手动检查：强制拉取最新（绕过自动检查的节流缓存），并刷新缓存状态
    const res = await checkSelfUpdateForce();
    if (res.ok && res.data) {
      status.value = res.data;
      if (status.value.running) {
        await pollUntilDone();
      }
    }
  } catch {
    // 忽略，下面仍刷新版本列表
  } finally {
    loading.value = false;
  }
  await refreshVersions();
  await refreshStatus();
};

// 页面打开时自动检查更新：状态过期（10 分钟）则触发后台扫描并轮询，最终加载版本列表
const handleAutoCheck = async () => {
  const now = Math.floor(Date.now() / 1000);
  const stale =
    !status.value.checked_at || now - status.value.checked_at >= 600;
  if (stale) {
    try {
      const res = await checkSelfUpdate();
      if (res.ok && res.data) {
        status.value = res.data;
        // 后台扫描进行中时轮询，直至拿到最新版本结果
        if (status.value.running) {
          await pollUntilDone();
        }
      }
    } catch {}
  }
  // 始终加载版本列表用于页面展示
  await refreshVersions();
};

const handlePrepare = async (item: SelfUpdateVersionItem) => {
  if (loading.value || isRunning.value) return;
  try {
    await ElMessageBox.confirm(
      `确定要更新到版本 ${item.version} 吗？\n更新将下载并安装新版本，请确认。`,
      "确认更新",
      {
        confirmButtonText: "确认更新",
        cancelButtonText: "取消",
        type: "warning"
      }
    );
  } catch {
    return;
  }
  loading.value = true;
  preparing.value = true;
  try {
    const res = await prepareSelfUpdate({
      version: item.version,
      download_url: item.download_url,
      sha256: item.sha256
    });
    if (res.ok && res.data) {
      status.value = res.data;
      if (status.value.running) {
        await pollUntilDone();
      }
      if (hasPrepareError.value) {
        ElMessage.error(status.value.prepare_error);
      } else if (autoReplaceOK.value) {
        promptRestart();
      } else {
        ElMessage.success("准备完成");
      }
    }
  } catch (e: any) {
    // 全局拦截器已弹过提示(handled)则不再重复
    if (!e?.handled) ElMessage.error(e?.response?.data?.error || "准备失败");
  } finally {
    loading.value = false;
    preparing.value = false;
  }
};

// doRestart 执行重启（不含确认），由弹窗确认或工具栏按钮调用
async function doRestart() {
  restarting.value = true;
  try {
    const res = await restartSelfUpdate();
    if (res.ok) {
      ElMessage.success("正在重启，请稍候，约 10 秒后刷新页面…");
      setTimeout(() => window.location.reload(), 10000);
    } else {
      ElMessage.error("重启失败");
      restarting.value = false;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || "重启请求失败");
    restarting.value = false;
  }
}

async function handleRestart() {
  try {
    await ElMessageBox.confirm(
      "确定要立即重启以加载新版本吗？重启期间服务将短暂不可用。",
      "确认重启",
      {
        confirmButtonText: "立即重启",
        cancelButtonText: "取消",
        type: "warning"
      }
    );
  } catch {
    return;
  }
  doRestart();
}

// promptRestart 安装成功后弹窗询问是否立即重启
function promptRestart() {
  ElMessageBox.confirm("新版本已安装，是否立即重启以生效？", "更新完成", {
    confirmButtonText: "立即重启",
    cancelButtonText: "稍后",
    type: "success"
  })
    .then(() => doRestart())
    .catch(() => {});
}

const handleSaveConfig = async () => {
  loading.value = true;
  try {
    const res = await updateSelfUpdateConfig(config.value);
    if (res.ok) {
      ElMessage.success("配置已保存");
    } else {
      ElMessage.error("保存失败");
    }
  } catch (e: any) {
    // 全局拦截器已弹过提示(handled)则不再重复
    if (!e?.handled) ElMessage.error(e?.response?.data?.error || "保存失败");
  } finally {
    loading.value = false;
  }
};

const handleTestConfig = async () => {
  testLoading.value = true;
  try {
    const res = await testSelfUpdateConfig();
    if (res.ok) {
      ElMessage.success(
        `连接成功${res.data?.versions_count != null ? `，发现 ${res.data.versions_count} 个版本` : ""}`
      );
    } else {
      ElMessage.error(res.message || "连接失败");
    }
  } catch (e: any) {
    // 全局拦截器已弹过提示(handled)则不再重复
    if (!e?.handled) ElMessage.error(e?.response?.data?.error || "连接失败");
  } finally {
    testLoading.value = false;
  }
};

onMounted(async () => {
  await refreshStatus();
  await refreshConfig();
  // 页面打开时自动检查更新并展示版本列表
  await handleAutoCheck();
});
</script>

<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <div class="font-bold">软件更新</div>
      </template>

      <el-tabs v-model="activeTab">
        <!-- Tab 1: 在线更新 -->
        <el-tab-pane label="在线更新" name="update">
          <div class="flex items-center justify-between mb-4">
            <div>
              当前版本：
              <el-tag size="small">{{ status.current_version || "-" }}</el-tag>
            </div>
            <div class="flex items-center gap-2">
              <el-button type="primary" :loading="loading" @click="handleScan">
                检查更新
              </el-button>
              <el-button
                v-if="latestNewer"
                type="success"
                :loading="loading"
                @click="handlePrepare(latestNewer)"
              >
                更新到最新版
              </el-button>
              <el-button
                v-if="autoReplaceOK"
                type="warning"
                :loading="restarting"
                @click="handleRestart"
              >
                立即重启
              </el-button>
            </div>
          </div>

          <!-- 准备与安装区域 -->
          <el-card v-if="showDownloadCard" shadow="never" class="mb-4">
            <template #header>
              <div class="font-bold">下载与安装</div>
            </template>
            <el-descriptions :column="1" border label-width="100px">
              <el-descriptions-item v-if="preparing" label="下载进度">
                <el-progress
                  :percentage="downloadProgress"
                  :status="downloadProgress === 100 ? 'success' : undefined"
                  :stroke-width="20"
                  :text-inside="true"
                />
              </el-descriptions-item>
              <el-descriptions-item label="准备状态">
                <el-tag :type="isPrepared ? 'success' : 'info'" size="small">
                  {{
                    isPrepared ? "已准备" : isRunning ? "下载中..." : "未准备"
                  }}
                </el-tag>
                <span
                  v-if="hasPrepareError"
                  class="ml-2 text-sm text-[var(--el-color-danger)]"
                >
                  {{ status.prepare_error }}
                </span>
              </el-descriptions-item>
              <el-descriptions-item
                v-if="status.prepared_version"
                label="准备版本"
              >
                {{ status.prepared_version }}
              </el-descriptions-item>
              <el-descriptions-item v-if="autoReplaceTried" label="自动安装">
                <el-tag
                  :type="autoReplaceOK ? 'success' : 'warning'"
                  size="small"
                >
                  {{ autoReplaceOK ? "成功" : "失败" }}
                </el-tag>
                <span
                  v-if="autoReplaceOK"
                  class="ml-2 text-sm text-[var(--el-color-success)]"
                >
                  新版本已安装，请点击上方「立即重启」生效
                </span>
                <span
                  v-if="!autoReplaceOK && status.auto_replace_error"
                  class="ml-2 text-sm text-[var(--el-color-danger)]"
                >
                  {{ status.auto_replace_error }}
                </span>
              </el-descriptions-item>
              <el-descriptions-item
                v-if="status.script_shell_path || status.script_powershell_path"
                label="更新命令"
              >
                <span class="font-mono text-xs break-all">
                  {{
                    status.script_shell_path || status.script_powershell_path
                  }}
                </span>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>

          <!-- 版本列表 -->
          <el-table :data="versions" stripe style="width: 100%">
            <el-table-column label="版本号" min-width="140">
              <template #default="{ row }">
                <span class="font-mono">{{ row.version }}</span>
                <el-tag
                  v-if="row.is_current"
                  type="info"
                  size="small"
                  class="ml-2"
                >
                  当前
                </el-tag>
                <el-tag
                  v-else-if="row.is_newer"
                  type="success"
                  size="small"
                  class="ml-2"
                >
                  新版
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="120">
              <template #default="{ row }">
                {{ row.size_formatted || "-" }}
              </template>
            </el-table-column>
            <el-table-column label="SHA256" min-width="200">
              <template #default="{ row }">
                <span class="font-mono text-xs break-all">
                  {{ row.sha256 || "-" }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" align="center">
              <template #default="{ row }">
                <template v-if="row.is_current">
                  <span class="text-[var(--el-text-color-secondary)] text-sm"
                    >—</span
                  >
                </template>
                <template v-else-if="row.is_newer">
                  <el-button
                    type="primary"
                    size="small"
                    :loading="loading || isRunning"
                    :disabled="!row.download_url"
                    @click="handlePrepare(row as any)"
                  >
                    更新
                  </el-button>
                </template>
                <template v-else>
                  <el-button
                    type="warning"
                    size="small"
                    :loading="loading || isRunning"
                    :disabled="!row.download_url"
                    @click="handlePrepare(row as any)"
                  >
                    降级
                  </el-button>
                </template>
              </template>
            </el-table-column>
          </el-table>

          <div
            v-if="versions.length === 0 && !loading"
            class="text-center text-[var(--el-text-color-secondary)] py-8"
          >
            点击「检查更新」获取可用版本列表
          </div>
        </el-tab-pane>

        <!-- Tab 2: 更新配置 -->
        <el-tab-pane label="更新配置" name="config">
          <el-form :model="config" label-width="100px" style="max-width: 600px">
            <el-form-item label="存储类型">
              <el-select v-model="config.type" placeholder="请选择">
                <el-option :value="0" label="不启用" />
                <el-option :value="1" label="腾讯云 COS" />
                <el-option :value="2" label="阿里云 OSS" />
              </el-select>
            </el-form-item>

            <template v-if="config.type > 0">
              <el-form-item label="密钥 ID">
                <el-input
                  v-model="config.secret_id"
                  placeholder="SecretID / AccessKeyID"
                />
              </el-form-item>
              <el-form-item label="密钥 Key">
                <el-input
                  v-model="config.secret_key"
                  type="password"
                  placeholder="SecretKey / AccessKeySecret"
                  show-password
                />
              </el-form-item>
              <el-form-item :label="config.type === 1 ? '区域' : 'Endpoint'">
                <el-input
                  v-model="config.region"
                  :placeholder="
                    config.type === 1
                      ? 'ap-guangzhou'
                      : 'oss-cn-hangzhou.aliyuncs.com'
                  "
                />
              </el-form-item>
              <el-form-item label="存储桶">
                <el-input v-model="config.bucket" placeholder="存储桶名称" />
              </el-form-item>
              <el-form-item label="路径前缀">
                <el-input v-model="config.prefix" placeholder="NetworkAuth/" />
              </el-form-item>
              <el-form-item label="自定义域名">
                <el-input
                  v-model="config.base_url"
                  placeholder="可选，留空使用默认域名"
                />
              </el-form-item>
            </template>

            <el-form-item>
              <el-button
                type="primary"
                :loading="loading"
                @click="handleSaveConfig"
              >
                保存
              </el-button>
              <el-button
                :loading="testLoading"
                :disabled="config.type === 0"
                @click="handleTestConfig"
              >
                测试连接
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>
