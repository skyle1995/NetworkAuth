<script setup lang="ts">
import { ref } from "vue";

// 一号通：钱包全局（到期时间/点数/等级），但「怎么收费」由各应用自定。
// 计费字段已移出本表单，统一在「应用管理 → 计费策略」里配置；
// 新建应用时后端给默认值。三种计费模式参见 billing/index.vue。
//   2 高级模式：等级 >= required_level，并按「该等级在本应用配置的点数」扣点
// free_mode=1 为计费豁免：跳过全部门禁与扣费。
export interface FormProps {
  formInline: {
    id?: number;
    name: string;
    plugin_id?: string;
    version: string;
    status: number;
    download_type: number;
    download_url: string;
    update_announcement?: string;
    free_mode: number;
    billing_mode: number;
    required_level: number;
    billing_scope: number;
    points_charge_mode: number;
    points_per_login: number;
    points_period_minutes: number;
    points_per_period: number;
    points_heartbeat_charge: number;
    // 授权期限（分钟）：并发控制与崩溃回收的兜底
    lease_ttl_minutes: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    name: "",
    plugin_id: "",
    version: "1.0.0",
    status: 1,
    download_type: 0,
    download_url: "",
    update_announcement: "",
    free_mode: 0,
    billing_mode: 0,
    required_level: 1,
    billing_scope: 0,
    points_charge_mode: 0,
    points_per_login: 1,
    points_period_minutes: 60,
    points_per_period: 1,
    points_heartbeat_charge: 0,
    lease_ttl_minutes: 30
  })
});

const newFormInline = ref(props.formInline);

function getRef() {
  return formRef.value;
}

const formRef = ref();
defineExpose({ getRef, newFormInline });
</script>

<template>
  <el-form ref="formRef" :model="newFormInline" label-width="82px">
    <el-form-item
      label="应用名称"
      prop="name"
      :rules="[{ required: true, message: '请输入应用名称' }]"
    >
      <el-input
        v-model="newFormInline.name"
        clearable
        placeholder="请输入应用名称"
      />
    </el-form-item>
    <el-form-item label="插件标识" prop="plugin_id">
      <el-input
        v-model="newFormInline.plugin_id"
        clearable
        placeholder="客户端本地插件 ID，可留空"
      />
      <div class="w-full text-xs text-gray-400 mt-1">
        填客户端侧插件的实际 ID（如包名或 .zex 文件名），用于权益清单直接映射到插件；留空则按应用名匹配。
      </div>
    </el-form-item>    <el-form-item label="应用状态" prop="status">
      <el-radio-group v-model="newFormInline.status">
        <el-radio :value="1">启用</el-radio>
        <el-radio :value="0">停用</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="更新方式" prop="download_type">
      <el-radio-group v-model="newFormInline.download_type">
        <el-radio :value="0">不启用</el-radio>
        <el-radio :value="1">强制更新</el-radio>
        <el-radio :value="2">自由更新</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item
      v-if="newFormInline.download_type !== 0"
      label="下载地址"
      prop="download_url"
    >
      <el-input
        v-model="newFormInline.download_url"
        clearable
        placeholder="客户端更新下载地址"
      />
    </el-form-item>    <!-- 授权期限：并发控制与崩溃回收的兜底 -->
    <el-form-item label="授权期限" prop="lease_ttl_minutes">
      <el-input-number
        v-model="newFormInline.lease_ttl_minutes"
        :min="1"
        :max="1440"
      />
      <div class="w-full text-xs text-gray-400 mt-1">
        访问凭证的有效期（默认 30 分钟）。到期前自动续；客户端崩溃或断网时，靠它回收并发名额。
      </div>
    </el-form-item>
  </el-form>
</template>
