<script setup lang="ts">
import { ref, computed } from "vue";

// 一号通：钱包全局（到期时间/点数/等级），但「怎么收费」由各应用自定。
// 三种计费模式：
//   0 计时模式：账号有效（未到期/永久）+ 会员等级 >= required_level，不扣点（卖时长）
//   1 点数模式：消耗账号点数
//   2 高级模式：等级 >= required_level，并按「该等级在本应用配置的点数」扣点
// free_mode=1 为计费豁免：跳过全部门禁与扣费。
export interface FormProps {
  formInline: {
    id?: number;
    name: string;
    version: string;
    status: number;
    download_type: number;
    download_url: string;
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
    version: "1.0.0",
    status: 1,
    download_type: 0,
    download_url: "",
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

const isFree = computed(() => newFormInline.value.free_mode === 1);
// 需要等级门槛：计时模式与高级模式（点数模式不要求等级）
const needsLevel = computed(
  () => !isFree.value && newFormInline.value.billing_mode !== 1
);
// 消耗点数：点数模式与高级模式
const usesPoints = computed(
  () => !isFree.value && newFormInline.value.billing_mode !== 0
);
const isHybrid = computed(
  () => !isFree.value && newFormInline.value.billing_mode === 2
);
const isPerTime = computed(() => newFormInline.value.points_charge_mode === 1);

// 当前计费模式的说明：只显示被选中那一条，避免三种模式混在一起读
const modeHint = computed(() => {
  if (isFree.value) {
    return "已开启计费豁免：不看等级、不看到期、不扣点，用户登录即可用。";
  }
  switch (newFormInline.value.billing_mode) {
    case 0:
      return "计时模式：卖时长。账号到期时间决定能否使用，全程不扣点数；可额外要求会员等级达标。";
    case 1:
      return "点数模式：卖点数。每次进入或每个周期扣点，不要求会员等级；余额不足即拒绝。";
    case 2:
      return "高级模式：等级达标 + 按等级扣点，等级越高扣得越少。单价在「更多 → 等级点数」逐级配置，未配置的等级按下方兜底值。";
    default:
      return "";
  }
});
const modeHintClass = computed(() =>
  isFree.value ? "text-orange-500" : "text-gray-400"
);

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
        placeholder="如 1.0.0"
      />
    </el-form-item>
    <el-form-item label="应用状态" prop="status">
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
    </el-form-item>

    <el-divider content-position="left">计费策略</el-divider>
    <el-form-item label="计费豁免" prop="free_mode">
      <el-switch
        v-model="newFormInline.free_mode"
        :active-value="1"
        :inactive-value="0"
        active-text="免费"
        inactive-text="计费"
        inline-prompt
      />
      <div class="w-full text-xs text-gray-400 mt-1">
        开启后跳过全部门禁与扣费（不看等级、不看到期、不扣点）
      </div>
    </el-form-item>
    <el-form-item prop="billing_mode">
      <template #label>计费模式</template>
      <el-radio-group v-model="newFormInline.billing_mode" :disabled="isFree">
        <el-radio :value="0">计时模式</el-radio>
        <el-radio :value="1">点数模式</el-radio>
        <el-radio :value="2">高级模式</el-radio>
      </el-radio-group>
      <!-- 只显示当前选中模式的说明，三种模式不混在一起 -->
      <div class="w-full text-xs mt-1" :class="modeHintClass">
        {{ modeHint }}
      </div>
    </el-form-item>
    <!-- 计时模式：等级仍可作门槛；点数模式不要求等级 -->
    <el-form-item v-if="needsLevel" label="要求等级" prop="required_level">
      <el-input-number
        v-model="newFormInline.required_level"
        :min="1"
        :disabled="isFree"
      />
      <div class="w-full text-xs text-gray-400 mt-1">
        会员权限等级，1=不限（默认档）
      </div>
    </el-form-item>

    <template v-if="usesPoints">
      <!-- 计费范围：仅点数型计费有差异；计时模式按账号到期时间统一校验，无需区分 -->
      <el-form-item label="计费范围" prop="billing_scope">
        <el-radio-group v-model="newFormInline.billing_scope">
          <el-radio :value="0">按设备单独</el-radio>
          <el-radio :value="1">账号合并</el-radio>
        </el-radio-group>
        <div class="w-full text-xs text-gray-400 mt-1">
          <b>合并</b>：整个账号只算 1 份——第 2 台设备进来不再重复扣费，按时模式共用同一个周期。<br />
          <b>单独</b>：每台设备各算一份，同一账号在 3 台电脑上就是 3 份。
        </div>
      </el-form-item>
      <el-form-item label="扣费方式" prop="points_charge_mode">
        <el-radio-group v-model="newFormInline.points_charge_mode">
          <el-radio :value="0">按次(登录扣点)</el-radio>
          <el-radio :value="1">按时(预扣费)</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="!isPerTime" label="登录扣点" prop="points_per_login">
        <el-input-number v-model="newFormInline.points_per_login" :step="0.1" :precision="3" :min="0" />
        <div class="w-full text-xs text-gray-400 mt-1">
          {{ isHybrid ? "未配置等级的兜底值，0=不扣" : "每次登录扣点，0=不扣" }}
        </div>
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
        <span class="ml-2 text-gray-500">分钟</span>
        <div class="w-full text-xs text-gray-400 mt-1">
          一个周期多长。所有等级共用同一个周期，等级之间只差「扣多少点」
        </div>
      </el-form-item>
      <el-form-item v-if="isPerTime" label="周期扣点" prop="points_per_period">
        <el-input-number v-model="newFormInline.points_per_period" :step="0.1" :precision="3" :min="1" />
        <span class="ml-2 text-gray-500">点</span>
        <div class="w-full text-xs text-gray-400 mt-1">
          {{ isHybrid ? "未在「等级点数」中配置的等级按此值兜底，已配置的以等级价为准；" : "" }}每周期至少 1 点，避免 0 点无限续期
        </div>
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
      </el-form-item>
    </template>

    <!-- 授权期限：并发控制与崩溃回收的兜底 -->

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
