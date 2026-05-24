<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import Sortable from "sortablejs";
import {
  createPortalNavigation,
  deletePortalNavigation,
  getPortalNavigationList,
  updatePortalNavigation,
  type PortalNavigationItem,
  type PortalNavigationResult
} from "@/api/admin/portalNavigation";

defineOptions({
  name: "PortalNavigation"
});

type DialogMode = "create" | "edit";

const loading = ref(false);
const submitting = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const tableRef = ref();
const tableData = ref<PortalNavigationItem[]>([]);
let sortableInstance: Sortable | null = null;

const form = reactive<PortalNavigationItem>({
  id: 0,
  name: "",
  type: "link",
  parent_id: 0,
  path: "",
  sort: 0,
  is_home: false,
  is_hidden: false,
  is_external: false
});

const formRules: FormRules = {
  name: [{ required: true, message: "请输入导航名称", trigger: "blur" }],
  path: [
    {
      validator: (_rule, value, callback) => {
        if (form.type === "link" && !String(value || "").trim()) {
          callback(new Error("请输入导航地址"));
          return;
        }
        callback();
      },
      trigger: "blur"
    }
  ]
};

const groupOptions = computed(() =>
  tableData.value.filter(item => item.type === "group" && item.id !== form.id)
);

const dialogTitle = computed(() => {
  switch (dialogMode.value) {
    case "edit":
      return "编辑导航";
    default:
      return "新增导航";
  }
});

/**
 * 重置表单数据
 * 用于新增前清空旧数据
 */
const resetFormData = () => {
  form.id = 0;
  form.name = "";
  form.type = "link";
  form.parent_id = 0;
  form.path = "";
  form.sort = 0;
  form.is_home = false;
  form.is_hidden = false;
  form.is_external = false;
};

/**
 * 销毁拖拽实例
 * 避免表格刷新后重复绑定拖拽事件
 */
const destroySortable = () => {
  sortableInstance?.destroy();
  sortableInstance = null;
};

/**
 * 初始化表格拖拽
 * 基于当前表格渲染结果启用行拖拽排序
 */
const initSortable = async () => {
  await nextTick();
  destroySortable();

  const tbody = tableRef.value?.$el?.querySelector?.(".el-table__body-wrapper tbody");
  if (!tbody) {
    return;
  }

  sortableInstance = Sortable.create(tbody, {
    animation: 200,
    handle: ".drag-handle",
    onEnd: async ({ oldIndex, newIndex }) => {
      switch (oldIndex === undefined || newIndex === undefined || oldIndex === newIndex) {
        case true:
          return;
        default:
          await handleSortChange(oldIndex, newIndex);
          break;
      }
    }
  });
};

/**
 * 获取门户导航列表
 * 按后台返回结果刷新表格数据
 */
const fetchPortalNavigationList = async () => {
  loading.value = true;
  try {
    const res = await getPortalNavigationList();
    switch (res.code) {
      case 0:
        tableData.value = res.data || [];
        await initSortable();
        break;
      default:
        ElMessage.error(res.msg || "获取导航列表失败");
        break;
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "获取导航列表失败");
  } finally {
    loading.value = false;
  }
};

/**
 * 打开新增弹窗
 * 初始化默认表单后展示对话框
 */
const handleCreate = () => {
  dialogMode.value = "create";
  resetFormData();
  form.sort = tableData.value.length;
  dialogVisible.value = true;
};

/**
 * 打开编辑弹窗
 * 将当前行数据回填到表单中
 */
const handleEdit = (row: PortalNavigationItem) => {
  dialogMode.value = "edit";
  form.id = row.id;
  form.name = row.name;
  form.type = row.type || "link";
  form.parent_id = row.parent_id || 0;
  form.path = row.path;
  form.sort = row.sort;
  form.is_home = row.is_home;
  form.is_hidden = row.is_hidden;
  form.is_external = row.is_external;
  dialogVisible.value = true;
};

/**
 * 关闭弹窗
 * 统一重置校验状态和表单数据
 */
const handleDialogClose = () => {
  dialogVisible.value = false;
  formRef.value?.clearValidate();
  resetFormData();
};

const handleTypeChange = () => {
  switch (form.type) {
    case "group":
      form.parent_id = 0;
      form.path = "";
      form.is_external = false;
      form.is_home = false;
      break;
    default:
      break;
  }
};

/**
 * 保存门户导航
 * 根据当前弹窗模式调用新增或编辑接口
 */
const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) {
    return;
  }

  submitting.value = true;
  try {
    const payload = {
      id: form.id,
      name: form.name,
      type: form.type,
      parent_id: form.type === "group" ? 0 : Number(form.parent_id || 0),
      path: form.path,
      sort:
        dialogMode.value === "create"
          ? tableData.value.length
          : Number(form.sort || 0),
      is_home: form.is_home,
      is_hidden: form.is_hidden,
      is_external: form.is_external
    };

    let res: PortalNavigationResult;
    switch (dialogMode.value) {
      case "edit":
        res = await updatePortalNavigation(payload);
        break;
      default:
        res = await createPortalNavigation(payload);
        break;
    }

    switch (res.code) {
      case 0:
        ElMessage.success(dialogMode.value === "edit" ? "更新成功" : "创建成功");
        handleDialogClose();
        fetchPortalNavigationList();
        break;
      default:
        ElMessage.error(res.msg || "保存失败");
        break;
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "保存失败");
  } finally {
    submitting.value = false;
  }
};

/**
 * 保存单行导航状态
 * 用于表格中的首页和隐藏开关即时更新
 */
const saveRowState = async (row: PortalNavigationItem, successMessage: string) => {
  try {
    const res = await updatePortalNavigation({
      id: row.id,
      name: row.name,
      type: row.type,
      parent_id: row.parent_id,
      path: row.path,
      sort: row.sort,
      is_home: row.is_home,
      is_hidden: row.is_hidden,
      is_external: row.is_external
    });
    switch (res.code) {
      case 0:
        ElMessage.success(successMessage);
        await fetchPortalNavigationList();
        break;
      default:
        ElMessage.error(res.msg || "保存失败");
        await fetchPortalNavigationList();
        break;
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "保存失败");
    await fetchPortalNavigationList();
  }
};

/**
 * 切换首页开关
 * 表格内直接更新首页状态，同时自动取消隐藏
 */
const handleHomeSwitch = async (row: PortalNavigationItem) => {
  switch (row.is_home) {
    case true:
      row.is_hidden = false;
      await saveRowState(row, "首页状态已更新");
      break;
    default:
      await saveRowState(row, "首页状态已更新");
      break;
  }
};

/**
 * 切换隐藏开关
 * 首页导航禁止隐藏，普通导航允许即时切换
 */
const handleHiddenSwitch = async (row: PortalNavigationItem) => {
  switch (row.is_home && row.is_hidden) {
    case true:
      row.is_hidden = false;
      ElMessage.warning("首页导航禁止隐藏");
      break;
    default:
      await saveRowState(row, "显示状态已更新");
      break;
  }
};

/**
 * 切换外部打开开关
 * 表格内直接更新外部打开状态
 */
const handleExternalSwitch = async (row: PortalNavigationItem) => {
  await saveRowState(row, "打开方式已更新");
};

const getTypeLabel = (row: PortalNavigationItem) => {
  return row.type === "group" ? "分组" : "链接";
};

const getParentGroupName = (row: PortalNavigationItem) => {
  if (!row.parent_id) return "-";
  return tableData.value.find(item => item.id === row.parent_id)?.name || "-";
};

const getDisplayName = (row: PortalNavigationItem) => {
  return row.parent_id ? `└ ${row.name}` : row.name;
};

/**
 * 保存拖拽排序结果
 * 按当前表格顺序重新计算 sort 并逐条提交
 */
const handleSortChange = async (oldIndex: number, newIndex: number) => {
  const list = [...tableData.value];
  const [movedItem] = list.splice(oldIndex, 1);
  switch (isLockedAdminEntry(movedItem)) {
    case true:
      ElMessage.warning("管理员登录导航为系统保留项，不允许调整顺序");
      await fetchPortalNavigationList();
      return;
    default:
      break;
  }
  const unlockedList = tableData.value.filter(item => !isLockedAdminEntry(item));
  const unlockedOldIndex = tableData.value
    .slice(0, oldIndex)
    .filter(item => !isLockedAdminEntry(item)).length;
  let unlockedNewIndex = list
    .slice(0, newIndex)
    .filter(item => !isLockedAdminEntry(item)).length;

  const [movedUnlockedItem] = unlockedList.splice(unlockedOldIndex, 1);
  if (!movedUnlockedItem) {
    await fetchPortalNavigationList();
    return;
  }

  if (unlockedNewIndex > unlockedList.length) {
    unlockedNewIndex = unlockedList.length;
  }
  unlockedList.splice(unlockedNewIndex, 0, movedUnlockedItem);

  const adminItem = tableData.value.find(item => isLockedAdminEntry(item));
  unlockedList.forEach((item, index) => {
    item.sort = index;
  });
  if (adminItem) {
    adminItem.sort = 999;
  }
  const nextTableData = adminItem ? [...unlockedList, adminItem] : [...unlockedList];
  tableData.value = nextTableData;

  loading.value = true;
  try {
    for (const item of unlockedList) {
      const res = await updatePortalNavigation({
        id: item.id,
        name: item.name,
        type: item.type,
        parent_id: item.parent_id,
        path: item.path,
        sort: item.sort,
        is_home: item.is_home,
        is_hidden: item.is_hidden,
        is_external: item.is_external
      });
      switch (res.code) {
        case 0:
          break;
        default:
          ElMessage.error(res.msg || "排序保存失败");
          await fetchPortalNavigationList();
          return;
      }
    }
    ElMessage.success("排序已更新");
    await fetchPortalNavigationList();
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || "排序保存失败");
    await fetchPortalNavigationList();
  } finally {
    loading.value = false;
  }
};

/**
 * 判断是否为系统保留导航
 * 管理员登录入口禁止修改名称、地址、排序和首页状态
 */
const isLockedAdminEntry = (row: PortalNavigationItem) => {
  switch (row.path) {
    case "admin":
      return true;
    default:
      return false;
  }
};

/**
 * 删除门户导航
 * 二次确认后调用删除接口
 */
const handleDelete = async (row: PortalNavigationItem) => {
  switch (isLockedAdminEntry(row)) {
    case true:
      ElMessage.warning("管理员登录导航为系统保留项，不允许删除");
      return;
    default:
      break;
  }

  try {
    await ElMessageBox.confirm(`确认删除导航“${row.name}”吗？`, "删除确认", {
      type: "warning"
    });
    const res = await deletePortalNavigation(row.id);
    switch (res.code) {
      case 0:
        ElMessage.success("删除成功");
        fetchPortalNavigationList();
        break;
      default:
        ElMessage.error(res.msg || "删除失败");
        break;
    }
  } catch (error: any) {
    if (error === "cancel" || error === "close") {
      return;
    }
    ElMessage.error(error.response?.data?.message || "删除失败");
  }
};

onMounted(() => {
  fetchPortalNavigationList();
});

onBeforeUnmount(() => {
  destroySortable();
});
</script>

<template>
  <div class="main">
    <el-card shadow="never">
      <template #header>
        <div class="flex items-center justify-between gap-4">
          <div>
            <div class="text-base font-bold">导航设置</div>
            <div class="text-xs text-gray-400 mt-1">
              支持配置名称、地址、排序和门户首页，管理员登录入口为系统保留项
              分组将在门户首页中展示为下拉菜单
            </div>
          </div>
          <el-button type="primary" @click="handleCreate">新增导航</el-button>
        </div>
      </template>

      <el-table ref="tableRef" v-loading="loading" :data="tableData" row-key="id" border class="w-full">
        <el-table-column label="拖拽" min-width="70" align="center">
          <template #default="{ row }">
            <span
              class="select-none text-gray-400"
              :class="isLockedAdminEntry(row) ? 'cursor-not-allowed opacity-50' : 'drag-handle cursor-move'"
            >
              拖动
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" min-width="90" align="center" />
        <el-table-column label="类型" min-width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.type === 'group' ? 'warning' : 'info'">
              {{ getTypeLabel(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="导航名称" min-width="180">
          <template #default="{ row }">
            {{ getDisplayName(row) }}
          </template>
        </el-table-column>
        <el-table-column label="所属分组" min-width="140" align="center">
          <template #default="{ row }">
            {{ getParentGroupName(row) }}
          </template>
        </el-table-column>
        <el-table-column label="导航地址" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.type === "group" ? "-" : row.path }}
          </template>
        </el-table-column>
        <el-table-column label="隐藏" min-width="120" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.is_hidden"
              :disabled="row.is_home"
              @change="handleHiddenSwitch(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="外部打开" min-width="120" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.is_external"
              :disabled="isLockedAdminEntry(row) || row.type === 'group'"
              @change="handleExternalSwitch(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="是否首页" min-width="120" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.is_home"
              :disabled="isLockedAdminEntry(row) || row.type === 'group' || row.parent_id > 0"
              @change="handleHomeSwitch(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="180" align="center" fixed="right">
          <template #default="{ row }">
            <div class="flex items-center justify-center gap-3">
              <el-button
                link
                type="primary"
                :disabled="isLockedAdminEntry(row)"
                @click="handleEdit(row)"
              >
                编辑
              </el-button>
              <el-button
                link
                type="danger"
                :disabled="isLockedAdminEntry(row)"
                @click="handleDelete(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="560px"
      destroy-on-close
      @closed="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-width="110px"
        label-position="right"
      >
        <el-form-item label="导航类型">
          <el-radio-group v-model="form.type" @change="handleTypeChange">
            <el-radio value="link">链接</el-radio>
            <el-radio value="group">分组</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="导航名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入导航名称" />
        </el-form-item>
        <el-form-item v-if="form.type === 'link'" label="所属分组">
          <el-select v-model="form.parent_id" placeholder="请选择所属分组，留空表示顶级链接" clearable class="w-full">
            <el-option :value="0" label="顶级链接" />
            <el-option
              v-for="item in groupOptions"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.type === 'link'" label="导航地址" prop="path">
          <el-input
            v-model="form.path"
            placeholder="请输入导航地址或路由路径，例如 /home/index"
          />
        </el-form-item>
        <el-form-item>
          <div class="text-xs text-gray-400">
            分组仅用于下拉展示，不支持首页与外部打开；首页、隐藏、外部打开和排序请直接在表格中调整
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="flex items-center justify-end gap-3">
          <el-button @click="handleDialogClose">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
