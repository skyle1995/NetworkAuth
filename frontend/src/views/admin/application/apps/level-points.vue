<script setup lang="ts">
import { ref } from "vue";
import { fmtPoints } from "@/utils/points";

// ============================================================================
// 高级模式：每个会员等级在本应用的点数消耗
// ============================================================================
//
// 关键语义（改这里之前务必读完）：
//
//   · **未配置的等级**不等于「扣 0 点」，而是**回退到应用的默认点数**
//     （按次取「登录扣点」，按时取「周期扣点」）。
//   · **已配置为 0** 才是真的「该等级不扣点」。
//
// 所以输入框里显示的必须是**实际生效值**：未配置时填默认点数，
// 否则会出现「状态写着默认 1、输入框却是 0」的矛盾，而且一点保存就会把
// 所有等级写成「显式不扣点」，高级模式从此不再扣费。
//
// 保存时只提交「原本已配置」或「本次改动过」的行，其余保持未配置
// （后端 SaveAppLevelPoints 是覆盖式写入，未提交的行会被清空）。

export interface LevelPointItem {
  level_uuid: string;
  name: string;
  level: number;
  color?: string;
  points: number;
  configured?: boolean;
}

export interface FormProps {
  appName?: string;
  billingMode?: number;
  defaultPoints?: number;
  items: LevelPointItem[];
}

const props = withDefaults(defineProps<FormProps>(), {
  appName: "",
  billingMode: 0,
  defaultPoints: 0,
  items: () => []
});

interface Row extends LevelPointItem {
  /** 原本是否已配置过（决定保存时是否提交） */
  wasConfigured: boolean;
  /** 本次是否改动过 */
  dirty: boolean;
}

const rows = ref<Row[]>(
  props.items.map(it => ({
    ...it,
    // 未配置 → 显示实际生效的默认点数
    points: it.configured ? it.points : props.defaultPoints,
    wasConfigured: !!it.configured,
    dirty: false
  }))
);

function onPointsChange(row: Row) {
  row.dirty = true;
}

/** 恢复为「未配置」，即跟随应用默认点数 */
function resetToDefault(row: Row) {
  row.points = props.defaultPoints;
  row.wasConfigured = false;
  row.dirty = false;
}

function getRef() {
  return formRef.value;
}

/** 提交给后端保存的数据：只提交已配置或改动过的行 */
function getPayload() {
  return rows.value
    .filter(r => r.wasConfigured || r.dirty)
    .map(r => ({
      level_uuid: r.level_uuid,
      points: Number(r.points) || 0
    }));
}

const formRef = ref();
defineExpose({ getRef, getPayload });
</script>

<template>
  <div>
    <el-alert
      v-if="billingMode !== 2"
      type="info"
      :closable="false"
      class="mb-3"
      title="当前应用不是高级模式"
      description="等级点数仅在「高级模式」下生效：等级达标后按该等级配置的点数扣点。会员/点数模式忽略此配置。"
    />
    <el-alert
      v-else
      type="success"
      :closable="false"
      class="mb-3"
      :title="`${appName || '本应用'}：高级模式等级点数`"
      description="每个等级在本应用扣多少点。留空或点「恢复默认」表示不单独设置，跟随应用默认点数。按时扣费时每个周期至少按 1 点结算，填 0 等同于 1。"
    />

    <el-table :data="rows" border size="small">
      <el-table-column label="等级" min-width="150">
        <template #default="{ row }">
          <span :style="{ color: row.color || '#909399' }">
            {{ row.name }}
          </span>
          <span class="ml-2 text-xs text-gray-400">Lv.{{ row.level }}</span>
        </template>
      </el-table-column>

      <el-table-column label="点数消耗" width="190">
        <template #default="{ row }">
          <el-input-number
            v-model="row.points"
            :min="0"
            :step="0.1"
            :precision="3"
            size="small"
            controls-position="right"
            @change="onPointsChange(row)"
          />
        </template>
      </el-table-column>

      <el-table-column label="状态" min-width="180">
        <template #default="{ row }">
          <el-tag v-if="row.wasConfigured" type="success" size="small">
            单独设置：{{ fmtPoints(row.points) }} 点
          </el-tag>
          <el-tag v-else-if="row.dirty" type="warning" size="small">
            待保存：{{ fmtPoints(row.points) }} 点
          </el-tag>
          <el-tag v-else type="info" size="small">
            默认：{{ fmtPoints(defaultPoints) }} 点
          </el-tag>
          <el-button
            v-if="row.wasConfigured || row.dirty"
            link
            type="primary"
            size="small"
            class="ml-2"
            @click="resetToDefault(row)"
          >
            改回默认
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="rows.length === 0" class="mt-3 text-sm text-gray-400">
      尚未创建任何会员等级，请先到「账号中心 → 会员等级」添加等级。
    </div>

    <div v-else class="mt-3 text-xs text-gray-500">
      提示：<b>0 点</b> 表示该等级完全免费（不扣点）；不设置则跟随应用默认点数（本次为
      <b>{{ fmtPoints(defaultPoints) }}</b> 点）。
    </div>
  </div>
</template>
