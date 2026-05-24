import { http } from "@/utils/http";

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

export const updateAppAnnouncement = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_announcement", {
    data
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

export const getAppBindConfig = (params?: object) => {
  return http.request<any>("get", "/api/admin/apps/get_bind_config", {
    params
  });
};

export const updateAppBindConfig = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/update_bind_config", {
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
