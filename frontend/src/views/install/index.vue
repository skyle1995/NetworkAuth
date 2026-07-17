<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import { submitInstall } from "@/api/install/index";
import { http } from "@/utils/http";

defineOptions({
  name: "Install"
});

const router = useRouter();
const formRef = ref<FormInstance>();
const loading = ref(false);
const checking = ref(true);

const form = reactive({
  db_type: "sqlite",
  db_host: "localhost",
  db_port: 3306,
  db_name: "networkauth",
  db_user: "",
  db_pass: "",
  redis_enabled: false,
  redis_host: "localhost",
  redis_port: 6379,
  redis_password: "",
  redis_db: 0,
  site_title: "NetworkAuth",
  admin_username: "admin",
  admin_password: "",
  confirm_password: ""
});

const rules = reactive<FormRules>({
  db_host: [{ required: true, message: "请输入主机地址", trigger: "blur" }],
  db_port: [{ required: true, message: "请输入端口号", trigger: "blur" }],
  db_name: [{ required: true, message: "请输入数据库名", trigger: "blur" }],
  db_user: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  db_pass: [{ required: true, message: "请输入密码", trigger: "blur" }],
  site_title: [{ required: true, message: "请输入站点标题", trigger: "blur" }],
  admin_username: [
    { required: true, message: "请输入管理员账号", trigger: "blur" }
  ],
  admin_password: [
    { required: true, message: "请输入管理员密码", trigger: "blur" },
    { min: 6, message: "密码至少6位", trigger: "blur" }
  ],
  confirm_password: [
    { required: true, message: "请再次输入密码", trigger: "blur" },
    {
      validator: (rule, value, callback) => {
        if (value !== form.admin_password) {
          callback(new Error("两次输入的密码不一致"));
        } else {
          callback();
        }
      },
      trigger: "blur"
    }
  ]
});

// 检查是否已经安装
const checkInstallStatus = async () => {
  try {
    checking.value = true;
    // 请求任意一个非安装接口，如果未安装会被拦截并返回 403 未初始化
    // 如果已安装，会正常返回
    const res = await http.request("get", "/api/home/settings/public");
    if (res) {
      const redirect = sessionStorage.getItem("install_redirect") || "/home";
      sessionStorage.removeItem("install_redirect");
      ElMessage.warning("系统已安装，即将跳转");
      router.push(redirect);
    }
  } catch (error) {
    // 忽略错误，继续显示安装页面
  } finally {
    checking.value = false;
  }
};

onMounted(() => {
  checkInstallStatus();
});

const handleInstall = async () => {
  if (!formRef.value) return;
  await formRef.value.validate(async valid => {
    if (valid) {
      try {
        loading.value = true;
        const res = await submitInstall({
          ...form,
          db_port: Number(form.db_port),
          redis_port: Number(form.redis_port),
          redis_db: Number(form.redis_db)
        });
        if (res.code === 0) {
          ElMessage.success("系统初始化成功！");
          ElMessageBox.confirm("安装成功！请选择接下来的操作", "安装完成", {
            confirmButtonText: "前往后台登录",
            cancelButtonText: "返回首页",
            type: "success",
            center: true,
            closeOnClickModal: false,
            closeOnPressEscape: false,
            showClose: false
          })
            .then(() => {
              sessionStorage.setItem("install_redirect", "/admin/login");
              window.location.reload();
            })
            .catch(() => {
              sessionStorage.setItem("install_redirect", "/home");
              window.location.reload();
            });
        } else {
          ElMessage.error(res.msg || "初始化失败");
        }
      } catch (error: any) {
        ElMessage.error(error?.response?.data?.msg || "初始化请求失败");
      } finally {
        loading.value = false;
      }
    }
  });
};
</script>

<template>
  <div v-loading.fullscreen.lock="checking" class="install-container">
    <el-card class="install-card" shadow="always">
      <template #header>
        <div class="card-header">
          <h2>系统初始化</h2>
          <p>欢迎使用，请完成以下初始化设置</p>
        </div>
      </template>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
        label-position="right"
      >
        <div class="section-title">1. 数据库配置</div>
        <el-form-item label="数据库类型" prop="db_type">
          <el-radio-group v-model="form.db_type">
            <el-radio label="sqlite">SQLite (默认)</el-radio>
            <el-radio label="mysql">MySQL</el-radio>
          </el-radio-group>
        </el-form-item>

        <template v-if="form.db_type === 'mysql'">
          <el-form-item label="主机地址" prop="db_host">
            <el-input v-model="form.db_host" placeholder="localhost" />
          </el-form-item>
          <el-form-item label="端口号" prop="db_port">
            <el-input-number
              v-model="form.db_port"
              :min="1"
              :max="65535"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="数据库名" prop="db_name">
            <el-input v-model="form.db_name" placeholder="demo" />
          </el-form-item>
          <el-form-item label="用户名" prop="db_user">
            <el-input v-model="form.db_user" placeholder="root" />
          </el-form-item>
          <el-form-item label="密码" prop="db_pass">
            <el-input
              v-model="form.db_pass"
              type="password"
              show-password
              placeholder="请输入数据库密码"
            />
          </el-form-item>
        </template>

        <div class="section-title mt-6">2. Redis 配置（可选）</div>
        <el-form-item label="启用 Redis">
          <el-switch v-model="form.redis_enabled" />
          <span class="redis-tip"
            >启用后用于设置缓存、会话等场景；未启用时系统以内存缓存运行</span
          >
        </el-form-item>
        <template v-if="form.redis_enabled">
          <el-form-item label="主机地址" prop="redis_host">
            <el-input v-model="form.redis_host" placeholder="localhost" />
          </el-form-item>
          <el-form-item label="端口号" prop="redis_port">
            <el-input-number
              v-model="form.redis_port"
              :min="1"
              :max="65535"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="form.redis_password"
              type="password"
              show-password
              placeholder="无密码可留空"
            />
          </el-form-item>
          <el-form-item label="数据库索引" prop="redis_db">
            <el-input-number
              v-model="form.redis_db"
              :min="0"
              :max="15"
              controls-position="right"
            />
          </el-form-item>
        </template>

        <div class="section-title mt-6">3. 站点信息</div>
        <el-form-item label="站点标题" prop="site_title">
          <el-input v-model="form.site_title" placeholder="NetworkAuth" />
        </el-form-item>

        <div class="section-title mt-6">4. 管理员设置</div>
        <el-form-item label="管理员账号" prop="admin_username">
          <el-input v-model="form.admin_username" placeholder="admin" />
        </el-form-item>
        <el-form-item label="管理员密码" prop="admin_password">
          <el-input
            v-model="form.admin_password"
            type="password"
            show-password
            placeholder="设置管理员密码（至少6位）"
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input
            v-model="form.confirm_password"
            type="password"
            show-password
            placeholder="请再次输入管理员密码"
          />
        </el-form-item>

        <div class="text-center mt-8">
          <el-button
            type="primary"
            size="large"
            class="w-[200px]"
            :loading="loading"
            @click="handleInstall"
          >
            立即初始化
          </el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.install-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 40px 0;
  background-color: #f0f2f5;
}

.install-card {
  width: 650px;
  border-radius: 8px;

  .card-header {
    text-align: center;

    h2 {
      margin: 0;
      font-size: 24px;
      font-weight: 500;
      color: #303133;
    }

    p {
      margin: 10px 0 0;
      font-size: 14px;
      color: #909399;
    }
  }

  .section-title {
    padding-bottom: 10px;
    margin-bottom: 20px;
    font-size: 15px;
    font-weight: bold;
    color: #303133;
    border-bottom: 1px solid #ebeef5;
  }

  .redis-tip {
    margin-left: 12px;
    font-size: 12px;
    color: #909399;
  }
}
</style>
