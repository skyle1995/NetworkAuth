import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

/** 会员等级列表（app_uuid 筛选） */
export const getMemberLevels = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member_level/list", {
    params
  });
};

/** 新增/更新会员等级（uuid 为空则新增） */
export const saveMemberLevel = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member_level/save", { data });
};

/** 删除会员等级 */
export const deleteMemberLevel = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member_level/delete", {
    data
  });
};
