<script setup lang="ts">
import { useCardPackage } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "PackagesIndex"
});


const {
  loading,
  columns,
  dataList,
  loadList,
  openDialog,
  handleDelete
} = useCardPackage();
</script>

<template>
  <div class="main">

    <PureTableBar title="卡密套餐" :columns="columns" @refresh="loadList">
      <template #buttons>
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:plus')"
          @click="openDialog()"
        >
          新增套餐
        </el-button>
      </template>
      <template v-slot="{ size, dynamicColumns }">
        <pure-table
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
        >
          <template #operation="{ row }">
            <el-button
              class="reset-margin"
              link
              type="primary"
              :size="size"
              :icon="useRenderIcon('ep:edit-pen')"
              @click="openDialog(row)"
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
      </template>
    </PureTableBar>
  </div>
</template>


