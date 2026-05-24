import { http } from "@/utils/http";

export type ProfileResult = {
  code: number;
  msg: string;
  data: {
    username: string;
    [key: string]: any;
  };
};

export const getProfile = () => {
  return http.request<ProfileResult>("get", "/api/admin/profile");
};

export const updateProfile = (data: object) => {
  return http.request<ProfileResult>("post", "/api/admin/profile/update", {
    data
  });
};

export const updatePassword = (data: object) => {
  return http.request<ProfileResult>("post", "/api/admin/profile/password", {
    data
  });
};
