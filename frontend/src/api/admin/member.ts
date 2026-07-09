import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

/** 账号列表（支持 app_uuid / type / status / search 筛选 + 分页） */
export const getMembers = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/list", { params });
};

/** 后台创建注册型账号 */
export const createMember = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/create", { data });
};

/** 批量设置账号状态（0封停/1正常/2黑名单） */
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

/** 获取账号的用户数据 */
export const getMemberData = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/get_data", { params });
};

/** 更新账号的用户数据 */
export const updateMemberData = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/update_data", {
    data
  });
};

/** 查询账号的机器码/IP 绑定列表 */
export const getMemberBindings = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/bindings", { params });
};

/** 清空账号绑定 */
export const clearMemberBindings = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/clear_bindings", {
    data
  });
};

/** 踢下线（会话ID或用户UUID全部） */
export const kickMemberSession = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/kick", { data });
};

/** 在线会话列表（跨用户，支持 app_uuid / search 筛选 + 分页） */
export const getOnlineSessions = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/online", { params });
};

/** 拉黑账号（可选同时拉黑其 设备/IP/地区） */
export const blacklistMember = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/blacklist", { data });
};

/** 从在线会话拉黑 设备/IP/地区（可连带拉黑账号） */
export const blacklistSession = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/online/blacklist", {
    data
  });
};

/** 黑名单列表（支持 app_uuid / type / search 筛选 + 分页） */
export const getBlacklist = (params?: object) => {
  return http.request<Result>("get", "/api/admin/blacklist/list", { params });
};

/** 手动新增黑名单条目（设备/IP/地区） */
export const addBlacklist = (data?: object) => {
  return http.request<Result>("post", "/api/admin/blacklist/add", { data });
};

/** 批量移除黑名单（解封） */
export const deleteBlacklist = (data?: object) => {
  return http.request<Result>("post", "/api/admin/blacklist/delete", { data });
};

/** 账号调用审计日志列表 */
export const getMemberLogs = (params?: object) => {
  return http.request<Result>("get", "/api/admin/member/logs", { params });
};

/** 清空审计日志 */
export const clearMemberLogs = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/logs/clear", { data });
};

/** 批量删除账号 */
export const batchDeleteMembers = (data?: object) => {
  return http.request<Result>("post", "/api/admin/member/batch_delete", {
    data
  });
};
