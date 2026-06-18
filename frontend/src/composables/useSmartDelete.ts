import { ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

/**
 * 智能删除参数。通用、无业务耦合，删除/清空动作由业务侧注入。
 */
export interface SmartDeleteOptions {
  /** 实体名称，用于提示文案（如「日志」「配置」），默认「数据」 */
  entityName?: string;
  /** 取当前勾选的 id 列表；空数组表示未勾选 */
  getSelectedIds: () => Array<number | string>;
  /** 批量删除选中项（注入业务 API），返回后端响应 */
  batchDelete: (ids: Array<number | string>) => Promise<any>;
  /** 清空全部（注入业务 API）；未勾选时调用，附二次高危确认。不传则未勾选时仅提示 */
  clearAll?: () => Promise<any>;
  /** 删除/清空完成后的回调（刷新列表、清空选中等） */
  onDone?: () => void;
  /** 自定义成功判定，默认 res.code === 0 */
  isSuccess?: (res: any) => boolean;
}

/**
 * 智能删除（脚手架通用能力）：
 * - 勾选了行 → 批量删除选中（普通确认）；
 * - 未勾选 → 清空全部（高危红色二次确认）。
 *
 * 与 PureTableBar 工具栏按钮配合：按钮文案随选中数切换「删除选中(n)」/「清空全部」。
 */
export function useSmartDelete(options: SmartDeleteOptions) {
  const deleting = ref(false);
  const name = options.entityName || "数据";
  const isSuccess = options.isSuccess || ((res: any) => res?.code === 0);

  /** 统一执行删除动作并处理提示/回调 */
  async function run(action: () => Promise<any>, successText: string) {
    deleting.value = true;
    try {
      const res = await action();
      if (isSuccess(res)) {
        ElMessage.success(successText);
        options.onDone?.();
      } else {
        ElMessage.error(res?.msg || "操作失败");
      }
    } catch (e) {
      console.error(e);
      ElMessage.error("操作失败");
    } finally {
      deleting.value = false;
    }
  }

  async function handleDelete() {
    if (deleting.value) return;
    const ids = options.getSelectedIds() || [];

    if (ids.length === 0) {
      // 未勾选 → 清空全部（高危）
      if (!options.clearAll) {
        ElMessage.warning(`请先勾选要删除的${name}`);
        return;
      }
      ElMessageBox.confirm(
        `未勾选任何项，将清空【全部】${name}，此操作不可恢复！`,
        "高危操作提示",
        {
          type: "error",
          confirmButtonText: "确定清空",
          cancelButtonText: "取消"
        }
      )
        .then(() => run(options.clearAll!, `已清空全部${name}`))
        .catch(() => {});
      return;
    }

    // 勾选 → 批量删除选中
    ElMessageBox.confirm(`确认删除选中的 ${ids.length} 条${name}吗？`, "提示", {
      type: "warning"
    })
      .then(() => run(() => options.batchDelete(ids), "批量删除成功"))
      .catch(() => {});
  }

  return { deleting, handleDelete };
}
