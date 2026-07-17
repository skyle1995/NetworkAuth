<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    app_uuid: string;
    type: number;
    value: string;
    province: string;
    city: string;
    remark: string;
  };
  apps: any[];
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    app_uuid: "",
    type: 0,
    value: "",
    province: "",
    city: "",
    remark: ""
  }),
  apps: () => []
});

const model = ref(props.formInline);
</script>

<template>
  <el-form :model="model" label-width="90px" class="py-1">
    <el-form-item label="所属应用" required>
      <el-select
        v-model="model.app_uuid"
        placeholder="请选择应用"
        class="w-full"
      >
        <el-option
          v-for="app in apps"
          :key="app.uuid"
          :label="app.name"
          :value="app.uuid"
        />
      </el-select>
    </el-form-item>

    <el-form-item label="类型" required>
      <el-radio-group v-model="model.type">
        <el-radio :value="0">设备(机器码)</el-radio>
        <el-radio :value="1">IP</el-radio>
        <el-radio :value="2">地区</el-radio>
      </el-radio-group>
    </el-form-item>

    <template v-if="model.type === 2">
      <el-form-item label="省份" required>
        <el-input v-model="model.province" placeholder="如：广东省" />
      </el-form-item>
      <el-form-item label="城市" required>
        <el-input v-model="model.city" placeholder="如：佛山市（地级市）" />
      </el-form-item>
    </template>
    <template v-else>
      <el-form-item :label="model.type === 1 ? 'IP地址' : '机器码'" required>
        <el-input
          v-model="model.value"
          :placeholder="model.type === 1 ? '如：14.212.106.141' : '设备机器码'"
        />
      </el-form-item>
    </template>

    <el-form-item label="备注">
      <el-input v-model="model.remark" placeholder="可选" />
    </el-form-item>
  </el-form>
</template>
