import { http } from "@/utils/http";

export type LogsResult = {
  code: number;
  message: string;
  data: {
    list: Array<any>;
    total: number;
  };
};

/** 获取登录日志列表 */
export const getLoginLogsList = (params?: object) => {
  return http.request<LogsResult>("get", "/api/admin/login_logs", { params });
};

/** 清空登录日志 */
export const clearLoginLogs = () => {
  return http.request<LogsResult>("post", "/api/admin/login_logs/clear");
};

/** 按 ID 批量删除登录日志 */
export const batchDeleteLoginLogs = (ids: Array<number | string>) => {
  return http.request<LogsResult>(
    "post",
    "/api/admin/login_logs/batch-delete",
    {
      data: { ids }
    }
  );
};

/** 获取操作日志列表 */
export const getOperationLogsList = (params?: object) => {
  return http.request<LogsResult>("get", "/api/admin/logs", { params });
};

/** 清空操作日志 */
export const clearOperationLogs = () => {
  return http.request<LogsResult>("post", "/api/admin/logs/clear");
};

/** 按 ID 批量删除操作日志 */
export const batchDeleteOperationLogs = (ids: Array<number | string>) => {
  return http.request<LogsResult>("post", "/api/admin/logs/batch-delete", {
    data: { ids }
  });
};
