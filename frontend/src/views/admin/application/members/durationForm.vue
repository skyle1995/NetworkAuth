<script setup lang="ts">
import { ref, computed, watch } from "vue";

export interface FormProps {
  formInline: {
    duration_value: number;
    duration_unit: string;
    points: number;
  };
  /** 是否提供“永久”选项（充值可设永久，扣时不可） */
  allowPermanent: boolean;
  /** 点数模式：默认选中「点数」单位 */
  pointsMode: boolean;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    duration_value: 30,
    duration_unit: "day",
    points: 10
  }),
  allowPermanent: false,
  pointsMode: false
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

// 单位是「点数」时，数量写进 points；否则写进时长
const isPointsUnit = computed(
  () => newFormInline.value.duration_unit === "points"
);

// 数量是统一输入框，按单位落到不同字段，提交前保持两字段自洽
const amount = computed({
  get: () =>
    isPointsUnit.value
      ? newFormInline.value.points
      : newFormInline.value.duration_value,
  set: v => {
    if (isPointsUnit.value) {
      newFormInline.value.points = v;
    } else {
      newFormInline.value.duration_value = v;
    }
  }
});

// 切换单位时把另一个字段清零，避免「既加时长又加点数」
watch(
  () => newFormInline.value.duration_unit,
  () => {
    if (isPointsUnit.value) {
      newFormInline.value.duration_value = 0;
    } else {
      newFormInline.value.points = 0;
      // 时间单位只收整数：从「点数」切回来时把残留小数抹平
      if (newFormInline.value.duration_unit !== "permanent") {
        newFormInline.value.duration_value = Math.round(
          newFormInline.value.duration_value
        );
        if (newFormInline.value.duration_value < 1) {
          newFormInline.value.duration_value = 1;
        }
      }
    }
  }
);

const isPermanent = computed(
  () => newFormInline.value.duration_unit === "permanent"
);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef });
</script>

<template>
  <el-form ref="ruleFormRef" :model="newFormInline" label-width="72px">
    <el-form-item label="数量" prop="duration_value">
      <el-input-number
        v-model="amount"
        :min="isPointsUnit ? 0 : 1"
        :step="1"
        :precision="isPointsUnit ? 2 : 0"
        :disabled="isPermanent"
        class="!w-full"
      />
    </el-form-item>

    <el-form-item label="单位" prop="duration_unit">
      <el-select v-model="newFormInline.duration_unit" class="!w-full">
        <el-option label="分钟" value="minute" />
        <el-option label="小时" value="hour" />
        <el-option label="天" value="day" />
        <el-option label="月" value="month" />
        <el-option label="年" value="year" />
        <el-option label="点数" value="points" />
        <el-option v-if="allowPermanent" label="永久" value="permanent" />
      </el-select>
    </el-form-item>
  </el-form>
</template>
