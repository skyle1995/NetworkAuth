<script setup lang="ts">
import { ref } from "vue";

// 全局账号策略（一号通）：注册 / 卡密登录 / 充值 / 试用。
// 这些策略产生或消耗的是**全局资源**（账号、钱包、卡密、试用额度），
// 因此不按应用配置，改由 settings 统一管理。
export interface FormProps {
  formInline: {
    register_enabled: number;
    email_verify_enabled: number;
    card_register_enabled: number;
    register_limit_enabled: number;
    register_device_limit_enabled: number;
    register_limit_time: number;
    register_count: number;
    card_login_enabled: number;
    recharge_enabled: number;
    trial_enabled: number;
    trial_limit_time: number;
    trial_points: number;
    trial_duration_minutes: number;
    trial_claim_mode: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    register_enabled: 1,
    email_verify_enabled: 0,
    card_register_enabled: 0,
    register_limit_enabled: 0,
    register_device_limit_enabled: 0,
    register_limit_time: 1,
    register_count: 3,
    card_login_enabled: 1,
    recharge_enabled: 1,
    trial_enabled: 0,
    trial_limit_time: 1,
    trial_points: 0,
    trial_duration_minutes: 0,
    trial_claim_mode: 1
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
  <el-form ref="formRef" :model="newFormInline" label-width="120px">
    <el-alert
      type="warning"
      :closable="false"
      style="margin-bottom: 16px"
      title="以下策略全站生效，不区分应用"
      description="注册、卡密登录、充值、试用产生或消耗的是全局账号与钱包资源，故统一在此配置。"
    />

    <el-divider content-position="left">卡密登录与充值</el-divider>
    <el-form-item label="卡密登录" prop="card_login_enabled">
      <el-radio-group v-model="newFormInline.card_login_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">
        开启后卡密可直接登录并自动创建绑定账号
      </span>
    </el-form-item>
    <el-form-item label="卡密充值" prop="recharge_enabled">
      <el-radio-group v-model="newFormInline.recharge_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">
        开启后卡密可为账号充值时长 / 点数
      </span>
    </el-form-item>

    <el-divider content-position="left">账号注册</el-divider>
    <el-form-item label="账号注册" prop="register_enabled">
      <el-radio-group v-model="newFormInline.register_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="邮箱验证" prop="email_verify_enabled">
      <el-radio-group v-model="newFormInline.email_verify_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">依赖系统 SMTP 配置</span>
    </el-form-item>
    <el-form-item label="卡密注册" prop="card_register_enabled">
      <el-radio-group v-model="newFormInline.card_register_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">
        开启后注册须额外提交有效卡密，注册即核销
      </span>
    </el-form-item>
    <el-form-item label="IP 限制" prop="register_limit_enabled">
      <el-radio-group v-model="newFormInline.register_limit_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="设备限制" prop="register_device_limit_enabled">
      <el-radio-group v-model="newFormInline.register_device_limit_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">开启后注册须提交设备码</span>
    </el-form-item>
    <el-form-item label="限制窗口" prop="register_limit_time">
      <el-radio-group v-model="newFormInline.register_limit_time">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
      <span class="ml-2 text-xs text-gray-400">IP 与设备维度共用</span>
    </el-form-item>
    <el-form-item label="注册次数" prop="register_count">
      <el-input-number v-model="newFormInline.register_count" :min="1" />
      <span class="ml-2 text-xs text-gray-400">
        <b>全站计数</b>：同一 IP / 设备全站最多注册该数量，建议大于 1
      </span>
    </el-form-item>

    <el-divider content-position="left">领取试用</el-divider>
    <el-form-item label="领取试用" prop="trial_enabled">
      <el-radio-group v-model="newFormInline.trial_enabled">
        <el-radio :value="0">关闭</el-radio>
        <el-radio :value="1">开启</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="限制窗口" prop="trial_limit_time">
      <el-radio-group v-model="newFormInline.trial_limit_time">
        <el-radio :value="0">每天</el-radio>
        <el-radio :value="1">永久</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="领取方案" prop="trial_claim_mode">
      <el-radio-group v-model="newFormInline.trial_claim_mode">
        <el-radio :value="0">无限制</el-radio>
        <el-radio :value="1">资源耗尽可领</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="试用点数" prop="trial_points">
      <el-input-number v-model="newFormInline.trial_points" :min="0" />
      <span class="ml-2 text-xs text-gray-400">
        发放到账号点数钱包，0=不发放点数
      </span>
    </el-form-item>
    <el-form-item label="试用时长" prop="trial_duration_minutes">
      <el-input-number
        v-model="newFormInline.trial_duration_minutes"
        :min="0"
      />
      <span class="ml-2 text-xs text-gray-400">
        分钟，发放到账号到期时间；0=不发放时长。两项可同时配置
      </span>
    </el-form-item>
  </el-form>
</template>
