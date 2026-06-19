import axios from "axios";
import type { App } from "vue";

let config: object = {};
const setConfig = (cfg?: unknown) => {
  config = Object.assign(config, cfg);
};

const getConfig = (key?: string): PlatformConfigs => {
  if (typeof key === "string") {
    const arr = key.split(".");
    if (arr && arr.length) {
      let data = config;
      arr.forEach(v => {
        if (data && typeof data[v] !== "undefined") {
          data = data[v];
        } else {
          data = null;
        }
      });
      return data;
    }
  }
  return config;
};

/** 获取项目动态全局配置 */
export const getPlatformConfig = async (app: App): Promise<undefined> => {
  app.config.globalProperties.$config = getConfig();
  return axios({
    method: "get",
    url: `/api/home/settings/public`
  })
    .then(({ data: res }) => {
      let $config = app.config.globalProperties.$config;
      // 自动注入系统配置
      if (app && $config && res && res.code === 0 && res.data) {
        const backendConfig = res.data;
        const formattedConfig: any = {};

        // 将后端的下划线命名(platform_xxx)转换为前端需要的帕斯卡命名(PascalCase)
        for (const key in backendConfig) {
          if (key.startsWith("platform_")) {
            const camelKey = key
              .replace("platform_", "")
              .split("_")
              .map(word => word.charAt(0).toUpperCase() + word.slice(1))
              .join("");

            let val = backendConfig[key];
            // 转换布尔值 0/1 到 true/false
            if (val === "1") val = true;
            if (val === "0") val = false;
            // 转换数字类型
            if (
              camelKey === "MenuSearchHistory" ||
              camelKey === "MaxTagsLevel"
            ) {
              val = Number(val);
            }

            formattedConfig[camelKey] = val;
          } else if (key === "site_title") {
            // 将后端的 site_title 映射为前端全局配置中的 Title
            formattedConfig["Title"] = backendConfig[key];
          } else if (key === "site_description") {
            formattedConfig["Description"] = backendConfig[key];
          } else if (key === "site_keywords") {
            formattedConfig["Keywords"] = backendConfig[key];
          } else if (key === "site_logo") {
            formattedConfig["Logo"] = backendConfig[key];
          } else if (key === "contact_email") {
            formattedConfig["ContactEmail"] = backendConfig[key];
          } else if (key === "maintenance_mode") {
            formattedConfig["MaintenanceMode"] = backendConfig[key] === "1";
          } else if (key === "footer_text") {
            formattedConfig["FooterText"] = backendConfig[key];
          } else if (key === "icp_record") {
            formattedConfig["IcpRecord"] = backendConfig[key];
          } else if (key === "icp_record_link") {
            formattedConfig["IcpRecordLink"] = backendConfig[key];
          } else if (key === "psb_record") {
            formattedConfig["PsbRecord"] = backendConfig[key];
          } else if (key === "psb_record_link") {
            formattedConfig["PsbRecordLink"] = backendConfig[key];
          }
        }

        $config = Object.assign($config, formattedConfig);
        app.config.globalProperties.$config = $config;
        // 设置全局配置
        setConfig($config);
      }
      return $config;
    })
    .catch(error => {
      // 检查是否是因为未初始化导致的403
      if (
        error.response &&
        error.response.status === 403 &&
        error.response.data?.msg?.includes("未初始化")
      ) {
        // 重定向到安装页面
        if (!window.location.hash.includes("/install")) {
          window.location.hash = "/install";
        }
        return app.config.globalProperties.$config;
      }
      throw "获取全局配置失败，请检查后端接口或数据库配置";
    });
};

/** 本地响应式存储的命名空间 */
const responsiveStorageNameSpace = () => getConfig().ResponsiveStorageNameSpace;

export { getConfig, setConfig, responsiveStorageNameSpace };
