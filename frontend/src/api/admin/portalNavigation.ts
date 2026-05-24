import { http } from "@/utils/http";

export type PortalNavigationItem = {
  id: number;
  name: string;
  type: "link" | "group";
  parent_id: number;
  path: string;
  sort: number;
  is_home: boolean;
  is_hidden: boolean;
  is_external: boolean;
  created_at?: string;
  updated_at?: string;
};

export type PortalNavigationResult = {
  code: number;
  msg: string;
  data: PortalNavigationItem[];
};

export type PortalNavigationSavePayload = {
  id?: number;
  name: string;
  type: "link" | "group";
  parent_id: number;
  path: string;
  sort: number;
  is_home: boolean;
  is_hidden: boolean;
  is_external: boolean;
};

/**
 * 获取门户导航列表
 * 返回后台导航设置页面使用的完整数据
 */
export const getPortalNavigationList = () => {
  return http.request<PortalNavigationResult>("get", "/api/admin/portal-navigation");
};

/**
 * 获取公开门户导航列表
 * 返回门户首页用于展示的可见导航数据
 */
export const getPublicPortalNavigationList = () => {
  return http.request<PortalNavigationResult>(
    "get",
    "/api/admin/portal-navigation/public"
  );
};

/**
 * 创建门户导航
 * 提交新建导航数据到后台
 */
export const createPortalNavigation = (data: PortalNavigationSavePayload) => {
  return http.request<PortalNavigationResult>(
    "post",
    "/api/admin/portal-navigation/create",
    {
      data
    }
  );
};

/**
 * 更新门户导航
 * 按ID提交编辑后的导航数据
 */
export const updatePortalNavigation = (data: PortalNavigationSavePayload) => {
  return http.request<PortalNavigationResult>(
    "post",
    "/api/admin/portal-navigation/update",
    {
      data
    }
  );
};

/**
 * 删除门户导航
 * 按ID删除指定导航记录
 */
export const deletePortalNavigation = (id: number) => {
  return http.request<PortalNavigationResult>(
    "post",
    "/api/admin/portal-navigation/delete",
    {
      data: { id }
    }
  );
};
