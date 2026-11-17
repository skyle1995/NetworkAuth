<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  deleteAppVersion,
  listAppVersions,
  saveAppVersion,
  type AppVersionRow
} from "@/api/admin/appVersion";

const props = defineProps<{ app: { uuid: string; name?: string } }>();

const loading = ref(false);
const saving = ref(false);
const rows = ref<AppVersionRow[]>([]);

const form = reactive({
  id: 0,
  version: "",
  download_type: 2,
  download_url: "",
  changelog: "",
  enabled: true
});

const dialogTitle = computed(() =>
  form.id ? `编辑版本 ${form.version}` : "新增版本"
);

const typeText = (value: number) =>
  value === 1 ? "强制更新" : value === 2 ? "自由更新" : "不启用";

const typeTag = (value: number) =>
  value === 1 ? "danger" : value === 2 ? "success" : "info";

async function load() {
  loading.value = true;
  try {
    const res = await listAppVersions({ app_uuid: props.app.uuid });
    if (res.code === 0) rows.value = res.data?.list ?? [];
    else ElMessage.error(res.msg || "读取版本列表失败");
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  form.id = 0;
  form.version = "";
  form.download_type = 2;
  form.download_url = "";
  form.changelog = "";
  form.enabled = true;
}

function editRow(row: any) {
  form.id = row.id;
  form.version = row.version;
  form.download_type = row.download_type;
  form.download_url = row.download_url;
  form.changelog = row.changelog;
  form.enabled = row.enabled;
}

async function submit() {
  if (!form.version.trim()) {
    ElMessage.warning("请填写版本号");
    return;
  }
  saving.value = true;
  try {
    const res = await saveAppVersion({
      id: form.id,
      app_uuid: props.app.uuid,
      version: form.version.trim(),
      download_type: form.download_type,
      download_url: form.download_url.trim(),
      changelog: form.changelog,
      enabled: form.enabled
    });
    if (res.code === 0) {
      ElMessage.success(form.id ? "已更新" : "已新增");
      resetForm();
      await load();
    } else {
      ElMessage.error(res.msg || "保存失败");
    }
  } finally {
    saving.value = false;
  }
}

async function toggle(row: any) {
  const res = await saveAppVersion({
    id: row.id,
    app_uuid: props.app.uuid,
    version: row.version,
    download_type: row.download_type,
    download_url: row.download_url,
    changelog: row.changelog,
    enabled: row.enabled
  });
  if (res.code !== 0) {
    row.enabled = !row.enabled;
    ElMessage.error(res.msg || "修改启用状态失败");
  }
}

async function remove(row: any) {
  try {
    await ElMessageBox.confirm(`确认删除版本 ${row.version} 吗？`, "提示", {
      type: "warning"
    });
  } catch {
    return;
  }
  const res = await deleteAppVersion({ id: row.id });
  if (res.code === 0) {
    ElMessage.success("已删除");
    await load();
  } else {
    ElMessage.error(res.msg || "删除失败");
  }
}

onMounted(load);
</script>

<template>
  <div class="version-list">
    <el-alert
      style="margin-bottom: 16px"
      type="info"
      :closable="false"
      show-icon
      title="更新检查只读这里的版本列表：取「已启用」中的最高版本，download_type 决定强制或自由更新。"
    />

    <!-- gutter 会给 row 加 margin:0 -6px，比容器宽 12px；
         外层 overflow-y-auto 会把 overflow-x 计算成 auto，12px 溢出即产生
         横向滚动条并带动整个弹窗内容平移，因此这里不用 gutter -->
    <el-form :model="form" label-width="90px" class="mb-3">
      <el-row>
        <el-col :span="8">
          <el-form-item label="版本号" required>
            <el-input v-model="form.version" placeholder="如 3.0.2" clearable />
          </el-form-item>
        </el-col>
        <el-col :span="10">
          <el-form-item label="更新方式">
            <el-radio-group v-model="form.download_type">
              <el-radio :value="1">强制</el-radio>
              <el-radio :value="2">自由</el-radio>
              <el-radio :value="0">不启用</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="启用">
            <el-switch v-model="form.enabled" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item label="下载地址">
        <el-input
          v-model="form.download_url"
          placeholder="留空表示只提示更新，需手动安装"
          clearable
        />
      </el-form-item>
      <el-form-item label="更新公告">
        <el-input
          v-model="form.changelog"
          type="textarea"
          :rows="3"
          placeholder="本次更新说明，客户端更新弹窗展示"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="submit">
          {{ form.id ? "保存修改" : "新增版本" }}
        </el-button>
        <el-button v-if="form.id" @click="resetForm">取消编辑</el-button>
        <span class="ml-3 text-sm text-gray-500">{{ dialogTitle }}</span>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="rows" border size="small" style="width: 100%" :scrollbar-always-on="false">
      <el-table-column prop="version" label="版本" width="90" />
      <el-table-column label="更新方式" width="96">
        <template #default="{ row }">
          <el-tag :type="typeTag(row.download_type)" size="small">
            {{ typeText(row.download_type) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="download_url" label="下载地址" min-width="240" show-overflow-tooltip />
      <el-table-column prop="changelog" label="更新公告" min-width="220" show-overflow-tooltip />
      <el-table-column label="启用" width="70">
        <template #default="{ row }">
          <el-switch v-model="row.enabled" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" align="center">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="editRow(row)">
            编辑
          </el-button>
          <el-button link type="danger" size="small" @click="remove(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
/* 滚动统一由 ReDialog 外层容器负责（overflow-y-auto + max-height），
   这里再加 max-height/overflow 会形成嵌套滚动，
   横向溢出时还会带着整个弹窗一起平移。 */
.version-list {
  width: 100%;
  max-width: 100%;
}
</style>
