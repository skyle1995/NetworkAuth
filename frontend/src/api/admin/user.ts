import { http } from "@/utils/http";

export type UserResult = {
  success: boolean;
  data: {
    /** 头像 */
    avatar: string;
    /** 用户名 */
    username: string;
    /** 昵称 */
    nickname: string;
    /** 当前登录用户的角色 */
    roles: Array<string>;
    /** 按钮级别权限 */
    permissions: Array<string>;
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间（格式'xxxx/xx/xx xx:xx:xx'） */
    expires: Date;
  };
};

export type RefreshTokenResult = {
  success: boolean;
  data: {
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间（格式'xxxx/xx/xx xx:xx:xx'） */
    expires: Date;
  };
};

/** 获取 CSRF Token */
export const getCsrfToken = () => {
  return http.request<any>("get", "/api/admin/csrf");
};

/** 登录 */
export const getLogin = async (data: any) => {
  try {
    const res = await http.request<any>("post", "/api/admin/login", { data });
    if (res.success || res.code === 0 || res.code === 200) {
      return {
        success: true,
        data: {
          avatar: res.data?.avatar || "",
          username: res.data?.username || data.username,
          nickname: res.data?.nickname || "管理员",
          roles: res.data?.role === 0 ? ["super_admin"] : ["admin"],
          permissions: ["*:*:*"],
          accessToken: res.data?.accessToken || res.data?.token || "",
          refreshToken: res.data?.refreshToken || "",
          expires: res.data?.expires
            ? new Date(res.data.expires)
            : new Date(new Date().getTime() + 2 * 60 * 60 * 1000)
        }
      } as UserResult;
    } else {
      throw new Error(res.msg || res.message || "登录失败");
    }
  } catch (error: any) {
    if (error.response && error.response.data) {
      throw new Error(
        error.response.data.msg || error.response.data.message || "登录失败"
      );
    }
    throw error;
  }
};

/** 获取验证码 */
export const getCaptcha = () => {
  return http.request<any>("get", "/api/admin/captcha");
};

/** 刷新`token` */
export const refreshTokenApi = (data?: object) => {
  return http.request<RefreshTokenResult>("post", "/api/admin/refresh-token", {
    data
  });
};
