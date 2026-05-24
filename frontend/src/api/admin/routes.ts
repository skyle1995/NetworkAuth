// import { http } from "@/utils/http";

type Result = {
  success: boolean;
  data: Array<any>;
};

export const getAsyncRoutes = () => {
  // 模拟后端动态生成路由 (对接时可以替换为真实的后端 API)
  return Promise.resolve({
    success: true,
    data: []
  } as Result);
};
