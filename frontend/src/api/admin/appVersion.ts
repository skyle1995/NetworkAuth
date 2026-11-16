import { http } from "@/utils/http";

export interface AppVersionRow {
  id: number;
  app_uuid: string;
  version: string;
  download_type: number;
  download_url: string;
  changelog: string;
  enabled: boolean;
  published_at: number;
}

export const listAppVersions = (params: { app_uuid: string }) => {
  return http.request<any>("get", "/api/admin/apps/versions", { params });
};

export const saveAppVersion = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/version_save", { data });
};

export const deleteAppVersion = (data?: object) => {
  return http.request<any>("post", "/api/admin/apps/version_delete", { data });
};
