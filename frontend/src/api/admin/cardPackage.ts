import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

/** 卡密套餐列表（app_uuid 筛选；enabled=1 只返回启用的，供制卡下拉用） */
export const getCardPackages = (params?: object) => {
  return http.request<Result>("get", "/api/admin/card_package/list", {
    params
  });
};

/** 新增/更新卡密套餐（uuid 为空则新增） */
export const saveCardPackage = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card_package/save", { data });
};

/** 删除卡密套餐 */
export const deleteCardPackage = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card_package/delete", {
    data
  });
};
