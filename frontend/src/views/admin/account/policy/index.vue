<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import {
  getAppRegisterConfig,
  updateAppRegisterConfig
} from "@/api/admin/app";

defineOptions({ name: "AccountPolicy" });

interface PolicyForm {
  // 卡密
  card_login_enabled: number;
  recharge_enabled: number;
  // 注册
  register_enabled: number;
  email_verify_enabled: number;
  card_register_enabled: number;
  register_limit_enabled: number;
  register_device_limit_enabled: number;
  register_limit_time: number;
  register_count: number;
  // 试用
  trial_enabled: number;
  trial_limit_time: number;
  trial_points: number;
  trial_duration_minutes: number;
  trial_claim_mode: number;
  // 绑定与转绑
  machine_verify: number;
  ip_verify: number;
  machine_rebind_enabled: number;
  machine_rebind_free_count: number;
  machine_rebind_limit_time: number;
  machine_rebind_max_count: number;
  machine_rebind_deduct_minutes: number;
  machine_rebind_deduct_points: number;
  ip_rebind_enabled: number;
  ip_rebind_free_count: number;
  ip_rebind_limit_time: number;
  ip_rebind_max_count: number;
  ip_rebind_deduct_minutes: number;
  ip_rebind_deduct_points: number;
  // 资源配额
  max_devices: number;
  max_concurrent: number;
}

const defaults = (): PolicyForm => ({
  card_login_enabled: 1,
  recharge_enabled: 1,
  register_enabled: 1,
  email_verify_enabled: 0,
  card_register_enabled: 0,
  register_limit_enabled: 0,
  register_device_limit_enabled: 0,
  register_limit_time: 1,
  register_count: 3,
  trial_enabled: 0,
  trial_limit_time: 1,
  trial_points: 0,
  trial_duration_minutes: 0,
  trial_claim_mode: 1,
  machine_verify: 0,
  ip_verify: 0,
  machine_rebind_enabled: 1,
  machine_rebind_free_count: 1,
  machine_rebind_limit_time: 1,
  machine_rebind_max_count: 0,
  machine_rebind_deduct_minutes: 0,
  machine_rebind_deduct_points: 0,
  ip_rebind_enabled: 1,
  ip_rebind_free_count: 2,
  ip_rebind_limit_time: 1,
  ip_rebind_max_count: 0,
  ip_rebind_deduct_minutes: 0,
  ip_rebind_deduct_points: 0,
  max_devices: 3,
  max_concurrent: 1
});

const form = ref<PolicyForm>(defaults());
const loading = ref(false);
const saving = ref(false);
const activeTab = ref("card");

// 转绑开关在表单里用 switch，提交时转成 0/1
const machineRebindOn = computed({
  get: () => form.value.machine_rebind_enabled === 1,
  set: v => (form.value.machine_rebind_enabled = v ? 1 : 0)
});
const ipRebindOn = computed({
  get: () => form.value.ip_rebind_enabled === 1,
  set: v => (form.value.ip_rebind_enabled = v ? 1 : 0)
});

async function loadPolicy() {
  loading.value = true;
  try {
    const res = await getAppRegisterConfig({});
    if (res.code === 0 && res.data) {
      form.value = { ...defaults(), ...res.data };
    }
  } catch (e) {
    ElMessage.error("加载账号策略失败");
  } finally {
    loading.value = false;
  }
}

// 每个 Tab 各有一个保存按钮；后端是整份策略覆盖写，
// 所以始终提交完整表单，避免"只存了当前页、其他页改动丢失"。
async function savePolicy(label: string) {
  saving.value = true;
  try {
    const res = await updateAppRegisterConfig({ ...form.value } as any);
    if (res.code === 0) {
      ElMessage.success(`${label}已保存，全站立即生效`);
      await loadPolicy();
    } else {
      ElMessage.error(res.msg || "保存失败");
    }
  } catch (e) {
    ElMessage.error("保存失败");
  } finally {
    saving.value = false;
  }
}

onMounted(loadPolicy);
</script>

<template>
  <div class="policy-page">
    <el-alert
      type="info"
      :closable="false"
      class="mb-3"
      title="以下策略全站生效，不区分应用"
      description="注册、卡密登录、充值、试用、设备绑定产生或消耗的都是全局账号资源，故统一在此配置。各应用自己怎么计费，请到「应用管理 → 应用程序」里设置。"
    />

    <el-card v-loading="loading" shadow="never">
      <el-tabs v-model="activeTab" class="mt-2">
        <!-- ① 卡密与充值 -->
        <el-tab-pane label="卡密与充值" name="card">
          <el-form :model="form" label-width="140px" class="mt-3">
            <el-form-item label="卡密登录">
              <el-switch
                v-model="form.card_login_enabled"
                :active-value="1"
                :inactive-value="0"
              />
              <span class="ml-3 text-xs text-gray-500">
                开启后卡密可直接登录并自动创建绑定该卡的账号（用户名=卡号）
              </span>
            </el-form-item>

            <el-form-item label="卡密充值">
              <el-switch
                v-model="form.recharge_enabled"
                :active-value="1"
                :inactive-value="0"
              />
              <span class="ml-3 text-xs text-gray-500">
                开启后卡密可为已有账号充值时长 / 点数
              </span>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="savePolicy('卡密与充值')"
              >
                保存卡密与充值
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ② 账号注册 -->
        <el-tab-pane label="账号注册" name="register">
          <el-form :model="form" label-width="140px" class="mt-3">
            <el-form-item label="开放注册">
              <el-switch
                v-model="form.register_enabled"
                :active-value="1"
                :inactive-value="0"
              />
            </el-form-item>

            <el-form-item label="邮箱验证">
              <el-switch
                v-model="form.email_verify_enabled"
                :active-value="1"
                :inactive-value="0"
              />
              <span class="ml-3 text-xs text-gray-500">
                依赖「系统设置」里的 SMTP 配置
              </span>
            </el-form-item>

            <el-form-item label="卡密注册">
              <el-switch
                v-model="form.card_register_enabled"
                :active-value="1"
                :inactive-value="0"
              />
              <span class="ml-3 text-xs text-gray-500">
                开启后注册须额外提交有效卡密，注册即核销
              </span>
            </el-form-item>

            <el-divider content-position="left">注册限制</el-divider>

            <el-form-item label="IP 限制">
              <el-switch
                v-model="form.register_limit_enabled"
                :active-value="1"
                :inactive-value="0"
              />
            </el-form-item>

            <el-form-item label="设备限制">
              <el-switch
                v-model="form.register_device_limit_enabled"
                :active-value="1"
                :inactive-value="0"
              />
              <span class="ml-3 text-xs text-gray-500">开启后注册须提交设备码</span>
            </el-form-item>

            <el-form-item label="限制窗口">
              <el-radio-group v-model="form.register_limit_time">
                <el-radio :value="0">每天</el-radio>
                <el-radio :value="1">永久</el-radio>
              </el-radio-group>
              <span class="ml-3 text-xs text-gray-500">IP 与设备维度共用</span>
            </el-form-item>

            <el-form-item label="注册次数">
              <el-input-number v-model="form.register_count" :min="1" />
              <span class="ml-3 text-xs text-gray-500">
                <b>全站计数</b>：同一 IP / 设备全站最多注册该数量，<b>填 1 会导致一个号之后就再也注册不了</b>
              </span>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="savePolicy('账号注册')"
              >
                保存账号注册
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ③ 领取试用 -->
        <el-tab-pane label="领取试用" name="trial">
          <el-form :model="form" label-width="140px" class="mt-3">
            <el-form-item label="开放试用">
              <el-switch
                v-model="form.trial_enabled"
                :active-value="1"
                :inactive-value="0"
              />
            </el-form-item>

            <el-form-item label="限制窗口">
              <el-radio-group v-model="form.trial_limit_time">
                <el-radio :value="0">每天</el-radio>
                <el-radio :value="1">永久</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="领取方案">
              <el-radio-group v-model="form.trial_claim_mode">
                <el-radio :value="0">无限制</el-radio>
                <el-radio :value="1">资源耗尽可领</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="试用点数">
              <el-input-number
                v-model="form.trial_points"
                :step="0.1"
                :precision="3"
                :min="0"
              />
              <span class="ml-3 text-xs text-gray-500">发放到点数钱包；0=不发放</span>
            </el-form-item>

            <el-form-item label="试用时长">
              <el-input-number v-model="form.trial_duration_minutes" :min="0" />
              <span class="ml-3 text-xs text-gray-500">
                分钟，发放到到期时间；0=不发放。两个额度可同时配置
              </span>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="savePolicy('领取试用')"
              >
                保存领取试用
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ④ 绑定与转绑 -->
        <el-tab-pane label="绑定与转绑" name="binding">
          <el-form :model="form" label-width="140px" class="mt-3">
            <el-divider content-position="left">校验</el-divider>

            <el-form-item label="设备校验">
              <el-radio-group v-model="form.machine_verify">
                <el-radio :value="0">不校验</el-radio>
                <el-radio :value="1">校验设备码</el-radio>
              </el-radio-group>
              <div class="w-full text-xs text-gray-400 mt-1">
                开启后客户端激活必须带设备码，账号的「可绑定设备」白名单才生效
              </div>
            </el-form-item>

            <el-form-item label="IP 校验">
              <el-select v-model="form.ip_verify" class="!w-[200px]">
                <el-option label="关闭" :value="0" />
                <el-option label="精确 IP" :value="1" />
                <el-option label="同市" :value="2" />
                <el-option label="同省" :value="3" />
              </el-select>
              <div class="w-full text-xs text-gray-400 mt-1">
                换网络后是否需要转绑才能继续用；「同市/同省」按归属地判定，限制更宽松
              </div>
            </el-form-item>

            <el-divider content-position="left">设备转绑</el-divider>

            <el-form-item label="允许转绑">
              <el-switch v-model="machineRebindOn" />
              <span class="ml-3 text-xs text-gray-500">
                关闭后用户换电脑只能找管理员处理
              </span>
            </el-form-item>

            <el-form-item label="免费次数">
              <el-input-number
                v-model="form.machine_rebind_free_count"
                :min="0"
                :max="100"
              />
              <span class="ml-3 text-xs text-gray-500">
                本窗口内不扣费；会员等级可额外赠送
              </span>
            </el-form-item>

            <el-form-item label="计数窗口">
              <el-radio-group v-model="form.machine_rebind_limit_time">
                <el-radio :value="0">每天重置</el-radio>
                <el-radio :value="1">永久累计</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="次数上限">
              <el-input-number
                v-model="form.machine_rebind_max_count"
                :min="0"
                :max="1000"
              />
              <span class="ml-3 text-xs text-gray-500">
                本窗口最多几次，0=不限；超出后连付费也不允许
              </span>
            </el-form-item>

            <el-form-item label="超额扣除">
              <el-input-number
                v-model="form.machine_rebind_deduct_minutes"
                :min="0"
                :max="100000"
                class="!w-[140px]"
              />
              <span class="ml-2 text-xs text-gray-500">分钟 · 计时模式</span>
              <div class="w-full h-2" />
              <el-input-number
                v-model="form.machine_rebind_deduct_points"
                :min="0"
                :max="100000"
                class="!w-[140px]"
              />
              <span class="ml-2 text-xs text-gray-500">点 · 点数模式</span>
              <div class="w-full text-xs text-gray-400 mt-1">
                超出免费次数后每次扣多少，两套钱包各配各的；0=只计次不扣
              </div>
            </el-form-item>

            <el-divider content-position="left">IP 转绑</el-divider>

            <el-form-item label="允许转绑">
              <el-switch v-model="ipRebindOn" />
              <span class="ml-3 text-xs text-gray-500">
                关闭后用户换网络只能找管理员处理
              </span>
            </el-form-item>

            <el-form-item label="免费次数">
              <el-input-number
                v-model="form.ip_rebind_free_count"
                :min="0"
                :max="100"
              />
              <span class="ml-3 text-xs text-gray-500">
                换网络比换电脑更频繁，建议比设备宽松
              </span>
            </el-form-item>

            <el-form-item label="计数窗口">
              <el-radio-group v-model="form.ip_rebind_limit_time">
                <el-radio :value="0">每天重置</el-radio>
                <el-radio :value="1">永久累计</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="次数上限">
              <el-input-number
                v-model="form.ip_rebind_max_count"
                :min="0"
                :max="1000"
              />
              <span class="ml-3 text-xs text-gray-500">本窗口最多几次，0=不限</span>
            </el-form-item>

            <el-form-item label="超额扣除">
              <el-input-number
                v-model="form.ip_rebind_deduct_minutes"
                :min="0"
                :max="100000"
                class="!w-[140px]"
              />
              <span class="ml-2 text-xs text-gray-500">分钟 · 计时模式</span>
              <div class="w-full h-2" />
              <el-input-number
                v-model="form.ip_rebind_deduct_points"
                :min="0"
                :max="100000"
                class="!w-[140px]"
              />
              <span class="ml-2 text-xs text-gray-500">点 · 点数模式</span>
              <div class="w-full text-xs text-gray-400 mt-1">
                超出免费次数后每次扣多少，两套钱包各配各的；0=只计次不扣
              </div>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="savePolicy('绑定与转绑')"
              >
                保存绑定与转绑
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ⑤ 资源配额 -->
        <el-tab-pane label="资源配额" name="quota">
          <el-form :model="form" label-width="140px" class="mt-3">
            <el-form-item label="可绑定设备">
              <el-input-number
                v-model="form.max_devices"
                :min="1"
                :max="100"
              />
              <span class="ml-3 text-xs text-gray-500">
                一个账号最多绑定几台电脑。开启「设备校验」时只认这里的设备
              </span>
            </el-form-item>

            <el-form-item label="同时在线">
              <el-input-number
                v-model="form.max_concurrent"
                :min="1"
                :max="100"
              />
              <span class="ml-3 text-xs text-gray-500">
                一个账号最多同时开着几个实例（跨所有应用合计）。超限时按各应用的「满员策略」处理
              </span>
            </el-form-item>

            <el-form-item>
              <div class="w-full text-xs text-gray-400">
                会员等级可在此基础上加量：额外设备、额外并发、赠送换绑次数
              </div>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="savePolicy('资源配额')"
              >
                保存资源配额
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<style scoped>
.policy-page {
  padding: 4px;
}
</style>
