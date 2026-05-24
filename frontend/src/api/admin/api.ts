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
