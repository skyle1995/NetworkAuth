import { http } from "@/utils/http";

export type InstallResult = {
  code: number;
  msg: string;
  data?: any;
};

export const submitInstall = (data: any) => {
  return http.request<any>("post", "/api/install", { data });
};
