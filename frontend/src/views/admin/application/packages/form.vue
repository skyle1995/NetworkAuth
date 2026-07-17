<script setup lang="ts">
import { ref, computed } from "vue";
import type { FormRules } from "element-plus";

export interface FormProps {
  formInline: {
    uuid: string;
    app_uuid: string;
    name: string;
    type: number;
    duration: number;
    points: number;
    price_yuan: number;
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
    type: 0,
    duration: 43200,
    points: 100,
    price_yuan: 10,
    sort: 0,
    status: 1,
    remark: ""
  }),
  apps: () => []
});

const rules: FormRules = {
  app_uuid: [{ required: true, message: "请选择所属应用", trigger: "change" }],
  name: [{ required: true, message: "请输入套餐名称", trigger: "blur" }]
};

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

const isPoints = computed(() => newFormInline.value.type === 1);
const isPermanent = computed(() => newFormInline.value.duration === -1);

function togglePermanent(val: boolean) {
  newFormInline.value.duration = val ? -1 : 43200;
}

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

    <el-form-item label="套餐名称" prop="name">
      <el-input v-model="newFormInline.name" placeholder="如 月卡 / 1000点" />
    </el-form-item>

    <el-form-item label="套餐类型" prop="type">
      <el-radio-group v-model="newFormInline.type">
        <el-radio :value="0">时长</el-radio>
        <el-radio :value="1">点数</el-radio>
      </el-radio-group>
    </el-form-item>

    <el-form-item v-if="isPoints" label="面值点数" prop="points">
      <el-input-number v-model="newFormInline.points" :min="1" />
      <span class="ml-2">点</span>
    </el-form-item>

    <template v-else>
      <el-form-item label="永久" prop="duration">
        <el-switch
          :model-value="isPermanent"
          @update:model-value="togglePermanent"
        />
      </el-form-item>
      <el-form-item v-if="!isPermanent" label="面值时长" prop="duration">
        <el-input-number v-model="newFormInline.duration" :min="1" />
        <span class="ml-2">分钟</span>
      </el-form-item>
    </template>

    <el-form-item label="售价" prop="price_yuan">
      <el-input-number
        v-model="newFormInline.price_yuan"
        :min="0"
        :precision="2"
        :step="1"
      />
      <span class="ml-2">元</span>
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
