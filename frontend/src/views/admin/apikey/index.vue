<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Plus,
  Search,
  Edit,
  Delete,
  Refresh,
  DocumentCopy
} from "@element-plus/icons-vue";
import { PureTableBar } from "@/components/RePureTableBar";
import {
  getApiKeyScopes,
  getApiKeys,
  createApiKey,
  updateApiKey,
  regenerateApiKey,
  deleteApiKey
} from "@/api/admin/apikey";

defineOptions({ name: "ApiKeyList" });

const loading = ref(false);
const list = ref<any[]>([]);
const keyword = ref("");

const scopes = ref<{ value: string; label: string }[]>([]);
const scopeLabel = (v: string) =>
  scopes.value.find(s => s.value === v)?.label || v;
const splitCsv = (s: string) =>
  (s || "")
    .split(",")
    .map(x => x.trim())
    .filter(Boolean);

// 时间格式化（后端返回 RFC3339）
const fmtTime = (t?: string) => (t ? t.slice(0, 19).replace("T", " ") : "—");

const columns: TableColumnList = [
  { label: "ID", prop: "id", width: 70 },
  { label: "名称", prop: "name", minWidth: 140 },
  { label: "密钥", prop: "key", minWidth: 440, slot: "key" },
  { label: "能力", minWidth: 240, slot: "scopes" },
  { label: "状态", width: 90, slot: "status" },
  { label: "过期", width: 170, slot: "expire" },
  { label: "最近使用", width: 170, slot: "lastUsed" },
  { label: "操作", fixed: "right", width: 220, slot: "operation" }
];

const fetchScopes = async () => {
  const res = await getApiKeyScopes();
  if (res.code === 0) scopes.value = res.data?.list || [];
};
const fetchList = async () => {
  loading.value = true;
  try {
    const res = await getApiKeys(
      keyword.value.trim() ? { keyword: keyword.value.trim() } : {}
    );
    if (res.code === 0) list.value = res.data?.list || [];
  } finally {
    loading.value = false;
  }
};

// ===== 新建 / 编辑 =====
const dialogVisible = ref(false);
const dialogTitle = ref("新建密钥");
const submitting = ref(false);
const form = ref<{
  id?: number;
  name: string;
  scopes: string[];
  expire_at: string;
  status: number;
}>({
  id: undefined,
  name: "",
  scopes: [],
  expire_at: "",
  status: 1
});

const openCreate = () => {
  form.value = {
    id: undefined,
    name: "",
    scopes: [],
    expire_at: "",
    status: 1
  };
  dialogTitle.value = "新建密钥";
  dialogVisible.value = true;
};
const openEdit = (row: any) => {
  form.value = {
    id: row.id,
    name: row.name,
    scopes: splitCsv(row.scopes),
    expire_at: fmtTime(row.expire_at) === "—" ? "" : fmtTime(row.expire_at),
    status: row.status
  };
  dialogTitle.value = "编辑密钥";
  dialogVisible.value = true;
};
const handleSubmit = async () => {
  if (!form.value.name.trim()) {
    ElMessage.warning("请填写用途名称");
    return;
  }
  if (!form.value.scopes.length) {
    ElMessage.warning("请至少选择一项能力");
    return;
  }
  submitting.value = true;
  try {
    const payload = {
      id: form.value.id,
      name: form.value.name.trim(),
      scopes: form.value.scopes,
      expire_at: form.value.expire_at || "",
      status: form.value.status
    };
    const res = form.value.id
      ? await updateApiKey(payload)
      : await createApiKey(payload);
    if (res.code === 0) {
      ElMessage.success(res.msg || "保存成功");
      dialogVisible.value = false;
      // 新建成功后把密钥展示出来便于复制
      if (!form.value.id && res.data?.key) {
        ElMessageBox.alert(res.data.key, "密钥已创建", {
          confirmButtonText: "复制并关闭",
          callback: () => copyText(res.data.key)
        });
      }
      fetchList();
    } else {
      ElMessage.error(res.msg || "保存失败");
    }
  } finally {
    submitting.value = false;
  }
};

const copyText = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success("已复制");
  } catch {
    ElMessage.error("复制失败");
  }
};

const handleRegenerate = (row: any) => {
  ElMessageBox.confirm(
    `重置后旧密钥【立即失效】，使用方需更换为新密钥。确认重置「${row.name}」？`,
    "重置密钥",
    { type: "warning" }
  )
    .then(async () => {
      const res = await regenerateApiKey(row.id);
      if (res.code === 0) {
        ElMessage.success("已重置");
        ElMessageBox.alert(res.data?.key || "", "新密钥", {
          confirmButtonText: "复制并关闭",
          callback: () => copyText(res.data?.key || "")
        });
        fetchList();
      } else {
        ElMessage.error(res.msg || "重置失败");
      }
    })
    .catch(() => {});
};

const handleDelete = (row: any) => {
  ElMessageBox.confirm(`确认删除密钥「${row.name}」？`, "提示", {
    type: "warning"
  })
    .then(async () => {
      const res = await deleteApiKey(row.id);
      if (res.code === 0) {
        ElMessage.success("删除成功");
        fetchList();
      } else {
        ElMessage.error(res.msg || "删除失败");
      }
    })
    .catch(() => {});
};

onMounted(() => {
  fetchScopes();
  fetchList();
});
</script>

<template>
  <div class="app-container">
    <el-form
      :inline="true"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="名称">
        <el-input
          v-model="keyword"
          placeholder="按用途名称搜索"
          clearable
          style="width: 200px"
          @clear="fetchList"
          @keyup.enter="fetchList"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :icon="Search" @click="fetchList">
          查询
        </el-button>
      </el-form-item>
    </el-form>

    <el-card shadow="never" class="table-wrapper mt-4">
      <div class="toolbar mb-4 px-2 overflow-x-auto whitespace-nowrap pb-2">
        <el-button type="primary" :icon="Plus" @click="openCreate">
          新建密钥
        </el-button>
      </div>

      <PureTableBar title="API密钥" :columns="columns" @refresh="fetchList">
        <template v-slot="{ size, dynamicColumns }">
          <pure-table
            row-key="id"
            table-layout="auto"
            show-overflow-tooltip
            border
            :loading="loading"
            :size="size"
            :data="list"
            :columns="dynamicColumns"
            :header-cell-style="{
              background: 'var(--el-fill-color-light)',
              color: 'var(--el-text-color-primary)'
            }"
            class="w-full"
          >
            <template #key="{ row }">
              <span class="font-mono">{{ row.key }}</span>
              <el-button
                link
                type="success"
                :icon="DocumentCopy"
                @click="copyText(row.key)"
              />
            </template>
            <template #scopes="{ row }">
              <el-tag
                v-for="s in splitCsv(row.scopes)"
                :key="s"
                size="small"
                class="mr-1"
              >
                {{ scopeLabel(s) }}
              </el-tag>
            </template>
            <template #status="{ row }">
              <el-tag
                :type="row.status === 1 ? 'success' : 'info'"
                size="small"
              >
                {{ row.status === 1 ? "启用" : "停用" }}
              </el-tag>
            </template>
            <template #expire="{ row }">
              {{ row.expire_at ? fmtTime(row.expire_at) : "永久" }}
            </template>
            <template #lastUsed="{ row }">
              {{ fmtTime(row.last_used_at) }}
            </template>
            <template #operation="{ row }">
              <el-button
                link
                type="primary"
                :icon="Edit"
                @click="openEdit(row)"
              >
                编辑
              </el-button>
              <el-button
                link
                type="warning"
                :icon="Refresh"
                @click="handleRegenerate(row)"
              >
                重置
              </el-button>
              <el-button
                link
                type="danger"
                :icon="Delete"
                @click="handleDelete(row)"
              >
                删除
              </el-button>
            </template>
          </pure-table>
        </template>
      </PureTableBar>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用途名称" required>
          <el-input
            v-model="form.name"
            placeholder="如：开放平台、第三方对接"
          />
        </el-form-item>
        <el-form-item label="密钥能力" required>
          <el-select
            v-model="form.scopes"
            multiple
            placeholder="选择该密钥可调用的能力"
            class="w-full"
          >
            <el-option
              v-for="s in scopes"
              :key="s.value"
              :label="s.label"
              :value="s.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expire_at"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="留空=永久"
            clearable
            class="w-full"
          />
        </el-form-item>
        <el-form-item v-if="form.id" label="状态">
          <el-switch
            v-model="form.status"
            :active-value="1"
            :inactive-value="0"
            active-text="启用"
            inactive-text="停用"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}
.w-full {
  width: 100%;
}
.font-mono {
  font-family: Consolas, Monaco, "Courier New", monospace;
}
</style>
