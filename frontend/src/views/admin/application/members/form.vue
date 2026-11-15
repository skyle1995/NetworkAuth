<script setup lang="ts">
import { ref, computed } from "vue";
import { formRules } from "./rule";

export interface FormProps {
  formInline: {
    username: string;
    password: string;
    duration_value: number;
    duration_unit: string;
    points: number;
    remark: string;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    username: "",
    password: "",
    duration_value: 30,
    duration_unit: "day",
      points: 0,
    remark: ""
  })
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

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
    label-width="90px"
  >

    <el-form-item label="用户名" prop="username">
      <el-input v-model="newFormInline.username" placeholder="请输入用户名" />
    </el-form-item>

    <el-form-item label="密码" prop="password">
      <el-input
        v-model="newFormInline.password"
        type="password"
        show-password
        placeholder="请输入初始密码"
      />
    </el-form-item>
    <!-- 一号通账号同时有两个钱包：初始时长与初始点数都可填（0 = 不发放） -->
    <el-form-item label="初始时长" prop="duration_value">
      <el-input-number
        v-model="newFormInline.duration_value"
        :min="1"
        :disabled="isPermanent"
        class="!w-[140px]"
      />
      <el-select
        v-model="newFormInline.duration_unit"
        class="ml-2 !w-[110px]"
      >
        <el-option label="分钟" value="minute" />
        <el-option label="小时" value="hour" />
        <el-option label="天" value="day" />
        <el-option label="月" value="month" />
        <el-option label="年" value="year" />
        <el-option label="永久" value="permanent" />
      </el-select>
    </el-form-item>

    <el-form-item label="初始点数" prop="points">
      <el-input-number
        v-model="newFormInline.points"
        :min="0"
        :step="1"
        :precision="2"
        class="!w-[140px]"
      />
    </el-form-item>

    <el-form-item label="备注说明" prop="remark">
      <el-input
        v-model="newFormInline.remark"
        type="textarea"
        :rows="3"
        placeholder="请输入备注说明（可选）"
      />
    </el-form-item>
  </el-form>
</template>
