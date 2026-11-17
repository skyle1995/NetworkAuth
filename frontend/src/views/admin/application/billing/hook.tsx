import { ref, onMounted } from "vue";
import { message } from "@/utils/message";
import { getAppBillingList, updateAppBilling } from "@/api/admin/app";
import { getMemberLevels } from "@/api/admin/memberLevel";
import { ElMessageBox } from "element-plus";

/** 单个应用的计费配置（三个正交开关 + 计费参数） */
export interface BillingRow {
  uuid: string;
  name: string;
  status: number;

  /** 计费豁免：开着就不看任何门禁、不扣费 */
  free_mode: number;

  /** 三个开关 */
  bill_time_enabled: number; // 计时：要求账号有效期
  bill_points_enabled: number; // 点数：扣点
  level_gate_enabled: number; // 会员限制：等级 ≥ required_level

  /** 点数开关打开后才有意义 */
  billing_scope: number; // 0 按设备 / 1 账号合并
  points_charge_mode: number; // 0 按次 / 1 按周期
  points_per_login: number; // 按次单价
  points_per_period: number; // 按周期单价
  points_period_minutes: number; // 周期分钟
  points_heartbeat_charge: number; // 0 登录预扣 / 1 心跳触发

  /** 会员限制开关打开后才有意义 */
  required_level: number;
}

export interface LevelOption {
  uuid: string;
  name: string;
  level: number;
}

export function useBillingConfig() {
  const loading = ref(false);
  const dataList = ref<BillingRow[]>([]);
  const levels = ref<LevelOption[]>([]);

  async function loadList() {
    loading.value = true;
    try {
      const { code, data } = await getAppBillingList();
      if (code === 0 && Array.isArray(data)) {
        dataList.value = data;
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  }

  async function loadLevels() {
    try {
      const { code, data } = await getMemberLevels({});
      if (code === 0 && Array.isArray(data)) {
        levels.value = data
          .map((lv: any) => ({
            uuid: lv.uuid,
            name: lv.name,
            level: Number(lv.level ?? 1)
          }))
          .sort((a: LevelOption, b: LevelOption) => a.level - b.level);
      }
    } catch (e) {
      console.error(e);
    }
  }

  /** 该行是否按点数收费 */
  function usesPoints(row: BillingRow) {
    return row.free_mode !== 1 && row.bill_points_enabled === 1;
  }

  /** 该行是否按时长收费 */
  function usesTime(row: BillingRow) {
    return row.free_mode !== 1 && row.bill_time_enabled === 1;
  }

  /** 该行是否限制会员等级 */
  function usesLevel(row: BillingRow) {
    return row.free_mode !== 1 && row.level_gate_enabled === 1;
  }

  async function saveRow(row: BillingRow) {
    // 与后端校验口径一致，先兜一遍，避免提交后回显突变
    if (row.required_level < 1) row.required_level = 1;
    if (row.points_period_minutes < 1) row.points_period_minutes = 60;
    if (row.points_per_login < 0) row.points_per_login = 0;
    if (row.points_per_period < 1) row.points_per_period = 1;

    try {
      const { code, msg } = await updateAppBilling({ ...row });
      if (code === 0) {
        message(`「${row.name}」计费策略已保存`, { type: "success" });
        await loadList();
      } else {
        message(msg || "保存失败", { type: "error" });
      }
    } catch (e: any) {
      message(String(e?.message || e), { type: "error" });
    }
  }

  /** 把该行恢复为默认值（不立即写库） */
  async function resetRow(row: BillingRow) {
    try {
      await ElMessageBox.confirm(
        `将「${row.name}」的计费策略恢复为默认值？<br/><span style="color:red;font-size:12px;">保存后才会生效</span>`,
        "提示",
        { type: "warning", dangerouslyUseHTMLString: true }
      );
    } catch {
      return;
    }
    row.free_mode = 0;
    row.bill_time_enabled = 1;
    row.bill_points_enabled = 0;
    row.level_gate_enabled = 0;
    row.required_level = 1;
    row.billing_scope = 0;
    row.points_charge_mode = 0;
    row.points_per_login = 1;
    row.points_per_period = 1;
    row.points_period_minutes = 60;
    row.points_heartbeat_charge = 0;
  }

  onMounted(() => {
    loadLevels();
    loadList();
  });

  return {
    loading,
    dataList,
    levels,
    loadList,
    saveRow,
    resetRow,
    usesPoints,
    usesTime,
    usesLevel
  };
}
