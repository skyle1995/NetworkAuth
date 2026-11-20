<script setup lang="ts">
import { ref, h } from "vue";
import { useApp } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessageBox, ElMessage } from "element-plus";
import { addDialog } from "@/components/ReDialog";

import Form from "./form.vue";
import MultiConfig from "./multi-config.vue";
import RegisterConfig from "./register-config.vue";
import LevelPoints from "./level-points.vue";
import TextConfig from "./text-config.vue";
import VersionList from "./version-list.vue";

import {
  createApp,
  updateApp,
  deleteApp,
  batchDeleteApps,
  batchUpdateAppStatus,
  resetAppSecret,
  getAppData,
  updateAppData,
  getAppMultiConfig,
  updateAppMultiConfig,
  getAppRegisterConfig,
  updateAppRegisterConfig,
  getAppLevelPoints,
  saveAppLevelPoints,
} from "@/api/admin/app";

defineOptions({
  name: "Apps"
});

const formRef = ref();
const tableRef = ref();

const {
  form,
  loading,
  columns,
  dataList,
  pagination,
  onSearch,
  resetForm,
  handleSizeChange,
  handleCurrentChange,
  handleSelectionChange
} = useApp();

const selectedNum = ref(0);
const selectedIds = ref<number[]>([]);

function handleSelectionChangeVue(val: any[]) {
  selectedNum.value = val.length;
  selectedIds.value = val.map(item => item.id);
  handleSelectionChange(val);
}

function openDialog(title = "新增", row?: any) {
  const dialogFormRef = ref();
  addDialog({
    title: `${title}应用`,
    props: {
      formInline: {
        id: row?.id,
        name: row?.name ?? "",
        plugin_id: row?.plugin_id ?? "",
        status: row?.status ?? 1,
            free_mode: row?.free_mode ?? 0,
            bill_time_enabled: row?.bill_time_enabled ?? 0,
            bill_points_enabled: row?.bill_points_enabled ?? 0,
            level_gate_enabled: row?.level_gate_enabled ?? 0,
            billing_mode: row?.billing_mode ?? 0,
            required_level: row?.required_level ?? 1,
        points_charge_mode: row?.points_charge_mode ?? 0,
        billing_scope: row?.billing_scope ?? 0,
        points_per_login: row?.points_per_login ?? 1,
        points_period_minutes: row?.points_period_minutes ?? 60,
        points_per_period: row?.points_per_period ?? 1,
        points_heartbeat_charge: row?.points_heartbeat_charge ?? 0,
        // 授权期限（分钟）。旧数据可能是 0，兜底 30，避免数字框空白
        lease_ttl_minutes: row?.lease_ttl_minutes || 30
      }
    },
    width: "460px",
    draggable: true,
    fullscreenIcon: true,
    closeOnClickModal: false,
    contentRenderer: () => h(Form, { ref: dialogFormRef } as any),
    beforeSure: (done, { options }) => {
      const curData = options.props.formInline as any;
      function chores() {
        ElMessage.success(`${title}成功`);
        done();
        onSearch();
      }
      const FormRef = dialogFormRef.value;
      if (!FormRef) {
        handleSave();
        return;
      }
      FormRef.getRef().validate(async valid => {
        if (valid) {
          handleSave();
        }
      });

      async function handleSave() {
        try {
          if (title === "新增") {
            const { code } = await createApp(curData);
            if (code === 0) chores();
          } else {
            const { code } = await updateApp(curData);
            if (code === 0) chores();
          }
        } catch (e) {
          console.error(e);
        }
      }
    }
  });
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认删除应用 ${row.name} 吗？<br><span style="color:red;font-size:12px;">注意：删除应用将同时删除该应用下所有的接口、变量、函数等相关数据，此操作不可恢复！</span>`,
      "提示",
      {
        type: "warning",
        dangerouslyUseHTMLString: true
      }
    );
    const { code } = await deleteApp({ id: row.id });
    if (code === 0) {
      ElMessage.success("删除成功");
      onSearch();
    }
  } catch (e) {
    // cancelled
  }
}

async function onBatchDel() {
  if (selectedIds.value.length === 0) {
    ElMessage.warning("请选择要删除的应用");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedNum.value} 个应用吗？<br><span style="color:red;font-size:12px;">注意：删除应用将同时删除该应用下所有的接口、变量、函数等相关数据，此操作不可恢复！</span>`,
      "提示",
      {
        type: "warning",
        dangerouslyUseHTMLString: true
      }
    );
    const { code } = await batchDeleteApps({ ids: selectedIds.value });
    if (code === 0) {
      ElMessage.success("批量删除成功");
      tableRef.value.getTableRef().clearSelection();
      onSearch();
    }
  } catch (e) {}
}

async function onBatchUpdateStatus(status: number) {
  if (selectedIds.value.length === 0) {
    ElMessage.warning("请选择要操作的应用");
    return;
  }
  try {
    const action = status === 1 ? "启用" : "禁用";
    await ElMessageBox.confirm(
      `确认${action}选中的 ${selectedNum.value} 个应用吗？`,
      "提示",
      {
        type: "warning"
      }
    );
    const { code } = await batchUpdateAppStatus({
      ids: selectedIds.value,
      status
    });
    if (code === 0) {
      ElMessage.success(`批量${action}成功`);
      tableRef.value.getTableRef().clearSelection();
      onSearch();
    }
  } catch (e) {}
}

async function handleCommand(command: string, row: any) {
  switch (command) {
    case "reset_secret":
      try {
        await ElMessageBox.confirm(
          "确认重置密钥吗？旧密钥将立即失效！",
          "提示",
          { type: "warning" }
        );
        const { code } = await resetAppSecret({ uuid: row.uuid });
        if (code === 0) {
          ElMessage.success("重置成功");
          onSearch();
        }
      } catch (e) {}
      break;
    case "app_data":
      openTextConfigDialog(
        "应用数据",
        row.uuid,
        getAppData,
        updateAppData,
        "app_data"
      );
      break;
    case "multi_config":
      openMultiConfigDialog(row.uuid);
      break;
    case "versions":
      openVersionDialog(row);
      break;
        case "level_points":
          openLevelPointsDialog(row.uuid, row.name);
          break;
    case "register_config":
      openRegisterConfigDialog();
      break;
  }
}

async function openTextConfigDialog(
  title: string,
  uuid: string,
  getApi: any,
  updateApi: any,
  dataKey: string
) {
  try {
    const res = await getApi({ uuid });
    if (res.code === 0) {
      addDialog({
        title,
        props: {
          formInline: {
            content: res.data[dataKey] || ""
          }
        },
        width: "600px",
        draggable: true,
        fullscreenIcon: true,
        closeOnClickModal: false,
        contentRenderer: () => TextConfig,
        beforeSure: async (done, { options }) => {
          const curData = options.props.formInline as any;
          const { code } = await updateApi({
            uuid,
            [dataKey]: curData.content
          });
          if (code === 0) {
            ElMessage.success("保存成功");
            done();
          }
        }
      });
    }
  } catch (e) {}
}

async function openVersionDialog(row: any) {
  addDialog({
    title: `版本列表 · ${row.name ?? row.uuid}`,
    width: "960px",
    hideFooter: true,
    props: {
      app: { uuid: row.uuid, name: row.name }
    },
    contentRenderer: () => h(VersionList, { app: { uuid: row.uuid, name: row.name } })
  });
}

async function openMultiConfigDialog(uuid: string) {
  try {
    const res = await getAppMultiConfig({ uuid });
    if (res.code === 0) {
      addDialog({
        title: "登录配置",
        props: {
          formInline: res.data || {
            login_type: 0,
                clean_interval: 1,
            check_interval: 10,
            offline_timeout: 30,
              }
        },
        width: "650px",
        draggable: true,
        fullscreenIcon: true,
        closeOnClickModal: false,
        contentRenderer: () => MultiConfig,
        beforeSure: async (done, { options }) => {
          const curData = options.props.formInline as any;
          const { code } = await updateAppMultiConfig({ uuid, ...curData });
          if (code === 0) {
            ElMessage.success("保存成功");
            done();
          }
        }
      });
    }
  } catch (e) {}
}

    async function openRegisterConfigDialog() {
  try {
    const res = await getAppRegisterConfig({});
    if (res.code === 0) {
      addDialog({
        title: "账号策略（全站生效）",
        props: {
          formInline: res.data || {
            register_enabled: 0,
            email_verify_enabled: 0,
            card_register_enabled: 0,
            register_limit_enabled: 0,
            register_device_limit_enabled: 0,
                register_limit_time: 1,
                register_count: 3,
            trial_enabled: 0,
            trial_limit_time: 0,
                card_login_enabled: 1,
                recharge_enabled: 1,
                trial_points: 0,
                trial_duration_minutes: 0,
                trial_claim_mode: 1
          }
        },
        width: "650px",
        draggable: true,
        fullscreenIcon: true,
        closeOnClickModal: false,
        contentRenderer: () => RegisterConfig,
        beforeSure: async (done, { options }) => {
          const curData = options.props.formInline as any;
          const { code } = await updateAppRegisterConfig({ ...curData });
          if (code === 0) {
            ElMessage.success("保存成功");
            done();
          }
        }
      });
    }
  } catch (e) {}
}

    const levelPointsRef = ref();

  // 高级模式：等级点数配置
  async function openLevelPointsDialog(uuid: string, appName: string) {
    try {
      const res = await getAppLevelPoints({ uuid });
      if (res.code !== 0) return;
      const data = res.data || {};
      addDialog({
        title: `等级点数 - ${appName || ""}`,
        width: "620px",
        draggable: true,
        fullscreenIcon: true,
        closeOnClickModal: false,
        props: {
          appName,
          billingMode: data.billing_mode ?? 0,
          defaultPoints: data.default_points ?? 0,
          items: data.levels || []
        },
            contentRenderer: () => h(LevelPoints, { ref: levelPointsRef } as any),
        beforeSure: (done, { options }) => {
              const comp = levelPointsRef.value as any;
          const items = comp?.getPayload?.() ?? [];
          saveAppLevelPoints({ uuid, items }).then((r: any) => {
            if (r.code === 0) {
              ElMessage.success("保存成功");
              done();
            } else {
              ElMessage.error(r.msg || "保存失败");
            }
          });
        }
      });
    } catch (e) {}
  }
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="搜索内容" prop="search">
        <el-input
          v-model="form.search"
          placeholder="应用名称/UUID"
          clearable
          class="w-50!"
        />
      </el-form-item>
      <el-form-item>
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:search')"
          :loading="loading"
          @click="onSearch"
        >
          查询
        </el-button>
        <el-button
          :icon="useRenderIcon('ep:refresh')"
          @click="resetForm(formRef)"
        >
          重置
        </el-button>
      </el-form-item>
    </el-form>

    <el-card shadow="never" class="table-wrapper mt-4">
      <div class="toolbar mb-4 px-2 overflow-x-auto whitespace-nowrap pb-2">
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:plus')"
          @click="openDialog('新增')"
        >
          新增应用
        </el-button>
        <el-button
          type="danger"
          :icon="useRenderIcon('ep:delete')"
          :disabled="selectedIds.length === 0"
          @click="onBatchDel"
        >
          批量删除
        </el-button>
        <el-button
          type="success"
          :icon="useRenderIcon('ep:circle-check')"
          :disabled="selectedIds.length === 0"
          @click="onBatchUpdateStatus(1)"
        >
          批量启用
        </el-button>
        <el-button
          type="warning"
          :icon="useRenderIcon('ep:circle-close')"
          :disabled="selectedIds.length === 0"
          @click="onBatchUpdateStatus(0)"
        >
          批量禁用
        </el-button>
      </div>

      <PureTableBar title="应用列表" :columns="columns" @refresh="onSearch">
        <template v-slot="{ size, dynamicColumns }">
          <pure-table
            ref="tableRef"
            row-key="id"
            table-layout="auto"
            show-overflow-tooltip
            border
            :loading="loading"
            :size="size"
            :data="dataList"
            :columns="dynamicColumns"
            :header-cell-style="{
              background: 'var(--el-fill-color-light)',
              color: 'var(--el-text-color-primary)'
            }"
            class="w-full"
            @selection-change="handleSelectionChangeVue"
          >
            <template #operation="{ row }">
              <div class="flex items-center justify-center">
                <el-button
                  class="reset-margin"
                  link
                  type="primary"
                  :size="size"
                  @click="openDialog('编辑', row)"
                >
                  编辑
                </el-button>
                <el-button
                  class="reset-margin ml-2"
                  link
                  type="danger"
                  :size="size"
                  @click="handleDelete(row)"
                >
                  删除
                </el-button>
                <el-dropdown
                  class="ml-2"
                  @command="command => handleCommand(command, row)"
                >
                  <el-button
                    class="reset-margin"
                    link
                    type="primary"
                    :size="size"
                  >
                    更多
                    <el-icon class="el-icon--right">
                      <component :is="useRenderIcon('ep:arrow-down')" />
                    </el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="reset_secret"
                        >重置密钥</el-dropdown-item
                      >
                      <el-dropdown-item command="app_data"
                        >应用数据</el-dropdown-item
                      >
                      <el-dropdown-item divided command="versions"
                        >版本列表</el-dropdown-item
                      >
                      <el-dropdown-item command="multi_config"
                        >登录配置</el-dropdown-item
                      >
                          <el-dropdown-item divided command="level_points"
                            >等级点数</el-dropdown-item
                          >
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </pure-table>
          <div class="flex mt-4 w-full overflow-x-auto">
            <div class="ml-auto shrink-0">
              <el-pagination
                v-model:current-page="pagination.currentPage"
                v-model:page-size="pagination.pageSize"
                :page-sizes="[10, 20, 30, 50, 100, 200, 500, 1000]"
                :background="true"
                layout="total, sizes, prev, pager, next, jumper"
                :total="pagination.total"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </div>
        </template>
      </PureTableBar>
    </el-card>
  </div>
</template>

<style lang="scss" scoped>
.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}

.toolbar {
  display: flex;
  gap: 10px;
}

// 操作列“更多”：点击展开后触发按钮保留 :focus-visible，
// 会残留一圈蓝色 outline（看起来像蓝色背景），这里去掉
:deep(.el-dropdown:focus-visible),
:deep(.el-dropdown .el-button:focus-visible) {
  outline: none;
}
</style>