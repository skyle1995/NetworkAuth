import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

/** 终端用户列表（支持 app_uuid / type / status / search 筛选 + 分页） */
export const getMembers = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/list", { params });
};

/** 后台创建注册型终端用户 */
export const createMember = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/create", { data });
};

/** 批量设置终端用户状态（0封停/1正常/2黑名单） */
export const setMemberStatus = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/set_status", { data });
};

/** 充值时长（duration_unit 为 permanent 时设为永久） */
export const rechargeMember = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/recharge", { data });
};

/** 扣除时长 */
export const deductMember = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/deduct", { data });
};

/** 重置密码 */
export const resetMemberPassword = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/reset_password", {
    data
  });
};

/** 更新备注 */
export const updateMemberRemark = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/update_remark", {
    data
  });
};

/** 查询终端用户的机器码/IP 绑定列表 */
export const getMemberBindings = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/bindings", { params });
};

/** 清空终端用户绑定 */
export const clearMemberBindings = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/clear_bindings", {
    data
  });
};

/** 批量删除终端用户 */
export const batchDeleteMembers = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/batch_delete", {
    data
  });
};
