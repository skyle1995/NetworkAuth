<script setup lang="ts">
import { ref, computed } from "vue";
import { formRules } from "./rule";

export interface FormProps {
  formInline: {
    app_uuid: string;
    prefix: string;
    length: number;
    count: number;
    duration_value: number;
    duration_unit: string;
    remark: string;
  };
  apps: Array<{ id: number; uuid: string; name: string }>;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    app_uuid: "",
    prefix: "",
    length: 16,
    count: 10,
    duration_value: 30,
    duration_unit: "day",
    remark: ""
  }),
  apps: () => []
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

// 永久时不需要填写时长数值
const isPermanent = computed(
  () => newFormInline.value.duration_unit === "permanent"
);

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
        <el-form-item label="所属应用" prop="app_uuid">
          <el-select
            v-model="newFormInline.app_uuid"
            placeholder="请选择所属应用"
            class="w-full"
            clearable
          >
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

    <el-row :gutter="16">
      <el-col :span="12">
        <el-form-item label="生成数量" prop="count">
          <el-input-number
            v-model="newFormInline.count"
            :min="1"
            :max="10000"
            class="!w-full"
          />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="卡号长度" prop="length">
          <el-input-number
            v-model="newFormInline.length"
            :min="8"
            :max="32"
            class="!w-full"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="卡号前缀" prop="prefix">
          <el-input
            v-model="newFormInline.prefix"
            placeholder="可选，如 VIP（仅字母数字，用于区分注册用户名）"
            maxlength="16"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="12">
        <el-form-item label="卡密时长" prop="duration_value">
          <el-input-number
            v-model="newFormInline.duration_value"
            :min="1"
            :disabled="isPermanent"
            class="!w-full"
          />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="时长单位" prop="duration_unit">
          <el-select v-model="newFormInline.duration_unit" class="w-full">
            <el-option label="分钟" value="minute" />
            <el-option label="小时" value="hour" />
            <el-option label="天" value="day" />
            <el-option label="月" value="month" />
            <el-option label="年" value="year" />
            <el-option label="永久" value="permanent" />
          </el-select>
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
            placeholder="请输入备注说明（可选）"
          />
        </el-form-item>
      </el-col>
    </el-row>
  </el-form>
</template>
