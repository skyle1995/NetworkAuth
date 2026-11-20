import dayjs from "dayjs";
import { message } from "@/utils/message";
import {
  getAppsList,
  createApp,
  updateApp,
  deleteApp,
  batchDeleteApps,
  updateAppStatus,
  batchUpdateAppStatus,
  resetAppSecret,
  getAppData,
  updateAppData,
  getAppMultiConfig,
  updateAppMultiConfig,
  getAppRegisterConfig,
  updateAppRegisterConfig
} from "@/api/admin/app";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw, h } from "vue";
import { ElMessageBox } from "element-plus";
import { addDialog } from "@/components/ReDialog";

export function useApp() {
  const form = reactive({
    search: ""
  });
  const dataList = ref([]);
  const loading = ref(true);

  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 30,
    currentPage: 1,
    background: true
  });
  const columns: TableColumnList = [
    {
      type: "selection",
      width: 55,
      align: "left",
      hide: false
    },
    {
      label: "ID",
      prop: "id",
      width: 100
    },
    {
      label: "应用名称",
      prop: "name",
      minWidth: 120
    },
    {
      label: "插件标识",
      prop: "plugin_id",
      minWidth: 140,
      cellRenderer: ({ row }) => row.plugin_id || "—"
    },
    {
      label: "UUID",
      prop: "uuid",
      width: 400,
      cellRenderer: ({ row }) => h("span", { style: "font-family:ui-monospace,monospace;font-size:12px" }, row.uuid)
    },
    {
      label: "密钥",
      prop: "secret",
      width: 320
    },
    {
      label: "版本",
      prop: "version",
      width: 100
    },
    {
      label: "状态",
      prop: "status",
      width: 100,
      align: "center",
      cellRenderer: ({ row, props }) => (
        <el-switch
          v-model={row.status}
          active-value={1}
          inactive-value={0}
          active-text="启用"
          inactive-text="禁用"
          inline-prompt
          onChange={(val: number) => handleStatusChange(row, val)}
        />
      )
    },
    {
      label: "创建时间",
      prop: "created_at",
      width: 180,
      formatter: ({ created_at }) =>
        dayjs(created_at).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "操作",
      fixed: "right",
      width: 200,
      slot: "operation"
    }
  ];

  function handleSizeChange(val: number) {
    pagination.pageSize = val;
    onSearch();
  }

  function handleCurrentChange(val: number) {
    pagination.currentPage = val;
    onSearch();
  }

  function handleSelectionChange(val) {
    // 可以在外部处理
  }

  async function handleStatusChange(row: any, val: number) {
    try {
      const res = await updateAppStatus({ id: row.id, status: val });
      if (res.code === 0) {
        message("状态更新成功", { type: "success" });
      } else {
        message(res.msg || "状态更新失败", { type: "error" });
        row.status = val === 1 ? 0 : 1; // 回滚状态
      }
    } catch (e) {
      console.error(e);
      row.status = val === 1 ? 0 : 1; // 回滚状态
    }
  }

  async function onSearch() {
    loading.value = true;
    try {
      const params = {
        ...toRaw(form),
        page: pagination.currentPage,
        limit: pagination.pageSize
      };
      const { data, code, count } = await getAppsList(params);
      if (code === 0) {
        dataList.value = data;
        pagination.total = count;
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  }

  const resetForm = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  onMounted(() => {
    onSearch();
  });

  return {
    form,
    loading,
    columns,
    dataList,
    pagination,
    onSearch,
    resetForm,
    handleSizeChange,
    handleCurrentChange,
    handleSelectionChange
  };
}
