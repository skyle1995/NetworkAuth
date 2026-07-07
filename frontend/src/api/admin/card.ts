import { http } from "@/utils/http";

type Result = {
  code: number;
  msg: string;
  count?: number;
  data?: any;
};

/** 卡密列表（支持 app_uuid / status / batch_no / search 筛选 + 分页） */
export const getCards = (params?: object) => {
  return http.request<Result>("get", "/api/admin/card/list", { params });
};

/** 批量制卡 */
export const createCards = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/create", { data });
};

/** 导出卡密：传 ids 导出选中，否则按筛选条件导出全部 */
export const exportCards = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/export", { data });
};

/** 批量冻结卡密 */
export const freezeCards = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/freeze", { data });
};

/** 批量解冻卡密 */
export const unfreezeCards = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/unfreeze", { data });
};

/** 批量删除卡密 */
export const batchDeleteCards = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/batch_delete", { data });
};

/** 按批次号删除整批卡密 */
export const deleteCardsByBatch = (data?: object) => {
  return http.request<Result>("post", "/api/admin/card/delete_batch", { data });
};
