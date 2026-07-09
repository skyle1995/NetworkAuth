<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    submit_algorithm: number;
    return_algorithm: number;
    count: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({ submit_algorithm: 0, return_algorithm: 0, count: 0 })
});

const newFormInline = ref(props.formInline);

const ALGORITHMS = [
  { value: 0, label: "不加密" },
  { value: 1, label: "RC4" },
  { value: 2, label: "RSA" },
  { value: 3, label: "RSA（动态）" },
  { value: 4, label: "易加密" }
];

function getRef() {
  return formRef.value;
}

const formRef = ref();
defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form ref="formRef" :model="newFormInline" label-width="110px">
    <el-alert
      type="warning"
      :closable="false"
      show-icon
      class="mb-3"
      :title="`将对选中的 ${newFormInline.count} 个接口设置加密方式，并自动重新生成密钥（原有密钥会被覆盖）`"
    />
    <el-form-item label="提交数据算法" prop="submit_algorithm">
      <el-select v-model="newFormInline.submit_algorithm" class="w-full">
        <el-option
          v-for="a in ALGORITHMS"
          :key="a.value"
          :label="a.label"
          :value="a.value"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="返回数据算法" prop="return_algorithm">
      <el-select v-model="newFormInline.return_algorithm" class="w-full">
        <el-option
          v-for="a in ALGORITHMS"
          :key="a.value"
          :label="a.label"
          :value="a.value"
        />
      </el-select>
    </el-form-item>
  </el-form>
</template>
