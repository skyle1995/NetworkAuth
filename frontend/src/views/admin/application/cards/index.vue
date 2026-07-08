<script setup lang="ts">
import { ref, computed } from "vue";
import { useCard } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessageBox, ElMessage } from "element-plus";
import { useSmartExport } from "@/composables/useSmartExport";
import {
  batchDeleteCards,
  freezeCards,
  unfreezeCards,
  exportCards
} from "@/api/admin/card";

defineOptions({
  name: "Cards"
});

const formRef = ref();
const tableRef = ref();

const {
  form,
  loading,
  columns,
  dataList,
  pagination,
  apps,
  onSearch,
  resetFormSearch,
  openCreateDialog,
  handleFreeze,
  handleUnfreeze,
  handleSizeChange,
  handleCurrentChange
} = useCard();

const selectedRows = ref<any[]>([]);
const selectedNum = computed(() => selectedRows.value.length);
const selectedIds = computed(() => selectedRows.value.map(r => r.id));

function handleSelectionChange(val: any[]) {
  selectedRows.value = val;
}

// 导出列定义（批量导出与导出批次共用）
const exportColumns = [
  { prop: "card_no", label: "卡号" },
  {
    prop: "app_uuid",
    label: "所属应用",
    formatter: (r: any) =>
      apps.value.find(a => a.uuid === r.app_uuid)?.name || r.app_uuid
  },
  { prop: "duration_text", label: "面值时长" },
  { prop: "status_text", label: "状态" },
  { prop: "batch_no", label: "批次号" },
  { prop: "used_at", label: "核销时间" },
  { prop: "remark", label: "备注" },
  { prop: "created_at", label: "创建时间" }
];

// 批量导出：勾选 → 导出选中；未勾选 → 按当前筛选导出全部
const { exporting, handleExport } = useSmartExport({
  filename: "cards",
  columns: exportColumns,
  getSelected: () => selectedRows.value,
  fetchAll: async () => {
    const { code, data } = await exportCards({
      app_uuid: form.app_uuid,
      status: form.status,
      batch_no: form.batch_no,
      search: form.search
    });
    return code === 0 ? data || [] : [];
  }
});

// 导出批次：指定批次号导出整批（忽略勾选与其它筛选）
const exportBatchNo = ref("");
const { exporting: exportingBatch, handleExport: doExportBatch } =
  useSmartExport({
    filename: "cards_batch",
    columns: exportColumns,
    getSelected: () => [],
    fetchAll: async () => {
      const { code, data } = await exportCards({
        batch_no: exportBatchNo.value,
        app_uuid: form.app_uuid
      });
      return code === 0 ? data || [] : [];
    }
  });

async function onExportBatch() {
  try {
    const { value } = await ElMessageBox.prompt(
      "请输入要导出的批次号",
      "导出批次",
      {
        confirmButtonText: "导出",
        cancelButtonText: "取消",
        inputValue: form.batch_no || "",
        inputValidator: (v: string) => (v && v.trim() ? true : "批次号不能为空")
      }
    );
    exportBatchNo.value = value.trim();
    await doExportBatch();
  } catch (e) {
    // cancelled
  }
}

async function onBatchFreeze() {
  if (selectedNum.value === 0) return;
  const { code, msg } = await freezeCards({ ids: selectedIds.value });
  if (code === 0) {
    ElMessage.success("批量冻结成功");
    onSearch();
  } else {
    ElMessage.error(msg || "批量冻结失败");
  }
}

async function onBatchUnfreeze() {
  if (selectedNum.value === 0) return;
  const { code, msg } = await unfreezeCards({ ids: selectedIds.value });
  if (code === 0) {
    ElMessage.success("批量解冻成功");
    onSearch();
  } else {
    ElMessage.error(msg || "批量解冻失败");
  }
}

async function onDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认删除卡密 <strong style="color:red">${row.card_no}</strong> 吗？<br><span style="color:red;font-size:12px;">注意：此操作不可恢复！</span>`,
      "提示",
      {
        type: "warning",
        dangerouslyUseHTMLString: true
      }
    );
    const { code, msg } = await batchDeleteCards({ ids: [row.id] });
    if (code === 0) {
      ElMessage.success("删除成功");
      onSearch();
    } else {
      ElMessage.error(msg || "删除失败");
    }
  } catch (e) {
    // cancelled
  }
}

async function onBatchDel() {
  if (selectedNum.value === 0) return;
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedNum.value} 张卡密吗？<br><span style="color:red;font-size:12px;">注意：此操作不可恢复！</span>`,
      "提示",
      {
        type: "warning",
        dangerouslyUseHTMLString: true
      }
    );
    const { code, msg } = await batchDeleteCards({ ids: selectedIds.value });
    if (code === 0) {
      ElMessage.success("批量删除成功");
      onSearch();
    } else {
      ElMessage.error(msg || "批量删除失败");
    }
  } catch (e) {
    // cancelled
  }
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
      <el-form-item label="卡号" prop="search">
        <el-input
          v-model="form.search"
          placeholder="精确卡号"
          clearable
          class="w-[180px]!"
          @keyup.enter="onSearch"
        />
      </el-form-item>

      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="请选择应用"
          clearable
          class="w-[180px]!"
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

      <el-form-item label="状态" prop="status">
        <el-select
          v-model="form.status"
          placeholder="全部"
          clearable
          class="w-[140px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option label="未使用" :value="0" />
          <el-option label="已使用" :value="1" />
          <el-option label="已冻结" :value="2" />
        </el-select>
      </el-form-item>

      <el-form-item label="批次号" prop="batch_no">
        <el-input
          v-model="form.batch_no"
          placeholder="批次号"
          clearable
          class="w-[160px]!"
          @keyup.enter="onSearch"
        />
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
          @click="resetFormSearch(formRef)"
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
          @click="openCreateDialog"
        >
          批量制卡
        </el-button>
        <el-button
          type="success"
          plain
          :icon="useRenderIcon('ep:download')"
          :loading="exporting"
          @click="handleExport"
        >
          {{ selectedNum > 0 ? `导出选中(${selectedNum})` : "批量导出" }}
        </el-button>
        <el-button
          plain
          :icon="useRenderIcon('ep:files')"
          :loading="exportingBatch"
          @click="onExportBatch"
        >
          导出批次
        </el-button>
        <el-button
          type="warning"
          :icon="useRenderIcon('ep:lock')"
          :disabled="selectedNum === 0"
          @click="onBatchFreeze"
        >
          批量冻结
        </el-button>
        <el-button
          type="success"
          :icon="useRenderIcon('ep:unlock')"
          :disabled="selectedNum === 0"
          @click="onBatchUnfreeze"
        >
          批量解冻
        </el-button>
        <el-button
          type="danger"
          :icon="useRenderIcon('ep:delete')"
          :disabled="selectedNum === 0"
          @click="onBatchDel"
        >
          批量删除
        </el-button>
      </div>

      <PureTableBar title="卡密管理" :columns="columns" @refresh="onSearch">
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
                v-if="row.status !== 2"
                class="reset-margin"
                link
                type="warning"
                :size="size"
                :icon="useRenderIcon('ep:lock')"
                @click="handleFreeze(row)"
              >
                冻结
              </el-button>
              <el-button
                v-else
                class="reset-margin"
                link
                type="success"
                :size="size"
                :icon="useRenderIcon('ep:unlock')"
                @click="handleUnfreeze(row)"
              >
                解冻
              </el-button>
              <el-button
                class="reset-margin"
                link
                type="danger"
                :size="size"
                :icon="useRenderIcon('ep:delete')"
                @click="onDelete(row)"
              >
                删除
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
:deep(.el-dropdown-menu__item i) {
  margin: 0;
}

.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}

.toolbar {
  display: flex;
  gap: 10px;
}
</style>
