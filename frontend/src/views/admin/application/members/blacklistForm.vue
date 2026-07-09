<script setup lang="ts">
import { ref } from "vue";

export interface FormProps {
  formInline: {
    username: string;
    blacklist_device: boolean;
    blacklist_ip: boolean;
    blacklist_region: boolean;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    username: "",
    blacklist_device: false,
    blacklist_ip: false,
    blacklist_region: false
  })
});

const newFormInline = ref(props.formInline);
</script>

<template>
  <div class="py-1">
    <el-alert
      :closable="false"
      type="warning"
      show-icon
      class="mb-3"
      :title="`即将拉黑账号「${newFormInline.username}」`"
      description="拉黑后该账号立即掉线且无法登录。可额外勾选下列维度，把它绑定的设备/IP/地区一并加入黑名单——命中的设备或网络将无法用任何账号登录本应用。"
    />
    <el-checkbox v-model="newFormInline.blacklist_device">
      同时拉黑该账号的<b>设备(机器码)</b>
    </el-checkbox>
    <br />
    <el-checkbox v-model="newFormInline.blacklist_ip">
      同时拉黑该账号的<b>IP地址</b>
    </el-checkbox>
    <br />
    <el-checkbox v-model="newFormInline.blacklist_region">
      同时拉黑该账号IP所属<b>地区(省/市)</b>
    </el-checkbox>
    <p class="mt-2 text-xs" style="color: var(--el-text-color-secondary)">
      地区按「地级市」粒度封禁，会拦截该省该市的所有登录，范围较大请谨慎勾选。
    </p>
  </div>
</template>
