import dayjs from "dayjs";
import { message } from "@/utils/message";
import { getOperationLogsList, clearOperationLogs } from "@/api/admin/logs";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";

export function useRole() {
  const form = reactive({
    operator: "",
    operation_type: ""
  });
  const dataList = ref([]);
  const loading = ref(true);
  // 当前勾选的行（供智能导出/智能删除使用）
  const selectedRows = ref([]);

  /** pure-table 多选变化回调 */
  function handleSelectionChange(rows) {
    selectedRows.value = rows;
  }

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
      align: "center",
      reserveSelection: true
    },
    {
      label: "序号",
      prop: "id",
      width: 100
    },
    {
      label: "操作人员",
      prop: "operator",
      width: 150
    },
    {
      label: "操作人员 UUID",
      prop: "operator_uuid",
      width: 320
    },
    {
      label: "操作时间",
      prop: "created_at",
      width: 180,
      formatter: ({ created_at }) =>
        dayjs(created_at).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "操作方式",
      prop: "operation_type",
      width: 320
    },
    {
      label: "操作详情",
      prop: "details",
      minWidth: 320,
      showOverflowTooltip: true
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

  /** 清空日志 */
  async function clearAll() {
    try {
      const { code } = await clearOperationLogs();
      if (code === 0) {
        message("已清空所有操作日志", { type: "success" });
        onSearch();
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function onSearch() {
    loading.value = true;
    const params = {
      ...toRaw(form),
      page: pagination.currentPage,
      limit: pagination.pageSize
    };
    try {
      const { code, data } = await getOperationLogsList(params);
      if (code === 0) {
        dataList.value = data.list || [];
        pagination.total = data.total || 0;
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
    selectedRows,
    onSearch,
    clearAll,
    resetForm,
    handleSelectionChange,
    handleSizeChange,
    handleCurrentChange
  };
}
