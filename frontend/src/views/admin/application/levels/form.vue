<script setup lang="ts">
import { ref } from "vue";
import type { FormRules } from "element-plus";

export interface FormProps {
  formInline: {
    uuid: string;
    app_uuid: string;
    name: string;
    threshold_yuan: number;
    rebate_rate: number;
    extra_multi_open: number;
    extra_rebind_count: number;
    sort: number;
    status: number;
    remark: string;
  };
  apps: Array<{ id: number; uuid: string; name: string }>;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    uuid: "",
    app_uuid: "",
    name: "",
    threshold_yuan: 0,
    rebate_rate: 0,
    extra_multi_open: 0,
    extra_rebind_count: 0,
    sort: 0,
    status: 1,
    remark: ""
  }),
  apps: () => []
});

const rules: FormRules = {
  app_uuid: [{ required: true, message: "请选择所属应用", trigger: "change" }],
  name: [{ required: true, message: "请输入等级名称", trigger: "blur" }]
};

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form
    ref="ruleFormRef"
    :model="newFormInline"
    :rules="rules"
    label-width="100px"
  >
    <el-form-item label="所属应用" prop="app_uuid">
      <el-select
        v-model="newFormInline.app_uuid"
        placeholder="请选择所属应用"
        class="w-full"
        :disabled="!!newFormInline.uuid"
      >
        <el-option
          v-for="app in apps"
          :key="app.uuid"
          :label="`${app.name} (ID: ${app.id})`"
          :value="app.uuid"
        />
      </el-select>
    </el-form-item>

    <el-form-item label="等级名称" prop="name">
      <el-input v-model="newFormInline.name" placeholder="如 白银 / 黄金" />
    </el-form-item>

    <el-form-item label="累充门槛" prop="threshold_yuan">
      <el-input-number
        v-model="newFormInline.threshold_yuan"
        :min="0"
        :precision="2"
        :step="10"
      />
      <span class="ml-2">元</span>
    </el-form-item>

    <el-form-item label="充值返利" prop="rebate_rate">
      <el-input-number
        v-model="newFormInline.rebate_rate"
        :min="0"
        :max="100"
      />
      <span class="ml-2">%</span>
    </el-form-item>

    <el-form-item label="额外多开" prop="extra_multi_open">
      <el-input-number v-model="newFormInline.extra_multi_open" :min="0" />
      <span class="ml-2">台</span>
    </el-form-item>

    <el-form-item label="赠送换绑" prop="extra_rebind_count">
      <el-input-number v-model="newFormInline.extra_rebind_count" :min="0" />
      <span class="ml-2">次</span>
    </el-form-item>

    <el-form-item label="排序" prop="sort">
      <el-input-number v-model="newFormInline.sort" :min="0" />
    </el-form-item>

    <el-form-item label="状态" prop="status">
      <el-radio-group v-model="newFormInline.status">
        <el-radio :value="0">禁用</el-radio>
        <el-radio :value="1">启用</el-radio>
      </el-radio-group>
    </el-form-item>

    <el-form-item label="备注" prop="remark">
      <el-input v-model="newFormInline.remark" type="textarea" :rows="2" />
    </el-form-item>
  </el-form>
</template>
