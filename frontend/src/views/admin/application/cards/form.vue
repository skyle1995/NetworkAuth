<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { formRules } from "./rule";
import { getCardPackages } from "@/api/admin/cardPackage";

export interface FormProps {
  formInline: {
    prefix: string;
    length: number;
    count: number;
    package_uuid: string;
    remark: string;
}
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    prefix: "",
    length: 16,
    count: 10,
    package_uuid: "",
    remark: ""
  })
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);
const packages = ref<any[]>([]);

// 面值与售价由套餐决定，制卡时快照进卡。
async function loadPackages() {
  try {
    const { code, data } = await getCardPackages({ enabled: 1 });
    if (code === 0 && Array.isArray(data)) {
      packages.value = data;
    }
  } catch (e) {
    console.error(e);
  }
}

// 进页面就加载全部启用套餐（不再随渠道变化）
onMounted(loadPackages);

function packageLabel(pkg: any) {
  const value =
    pkg.type === 1
      ? `${pkg.points} 点`
      : pkg.duration === -1
        ? "永久"
        : `${pkg.duration} 分钟`;
  return `${pkg.name}（${value} / ${(pkg.price / 100).toFixed(2)} 元）`;
}

const noPackage = computed(() => packages.value.length === 0);

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

    <el-form-item label="卡密套餐" prop="package_uuid">
      <el-select
        v-model="newFormInline.package_uuid"
        :placeholder="noPackage ? '暂无可用套餐，请先到「卡密套餐」创建' : '请选择卡密套餐'"
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
