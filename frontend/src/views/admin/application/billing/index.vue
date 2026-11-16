<script setup lang="ts">
import { useBillingConfig, type BillingRow } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "BillingConfigIndex"
});

const { loading, dataList, loadList, saveRow, resetRow, MODE_NAMES } =
  useBillingConfig();

/** 该应用是否按点数扣费（点数/高级模式才有点价相关字段） */
function usesPoints(row: BillingRow) {
  return row.free_mode !== 1 && row.billing_mode !== 0;
}
</script>

<template>
  <div class="main">
    <PureTableBar title="计费策略" :columns="[]" @refresh="loadList">
      <template #buttons>
        <div class="text-xs text-gray-400 leading-5">
          计费按应用独立配置：每个插件的模式、等级要求、单价互不影响，<br />
          但扣的都是同一个账号钱包（点数余额 / 到期时间）。
        </div>
      </template>

      <el-table
        v-loading="loading"
        :data="dataList"
        border
        row-key="uuid"
        size="small"
        table-layout="auto"
      >
        <el-table-column label="应用" prop="name" min-width="140" fixed>
          <template #default="{ row }">
            <span :class="{ 'text-gray-400': row.status !== 1 }">
              {{ row.name }}
            </span>
            <el-tag v-if="row.status !== 1" size="small" type="info" class="ml-1">
              停用
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="计费豁免" width="100" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.free_mode"
              :active-value="1"
              :inactive-value="0"
              inline-prompt
              active-text="免"
              inactive-text="费"
            />
          </template>
        </el-table-column>

        <el-table-column label="计费模式" width="130">
          <template #default="{ row }">
            <el-select v-model="row.billing_mode" :disabled="row.free_mode === 1">
              <el-option
                v-for="(n, i) in MODE_NAMES"
                :key="i"
                :label="n"
                :value="i"
              />
            </el-select>
          </template>
        </el-table-column>

        <el-table-column label="要求等级" width="110" align="center">
          <template #default="{ row }">
            <el-input-number
              v-model="row.required_level"
              :min="1"
              :max="99"
              :controls="false"
              :disabled="row.free_mode === 1 || row.billing_mode === 1"
              class="!w-[72px]"
            />
          </template>
        </el-table-column>

        <el-table-column label="计费范围" width="120">
          <template #default="{ row }">
            <el-select
              v-model="row.billing_scope"
              :disabled="!usesPoints(row)"
              placeholder="—"
            >
              <el-option label="按设备单独" :value="0" />
              <el-option label="账号合并" :value="1" />
            </el-select>
          </template>
        </el-table-column>

        <el-table-column label="扣费方式" width="120">
          <template #default="{ row }">
            <el-select
              v-model="row.points_charge_mode"
              :disabled="!usesPoints(row)"
              placeholder="—"
            >
              <el-option label="按次" :value="0" />
              <el-option label="按时" :value="1" />
            </el-select>
          </template>
        </el-table-column>

        <el-table-column label="单价" min-width="200">
          <template #default="{ row }">
            <template v-if="!usesPoints(row)">
              <span class="text-xs text-gray-400">不扣点</span>
            </template>
            <template v-else-if="row.points_charge_mode === 0">
              <el-input-number
                v-model="row.points_per_login"
                :min="0"
                :step="0.1"
                :precision="3"
                :controls="false"
                class="!w-[90px]"
              />
              <span class="ml-1 text-xs text-gray-500">点/次</span>
            </template>
            <template v-else>
              <el-input-number
                v-model="row.points_per_period"
                :min="1"
                :step="0.1"
                :precision="3"
                :controls="false"
                class="!w-[80px]"
              />
              <span class="ml-1 text-xs text-gray-500">点 /</span>
              <el-input-number
                v-model="row.points_period_minutes"
                :min="1"
                :controls="false"
                class="!w-[70px] ml-1"
              />
              <span class="ml-1 text-xs text-gray-500">分钟</span>
            </template>
          </template>
        </el-table-column>

        <el-table-column label="扣费触发" width="120">
          <template #default="{ row }">
            <el-select
              v-model="row.points_heartbeat_charge"
              :disabled="!usesPoints(row) || row.points_charge_mode === 0"
              placeholder="—"
            >
              <el-option label="登录预扣" :value="0" />
              <el-option label="心跳触发" :value="1" />
            </el-select>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="168" fixed="right" align="center">
          <template #default="{ row }">
            <div class="flex items-center justify-center">
              <el-button
                type="primary"
                size="small"
                :icon="useRenderIcon('ep:check')"
                @click="saveRow(row)"
              >
                保存
              </el-button>
              <el-button
                type="info"
                link
                size="small"
                class="!ml-2"
                @click="resetRow(row)"
              >
                默认值
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </PureTableBar>
  </div>
</template>
