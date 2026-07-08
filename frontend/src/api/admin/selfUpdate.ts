import { http } from "@/utils/http";

// 自更新状态
export type SelfUpdateStatus = {
  running: boolean;
  checked_at: number;
  checked_at_str: string;
  last_error: string;
  current_version: string;
  latest_version: string;
  versions_count: number;
  prepared: boolean;
  prepared_version: string;
  prepare_error: string;
  auto_replace_tried: boolean;
  auto_replace_ok: boolean;
  auto_replace_error: string;
  script_shell_path: string;
  script_powershell_path: string;
  download_progress: number;
};

// 自更新版本条目
export type SelfUpdateVersionItem = {
  version: string;
  size: number;
  size_formatted: string;
  sha256: string;
  download_url: string;
  is_newer: boolean;
  is_current: boolean;
};

// 自更新配置
export type SelfUpdateConfig = {
  type: number;
  secret_id: string;
  secret_key: string;
  region: string;
  bucket: string;
  prefix: string;
  base_url: string;
};

// 获取更新状态
export const getSelfUpdateStatus = () => {
  return http.request<{ ok: boolean; data: SelfUpdateStatus }>(
    "get",
    "/api/admin/system/self-update/status"
  );
};

// 触发异步检查更新（带防抖）
export const checkSelfUpdate = () => {
  return http.request<{ ok: boolean; data: SelfUpdateStatus }>(
    "post",
    "/api/admin/system/self-update/check"
  );
};

// 手动强制检查更新（绕过节流缓存，每次强拉最新）
export const checkSelfUpdateForce = () => {
  return http.request<{ ok: boolean; data: SelfUpdateStatus }>(
    "post",
    "/api/admin/system/self-update/check-force"
  );
};

// 手动重启以加载已安装的新版本
export const restartSelfUpdate = () => {
  return http.request<{ ok: boolean; message?: string }>(
    "post",
    "/api/admin/system/self-update/restart"
  );
};

// 扫描版本列表
export const getSelfUpdateVersions = () => {
  return http.request<{ ok: boolean; data: SelfUpdateVersionItem[] }>(
    "get",
    "/api/admin/system/self-update/versions"
  );
};

// 准备更新
export const prepareSelfUpdate = (data: {
  version: string;
  download_url: string;
  sha256: string;
}) => {
  return http.request<{ ok: boolean; data: SelfUpdateStatus }>(
    "post",
    "/api/admin/system/self-update/prepare",
    { data }
  );
};

// 获取更新配置
export const getSelfUpdateConfig = () => {
  return http.request<{ ok: boolean; data: SelfUpdateConfig }>(
    "get",
    "/api/admin/system/self-update/config"
  );
};

// 保存更新配置
export const updateSelfUpdateConfig = (data: Partial<SelfUpdateConfig>) => {
  return http.request<{ ok: boolean; message: string }>(
    "put",
    "/api/admin/system/self-update/config",
    { data }
  );
};

// 测试存储桶连接
export const testSelfUpdateConfig = () => {
  return http.request<{
    ok: boolean;
    message: string;
    data?: { versions_count: number };
  }>("post", "/api/admin/system/self-update/test");
};
