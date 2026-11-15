<script setup lang="ts">
import { ref } from "vue";
import type { FormRules } from "element-plus";

export interface FormProps {
  formInline: {
    uuid: string;
    name: string;
    level: number;
    color: string;
    threshold_yuan: number;
    rebate_rate: number;
    extra_devices: number;
  extra_concurrent: number;
    extra_rebind_count: number;
    sort: number;
    status: number;
    remark: string;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    uuid: "",
    name: "",
    level: 1,
    color: "#909399",
    threshold_yuan: 0,
    rebate_rate: 0,
    extra_devices: 0,
  extra_concurrent: 0,
    extra_rebind_count: 0,
    sort: 0,
    status: 1,
    remark: ""
  }),
});

const rules: FormRules = {
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
    <el-form-item label="等级名称" prop="name">
      <el-input v-model="newFormInline.name" placeholder="如 白银 / 黄金" />
    </el-form-item>

    <el-form-item label="权限等级" prop="level">
      <el-input-number v-model="newFormInline.level" :min="1" />
    </el-form-item>

    <el-form-item label="显示颜色" prop="color">
      <el-color-picker v-model="newFormInline.color" />
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
    <el-form-item label="额外设备" prop="extra_devices">
      <el-input-number v-model="newFormInline.extra_devices" :min="0" />
      <span class="ml-2">台</span>
      <div class="w-full text-xs text-gray-400 mt-1">
        在账号策略「可绑定设备」之上多给几台
      </div>
    </el-form-item>

    <el-form-item label="额外并发" prop="extra_concurrent">
      <el-input-number v-model="newFormInline.extra_concurrent" :min="0" />
      <span class="ml-2">个</span>
      <div class="w-full text-xs text-gray-400 mt-1">
        在账号策略「同时在线」之上多给几个实例
      </div>
    </el-form-item>

    <el-form-item label="赠送换绑" prop="extra_rebind_count">
      <el-input-number v-model="newFormInline.extra_rebind_count" :min="0" />
      <span class="ml-2">次</span>
      <div class="w-full text-xs text-gray-400 mt-1">
        在账号策略「免费转绑」之上多给几次
      </div>
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
