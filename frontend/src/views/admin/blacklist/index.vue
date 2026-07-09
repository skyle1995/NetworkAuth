<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from "vue";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessage, ElMessageBox, ElTag } from "element-plus";
import { addDialog } from "@/components/ReDialog";
import type { PaginationProps } from "@pureadmin/table";
import {
  getBlacklist,
  addBlacklist,
  deleteBlacklist
} from "@/api/admin/member";
import { getAppsSimple } from "@/api/admin/app";
import addForm from "./addForm.vue";

defineOptions({
  name: "Blacklist"
});

const TYPE_META: Record<number, { text: string; type: any }> = {
  0: { text: "设备", type: "warning" },
  1: { text: "IP", type: "primary" },
  2: { text: "地区", type: "danger" }
};

const formRef = ref();
const tableRef = ref();
const loading = ref(true);
const dataList = ref<any[]>([]);
const apps = ref<any[]>([]);
const selectedRows = ref<any[]>([]);
const selectedNum = computed(() => selectedRows.value.length);

const form = reactive({
  search: "",
  app_uuid: "",
  type: ""
});

const pagination = reactive<PaginationProps>({
  total: 0,
  pageSize: 30,
  currentPage: 1,
  background: true
});

const columns: TableColumnList = [
  { type: "selection", width: 55, align: "center" },
  {
    label: "类型",
    prop: "type",
    width: 90,
    cellRenderer: ({ row }) => {
      const meta = TYPE_META[row.type] ?? { text: "未知", type: "info" };
      return h(ElTag, { type: meta.type, effect: "light" }, () => meta.text);
    }
  },
  { label: "命中值", prop: "value", minWidth: 180 },
  {
    label: "归属地",
    prop: "province",
    minWidth: 140,
    cellRenderer: ({ row }) =>
      [row.province, row.city].filter(Boolean).join(" ") || "—"
  },
  {
    label: "所属应用",
    prop: "app_uuid",
    minWidth: 120,
    cellRenderer: ({ row }) => {
      const app = apps.value.find(a => a.uuid === row.app_uuid);
      return app ? app.name : "未知应用";
    }
  },
  {
    label: "来源账号",
    prop: "username",
    minWidth: 130,
    cellRenderer: ({ row }) => row.username || "—"
  },
  { label: "备注", prop: "remark", minWidth: 110 },
  { label: "加入时间", prop: "created_at", width: 170 },
  { label: "操作", fixed: "right", width: 90, slot: "operation" }
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
    const { code, data, count } = await getBlacklist({
      page: pagination.currentPage,
      limit: pagination.pageSize,
      search: form.search,
      app_uuid: form.app_uuid,
      type: form.type
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

function openAddDialog() {
  addDialog({
    title: "新增黑名单",
    width: "520px",
    draggable: true,
    closeOnClickModal: false,
    props: {
      formInline: {
        app_uuid: form.app_uuid || "",
        type: 0,
        value: "",
        province: "",
        city: "",
        remark: ""
      }
    },
    contentRenderer: () => h(addForm, { apps: apps.value } as any),
    footerButtons: [
      {
        label: "取消",
        text: true,
        bg: true,
        btnClick: ({ dialog: { options } }) => (options.visible = false)
      },
      {
        label: "确认",
        type: "primary",
        text: true,
        bg: true,
        btnClick: async ({ dialog: { options } }) => {
          const f = (options.props as any).formInline;
          if (!f.app_uuid) {
            ElMessage.warning("请选择应用");
            return;
          }
          const { code, msg } = await addBlacklist(f);
          if (code === 0) {
            ElMessage.success("已加入黑名单");
            options.visible = false;
            onSearch();
          } else {
            ElMessage.error(msg || "添加失败");
          }
        }
      }
    ]
  });
}

async function handleRemove(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认移除该「${TYPE_META[row.type]?.text}」黑名单吗？移除后将恢复其登录。`,
      "提示",
      { type: "warning" }
    );
    const { code, msg } = await deleteBlacklist({ ids: [row.id] });
    if (code === 0) {
      ElMessage.success("已移除");
      onSearch();
    } else {
      ElMessage.error(msg || "移除失败");
    }
  } catch {
    // cancelled
  }
}

async function onBatchRemove() {
  if (selectedNum.value === 0) return;
  try {
    await ElMessageBox.confirm(
      `确认移除选中的 ${selectedNum.value} 条黑名单吗？`,
      "提示",
      { type: "warning" }
    );
    const { code, msg } = await deleteBlacklist({
      ids: selectedRows.value.map(r => r.id)
    });
    if (code === 0) {
      ElMessage.success("批量移除成功");
      onSearch();
    } else {
      ElMessage.error(msg || "移除失败");
    }
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
          placeholder="命中值 / 来源账号"
          clearable
          class="w-[180px]!"
          @keyup.enter="onSearch"
        />
      </el-form-item>

      <el-form-item label="类型" prop="type">
        <el-select
          v-model="form.type"
          placeholder="全部"
          clearable
          class="w-[120px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option label="设备" :value="0" />
          <el-option label="IP" :value="1" />
          <el-option label="地区" :value="2" />
        </el-select>
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
          type="primary"
          :icon="useRenderIcon('ep:plus')"
          @click="openAddDialog"
        >
          新增黑名单
        </el-button>
        <el-button
          type="danger"
          :icon="useRenderIcon('ep:delete')"
          :disabled="selectedNum === 0"
          @click="onBatchRemove"
        >
          批量移除
        </el-button>
      </div>

      <PureTableBar title="黑名单管理" :columns="columns" @refresh="onSearch">
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
              <el-button
                class="reset-margin"
                link
                type="danger"
                :size="size"
                :icon="useRenderIcon('ep:circle-close')"
                @click="handleRemove(row)"
              >
                移除
              </el-button>
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
