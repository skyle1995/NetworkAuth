<script setup lang="ts">
import Motion from "./utils/motion";
import { useRouter } from "vue-router";
import { message } from "@/utils/message";
import { loginRules } from "./utils/rule";
import { ref, reactive, toRaw } from "vue";
import { debounce } from "@pureadmin/utils";
import { useNav } from "@/layout/hooks/useNav";
import { useEventListener } from "@vueuse/core";
import type { FormInstance } from "element-plus";
import { useLayout } from "@/layout/hooks/useLayout";
import { useUserStoreHook } from "@/store/modules/user";
import { initRouter, getTopMenu } from "@/router/utils";
import { bg, illustration } from "./utils/static";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { useDataThemeChange } from "@/layout/hooks/useDataThemeChange";

import dayIcon from "@/assets/svg/day.svg?component";
import darkIcon from "@/assets/svg/dark.svg?component";
import Lock from "~icons/ri/lock-fill";
import User from "~icons/ri/user-3-fill";

defineOptions({
  name: "Login"
});

const router = useRouter();
const loading = ref(false);
const disabled = ref(false);
const ruleFormRef = ref<FormInstance>();

const { initStorage } = useLayout();
initStorage();

const { dataTheme, overallStyle, dataThemeChange } = useDataThemeChange();
dataThemeChange(overallStyle.value);
const { title } = useNav();

const ruleForm = reactive({
  username: "admin",
  password: "admin123",
  captcha: "",
  csrf_token: ""
});

const captchaUrl = ref("/api/admin/captcha?" + new Date().getTime());
const refreshCaptcha = () => {
  captchaUrl.value = "/api/admin/captcha?" + new Date().getTime();
};

import { getCsrfToken } from "@/api/admin/user";
getCsrfToken().then(res => {
  if (res.success || res.code === 0 || res.code === 200) {
    ruleForm.csrf_token = res.data.csrf_token || res.data;
  }
});

const onLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  if (loading.value) return;
  loading.value = true;
  await formEl.validate(valid => {
    if (valid) {
      useUserStoreHook()
        .loginByUsername({
          username: ruleForm.username,
          password: ruleForm.password,
          captcha: ruleForm.captcha,
          csrf_token: ruleForm.csrf_token
        })
        .then(res => {
          if (res.success) {
            // 获取后端路由
            return initRouter().then(() => {
              disabled.value = true;
              router
                .push(getTopMenu(true).path)
                .then(() => {
                  message("登录成功", { type: "success" });
                })
                .finally(() => (disabled.value = false));
            });
          } else {
            message("登录失败", { type: "error" });
          }
        })
        .catch(err => {
          message(err.message || "登录异常", { type: "error" });
          refreshCaptcha(); // 登录失败后刷新验证码
        })
        .finally(() => (loading.value = false));
    } else {
      loading.value = false;
    }
  });
};

const immediateDebounce: any = debounce(
  formRef => onLogin(formRef),
  1000,
  true
);

useEventListener(document, "keydown", ({ code }) => {
  if (
    ["Enter", "NumpadEnter"].includes(code) &&
    !disabled.value &&
    !loading.value
  )
    immediateDebounce(ruleFormRef.value);
});
</script>

<template>
  <div class="select-none">
    <img :src="bg" class="wave" />
    <div class="flex-c absolute right-5 top-3">
      <!-- 主题 -->
      <el-switch
        v-model="dataTheme"
        inline-prompt
        :active-icon="dayIcon"
        :inactive-icon="darkIcon"
        @change="dataThemeChange"
      />
    </div>
    <div class="login-container">
      <div class="img">
        <component :is="toRaw(illustration)" />
      </div>
      <div class="login-box">
        <div class="login-form">
          <Motion>
            <div class="flex items-center justify-center gap-2 mb-4">
              <!-- <img
                v-if="getConfig()?.Logo"
                :src="getConfig()?.Logo"
                alt="logo"
                class="login-logo"
              />
              <avatar v-else class="avatar !mb-0" /> -->
            </div>
            <h2 class="outline-hidden text-center">{{ title }}</h2>
          </Motion>

          <el-form
            ref="ruleFormRef"
            :model="ruleForm"
            :rules="loginRules"
            size="large"
          >
            <Motion :delay="100">
              <el-form-item
                :rules="[
                  {
                    required: true,
                    message: '请输入账号',
                    trigger: 'blur'
                  }
                ]"
                prop="username"
              >
                <el-input
                  v-model="ruleForm.username"
                  clearable
                  placeholder="账号"
                  :prefix-icon="useRenderIcon(User)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="150">
              <el-form-item prop="password">
                <el-input
                  v-model="ruleForm.password"
                  clearable
                  show-password
                  placeholder="密码"
                  :prefix-icon="useRenderIcon(Lock)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="200">
              <el-form-item
                prop="captcha"
                :rules="[
                  {
                    required: true,
                    message: '请输入验证码',
                    trigger: 'blur'
                  }
                ]"
              >
                <div class="flex w-full justify-between">
                  <el-input
                    v-model="ruleForm.captcha"
                    clearable
                    placeholder="验证码"
                    class="w-[60%]"
                    @keyup.enter="onLogin(ruleFormRef)"
                  />
                  <img
                    :src="captchaUrl"
                    class="w-[35%] h-[40px] cursor-pointer"
                    alt="captcha"
                    @click="refreshCaptcha"
                  />
                </div>
              </el-form-item>
            </Motion>

            <Motion :delay="250">
              <el-button
                class="w-full mt-4!"
                size="default"
                type="primary"
                :loading="loading"
                :disabled="disabled"
                @click="onLogin(ruleFormRef)"
              >
                登录
              </el-button>
            </Motion>
          </el-form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import url("@/style/login.css");
</style>

<style lang="scss" scoped>
:deep(.el-input-group__append, .el-input-group__prepend) {
  padding: 0;
}

.login-logo {
  max-width: 100%;
  height: 60px;
  object-fit: contain;
}
</style>
