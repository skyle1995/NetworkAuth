<script setup lang="ts">
export interface Session {
  id: number;
  machine_code?: string;
  ip?: string;
  last_active_at?: string;
  created_at?: string;
}

defineProps<{
  sessions: Session[];
}>();

const emit = defineEmits<{
  (e: "kick", id: number): void;
}>();
</script>

<template>
  <div class="py-1">
    <el-table :data="sessions" border size="small" empty-text="暂无在线会话">
      <el-table-column
        prop="machine_code"
        label="机器码"
        min-width="160"
        show-overflow-tooltip
      />
      <el-table-column prop="ip" label="登录IP" width="130" />
      <el-table-column prop="last_active_at" label="最近活跃" width="160" />
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-button
            link
            type="danger"
            size="small"
            @click="emit('kick', row.id)"
          >
            踢下线
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
