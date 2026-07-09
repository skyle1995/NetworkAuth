import { http } from "@/utils/http";

export type ApiResult = {
  code: number;
  msg: string;
  data?: any;
};

/** 获取接口列表 */
export const getApiList = (params?: object) => {
  return http.request<ApiResult>("get", "/api/admin/apis/list", { params });
};

/** 获取接口类型 */
export const getApiTypes = () => {
  return http.request<ApiResult>("get", "/api/admin/apis/types");
};

/** 导出应用对接密钥（应用密钥 + 各接口算法与密钥） */
export const exportApiKeys = (params?: object) => {
  return http.request<ApiResult>("get", "/api/admin/apis/export", { params });
};

/** 更新接口配置 */
export const updateApi = (data?: object) => {
  return http.request<ApiResult>("post", "/api/admin/apis/update", { data });
};

/** 更新接口状态 */
export const updateApiStatus = (data?: object) => {
  return http.request<ApiResult>("post", "/api/admin/apis/update_status", {
    data
  });
};

/** 生成密钥 */
export const generateApiKeys = (data?: object) => {
  return http.request<ApiResult>("post", "/api/admin/apis/generate_keys", {
    data
  });
};

/** 批量设置接口加密方式并自动(重新)生成密钥 */
export const batchSetApiAlgorithm = (data?: object) => {
  return http.request<ApiResult>("post", "/api/admin/apis/batch_set", { data });
};
