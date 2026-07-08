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

const formRef = ref();
const exporting = ref(false);
const {
  form,
  loading,
  columns,
  dataList,
  appList,
  apiTypes,
  pagination,
  onSearch,
  resetForm,
  openDialog,
  handleSizeChange,
  handleCurrentChange
} = useApi();

// 导出所选应用的对接密钥为 JSON 文件
async function handleExport() {
  if (!form.app_uuid) {
    ElMessage.warning("请先在上方选择要导出的应用");
    return;
  }
  exporting.value = true;
  try {
    const { code, msg, data } = await exportApiKeys({
      app_uuid: form.app_uuid
    });
    if (code !== 0) {
      ElMessage.error(msg || "导出失败");
      return;
    }
    const appName = data?.app?.name || "app";
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json"
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${appName}_对接密钥.json`;
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
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="请选择应用"
          clearable
          class="!w-[200px]"
        >
          <el-option
            v-for="item in appList"
            :key="item.uuid"
            :label="item.name"
            :value="item.uuid"
          />
        </el-select>
      </el-form-item>

      <el-form-item label="接口类型" prop="api_type">
        <el-select
          v-model="form.api_type"
          placeholder="请选择接口类型"
          clearable
          class="!w-[200px]"
        >
          <el-option
            v-for="item in apiTypes"
            :key="item.value"
            :label="item.label"
            :value="item.value"
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
        <el-button
          type="success"
          :icon="useRenderIcon('ep:download')"
          :loading="exporting"
          :disabled="!form.app_uuid"
          @click="handleExport"
        >
          导出密钥
        </el-button>
      </el-form-item>
    </el-form>

    <el-card shadow="never" class="table-wrapper mt-4">
      <PureTableBar title="接口设置" :columns="columns" @refresh="onSearch">
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

.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}
</style>
