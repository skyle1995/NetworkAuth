import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

export const getVariables = (params?: object) => {
  return http.request<Result>("get", "/api/admin/variable/list", { params });
};

export const createVariable = (data?: object) => {
  return http.request<Result>("post", "/api/admin/variable/create", { data });
};

export const updateVariable = (data?: object) => {
  return http.request<Result>("post", "/api/admin/variable/update", { data });
};

export const deleteVariable = (data?: object) => {
  return http.request<Result>("post", "/api/admin/variable/delete", { data });
};

export const batchDeleteVariables = (data?: object) => {
  return http.request<Result>("post", "/api/admin/variable/batch_delete", {
    data
  });
};
