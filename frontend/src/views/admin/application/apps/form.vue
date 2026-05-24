<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    id?: number;
    name: string;
    version: string;
    status: number;
    force_update: number;
    download_type: number;
    download_url: string;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    name: "",
    version: "1.0.0",
    status: 1,
    force_update: 0,
    download_type: 0,
    download_url: ""
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
  <el-form ref="formRef" :model="newFormInline" label-width="82px">
    <el-form-item
      label="应用名称"
      prop="name"
      :rules="[{ required: true, message: '请输入应用名称' }]"
    >
      <el-input
        v-model="newFormInline.name"
        clearable
        placeholder="请输入应用名称"
      />
    </el-form-item>
    <el-form-item label="应用版本" prop="version">
      <el-input
        v-model="newFormInline.version"
        clearable
        placeholder="请输入应用版本，默认1.0.0"
      />
    </el-form-item>
    <el-form-item label="应用状态" prop="status">
      <el-switch
        v-model="newFormInline.status"
        :active-value="1"
        :inactive-value="0"
        active-text="启用"
        inactive-text="禁用"
        inline-prompt
      />
    </el-form-item>
    <el-form-item label="强制更新" prop="force_update">
      <el-switch
        v-model="newFormInline.force_update"
        :active-value="1"
        :inactive-value="0"
        active-text="开启"
        inactive-text="关闭"
        inline-prompt
      />
    </el-form-item>
    <el-form-item label="更新方式" prop="download_type">
      <el-radio-group v-model="newFormInline.download_type">
        <el-radio :value="0">不启用</el-radio>
        <el-radio :value="1">自动更新</el-radio>
        <el-radio :value="2">手动下载</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item
      v-if="newFormInline.download_type !== 0"
      label="下载地址"
      prop="download_url"
    >
      <el-input
        v-model="newFormInline.download_url"
        clearable
        placeholder="请输入下载/更新地址"
      />
    </el-form-item>
  </el-form>
</template>
