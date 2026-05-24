<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    login_type: number;
    multi_open_scope: number;
    clean_interval: number;
    check_interval: number;
    multi_open_count: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    login_type: 0,
    multi_open_scope: 0,
    clean_interval: 1,
    check_interval: 1,
    multi_open_count: 1
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
  <el-form ref="formRef" :model="newFormInline" label-width="100px">
    <el-form-item label="登录方式" prop="login_type">
      <el-radio-group v-model="newFormInline.login_type">
        <el-radio :value="0">顶号登录</el-radio>
        <el-radio :value="1">非顶号登录</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="多开范围" prop="multi_open_scope">
      <el-radio-group v-model="newFormInline.multi_open_scope">
        <el-radio :value="0">单设备</el-radio>
        <el-radio :value="1">单IP</el-radio>
        <el-radio :value="2">全部设备</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="清理间隔" prop="clean_interval">
      <el-input-number v-model="newFormInline.clean_interval" :min="1" />
      <span class="ml-2">小时</span>
    </el-form-item>
    <el-form-item label="校验间隔" prop="check_interval">
      <el-input-number v-model="newFormInline.check_interval" :min="1" />
      <span class="ml-2">分钟</span>
    </el-form-item>
    <el-form-item label="多开数量" prop="multi_open_count">
      <el-input-number v-model="newFormInline.multi_open_count" :min="1" />
    </el-form-item>
  </el-form>
</template>
