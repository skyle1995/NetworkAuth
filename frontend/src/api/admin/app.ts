import { http } from "@/utils/http";
import type { ApiResult } from "./api";

export const getAppsList = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/list", { params });
};

export const getAppsSimple = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/simple", { params });
};

export const createApp = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/create", { data });
};

export const updateApp = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update", { data });
};

export const deleteApp = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/delete", { data });
};

export const batchDeleteApps = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/batch_delete", { data });
};

export const updateAppStatus = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_status", { data });
};

export const batchUpdateAppStatus = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/batch_update_status", {
    data
  });
};

export const resetAppSecret = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/reset_secret", { data });
};

export const getAppData = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_app_data", { params });
};

export const updateAppData = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_app_data", { data });
};

export const getAppAnnouncement = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_announcement", {
    params
  });
};

export const getAppMultiConfig = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_multi_config", {
    params
  });
};

export const updateAppMultiConfig = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_multi_config", {
    data
  });
};


export const getAppRegisterConfig = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_register_config", {
    params
  });
};

export const updateAppRegisterConfig = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_register_config", {
    data
  });
};

/** 高级模式：读取某应用的各会员等级点数消耗 */
export const getAppLevelPoints = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_level_points", {
    params
  });
};

/** 高级模式：覆盖保存某应用的各会员等级点数消耗 */
export const saveAppLevelPoints = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/save_level_points", {
    data
  });
};

/** 计费策略：列出所有应用的计费配置 */
export const getAppBillingList = () => {
  return http.request<ApiResult>("get", "/api/admin/apps/list_billing");
};

/** 计费策略：保存单个应用的计费配置 */
export const updateAppBilling = (data?: object) => {
  return http.request<ApiResult>("post", "/api/admin/apps/update_billing", { data });
};
