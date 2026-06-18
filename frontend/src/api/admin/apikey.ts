import { http } from "@/utils/http";

const base = "/api/admin/apikey";

// 统一响应结构
export type ApiKeyResult = {
  code: number;
  msg: string;
  data?: any;
};

// 获取可分配的能力列表
export const getApiKeyScopes = () =>
  http.request<ApiKeyResult>("get", `${base}/scopes`);
// 密钥列表（可按名称搜索）
export const getApiKeys = (params?: object) =>
  http.request<ApiKeyResult>("get", `${base}/list`, { params });
// 新建密钥
export const createApiKey = (data: object) =>
  http.request<ApiKeyResult>("post", `${base}/create`, { data });
// 编辑密钥
export const updateApiKey = (data: object) =>
  http.request<ApiKeyResult>("put", `${base}/update`, { data });
// 重置密钥串
export const regenerateApiKey = (id: number | string) =>
  http.request<ApiKeyResult>("post", `${base}/regenerate`, { data: { id } });
// 删除密钥
export const deleteApiKey = (id: number | string) =>
  http.request<ApiKeyResult>("delete", `${base}/delete/${id}`);
