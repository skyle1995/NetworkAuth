<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { getProfile, updateProfile, updatePassword } from "@/api/admin/profile";
import { useUserStoreHook } from "@/store/modules/user";
import UserAvatar from "~icons/ep/user-filled";

defineOptions({
  name: "Profile"
});

const activeTab = ref("password");

const profileForm = ref({
  current_username: "",
  username: "",
  old_password: ""
});

const nicknameForm = ref({
  current_nickname: "",
  nickname: "",
  current_avatar: "",
  avatar: ""
});

const pwdForm = ref({
  old_password: "",
  new_password: "",
  confirm_password: ""
});

/**
 * 获取个人资料信息
 * @returns {Promise<void>}
 */
const fetchProfile = async () => {
  try {
    const res = await getProfile();
    if (res.code === 0) {
      profileForm.value.current_username = res.data.username;
      profileForm.value.username = res.data.username;
      nicknameForm.value.current_nickname = res.data.nickname;
      nicknameForm.value.nickname = res.data.nickname;
      nicknameForm.value.current_avatar = res.data.avatar;
      nicknameForm.value.avatar = res.data.avatar;
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "获取个人资料失败");
  }
};

/**
 * 提交修改账号
 * @returns {Promise<void>}
 */
const handleUpdateProfile = async () => {
  if (!profileForm.value.username) {
    ElMessage.warning("新账号不能为空");
    return;
  }
  if (!profileForm.value.old_password) {
    ElMessage.warning("请填写确认密码");
    return;
  }
  try {
    const res = await updateProfile({
      username: profileForm.value.username,
      nickname: nicknameForm.value.current_nickname, // 保持昵称不变
      avatar: nicknameForm.value.current_avatar, // 保持头像不变
      old_password: profileForm.value.old_password
    });
    if (res.code === 0) {
      ElMessage.success("账号更新成功");
      profileForm.value.current_username =
        res.data?.username || profileForm.value.username;
      profileForm.value.old_password = "";
    } else {
      ElMessage.error(res.msg || "账号更新失败");
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "账号更新失败");
  }
};

/**
 * 提交修改资料
 * @returns {Promise<void>}
 */
const handleUpdateNickname = async () => {
  if (!nicknameForm.value.nickname) {
    ElMessage.warning("新昵称不能为空");
    return;
  }
  try {
    const res = await updateProfile({
      username: profileForm.value.current_username, // 保持账号不变
      nickname: nicknameForm.value.nickname,
      avatar: nicknameForm.value.avatar,
      old_password: "" // 修改资料不需要密码
    });
    if (res.code === 0) {
      ElMessage.success("资料更新成功");
      nicknameForm.value.current_nickname =
        res.data?.nickname || nicknameForm.value.nickname;
      nicknameForm.value.current_avatar =
        res.data?.avatar || nicknameForm.value.avatar;

      if (res.data?.nickname) {
        useUserStoreHook().SET_NICKNAME(res.data.nickname);
      }
      if (res.data?.avatar !== undefined) {
        useUserStoreHook().SET_AVATAR(res.data.avatar);
      }
    } else {
      ElMessage.error(res.msg || "资料更新失败");
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "资料更新失败");
  }
};

/**
 * 提交修改密码
 * @returns {Promise<void>}
 */
const handleUpdatePassword = async () => {
  if (
    !pwdForm.value.old_password ||
    !pwdForm.value.new_password ||
    !pwdForm.value.confirm_password
  ) {
    ElMessage.warning("请填写完整密码信息");
    return;
  }
  if (pwdForm.value.new_password !== pwdForm.value.confirm_password) {
    ElMessage.warning("两次新密码不一致");
    return;
  }
  try {
    const res = await updatePassword(pwdForm.value);
    if (res.code === 0) {
      ElMessage.success("密码修改成功，请重新登录");
      useUserStoreHook().logOut();
    } else {
      ElMessage.error(res.msg || "修改密码失败");
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "修改密码失败");
  }
};

onMounted(() => {
  fetchProfile();
});
</script>

<template>
  <div>
    <el-card shadow="never">
      <template #header>
        <span class="font-bold">个人资料</span>
      </template>

      <el-tabs v-model="activeTab" class="mt-2">
        <!-- 修改密码 -->
        <el-tab-pane label="修改密码" name="password">
          <el-form
            :model="pwdForm"
            label-width="100px"
            class="max-w-[500px] mt-4"
            @submit.prevent
          >
            <el-form-item label="当前密码" required>
              <el-input
                v-model="pwdForm.old_password"
                type="password"
                show-password
                placeholder="请输入当前密码"
              />
            </el-form-item>
            <el-form-item label="新的密码" required>
              <el-input
                v-model="pwdForm.new_password"
                type="password"
                show-password
                placeholder="请输入新的密码"
              />
            </el-form-item>
            <el-form-item label="确认密码" required>
              <el-input
                v-model="pwdForm.confirm_password"
                type="password"
                show-password
                placeholder="请确认新的密码"
              />
            </el-form-item>
            <el-form-item class="mt-6">
              <el-button type="primary" @click="handleUpdatePassword"
                >修改密码</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 修改账号 -->
        <el-tab-pane label="修改账号" name="profile">
          <el-form
            :model="profileForm"
            label-width="100px"
            class="max-w-[500px] mt-4"
            @submit.prevent
          >
            <el-form-item label="当前账号" required>
              <el-input
                v-model="profileForm.current_username"
                disabled
                placeholder="当前账号"
              />
            </el-form-item>
            <el-form-item label="新的账号" required>
              <el-input
                v-model="profileForm.username"
                placeholder="请输入新的账号"
              />
            </el-form-item>
            <el-form-item label="确认密码" required>
              <el-input
                v-model="profileForm.old_password"
                type="password"
                show-password
                placeholder="修改账号需验证当前密码"
              />
            </el-form-item>
            <el-form-item class="mt-6">
              <el-button type="primary" @click="handleUpdateProfile"
                >保存账号</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 修改资料 -->
        <el-tab-pane label="修改资料" name="nickname">
          <el-form
            :model="nicknameForm"
            label-width="100px"
            class="max-w-[500px] mt-4"
            @submit.prevent
          >
            <el-form-item label="当前头像">
              <el-avatar
                v-if="nicknameForm.current_avatar"
                :src="nicknameForm.current_avatar"
                :size="60"
              />
              <el-avatar v-else :size="60">
                <IconifyIconOffline :icon="UserAvatar" />
              </el-avatar>
            </el-form-item>

            <el-form-item label="头像地址">
              <el-input
                v-model="nicknameForm.avatar"
                placeholder="请输入头像 URL 地址"
                clearable
              />
            </el-form-item>

            <el-form-item label="当前昵称">
              <el-input
                v-model="nicknameForm.current_nickname"
                disabled
                placeholder=""
              />
            </el-form-item>

            <el-form-item label="新的昵称" required>
              <el-input
                v-model="nicknameForm.nickname"
                placeholder="请输入新的昵称"
              />
            </el-form-item>
            <el-form-item class="mt-6">
              <el-button type="primary" @click="handleUpdateNickname"
                >保存资料</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>
