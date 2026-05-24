import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

export const getFunctions = (params?: object) => {
  return http.request<Result>("get", "/api/admin/function/list", { params });
};

export const createFunction = (data?: object) => {
  return http.request<Result>("post", "/api/admin/function/create", { data });
};

export const updateFunction = (data?: object) => {
  return http.request<Result>("post", "/api/admin/function/update", { data });
};

export const deleteFunction = (data?: object) => {
  return http.request<Result>("post", "/api/admin/function/delete", { data });
};

export const batchDeleteFunctions = (data?: object) => {
  return http.request<Result>("post", "/api/admin/function/batch_delete", {
    data
  });
};
