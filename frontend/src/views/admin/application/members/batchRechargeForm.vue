<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    scope: "selected" | "all";
    app_uuid: string;
    duration_value: number;
    duration_unit: string;
    points: number;
    selectedNum: number;
  };
  apps: any[];
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    scope: "selected",
    app_uuid: "",
    duration_value: 1,
    duration_unit: "day",
    points: 0,
    selectedNum: 0
  }),
  apps: () => []
});

const model = ref(props.formInline);
</script>

<template>
  <el-form :model="model" label-width="90px" class="py-1">
    <el-alert
      :closable="false"
      type="info"
      show-icon
      class="mb-3"
      title="维护补偿等场景批量加时/点"
      description="时长模式账号加时长、点数模式账号加点数（按各自应用模式自动选择）。永久账号加时会自动跳过。"
    />

    <el-form-item label="操作范围">
      <el-radio-group v-model="model.scope">
        <el-radio value="selected">
          选中账号（{{ model.selectedNum }}）
        </el-radio>
        <el-radio value="all">全体账号</el-radio>
      </el-radio-group>
    </el-form-item>

    <el-form-item v-if="model.scope === 'all'" label="限定应用">
      <el-select
        v-model="model.app_uuid"
        placeholder="全部应用"
        clearable
        class="w-full"
      >
        <el-option label="全部应用" value="" />
        <el-option
          v-for="app in apps"
          :key="app.uuid"
          :label="app.name"
          :value="app.uuid"
        />
      </el-select>
    </el-form-item>

    <el-divider content-position="left">加时长（时长模式账号）</el-divider>
    <el-form-item label="时长">
      <el-input-number v-model="model.duration_value" :min="0" :max="100000" />
      <el-select v-model="model.duration_unit" class="ml-2 w-[100px]!">
        <el-option label="分钟" value="minute" />
        <el-option label="小时" value="hour" />
        <el-option label="天" value="day" />
        <el-option label="月" value="month" />
        <el-option label="年" value="year" />
      </el-select>
    </el-form-item>

    <el-divider content-position="left">加点数（点数模式账号）</el-divider>
    <el-form-item label="点数">
      <el-input-number v-model="model.points" :min="0" :max="10000000" />
    </el-form-item>

    <p class="text-xs" style="color: var(--el-text-color-secondary)">
      时长填 0 表示不加时；点数填 0 表示不加点。两者可只填其一。
    </p>
  </el-form>
</template>
