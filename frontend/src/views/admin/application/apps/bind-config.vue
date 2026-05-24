<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    machine_verify: number;
    machine_rebind_enabled: number;
    machine_rebind_limit: number;
    machine_free_count: number;
    machine_rebind_count: number;
    machine_rebind_deduct: number;
    ip_verify: number;
    ip_rebind_enabled: number;
    ip_rebind_limit: number;
    ip_free_count: number;
    ip_rebind_count: number;
    ip_rebind_deduct: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    machine_verify: 0,
    machine_rebind_enabled: 0,
    machine_rebind_limit: 0,
    machine_free_count: 0,
    machine_rebind_count: 0,
    machine_rebind_deduct: 0,
    ip_verify: 0,
    ip_rebind_enabled: 0,
    ip_rebind_limit: 0,
    ip_free_count: 0,
    ip_rebind_count: 0,
    ip_rebind_deduct: 0
  })
});

const newFormInline = ref(props.formInline);

function getRef() {
  return formRef.value;
}

const formRef = ref();
defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form ref="formRef" :model="newFormInline" label-width="120px">
    <el-divider>机器验证设置</el-divider>
    <el-form-item label="机器码验证" prop="machine_verify">
      <el-radio-group v-model="newFormInline.machine_verify">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="机器码重绑" prop="machine_rebind_enabled">
      <el-radio-group v-model="newFormInline.machine_rebind_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="重绑限制" prop="machine_rebind_limit">
      <el-radio-group v-model="newFormInline.machine_rebind_limit">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="免费次数" prop="machine_free_count">
      <el-input-number v-model="newFormInline.machine_free_count" :min="0" />
    </el-form-item>
    <el-form-item label="重绑次数" prop="machine_rebind_count">
      <el-input-number v-model="newFormInline.machine_rebind_count" :min="0" />
    </el-form-item>
    <el-form-item label="重绑扣除" prop="machine_rebind_deduct">
      <el-input-number v-model="newFormInline.machine_rebind_deduct" :min="0" />
      <span class="ml-2">分钟</span>
    </el-form-item>

    <el-divider>IP验证设置</el-divider>
    <el-form-item label="IP地址验证" prop="ip_verify">
      <el-select v-model="newFormInline.ip_verify" class="w-full">
        <el-option label="关闭" :value="0" />
        <el-option label="仅验证不扣除" :value="1" />
        <el-option label="验证并扣除时长" :value="2" />
        <el-option label="验证并扣除点数" :value="3" />
      </el-select>
    </el-form-item>
    <el-form-item label="IP地址重绑" prop="ip_rebind_enabled">
      <el-radio-group v-model="newFormInline.ip_rebind_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="重绑限制" prop="ip_rebind_limit">
      <el-radio-group v-model="newFormInline.ip_rebind_limit">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="免费次数" prop="ip_free_count">
      <el-input-number v-model="newFormInline.ip_free_count" :min="0" />
    </el-form-item>
    <el-form-item label="重绑次数" prop="ip_rebind_count">
      <el-input-number v-model="newFormInline.ip_rebind_count" :min="0" />
    </el-form-item>
    <el-form-item label="重绑扣除" prop="ip_rebind_deduct">
      <el-input-number v-model="newFormInline.ip_rebind_deduct" :min="0" />
      <span class="ml-2">分钟/点数</span>
    </el-form-item>
  </el-form>
</template>
