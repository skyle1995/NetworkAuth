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
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    username: "",
    permanent: false,
    expired_at: "",
    points: 0,
    total_recharge_yuan: 0,
    remark: ""
  })
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

const isPermanent = computed(() => newFormInline.value.permanent);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef });
</script>

<template>
  <el-form ref="ruleFormRef" :model="newFormInline" label-width="100px">
    <el-form-item label="用户名">
      <el-input v-model="newFormInline.username" disabled />
    </el-form-item>

    <!-- 一号通账号同时拥有「时长」与「点数」两个钱包，两者都能改 -->
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

    <el-form-item label="点数余额" prop="points">
      <el-input-number
        v-model="newFormInline.points"
        :step="0.1"
        :precision="3"
        :min="0"
        class="!w-[180px]"
      />
      <span class="ml-2">点</span>
    </el-form-item>

    <el-form-item label="累计充值" prop="total_recharge_yuan">
      <el-input-number
        v-model="newFormInline.total_recharge_yuan"
        :min="0"
        :precision="2"
        :step="10"
        class="!w-[180px]"
      />
      <span class="ml-2">元</span>
      <div class="w-full text-xs text-gray-400 mt-1">
        改动后按新值重新校准会员等级
      </div>
    </el-form-item>

    <el-form-item label="备注" prop="remark">
      <el-input v-model="newFormInline.remark" type="textarea" :rows="2" />
    </el-form-item>
  </el-form>
</template>
