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

/** 发送测试邮件，验证 SMTP 配置 */
export const testMail = (data: object) => {
  return http.request<SettingsResult>("post", "/api/admin/settings/test-mail", {
    data
  });
};
