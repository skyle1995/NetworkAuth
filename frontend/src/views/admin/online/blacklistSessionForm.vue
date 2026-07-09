<script setup lang="ts">
import { ref, computed } from "vue";

export interface FormProps {
  formInline: {
    username: string;
    machine_code: string;
    ip: string;
    province: string;
    city: string;
    blacklist_device: boolean;
    blacklist_ip: boolean;
    blacklist_region: boolean;
    blacklist_account: boolean;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    username: "",
    machine_code: "",
    ip: "",
    province: "",
    city: "",
    blacklist_device: false,
    blacklist_ip: false,
    blacklist_region: false,
    blacklist_account: false
  })
});

const f = ref(props.formInline);

const regionText = computed(
  () => [f.value.province, f.value.city].filter(Boolean).join(" ") || "无法识别"
);
const hasRegion = computed(() => !!(f.value.province && f.value.city));
</script>

<template>
  <div class="py-1">
    <el-alert
      :closable="false"
      type="warning"
      show-icon
      class="mb-3"
      title="拉黑后命中的设备/网络将无法用任何账号登录本应用，并立即踢掉在线会话。"
    />
    <el-checkbox v-model="f.blacklist_device" :disabled="!f.machine_code">
      拉黑设备(机器码)：<b>{{ f.machine_code || "无" }}</b>
    </el-checkbox>
    <br />
    <el-checkbox v-model="f.blacklist_ip" :disabled="!f.ip">
      拉黑IP：<b>{{ f.ip || "无" }}</b>
    </el-checkbox>
    <br />
    <el-checkbox v-model="f.blacklist_region" :disabled="!hasRegion">
      拉黑地区(省/市)：<b>{{ regionText }}</b>
    </el-checkbox>
    <br />
    <el-checkbox v-model="f.blacklist_account">
      同时拉黑账号：<b>{{ f.username }}</b>（该账号置黑并全部下线）
    </el-checkbox>
    <p class="mt-2 text-xs" style="color: var(--el-text-color-secondary)">
      地区按「地级市」粒度封禁，范围较大请谨慎；未能识别归属地时地区项不可选。
    </p>
  </div>
</template>
