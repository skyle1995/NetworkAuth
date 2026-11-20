<script setup lang="ts">
import { ref } from "vue";
import { useApi } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessage } from "element-plus";
import { exportApiKeys } from "@/api/admin/api";

defineOptions({
  name: "Apis"
});

const exporting = ref(false);
const {
  loading,
  columns,
  dataList,
  pagination,
  selectedIds,
  handleSelectionChange,
  openBatchDialog,
  onSearch,
  openDialog,
  handleSizeChange,
  handleCurrentChange
} = useApi();

function categorySpan({ column, rowIndex }: {
  column: { property?: string };
  rowIndex: number;
}) {
  if (column.property !== "category") return [1, 1];
  const rows = dataList.value;
  const category = rows[rowIndex].category;
  if (rowIndex > 0 && rows[rowIndex - 1].category === category) return [0, 0];
  let end = rowIndex + 1;
  while (end < rows.length && rows[end].category === category) end++;
  return [end - rowIndex, 1];
}

// 导出全站统一的接口配置
async function handleExport() {
  exporting.value = true;
  try {
    const { code, msg, data } = await exportApiKeys();
    if (code !== 0) {
      ElMessage.error(msg || "导出失败");
      return;
    }
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json"
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "接口对接配置.json";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    ElMessage.success("已导出对接密钥");
  } catch (e: any) {
    ElMessage.error(e?.message || "导出失败");
  } finally {
    exporting.value = false;
  }
}
</script>

<template>
  <div class="main-content">
    <el-card shadow="never" class="table-wrapper">
      <PureTableBar title="接口设置" :columns="columns" @refresh="onSearch">
        <template #buttons>
          <el-button
            type="primary"
            :icon="useRenderIcon('ep:lock')"
            :disabled="selectedIds.length === 0"
            @click="openBatchDialog"
          >
            批量设置算法{{
              selectedIds.length ? `(${selectedIds.length})` : ""
            }}
          </el-button>
          <el-button
            type="success"
            :icon="useRenderIcon('ep:download')"
            :loading="exporting"
            @click="handleExport"
          >
            导出密钥
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
            :span-method="categorySpan"
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
                type="primary"
                :size="size"
                :icon="useRenderIcon('ep:edit')"
                @click="openDialog('修改接口配置', row)"
              >
                配置
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

<style lang="scss" scoped>
:deep(.el-dropdown-menu__item i) {
  margin: 0;
}

.main-content {
  margin: 24px !important;
}


</style>
