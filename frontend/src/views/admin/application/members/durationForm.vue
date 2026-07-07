<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    duration_value: number;
    duration_unit: string;
  };
  /** 是否提供“永久”选项（充值可设永久，扣时不可） */
  allowPermanent: boolean;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    duration_value: 30,
    duration_unit: "day"
  }),
  allowPermanent: false
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef });
</script>

<template>
  <el-form ref="ruleFormRef" :model="newFormInline" label-width="90px">
    <el-row :gutter="16">
      <el-col :span="12">
        <el-form-item label="时长数值" prop="duration_value">
          <el-input-number
            v-model="newFormInline.duration_value"
            :min="1"
            :disabled="newFormInline.duration_unit === 'permanent'"
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
            <el-option v-if="allowPermanent" label="永久" value="permanent" />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>
  </el-form>
</template>
