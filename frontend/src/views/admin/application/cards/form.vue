<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { formRules } from "./rule";
import { getCardPackages } from "@/api/admin/cardPackage";

export interface FormProps {
  formInline: {
    app_uuid: string;
    prefix: string;
    length: number;
    count: number;
    package_uuid: string;
    remark: string;
  };
  apps: Array<{
    id: number;
    uuid: string;
    name: string;
    operation_mode?: number;
  }>;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    app_uuid: "",
    prefix: "",
    length: 16,
    count: 10,
    package_uuid: "",
    remark: ""
  }),
  apps: () => []
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);
const packages = ref<any[]>([]);

// 面值与售价由套餐决定，制卡时快照进卡
async function loadPackages(appUUID: string) {
  packages.value = [];
  if (!appUUID) return;
  try {
    const { code, data } = await getCardPackages({
      app_uuid: appUUID,
      enabled: 1
    });
    if (code === 0 && Array.isArray(data)) {
      packages.value = data;
    }
  } catch (e) {
    console.error(e);
  }
}

watch(
  () => newFormInline.value.app_uuid,
  (uuid, old) => {
    if (old !== undefined) newFormInline.value.package_uuid = "";
    loadPackages(uuid);
  },
  { immediate: true }
);

function packageLabel(pkg: any) {
  const value =
    pkg.type === 1
      ? `${pkg.points} 点`
      : pkg.duration === -1
        ? "永久"
        : `${pkg.duration} 分钟`;
  return `${pkg.name}（${value} / ${(pkg.price / 100).toFixed(2)} 元）`;
}

const noPackage = computed(
  () => !!newFormInline.value.app_uuid && packages.value.length === 0
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

    <el-form-item label="卡密套餐" prop="package_uuid">
      <el-select
        v-model="newFormInline.package_uuid"
        :placeholder="noPackage ? '该应用暂无可用套餐' : '请选择卡密套餐'"
        :disabled="noPackage"
        class="w-full"
        clearable
      >
        <el-option
          v-for="pkg in packages"
          :key="pkg.uuid"
          :label="packageLabel(pkg)"
          :value="pkg.uuid"
        />
      </el-select>
    </el-form-item>

    <div class="flex gap-4">
      <el-form-item label="生成数量" prop="count" class="flex-1 !mr-0">
        <el-input-number
          v-model="newFormInline.count"
          :min="1"
          :max="10000"
          class="!w-full"
        />
      </el-form-item>
      <el-form-item label="卡号长度" prop="length" class="flex-1 !mr-0">
        <el-input-number
          v-model="newFormInline.length"
          :min="8"
          :max="32"
          class="!w-full"
        />
      </el-form-item>
    </div>

    <el-form-item label="卡号前缀" prop="prefix">
      <el-input
        v-model="newFormInline.prefix"
        placeholder="可选，如 VIP（仅字母数字，用于区分注册用户名）"
        maxlength="16"
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
