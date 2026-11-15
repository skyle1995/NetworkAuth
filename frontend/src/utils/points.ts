/**
 * 点数格式化：整数不带小数点，小数最多 3 位。
 * 点数现在支持小数（如 1.5），直接插值会出现 1.5000000000000002 这类浮点尾巴。
 */
export function fmtPoints(v: unknown): string {
  const n = Number(v);
  if (!Number.isFinite(n)) return "0";
  if (Number.isInteger(n)) return String(n);
  return String(Math.round(n * 1000) / 1000);
}