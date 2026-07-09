<script setup lang="ts">
import { ref, computed } from "vue";

export interface FormProps {
  operation_mode?: number;
  formInline: {
    register_enabled: number;
    email_verify_enabled: number;
    register_limit_enabled: number;
    register_device_limit_enabled: number;
    register_limit_time: number;
    register_count: number;
    trial_enabled: number;
    trial_limit_time: number;
    trial_duration: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  operation_mode: 0,
  formInline: () => ({
    register_enabled: 0,
    email_verify_enabled: 0,
    register_limit_enabled: 0,
    register_device_limit_enabled: 0,
    register_limit_time: 0,
    register_count: 1,
    trial_enabled: 0,
    trial_limit_time: 0,
    trial_duration: 1
  })
});

const newFormInline = ref(props.formInline);

// 点数模式发放试用点数，时长模式发放试用时长
const isPoints = computed(() => props.operation_mode === 1);
const trialLabel = computed(() => (isPoints.value ? "试用点数" : "试用时长"));
const trialUnit = computed(() => (isPoints.value ? "点" : "分钟"));

function getRef() {
  return formRef.value;
}

const formRef = ref();
defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form ref="formRef" :model="newFormInline" label-width="120px">
    <el-divider>账号注册设置</el-divider>
    <el-form-item label="账号注册" prop="register_enabled">
      <el-radio-group v-model="newFormInline.register_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="邮箱验证" prop="email_verify_enabled">
      <el-radio-group v-model="newFormInline.email_verify_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="IP注册限制" prop="register_limit_enabled">
      <el-radio-group v-model="newFormInline.register_limit_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="设备注册限制" prop="register_device_limit_enabled">
      <el-radio-group v-model="newFormInline.register_device_limit_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs" style="color: var(--el-text-color-secondary)">
        开启后客户端注册须提交设备码
      </span>
    </el-form-item>
    <el-form-item label="限制时间" prop="register_limit_time">
      <el-radio-group v-model="newFormInline.register_limit_time">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="注册次数" prop="register_count">
      <el-input-number v-model="newFormInline.register_count" :min="1" />
    </el-form-item>

    <el-divider>领取试用设置</el-divider>
    <el-form-item label="领取试用" prop="trial_enabled">
      <el-radio-group v-model="newFormInline.trial_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="限制时间" prop="trial_limit_time">
      <el-radio-group v-model="newFormInline.trial_limit_time">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item :label="trialLabel" prop="trial_duration">
      <el-input-number v-model="newFormInline.trial_duration" :min="1" />
      <span class="ml-2">{{ trialUnit }}</span>
    </el-form-item>
  </el-form>
</template>
