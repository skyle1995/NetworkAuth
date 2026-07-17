<script setup lang="ts">
import { ref, computed } from "vue";

export interface FormProps {
  formInline: {
    username: string;
    permanent: boolean;
    expired_at: string;
    points: number;
    total_recharge_yuan: number;
    remark: string;
  };
  pointsMode: boolean;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    username: "",
    permanent: false,
    expired_at: "",
    points: 0,
    total_recharge_yuan: 0,
    remark: ""
  }),
  pointsMode: false
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

const isPermanent = computed(() => newFormInline.value.permanent);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form ref="ruleFormRef" :model="newFormInline" label-width="100px">
    <el-form-item label="用户名">
      <el-input v-model="newFormInline.username" disabled />
    </el-form-item>

    <template v-if="pointsMode">
      <el-form-item label="点数余额" prop="points">
        <el-input-number v-model="newFormInline.points" :min="0" />
        <span class="ml-2">点</span>
      </el-form-item>
    </template>

    <template v-else>
      <el-form-item label="永久有效" prop="permanent">
        <el-switch v-model="newFormInline.permanent" />
      </el-form-item>
      <el-form-item v-if="!isPermanent" label="到期时间" prop="expired_at">
        <el-date-picker
          v-model="newFormInline.expired_at"
          type="datetime"
          value-format="YYYY-MM-DD HH:mm:ss"
          placeholder="选择到期时间"
          class="!w-full"
        />
      </el-form-item>
    </template>

    <el-form-item label="累计充值" prop="total_recharge_yuan">
      <el-input-number
        v-model="newFormInline.total_recharge_yuan"
        :min="0"
        :precision="2"
        :step="10"
      />
      <span class="ml-2">元</span>
    </el-form-item>

    <el-form-item label="备注" prop="remark">
      <el-input v-model="newFormInline.remark" type="textarea" :rows="2" />
    </el-form-item>
  </el-form>
</template>
