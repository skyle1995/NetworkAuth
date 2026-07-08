<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ row: any }>();

const typeText = computed(() =>
  props.row.type === 1 ? "卡密账号" : "注册账号"
);
const statusText = computed(
  () => props.row.status_text || String(props.row.status)
);
const quota = computed(() =>
  props.row.mode === 1 ? `${props.row.points} 点` : props.row.expired_at
);
const trialText = computed(() =>
  props.row.trial_used
    ? `已领取${props.row.trial_date ? `（${props.row.trial_date}）` : ""}`
    : "未领取"
);

function dash(v: any) {
  return v === null || v === undefined || v === "" ? "—" : v;
}
</script>

<template>
  <div class="py-1">
    <el-descriptions :column="2" border size="small">
      <el-descriptions-item label="用户名">{{
        dash(row.username)
      }}</el-descriptions-item>
      <el-descriptions-item label="邮箱">{{
        dash(row.email)
      }}</el-descriptions-item>
      <el-descriptions-item label="类型">{{ typeText }}</el-descriptions-item>
      <el-descriptions-item label="状态">{{ statusText }}</el-descriptions-item>
      <el-descriptions-item label="额度(到期/余额)">{{
        dash(quota)
      }}</el-descriptions-item>
      <el-descriptions-item label="注册IP">{{
        dash(row.register_ip)
      }}</el-descriptions-item>
      <el-descriptions-item label="最近登录IP">{{
        dash(row.last_login_ip)
      }}</el-descriptions-item>
      <el-descriptions-item label="最近登录时间">{{
        dash(row.last_login_at)
      }}</el-descriptions-item>
      <el-descriptions-item label="机器码转绑次数">{{
        row.machine_rebind_used ?? 0
      }}</el-descriptions-item>
      <el-descriptions-item label="IP转绑次数">{{
        row.ip_rebind_used ?? 0
      }}</el-descriptions-item>
      <el-descriptions-item label="试用领取">{{
        trialText
      }}</el-descriptions-item>
      <el-descriptions-item label="来源卡">{{
        dash(row.card_uuid)
      }}</el-descriptions-item>
      <el-descriptions-item label="备注" :span="2">{{
        dash(row.remark)
      }}</el-descriptions-item>
      <el-descriptions-item label="创建时间" :span="2">{{
        dash(row.created_at)
      }}</el-descriptions-item>
    </el-descriptions>
  </div>
</template>
