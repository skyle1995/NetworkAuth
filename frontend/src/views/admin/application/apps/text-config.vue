<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    content: string;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    content: ""
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
  <el-form ref="formRef" :model="newFormInline">
    <el-form-item prop="content">
      <el-input
        v-model="newFormInline.content"
        type="textarea"
        :rows="10"
        placeholder="请输入内容"
      />
    </el-form-item>
  </el-form>
</template>
