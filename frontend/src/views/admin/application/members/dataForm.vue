<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    data: string;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({ data: "" })
});

const newFormInline = ref(props.formInline);
</script>

<template>
  <div class="py-1">
    <el-input
      v-model="newFormInline.data"
      type="textarea"
      :rows="10"
      placeholder="该终端用户的独有数据，客户端可通过「获取程序数据」接口读取（user_data 字段）"
    />
  </div>
</template>
