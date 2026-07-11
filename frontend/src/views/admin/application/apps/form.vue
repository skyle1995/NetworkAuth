<script setup lang="ts">
import { ref, computed } from "vue";

export interface FormProps {
  formInline: {
    id?: number;
    name: string;
    version: string;
    status: number;
    download_type: number;
    download_url: string;
    operation_mode: number;
    points_charge_mode: number;
    points_per_login: number;
    points_period_minutes: number;
    points_per_period: number;
    points_heartbeat_charge: number;
    card_login_enabled: number;
    recharge_enabled: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    name: "",
    version: "1.0.0",
    status: 1,
    download_type: 0,
    download_url: "",
    operation_mode: 0,
    points_charge_mode: 0,
    points_per_login: 1,
    points_period_minutes: 60,
    points_per_period: 1,
    points_heartbeat_charge: 0,
    card_login_enabled: 1,
    recharge_enabled: 1
  })
});

const newFormInline = ref(props.formInline);

const isPoints = computed(() => newFormInline.value.operation_mode === 1);
const isFree = computed(() => newFormInline.value.operation_mode === 2);
const isPerTime = computed(() => newFormInline.value.points_charge_mode === 1);

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
    <el-form-item label="应用版本" prop="version">
      <el-input
        v-model="newFormInline.version"
        clearable
        placeholder="请输入应用版本，默认1.0.0"
      />
    </el-form-item>
    <el-form-item label="应用状态" prop="status">
      <el-switch
        v-model="newFormInline.status"
        :active-value="1"
        :inactive-value="0"
        active-text="启用"
        inactive-text="禁用"
        inline-prompt
      />
    </el-form-item>
    <el-form-item label="更新方式" prop="download_type">
      <el-radio-group v-model="newFormInline.download_type">
        <el-radio :value="0">不启用</el-radio>
        <el-radio :value="1">强制更新</el-radio>
        <el-radio :value="2">自由更新</el-radio>
      </el-radio-group>
      <div class="text-xs text-gray-400 mt-1">
        强制更新：有新版本必须更新才能继续；自由更新：提示新版本，用户可自行选择。
      </div>
    </el-form-item>
    <el-form-item
      v-if="newFormInline.download_type !== 0"
      label="下载地址"
      prop="download_url"
    >
      <el-input
        v-model="newFormInline.download_url"
        clearable
        placeholder="请输入下载/更新地址"
      />
    </el-form-item>

    <el-divider content-position="left">运营模式</el-divider>
    <el-form-item label="运营模式" prop="operation_mode">
      <el-radio-group v-model="newFormInline.operation_mode">
        <el-radio :value="2">免费模式</el-radio>
        <el-radio :value="0">时长模式</el-radio>
        <el-radio :value="1">点数模式</el-radio>
      </el-radio-group>
      <div v-if="isFree" class="text-xs text-gray-400 mt-1">
        免费模式：不计费，账号即便已到期/无点数也可正常登录使用；心跳照常校验但不扣费；转绑不扣费。
        （已充值的时长/点数仍保留，只是不再消耗）
      </div>
    </el-form-item>
    <template v-if="isPoints">
      <el-form-item label="扣费方式" prop="points_charge_mode">
        <el-radio-group v-model="newFormInline.points_charge_mode">
          <el-radio :value="0">按次(登录扣点)</el-radio>
          <el-radio :value="1">按时(预扣费)</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="!isPerTime" label="登录扣点" prop="points_per_login">
        <el-input-number v-model="newFormInline.points_per_login" :min="0" />
        <span class="ml-2 text-xs text-gray-400">每次登录扣点，0=不扣</span>
      </el-form-item>
      <el-form-item
        v-if="isPerTime"
        label="计费周期"
        prop="points_period_minutes"
      >
        <el-input-number
          v-model="newFormInline.points_period_minutes"
          :min="1"
        />
        <span class="ml-2">分钟</span>
      </el-form-item>
      <el-form-item v-if="isPerTime" label="周期扣点" prop="points_per_period">
        <el-input-number v-model="newFormInline.points_per_period" :min="1" />
        <span class="ml-2">点</span>
      </el-form-item>
      <el-form-item
        v-if="isPerTime"
        label="扣费触发"
        prop="points_heartbeat_charge"
      >
        <el-radio-group v-model="newFormInline.points_heartbeat_charge">
          <el-radio :value="0">登录预扣</el-radio>
          <el-radio :value="1">心跳触发</el-radio>
        </el-radio-group>
        <div class="text-xs text-gray-400 mt-1">
          登录预扣：登录即扣一个周期。心跳触发：登录不扣，首个周期延到首次心跳时扣。
          两种方式心跳均<strong>默认按周期扣费</strong>，客户端可对免费功能传
          no_charge=true 跳过本次扣费。
        </div>
      </el-form-item>
    </template>

    <el-divider content-position="left">登录方式开关</el-divider>
    <el-form-item label="卡密登录" prop="card_login_enabled">
      <el-switch
        v-model="newFormInline.card_login_enabled"
        :active-value="1"
        :inactive-value="0"
        active-text="开启"
        inactive-text="关闭"
        inline-prompt
      />
    </el-form-item>
    <el-form-item label="卡密充值" prop="recharge_enabled">
      <el-switch
        v-model="newFormInline.recharge_enabled"
        :active-value="1"
        :inactive-value="0"
        active-text="开启"
        inactive-text="关闭"
        inline-prompt
      />
    </el-form-item>
  </el-form>
</template>
