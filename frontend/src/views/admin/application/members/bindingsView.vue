<script setup lang="ts">
export interface Binding {
  type: number;
  value: string;
  province?: string;
  city?: string;
  created_at?: string;
}

defineProps<{
  bindings: Binding[];
}>();

function typeText(t: number) {
  return t === 1 ? "IP" : "机器码";
}
</script>

<template>
  <div class="py-1">
    <el-table :data="bindings" border size="small" empty-text="暂无绑定">
      <el-table-column label="类型" width="90">
        <template #default="{ row }">{{ typeText(row.type) }}</template>
      </el-table-column>
      <el-table-column
        prop="value"
        label="绑定值"
        min-width="180"
        show-overflow-tooltip
      />
      <el-table-column label="归属地" min-width="140">
        <template #default="{ row }">
          {{ [row.province, row.city].filter(Boolean).join(" ") || "—" }}
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="绑定时间" width="170" />
    </el-table>
  </div>
</template>
