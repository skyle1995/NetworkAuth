<script setup lang="ts">
import { ref } from "vue";
import { formRules } from "./rule";

export interface FormProps {
  formInline: {
    id?: number;
    uuid?: string;
    alias: string;
    app_uuid: string;
    data: string;
    remark: string;
  };
  apps: Array<{ id: number; uuid: string; name: string }>;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    alias: "",
    app_uuid: "0",
    data: "",
    remark: ""
  }),
  apps: () => []
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef });
</script>

<template>
  <el-form
    ref="ruleFormRef"
    :model="newFormInline"
    :rules="formRules"
    label-width="100px"
  >
    <el-row>
      <el-col :span="24">
        <el-form-item label="变量别名" prop="alias">
          <el-input
            v-model="newFormInline.alias"
            placeholder="请输入变量别名 (字母开头，字母数字组合)"
            :disabled="!!newFormInline.id"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="所属应用" prop="app_uuid">
          <el-select
            v-model="newFormInline.app_uuid"
            placeholder="请选择所属应用 (默认全局变量)"
            class="w-full"
            clearable
          >
            <el-option label="全局变量" value="0" />
            <el-option
              v-for="app in apps"
              :key="app.uuid"
              :label="`${app.name} (ID: ${app.id})`"
              :value="app.uuid"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="变量数据" prop="data">
          <el-input
            v-model="newFormInline.data"
            type="textarea"
            :rows="6"
            placeholder="请输入变量数据"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="备注说明" prop="remark">
          <el-input
            v-model="newFormInline.remark"
            type="textarea"
            :rows="3"
            placeholder="请输入备注说明"
          />
        </el-form-item>
      </el-col>
    </el-row>
  </el-form>
</template>
