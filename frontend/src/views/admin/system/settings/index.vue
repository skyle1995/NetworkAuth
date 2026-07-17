<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getSettings,
  updateSettings,
  generateKey,
  testMail
} from "@/api/admin/settings";
import { useUserStoreHook } from "@/store/modules/user";

defineOptions({
  name: "Settings"
});

const activeTab = ref("basic");

// 仅超级管理员可读写安全关键键（jwt_secret/encryption_key）；普通管理员只读+部分掩码，相关控件禁用
const isSuperAdmin = computed(() =>
  useUserStoreHook().roles.includes("super_admin")
);

const form = ref<Record<string, any>>({
  // 基本信息
  site_title: "",
  site_description: "",
  site_keywords: "",
  site_logo: "",
  contact_email: "",

  // 系统和安全
  maintenance_mode: "0",
  captcha_type: "slide",
  encryption_key: "",
  jwt_secret: "",
  jwt_expire: 2,
  refresh_token_expire_days: 7,
  session_absolute_expire_days: 30,
  refresh_advance_seconds: 60,
  max_upload_size: 10,
  max_upload_size_unit: "MB",

  // 邮件服务 (SMTP)
  smtp_enabled: "0",
  smtp_host: "",
  smtp_port: 465,
  smtp_ssl: "1",
  smtp_username: "",
  smtp_password: "",
  smtp_from: "",
  smtp_from_name: "NetworkAuth",
  smtp_code_cooldown: 60,

  // IP 地区库
  ip_region_provider: "ip2region",
  ip2region_db: "data/ip2region.xdb",
  ip2location_db: "data/IP2LOCATION-LITE.BIN",

  // 日志清理
  login_log_cleanup_days: 30,
  login_log_cleanup_limit: 10000,
  operation_log_cleanup_days: 30,
  operation_log_cleanup_limit: 10000,
  refresh_token_cleanup_days: 7,

  // Cookie
  cookie_secure: "true",
  cookie_same_site: "Lax",
  cookie_domain: "",
  cookie_max_age: 86400,

  // 页脚与备案
  footer_text: "",
  icp_record: "",
  icp_record_link: "",
  psb_record: "",
  psb_record_link: "",

  // 模板设置
  platform_fixed_header: "1",
  platform_hidden_side_bar: "0",
  platform_multi_tags_cache: "0",
  platform_keep_alive: "1",
  platform_layout: "vertical",
  platform_theme: "light",
  platform_dark_mode: "0",
  platform_overall_style: "light",
  platform_grey: "0",
  platform_weak: "0",
  platform_hide_tabs: "0",
  platform_hide_footer: "0",
  platform_stretch: "0",
  platform_sidebar_status: "1",
  platform_ep_theme_color: "#409EFF",
  platform_show_logo: "1",
  platform_show_model: "smart",
  platform_menu_arrow_icon_no_transition: "0",
  platform_caching_async_routes: "0",
  platform_tooltip_effect: "light",
  platform_responsive_storage_name_space: "responsive-",
  platform_menu_search_history: 6
});

const numericFields = [
  "jwt_expire",
  "refresh_token_expire_days",
  "session_absolute_expire_days",
  "refresh_advance_seconds",
  "max_upload_size",
  "login_log_cleanup_days",
  "login_log_cleanup_limit",
  "operation_log_cleanup_days",
  "operation_log_cleanup_limit",
  "refresh_token_cleanup_days",
  "cookie_max_age",
  "platform_menu_search_history",
  "smtp_port",
  "smtp_code_cooldown"
];

/**
 * 获取系统设置数据
 * @returns {Promise<void>}
 */
const fetchSettings = async () => {
  try {
    const res = await getSettings();
    if (res.code === 0) {
      for (const key in res.data) {
        if (
          Object.prototype.hasOwnProperty.call(form.value, key) ||
          res.data[key] !== undefined
        ) {
          if (numericFields.includes(key) && res.data[key]) {
            form.value[key] = Number(res.data[key]);
          } else {
            form.value[key] = res.data[key];
          }
        }
      }
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "获取系统设置失败");
  }
};

/**
 * 保存模块设置数据
 * @param {string[]} keys - 要保存的字段名数组
 * @returns {Promise<void>}
 */
const handleSave = async (
  keys: string[],
  typeKey: string = activeTab.value
) => {
  try {
    const submitData: Record<string, string> = {};
    for (const key of keys) {
      submitData[key] = String(form.value[key] || "");
    }
    // 提交数据并带上分类信息，用于后台操作日志记录
    const res = await updateSettings({
      ...submitData,
      category:
        typeKey === "basic"
          ? "站点基本信息"
          : typeKey === "security"
            ? "系统和安全设置"
            : typeKey === "cookie"
              ? "Cookie安全设置"
              : typeKey === "log_cleanup"
                ? "日志清理设置"
                : typeKey === "template"
                  ? "模板设置"
                  : "其他设置"
    });
    if (res.code === 0) {
      ElMessage.success("保存成功");
    } else {
      ElMessage.error(res.msg || "保存失败");
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "保存失败");
  }
};

/**
 * 随机生成指定类型的安全密钥
 * @param {string} type - 密钥类型，例如 "jwt" 或 "encryption"
 * @returns {Promise<void>}
 */
const handleGenerateKey = async (type: string) => {
  try {
    const res = await generateKey(type);
    if (res.code === 0 && res.data?.key) {
      if (type === "jwt") form.value.jwt_secret = res.data.key;
      if (type === "encryption") form.value.encryption_key = res.data.key;
      ElMessage.success("生成成功，请保存生效");
    } else {
      ElMessage.error(res.msg || "生成失败");
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "生成失败");
  }
};

/** 发送测试邮件，验证 SMTP 配置 */
const handleTestMail = async () => {
  try {
    const { value } = await ElMessageBox.prompt(
      "请输入接收测试邮件的邮箱地址（请先保存 SMTP 配置）",
      "发送测试邮件",
      {
        confirmButtonText: "发送",
        cancelButtonText: "取消",
        inputPlaceholder: "you@example.com",
        inputValidator: (v: string) =>
          /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? true : "邮箱格式不正确"
      }
    );
    const res = await testMail({ to: value.trim() });
    if (res.code === 0) {
      ElMessage.success("测试邮件已发送，请查收");
    } else {
      ElMessage.error(res.msg || "发送失败");
    }
  } catch (e) {
    // cancelled
  }
};

onMounted(() => {
  fetchSettings();
});
</script>

<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <span class="font-bold">系统设置</span>
      </template>

      <el-tabs v-model="activeTab" class="mt-2">
        <!-- 站点基本信息 -->
        <el-tab-pane label="基本信息" name="basic">
          <el-form
            :model="form"
            label-width="120px"
            label-position="right"
            class="max-w-[700px] mt-4"
          >
            <el-form-item label="站点标题">
              <el-input
                v-model="form.site_title"
                placeholder="请输入站点标题"
              />
            </el-form-item>
            <el-form-item label="站点描述">
              <el-input
                v-model="form.site_description"
                type="textarea"
                :rows="3"
                placeholder="请输入站点描述"
              />
            </el-form-item>
            <el-form-item label="站点关键词">
              <el-input
                v-model="form.site_keywords"
                placeholder="请输入站点关键词"
              />
              <div class="text-gray-400 text-xs mt-1">多个关键词用逗号分隔</div>
            </el-form-item>
            <el-form-item label="网站 Logo">
              <el-input
                v-model="form.site_logo"
                placeholder="请输入 Logo 路径"
              />
            </el-form-item>
            <el-form-item label="联系邮箱">
              <el-input
                v-model="form.contact_email"
                placeholder="请输入联系邮箱"
              />
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'site_title',
                    'site_description',
                    'site_keywords',
                    'site_logo',
                    'contact_email'
                  ])
                "
                >保存基本信息</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 邮件服务 (SMTP) -->
        <el-tab-pane label="邮件服务" name="smtp">
          <el-form :model="form" label-width="140px">
            <el-alert
              class="mb-4"
              type="info"
              :closable="false"
              show-icon
              title="用于「账号注册邮箱验证」发送验证码。开启邮箱验证的应用依赖此处配置。"
            />
            <el-form-item label="启用邮件服务">
              <el-radio-group v-model="form.smtp_enabled">
                <el-radio value="0">关闭</el-radio>
                <el-radio value="1">开启</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="SMTP 服务器">
              <el-input v-model="form.smtp_host" placeholder="如 smtp.qq.com" />
            </el-form-item>
            <el-form-item label="端口">
              <el-input-number v-model="form.smtp_port" :min="1" :max="65535" />
              <span class="ml-2 text-xs text-gray-400"
                >465=SSL，587=STARTTLS，25=明文</span
              >
            </el-form-item>
            <el-form-item label="使用 SSL">
              <el-radio-group v-model="form.smtp_ssl">
                <el-radio value="1">是 (465)</el-radio>
                <el-radio value="0">否 (587/25)</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="登录账号">
              <el-input
                v-model="form.smtp_username"
                placeholder="SMTP 登录邮箱账号"
              />
            </el-form-item>
            <el-form-item label="登录密码">
              <el-input
                v-model="form.smtp_password"
                type="password"
                show-password
                :disabled="!isSuperAdmin"
                placeholder="密码/授权码（仅超级管理员可读写）"
              />
            </el-form-item>
            <el-form-item label="发件人邮箱">
              <el-input
                v-model="form.smtp_from"
                placeholder="留空则用登录账号"
              />
            </el-form-item>
            <el-form-item label="发件人名称">
              <el-input
                v-model="form.smtp_from_name"
                placeholder="如 NetworkAuth"
              />
            </el-form-item>
            <el-form-item label="发送限流">
              <el-input-number
                v-model="form.smtp_code_cooldown"
                :min="0"
                :max="3600"
                :step="10"
              />
              <span
                class="ml-2 text-xs"
                style="color: var(--el-text-color-secondary)"
              >
                秒 —— 同一邮箱两次发送验证码的最小间隔
              </span>
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'smtp_enabled',
                    'smtp_host',
                    'smtp_port',
                    'smtp_ssl',
                    'smtp_username',
                    'smtp_password',
                    'smtp_from',
                    'smtp_from_name',
                    'smtp_code_cooldown'
                  ])
                "
                >保存邮件配置</el-button
              >
              <el-button @click="handleTestMail">发送测试邮件</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- IP 地区库 -->
        <el-tab-pane label="IP地区库" name="ipregion">
          <el-form :model="form" label-width="140px">
            <el-alert
              class="mb-4"
              type="info"
              :closable="false"
              show-icon
              title="用于市级/省级 IP 验证。库缺失或关闭时，IP 验证自动退回精确 IP 匹配。保存后即时热重载生效。"
            />
            <el-form-item label="地区库">
              <el-radio-group v-model="form.ip_region_provider">
                <el-radio value="ip2region">ip2region(国内优)</el-radio>
                <el-radio value="ip2location">IP2Location(全球)</el-radio>
                <el-radio value="">关闭</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item
              v-if="form.ip_region_provider === 'ip2region'"
              label="ip2region 路径"
            >
              <el-input
                v-model="form.ip2region_db"
                placeholder="如 data/ip2region.xdb"
              />
            </el-form-item>
            <el-form-item
              v-if="form.ip_region_provider === 'ip2location'"
              label="IP2Location 路径"
            >
              <el-input
                v-model="form.ip2location_db"
                placeholder="如 data/IP2LOCATION-LITE.BIN（LITE 库需从官网下载）"
              />
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'ip_region_provider',
                    'ip2region_db',
                    'ip2location_db'
                  ])
                "
                >保存地区库配置</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 系统和安全设置 -->
        <el-tab-pane label="系统和安全" name="security">
          <el-form
            :model="form"
            label-width="180px"
            label-position="right"
            class="max-w-[760px] mt-4"
          >
            <el-divider content-position="left">运行模式</el-divider>
            <el-form-item label="维护模式">
              <el-switch
                v-model="form.maintenance_mode"
                :active-value="'1'"
                :inactive-value="'0'"
                active-text="开启"
                inactive-text="关闭"
                inline-prompt
              />
              <div class="text-gray-400 text-xs mt-1 w-full">
                开启后，前端将展示维护页面（如果支持）
              </div>
            </el-form-item>

            <el-form-item label="验证码类型">
              <el-radio-group v-model="form.captcha_type">
                <el-radio value="slide">滑动拼图</el-radio>
                <el-radio value="click">点击文字</el-radio>
                <el-radio value="image">字符验证码</el-radio>
              </el-radio-group>
              <div class="text-gray-400 text-xs mt-1 w-full">
                登录页验证码形式；滑动拼图/点击文字体验更好，字符验证码更轻量
              </div>
            </el-form-item>

            <el-divider content-position="left">密钥配置</el-divider>
            <el-form-item label="数据加密密钥">
              <div class="flex w-full gap-2">
                <el-input
                  v-model="form.encryption_key"
                  :disabled="!isSuperAdmin"
                  placeholder="请输入数据加密密钥"
                />
                <el-button
                  :disabled="!isSuperAdmin"
                  @click="handleGenerateKey('encryption')"
                  >随机生成</el-button
                >
              </div>
            </el-form-item>

            <el-form-item label="JWT 密钥">
              <div class="flex w-full gap-2">
                <el-input
                  v-model="form.jwt_secret"
                  :disabled="!isSuperAdmin"
                  placeholder="请输入 JWT 密钥"
                />
                <el-button
                  :disabled="!isSuperAdmin"
                  @click="handleGenerateKey('jwt')"
                  >随机生成</el-button
                >
              </div>
            </el-form-item>

            <el-divider content-position="left">会话与令牌</el-divider>
            <el-form-item label="认证令牌有效期">
              <el-input-number v-model="form.jwt_expire" :min="1" :max="720" />
              <div class="text-gray-400 text-xs ml-3">
                单位：小时；建议 1~2，最大 720（30 天）
              </div>
            </el-form-item>
            <el-form-item label="刷新令牌有效期">
              <el-input-number
                v-model="form.refresh_token_expire_days"
                :min="1"
                :max="365"
              />
              <div class="text-gray-400 text-xs ml-3">
                单位：天；滑动续期，每次刷新重新计算，最大 365
              </div>
            </el-form-item>
            <el-form-item label="会话绝对上限">
              <el-input-number
                v-model="form.session_absolute_expire_days"
                :min="1"
              />
              <div class="text-gray-400 text-xs ml-3">
                单位：天；超过必须重新登录，不可被滑动续期突破
              </div>
            </el-form-item>
            <el-form-item label="提前刷新阈值">
              <el-input-number
                v-model="form.refresh_advance_seconds"
                :min="0"
                :step="30"
              />
              <div class="text-gray-400 text-xs ml-3">
                单位：秒；认证令牌过期前多少秒触发自动刷新
              </div>
            </el-form-item>

            <el-divider content-position="left">其他</el-divider>
            <el-form-item label="文件上传最大尺寸">
              <el-input-number v-model="form.max_upload_size" :min="1" />
            </el-form-item>

            <el-form-item label="文件上传大小单位">
              <el-select v-model="form.max_upload_size_unit">
                <el-option label="B (字节)" value="B" />
                <el-option label="KB (千字节)" value="KB" />
                <el-option label="MB (兆字节)" value="MB" />
                <el-option label="GB (吉字节)" value="GB" />
              </el-select>
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'maintenance_mode',
                    'captcha_type',
                    'encryption_key',
                    'jwt_secret',
                    'jwt_expire',
                    'refresh_token_expire_days',
                    'session_absolute_expire_days',
                    'refresh_advance_seconds',
                    'max_upload_size',
                    'max_upload_size_unit'
                  ])
                "
                >保存安全设置</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 日志清理策略 -->
        <el-tab-pane label="日志清理" name="log_cleanup">
          <el-form
            :model="form"
            label-width="120px"
            label-position="right"
            class="max-w-[700px] mt-4"
          >
            <el-divider content-position="left">登录日志</el-divider>
            <el-form-item label="保留天数">
              <el-input-number v-model="form.login_log_cleanup_days" :min="0" />
              <div class="text-gray-400 text-xs ml-3">0 表示不按天清理</div>
            </el-form-item>
            <el-form-item label="保留条数">
              <el-input-number
                v-model="form.login_log_cleanup_limit"
                :min="0"
                :step="1000"
              />
              <div class="text-gray-400 text-xs ml-3">0 表示不按数量清理</div>
            </el-form-item>

            <el-divider content-position="left">操作日志</el-divider>
            <el-form-item label="保留天数">
              <el-input-number
                v-model="form.operation_log_cleanup_days"
                :min="0"
              />
              <div class="text-gray-400 text-xs ml-3">0 表示不按天清理</div>
            </el-form-item>
            <el-form-item label="保留条数">
              <el-input-number
                v-model="form.operation_log_cleanup_limit"
                :min="0"
                :step="1000"
              />
              <div class="text-gray-400 text-xs ml-3">0 表示不按数量清理</div>
            </el-form-item>

            <el-divider content-position="left">刷新令牌</el-divider>
            <el-form-item label="保留天数">
              <el-input-number
                v-model="form.refresh_token_cleanup_days"
                :min="0"
              />
              <div class="text-gray-400 text-xs ml-3">
                过期后保留天数，0 表示不自动清理
              </div>
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'login_log_cleanup_days',
                    'login_log_cleanup_limit',
                    'operation_log_cleanup_days',
                    'operation_log_cleanup_limit',
                    'refresh_token_cleanup_days'
                  ])
                "
                >保存日志清理策略</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Cookie设置 -->
        <el-tab-pane label="Cookie 设置" name="cookie">
          <el-form
            :model="form"
            label-width="150px"
            label-position="right"
            class="max-w-[700px] mt-4"
          >
            <el-form-item label="Secure 属性">
              <el-switch
                v-model="form.cookie_secure"
                :active-value="'true'"
                :inactive-value="'false'"
                active-text="开启"
                inactive-text="关闭"
                inline-prompt
              />
              <div class="text-gray-400 text-xs mt-1 w-full">
                仅 HTTPS 请求下支持
              </div>
            </el-form-item>
            <el-form-item label="SameSite 属性">
              <el-select
                v-model="form.cookie_same_site"
                placeholder="请选择 SameSite"
              >
                <el-option label="Strict" value="Strict" />
                <el-option label="Lax" value="Lax" />
                <el-option label="None" value="None" />
              </el-select>
            </el-form-item>
            <el-form-item label="Cookie 域名">
              <el-input
                v-model="form.cookie_domain"
                placeholder="留空则使用当前域名"
              />
            </el-form-item>
            <el-form-item label="最大存活时间(秒)">
              <el-input-number
                v-model="form.cookie_max_age"
                :min="0"
                :step="3600"
              />
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'cookie_secure',
                    'cookie_same_site',
                    'cookie_domain',
                    'cookie_max_age'
                  ])
                "
                >保存 Cookie 设置</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 页脚与备案 -->
        <el-tab-pane label="页脚与备案" name="footer">
          <el-form
            :model="form"
            label-width="120px"
            label-position="right"
            class="max-w-[700px] mt-4"
          >
            <el-form-item label="页脚版权信息">
              <el-input
                v-model="form.footer_text"
                placeholder="请输入版权信息"
              />
            </el-form-item>
            <el-form-item label="ICP 备案号">
              <el-input
                v-model="form.icp_record"
                placeholder="例如：京ICP备00000000号-1"
              />
            </el-form-item>
            <el-form-item label="ICP 备案链接">
              <el-input
                v-model="form.icp_record_link"
                placeholder="例如：https://beian.miit.gov.cn"
              />
            </el-form-item>
            <el-form-item label="公安备案号">
              <el-input
                v-model="form.psb_record"
                placeholder="例如：京公网安备 00000000000000号"
              />
            </el-form-item>
            <el-form-item label="公安备案链接">
              <el-input
                v-model="form.psb_record_link"
                placeholder="例如：http://www.beian.gov.cn"
              />
            </el-form-item>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'footer_text',
                    'icp_record',
                    'icp_record_link',
                    'psb_record',
                    'psb_record_link'
                  ])
                "
                >保存页脚与备案</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 模板设置 -->
        <el-tab-pane label="模板设置" name="template">
          <el-form
            :model="form"
            label-width="150px"
            label-position="right"
            class="max-w-[800px] mt-4"
          >
            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="布局模式">
                  <el-select v-model="form.platform_layout">
                    <el-option label="垂直布局" value="vertical" />
                    <el-option label="水平布局" value="horizontal" />
                    <el-option label="混合布局" value="mix" />
                    <el-option label="综合布局" value="comprehensive" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="主题配色">
                  <el-select v-model="form.platform_theme">
                    <el-option label="浅色" value="light" />
                    <el-option label="深色" value="dark" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="暗黑模式">
                  <el-switch
                    v-model="form.platform_dark_mode"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="整体风格">
                  <el-select v-model="form.platform_overall_style">
                    <el-option label="浅色" value="light" />
                    <el-option label="深色" value="dark" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="固定页头">
                  <el-switch
                    v-model="form.platform_fixed_header"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="隐藏侧边栏">
                  <el-switch
                    v-model="form.platform_hidden_side_bar"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="多标签页缓存">
                  <el-switch
                    v-model="form.platform_multi_tags_cache"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="组件缓存">
                  <el-switch
                    v-model="form.platform_keep_alive"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="灰色模式">
                  <el-switch
                    v-model="form.platform_grey"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="色弱模式">
                  <el-switch
                    v-model="form.platform_weak"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="隐藏标签页">
                  <el-switch
                    v-model="form.platform_hide_tabs"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="隐藏页脚">
                  <el-switch
                    v-model="form.platform_hide_footer"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="显示Logo">
                  <el-switch
                    v-model="form.platform_show_logo"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="页面宽度拉伸">
                  <el-switch
                    v-model="form.platform_stretch"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="侧边栏状态">
                  <el-switch
                    v-model="form.platform_sidebar_status"
                    active-value="1"
                    inactive-value="0"
                    active-text="展开"
                    inactive-text="折叠"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="取消菜单动画">
                  <el-switch
                    v-model="form.platform_menu_arrow_icon_no_transition"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="主题色 (Element Plus)">
                  <el-color-picker v-model="form.platform_ep_theme_color" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="显示模式">
                  <el-select v-model="form.platform_show_model">
                    <el-option label="智能 (smart)" value="smart" />
                    <el-option label="宽屏 (broad)" value="broad" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="缓存异步路由">
                  <el-switch
                    v-model="form.platform_caching_async_routes"
                    active-value="1"
                    inactive-value="0"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="提示框效果">
                  <el-select v-model="form.platform_tooltip_effect">
                    <el-option label="浅色" value="light" />
                    <el-option label="深色" value="dark" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="20">
              <el-col :xs="24" :sm="12">
                <el-form-item label="响应式存储命名空间">
                  <el-input
                    v-model="form.platform_responsive_storage_name_space"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="菜单搜索历史数量">
                  <el-input-number
                    v-model="form.platform_menu_search_history"
                    :min="0"
                    :max="50"
                  />
                </el-form-item>
              </el-col>
            </el-row>

            <el-form-item class="mt-6">
              <el-button
                type="primary"
                @click="
                  handleSave([
                    'platform_fixed_header',
                    'platform_hidden_side_bar',
                    'platform_multi_tags_cache',
                    'platform_keep_alive',
                    'platform_layout',
                    'platform_theme',
                    'platform_dark_mode',
                    'platform_overall_style',
                    'platform_grey',
                    'platform_weak',
                    'platform_hide_tabs',
                    'platform_hide_footer',
                    'platform_stretch',
                    'platform_sidebar_status',
                    'platform_ep_theme_color',
                    'platform_show_logo',
                    'platform_show_model',
                    'platform_menu_arrow_icon_no_transition',
                    'platform_caching_async_routes',
                    'platform_tooltip_effect',
                    'platform_responsive_storage_name_space',
                    'platform_menu_search_history'
                  ])
                "
                >保存模板设置</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>
