import { ref, onMounted } from "vue";
import { message } from "@/utils/message";
import { getAppBillingList, updateAppBilling } from "@/api/admin/app";
import { ElMessageBox } from "element-plus";

/** 单个应用的计费配置 */
export interface BillingRow {
  uuid: string;
  name: string;
  status: number;
  free_mode: number;
  billing_mode: number;
  required_level: number;
  billing_scope: number;
  points_charge_mode: number;
  points_per_login: number;
  points_period_minutes: number;
  points_per_period: number;
  points_heartbeat_charge: number;
}

const MODE_NAMES = ["计时模式", "点数模式", "高级模式"];

export function useBillingConfig() {
  const loading = ref(false);
  const dataList = ref<BillingRow[]>([]);

  /** 载入所有应用的计费配置 */
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

  /** 保存单个应用：只提交计费字段 */
  async function saveRow(row: BillingRow) {
    // 与后端校验口径一致，前端先兜一遍，避免提交后回显值突变
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

  /** 恢复为默认值（不改数据库，只改表格里的待保存值） */
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
    row.billing_mode = 0;
    row.required_level = 1;
    row.billing_scope = 0;
    row.points_charge_mode = 0;
    row.points_per_login = 1;
    row.points_period_minutes = 60;
    row.points_per_period = 1;
    row.points_heartbeat_charge = 0;
  }

  onMounted(() => {
    loadList();
  });

  return {
    loading,
    dataList,
    loadList,
    saveRow,
    resetRow,
    MODE_NAMES
  };
}
