<script setup lang="ts">
import { ref } from "vue";
import dayjs from "dayjs";
import { useRole } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { useUserStoreHook } from "@/store/modules/user";
import { useSmartExport } from "@/composables/useSmartExport";
import { useSmartDelete } from "@/composables/useSmartDelete";
import { clearOperationLogs, batchDeleteOperationLogs } from "@/api/admin/logs";

defineOptions({
  name: "OperationLog"
});

const userStore = useUserStoreHook();
const formRef = ref();
const tableRef = ref();

const {
  form,
  loading,
  columns,
  dataList,
  pagination,
  selectedRows,
  onSearch,
  resetForm,
  handleSelectionChange,
  handleSizeChange,
  handleCurrentChange
} = useRole();

// 智能导出：勾选 → 导出选中；未勾选 → 导出当前筛选页（CSV）
const { exporting, handleExport } = useSmartExport({
  filename: "operation_logs",
  columns: [
    { prop: "id", label: "序号" },
    { prop: "operator", label: "操作人员" },
    { prop: "operator_uuid", label: "操作人员 UUID" },
    {
      prop: "created_at",
      label: "操作时间",
      formatter: (r: any) => dayjs(r.created_at).format("YYYY-MM-DD HH:mm:ss")
    },
    { prop: "operation_type", label: "操作方式" },
    { prop: "details", label: "操作详情" }
  ],
  getSelected: () => selectedRows.value,
  getFallback: () => dataList.value
});

// 智能删除：勾选 → 批量删除；未勾选 → 清空全部（高危确认）
const { deleting, handleDelete } = useSmartDelete({
  entityName: "操作日志",
  getSelectedIds: () => selectedRows.value.map((r: any) => r.id),
  batchDelete: ids => batchDeleteOperationLogs(ids),
  clearAll: () => clearOperationLogs(),
  onDone: () => {
    selectedRows.value = [];
    onSearch();
  }
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
      <el-form-item label="操作人员" prop="operator">
        <el-input
          v-model="form.operator"
          placeholder="请输入操作人员"
          clearable
          class="w-[150px]!"
        />
      </el-form-item>
      <el-form-item label="操作方式" prop="operation_type">
        <el-input
          v-model="form.operation_type"
          placeholder="请输入操作方式"
          clearable
          class="w-[150px]!"
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
          @click="resetForm(formRef)"
        >
          重置
        </el-button>
      </el-form-item>
    </el-form>

    <PureTableBar title="操作日志" :columns="columns" @refresh="onSearch">
      <template #buttons>
        <el-button
          :icon="useRenderIcon('ep:download')"
          :loading="exporting"
          @click="handleExport"
        >
          {{
            selectedRows.length
              ? `导出选中(${selectedRows.length})`
              : "导出筛选"
          }}
        </el-button>
        <el-button
          v-if="userStore.roles.includes('super_admin')"
          type="danger"
          :icon="useRenderIcon('ep:delete')"
          :loading="deleting"
          @click="handleDelete"
        >
          {{
            selectedRows.length
              ? `删除选中(${selectedRows.length})`
              : "清空日志"
          }}
        </el-button>
      </template>
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
          @selection-change="handleSelectionChange"
        />
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
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-dropdown-menu__item i) {
  margin: 0;
}

.main-content {
  margin: 24px !important;
}

.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}
</style>
