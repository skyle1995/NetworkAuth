<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    login_type: number;
      clean_interval: number;
    check_interval: number;
    offline_timeout: number;
    };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    login_type: 0,
    clean_interval: 1,
    check_interval: 10,
    offline_timeout: 30,
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
    <el-form-item label="满员策略" prop="login_type">
      <el-radio-group v-model="newFormInline.login_type">
        <el-radio :value="0">顶掉最早的一个</el-radio>
        <el-radio :value="1">拒绝新的</el-radio>
        <el-radio :value="2">让用户手动选</el-radio>
      </el-radio-group>
      <div class="text-xs text-gray-500 mt-1">
        账号的「同时在线」名额满了以后，进入本应用时怎么处理
      </div>
    </el-form-item>

    <el-form-item label="清理间隔" prop="clean_interval">
      <el-input-number v-model="newFormInline.clean_interval" :min="1" />
      <span class="ml-2">小时</span>
    </el-form-item>
    <el-form-item label="心跳间隔" prop="check_interval">
      <el-input-number v-model="newFormInline.check_interval" :min="1" />
      <span class="ml-2 text-xs text-gray-400"
        >分钟 · 登录后返回给客户端，告知其心跳周期</span
      >
    </el-form-item>
    <el-form-item label="自动离线" prop="offline_timeout">
      <el-input-number v-model="newFormInline.offline_timeout" :min="1" />
      <span class="ml-2 text-xs text-gray-400"
        >分钟 · 超过该时长未心跳则后台自动离线（建议 ≥ 心跳间隔）</span
      >
    </el-form-item>
  </el-form>
</template>
