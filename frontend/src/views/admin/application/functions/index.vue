<script setup lang="ts">
import { ref, h } from "vue";
import { useFunction } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessageBox, ElMessage } from "element-plus";
import { batchDeleteFunctions } from "@/api/admin/function";

defineOptions({
  name: "Functions"
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
  openDialog,
  handleDelete,
  handleSizeChange,
  handleCurrentChange,
  handleSelectionChange
} = useFunction();

const selectedNum = ref(0);
const selectedIds = ref<number[]>([]);

function handleSelectionChangeVue(val: any[]) {
  selectedNum.value = val.length;
  selectedIds.value = val.map(item => item.id);
  handleSelectionChange(val);
}

async function onBatchDel() {
  if (selectedNum.value === 0) return;
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedNum.value} 个函数吗？<br><span style="color:red;font-size:12px;">注意：此操作不可恢复！</span>`,
      "提示",
      {
        type: "warning",
        dangerouslyUseHTMLString: true
      }
    );
    const { code, msg } = await batchDeleteFunctions({ ids: selectedIds.value });
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
      <el-form-item label="搜索内容" prop="search">
        <el-input
          v-model="form.search"
          placeholder="编号/别名/代码/备注"
          clearable
          class="!w-[180px]"
          @keyup.enter="onSearch"
        />
      </el-form-item>
      
      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="请选择所属应用"
          clearable
          class="!w-[180px]"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option label="全局函数" value="0" />
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
          @click="openDialog('新增')"
        >
          新增函数
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

      <PureTableBar title="公共函数管理" :columns="columns" @refresh="onSearch">
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
            <el-button
              class="reset-margin"
              link
              type="primary"
              :size="size"
              :icon="useRenderIcon('ep:edit-pen')"
              @click="openDialog('编辑', row)"
            >
              编辑
            </el-button>
            <el-button
              class="reset-margin"
              link
              type="danger"
              :size="size"
              :icon="useRenderIcon('ep:delete')"
              @click="handleDelete(row)"
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
</style>
