<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from "vue";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessage, ElMessageBox } from "element-plus";
import { addDialog } from "@/components/ReDialog";
import type { PaginationProps } from "@pureadmin/table";
import {
  getOnlineSessions,
  kickMemberSession,
  blacklistSession
} from "@/api/admin/member";
import { getAppsSimple } from "@/api/admin/app";
import blacklistSessionForm from "./blacklistSessionForm.vue";

defineOptions({
  name: "OnlineManage"
});

const formRef = ref();
const tableRef = ref();
const loading = ref(true);
const dataList = ref<any[]>([]);
const apps = ref<any[]>([]);
const selectedRows = ref<any[]>([]);
const selectedNum = computed(() => selectedRows.value.length);

const form = reactive({
  search: "",
  app_uuid: ""
});

const pagination = reactive<PaginationProps>({
  total: 0,
  pageSize: 30,
  currentPage: 1,
  background: true
});

const columns: TableColumnList = [
  { type: "selection", width: 55, align: "center" },
  { label: "用户名", prop: "username", minWidth: 140 },
  {
    label: "所属应用",
    prop: "app_uuid",
    minWidth: 130,
    cellRenderer: ({ row }) => {
      const app = apps.value.find(a => a.uuid === row.app_uuid);
      return app ? app.name : "未知应用";
    }
  },
  { label: "机器码", prop: "machine_code", minWidth: 160 },
  { label: "登录IP", prop: "ip", width: 140 },
  { label: "版本", prop: "version", width: 100 },
  {
    label: "归属地",
    prop: "province",
    minWidth: 150,
    cellRenderer: ({ row }) =>
      [row.province, row.city].filter(Boolean).join(" ") || "—"
  },
  { label: "最近活跃", prop: "last_active_at", width: 170 },
  { label: "上线时间", prop: "created_at", width: 170 },
  { label: "操作", fixed: "right", width: 230, slot: "operation" }
];

async function fetchApps() {
  try {
    const { code, data } = await getAppsSimple();
    if (code === 0 && Array.isArray(data)) apps.value = data;
  } catch (e) {
    console.error(e);
  }
}

async function onSearch() {
  loading.value = true;
  try {
    const { code, data, count } = await getOnlineSessions({
      page: pagination.currentPage,
      limit: pagination.pageSize,
      search: form.search,
      app_uuid: form.app_uuid
    });
    if (code === 0) {
      dataList.value = data || [];
      pagination.total = count || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

function resetForm(formEl: any) {
  if (!formEl) return;
  formEl.resetFields();
  pagination.currentPage = 1;
  onSearch();
}

function handleSelectionChange(val: any[]) {
  selectedRows.value = val;
}

function handleSizeChange(size: number) {
  pagination.pageSize = size;
  onSearch();
}

function handleCurrentChange(page: number) {
  pagination.currentPage = page;
  onSearch();
}

// 踢下线单个会话
async function handleKick(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认将用户「${row.username || row.member_uuid}」的该会话踢下线吗？`,
      "提示",
      { type: "warning" }
    );
    const { code, msg } = await kickMemberSession({ id: row.id });
    if (code === 0) {
      ElMessage.success("已踢下线");
      onSearch();
    } else {
      ElMessage.error(msg || "操作失败");
    }
  } catch {
    // cancelled
  }
}

// 踢下线该用户的全部会话
async function handleKickAll(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认将用户「${row.username || row.member_uuid}」的全部在线会话踢下线吗？`,
      "提示",
      { type: "warning" }
    );
    const { code, msg } = await kickMemberSession({
      member_uuid: row.member_uuid
    });
    if (code === 0) {
      ElMessage.success("已全部踢下线");
      onSearch();
    } else {
      ElMessage.error(msg || "操作失败");
    }
  } catch {
    // cancelled
  }
}

// 从会话拉黑：设备/IP/地区，可连带拉黑账号
function openBlacklistDialog(row: any) {
  addDialog({
    title: `拉黑 - ${row.username || row.member_uuid}`,
    width: "540px",
    draggable: true,
    closeOnClickModal: false,
    props: {
      formInline: {
        username: row.username,
        machine_code: row.machine_code,
        ip: row.ip,
        province: row.province,
        city: row.city,
        blacklist_device: false,
        blacklist_ip: false,
        blacklist_region: false,
        blacklist_account: false
      }
    },
    contentRenderer: () => h(blacklistSessionForm),
    footerButtons: [
      {
        label: "取消",
        text: true,
        bg: true,
        btnClick: ({ dialog: { options } }) => (options.visible = false)
      },
      {
        label: "确认拉黑",
        type: "danger",
        text: true,
        bg: true,
        btnClick: async ({ dialog: { options } }) => {
          const f = (options.props as any).formInline;
          if (
            !f.blacklist_device &&
            !f.blacklist_ip &&
            !f.blacklist_region &&
            !f.blacklist_account
          ) {
            ElMessage.warning("请至少选择一个拉黑维度");
            return;
          }
          const { code, msg, data } = await blacklistSession({
            app_uuid: row.app_uuid,
            member_uuid: row.member_uuid,
            username: row.username,
            machine_code: row.machine_code,
            ip: row.ip,
            province: row.province,
            city: row.city,
            blacklist_device: f.blacklist_device,
            blacklist_ip: f.blacklist_ip,
            blacklist_region: f.blacklist_region,
            blacklist_account: f.blacklist_account
          });
          if (code === 0) {
            ElMessage.success(
              `已拉黑（踢下线 ${data?.kicked ?? 0} 个会话）`
            );
            options.visible = false;
            onSearch();
          } else {
            ElMessage.error(msg || "拉黑失败");
          }
        }
      }
    ]
  });
}

// 批量踢下线选中会话
async function onBatchKick() {
  if (selectedNum.value === 0) return;
  try {
    await ElMessageBox.confirm(
      `确认将选中的 ${selectedNum.value} 个会话踢下线吗？`,
      "提示",
      { type: "warning" }
    );
    await Promise.all(
      selectedRows.value.map(r => kickMemberSession({ id: r.id }))
    );
    ElMessage.success("批量踢下线完成");
    onSearch();
  } catch {
    // cancelled
  }
}

onMounted(() => {
  fetchApps();
  onSearch();
});
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="搜索" prop="search">
        <el-input
          v-model="form.search"
          placeholder="用户名 / IP / 机器码"
          clearable
          class="w-[200px]!"
          @keyup.enter="onSearch"
        />
      </el-form-item>

      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="请选择应用"
          clearable
          class="w-[160px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option
            v-for="app in apps"
            :key="app.uuid"
            :label="app.name"
            :value="app.uuid"
          />
        </el-select>
      </el-form-item>

      <el-form-item>
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:search')"
          :loading="loading"
          @click="onSearch"
        >
          搜索
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
          type="danger"
          :icon="useRenderIcon('ep:switch-button')"
          :disabled="selectedNum === 0"
          @click="onBatchKick"
        >
          批量踢下线
        </el-button>
      </div>

      <PureTableBar title="在线用户管理" :columns="columns" @refresh="onSearch">
        <template v-slot="{ size, dynamicColumns }">
          <pure-table
            ref="tableRef"
            row-key="id"
            align-whole="center"
            header-align="center"
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
            @selection-change="handleSelectionChange"
          >
            <template #operation="{ row }">
              <div class="flex items-center justify-center">
                <el-button
                  class="reset-margin"
                  link
                  type="danger"
                  :size="size"
                  :icon="useRenderIcon('ep:switch-button')"
                  @click="handleKick(row)"
                >
                  踢下线
                </el-button>
                <el-button
                  class="reset-margin ml-2"
                  link
                  type="warning"
                  :size="size"
                  @click="handleKickAll(row)"
                >
                  踢全部
                </el-button>
                <el-button
                  class="reset-margin ml-2"
                  link
                  type="danger"
                  :size="size"
                  :icon="useRenderIcon('ep:circle-close')"
                  @click="openBlacklistDialog(row)"
                >
                  拉黑
                </el-button>
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

<style scoped lang="scss">
.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}

.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
