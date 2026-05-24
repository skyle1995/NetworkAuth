import { http } from "@/utils/http";

export type SettingsResult = {
  code: number;
  msg: string;
  data: Record<string, string>;
};

export const getSettings = () => {
  return http.request<SettingsResult>("get", "/api/admin/settings");
};

export const updateSettings = (data: object) => {
  return http.request<SettingsResult>("post", "/api/admin/settings/update", {
    data
  });
};

export const generateKey = (type: string) => {
  return http.request<SettingsResult>(
    "post",
    `/api/admin/settings/generate-key?type=${type}`
  );
};
