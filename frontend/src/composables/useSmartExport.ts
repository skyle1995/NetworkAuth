import { ref } from "vue";
import { ElMessage } from "element-plus";

/**
 * 智能导出列定义。
 */
export interface SmartExportColumn {
  /** 数据字段名 */
  prop: string;
  /** CSV 表头标题 */
  label: string;
  /** 可选：自定义取值/格式化，默认取 row[prop] */
  formatter?: (row: any) => any;
}

/**
 * 智能导出参数。通用、无业务耦合，数据来源全部通过回调注入。
 */
export interface SmartExportOptions {
  /** 导出列（决定 CSV 表头与字段顺序） */
  columns: SmartExportColumn[];
  /** 文件名前缀（自动追加时间戳与 .csv 后缀） */
  filename?: string;
  /** 取当前已勾选的行；返回空数组表示未勾选 */
  getSelected: () => any[];
  /**
   * 未勾选时的全量数据来源：按当前筛选向后端取「全部」后导出（推荐，数据完整）。
   * 不传则回退到 getFallback。
   */
  fetchAll?: () => Promise<any[]>;
  /** 未勾选且未提供 fetchAll 时的回退数据（通常是当前页 dataList） */
  getFallback?: () => any[];
}

/** CSV 单元格转义：含逗号/引号/换行时用引号包裹并转义内部引号 */
function escapeCSV(value: any): string {
  const s = value === null || value === undefined ? "" : String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** 行数组转 CSV 文本（首行表头，前置 BOM 以兼容 Excel 中文） */
function rowsToCSV(rows: any[], columns: SmartExportColumn[]): string {
  const header = columns.map(c => escapeCSV(c.label)).join(",");
  const body = rows.map(row =>
    columns
      .map(c => escapeCSV(c.formatter ? c.formatter(row) : row[c.prop]))
      .join(",")
  );
  // 前置 BOM(﻿) 让 Excel 正确识别 UTF-8 中文
  return "﻿" + [header, ...body].join("\r\n");
}

/** 触发浏览器下载文本文件 */
function downloadText(content: string, filename: string) {
  const blob = new Blob([content], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

/** 生成时间戳后缀，避免文件重名 */
function stamp(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  return (
    `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}` +
    `_${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`
  );
}

/**
 * 智能导出（脚手架通用能力）：
 * - 勾选了行 → 仅导出选中行；
 * - 未勾选 → 有 fetchAll 则按当前筛选取全部导出，否则回退导出当前页数据。
 *
 * 业务侧通过 columns 定义表头、通过回调注入数据来源，composable 不耦合任何接口。
 */
export function useSmartExport(options: SmartExportOptions) {
  const exporting = ref(false);

  async function handleExport() {
    if (exporting.value) return;
    const { columns, getSelected, fetchAll, getFallback } = options;
    const prefix = options.filename || "export";
    exporting.value = true;
    try {
      const selected = getSelected() || [];
      let rows: any[];
      if (selected.length > 0) {
        rows = selected;
      } else if (fetchAll) {
        rows = (await fetchAll()) || [];
      } else {
        rows = getFallback?.() || [];
      }
      if (!rows.length) {
        ElMessage.warning("没有可导出的数据");
        return;
      }
      downloadText(rowsToCSV(rows, columns), `${prefix}_${stamp()}.csv`);
      ElMessage.success(`已导出 ${rows.length} 条`);
    } catch (e) {
      console.error(e);
      ElMessage.error("导出失败");
    } finally {
      exporting.value = false;
    }
  }

  return { exporting, handleExport };
}
