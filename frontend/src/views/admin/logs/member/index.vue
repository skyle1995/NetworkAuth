<script setup lang="ts">
import { ref, reactive, onMounted, h } from "vue";
import { ElMessage, ElMessageBox, ElTag } from "element-plus";
import type { PaginationProps } from "@pureadmin/table";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { getMemberLogs, clearMemberLogs } from "@/api/admin/member";
import { getAppsSimple } from "@/api/admin/app";

defineOptions({ name: "MemberLogs" });

const formRef = ref();
const apps = ref<any[]>([]);
const dataList = ref<any[]>([]);
const loading = ref(true);

const form = reactive({
  app_uuid: "",
  username: "",
  action: ""
});

const ACTIONS = [
  "卡密登录",
  "账号登录",
  "注册",
  "充值",
  "扣点",
  "扣时",
  "机器码转绑",
  "IP转绑",
  "封停",
  "拉黑",
  "解封"
];

// 动作 → 标签颜色
function actionTagType(action: string) {
  if (["封停", "拉黑", "扣时", "扣点"].includes(action)) return "danger";
  if (["充值", "解封"].includes(action)) return "success";
  if (["卡密登录", "账号登录"].includes(action)) return "primary";
  return "info";
}

const pagination = reactive<PaginationProps>({
  total: 0,
  pageSize: 30,
  currentPage: 1,
  background: true
});

const columns: TableColumnList = [
  { label: "ID", prop: "id", width: 80 },
  {
    label: "所属应用",
    prop: "app_uuid",
    minWidth: 130,
    cellRenderer: ({ row }) =>
      apps.value.find(a => a.uuid === row.app_uuid)?.name || "未知应用"
  },
  { label: "用户名/卡号", prop: "username", minWidth: 160 },
  {
    label: "动作",
    prop: "action",
    minWidth: 110,
    cellRenderer: ({ row }) =>
      h(ElTag, { type: actionTagType(row.action), effect: "light" }, () =>
        String(row.action)
      )
  },
  { label: "详情", prop: "detail", minWidth: 160 },
  { label: "IP", prop: "ip", minWidth: 120 },
  { label: "时间", prop: "created_at", minWidth: 160 }
];

async function fetchApps() {
  const { code, data } = await getAppsSimple();
  if (code === 0 && Array.isArray(data)) apps.value = data;
}

async function onSearch() {
  loading.value = true;
  try {
    const { code, data, count } = await getMemberLogs({
      page: pagination.currentPage,
      limit: pagination.pageSize,
      app_uuid: form.app_uuid,
      username: form.username,
      action: form.action
    });
    if (code === 0) {
      dataList.value = data || [];
      pagination.total = count || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

function resetForm(formEl: any) {
  formEl?.resetFields();
  onSearch();
}

async function onClear() {
  try {
    await ElMessageBox.confirm(
      "确认清空全部调用审计日志吗？此操作不可恢复！",
      "提示",
      { type: "warning" }
    );
    const { code, msg } = await clearMemberLogs();
    if (code === 0) {
      ElMessage.success("已清空");
      onSearch();
    } else {
      ElMessage.error(msg || "清空失败");
    }
  } catch {
    // cancelled
  }
}

function handleSizeChange(val: number) {
  pagination.pageSize = val;
  onSearch();
}
function handleCurrentChange(val: number) {
  pagination.currentPage = val;
  onSearch();
}

onMounted(() => {
  fetchApps();
  onSearch();
});
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="全部"
          clearable
          class="w-[160px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option
            v-for="app in apps"
            :key="app.uuid"
            :label="app.name"
            :value="app.uuid"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="用户名/卡号" prop="username">
        <el-input
          v-model="form.username"
          placeholder="精确匹配"
          clearable
          class="w-[160px]!"
          @keyup.enter="onSearch"
        />
      </el-form-item>
      <el-form-item label="动作" prop="action">
        <el-select
          v-model="form.action"
          placeholder="全部"
          clearable
          class="w-[140px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option v-for="a in ACTIONS" :key="a" :label="a" :value="a" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:search')"
          :loading="loading"
          @click="onSearch"
        >
          搜索
        </el-button>
        <el-button
          :icon="useRenderIcon('ep:refresh')"
          @click="resetForm(formRef)"
        >
          重置
        </el-button>
      </el-form-item>
    </el-form>

    <el-card shadow="never" class="table-wrapper mt-4">
      <div class="toolbar mb-4 px-2">
        <el-button
          type="danger"
          plain
          :icon="useRenderIcon('ep:delete')"
          @click="onClear"
        >
          清空日志
        </el-button>
      </div>
      <PureTableBar title="调用审计日志" :columns="columns" @refresh="onSearch">
        <template v-slot="{ size, dynamicColumns }">
          <pure-table
            row-key="id"
            align-whole="center"
            header-align="center"
            table-layout="auto"
            show-overflow-tooltip
            border
            :loading="loading"
            :size="size"
            :data="dataList"
            :columns="dynamicColumns"
            :header-cell-style="{
              background: 'var(--el-fill-color-light)',
              color: 'var(--el-text-color-primary)'
            }"
            class="w-full"
          />
          <div class="flex mt-4 w-full overflow-x-auto">
            <div class="ml-auto shrink-0">
              <el-pagination
                v-model:current-page="pagination.currentPage"
                v-model:page-size="pagination.pageSize"
                :page-sizes="[10, 20, 30, 50, 100, 200]"
                :background="true"
                layout="total, sizes, prev, pager, next, jumper"
                :total="pagination.total"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </div>
        </template>
      </PureTableBar>
    </el-card>
  </div>
</template>
