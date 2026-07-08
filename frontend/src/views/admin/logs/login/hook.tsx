import dayjs from "dayjs";
import { message } from "@/utils/message";
import { getLoginLogsList, clearLoginLogs } from "@/api/admin/logs";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";

export function useRole() {
  const form = reactive({
    uuid: "",
    username: "",
    status: ""
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
      label: "用户名",
      prop: "username",
      width: 180
    },
    {
      label: "UUID",
      prop: "uuid",
      width: 320
    },
    {
      label: "登录 IP",
      prop: "ip",
      width: 200
    },
    {
      label: "登录时间",
      prop: "created_at",
      width: 180,
      formatter: ({ created_at }) =>
        dayjs(created_at).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "登录状态",
      prop: "status",
      width: 100,
      align: "center",
      cellRenderer: ({ row, props }) => (
        <el-tag
          size={props.size}
          type={row.status === 1 ? "success" : "danger"}
        >
          {row.status === 1 ? "成功" : "失败"}
        </el-tag>
      )
    },
    {
      label: "登录信息",
      prop: "message",
      width: 200
    },
    {
      label: "浏览器类型",
      prop: "user_agent",
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
      const { code } = await clearLoginLogs();
      if (code === 0) {
        message("已清空所有登录日志", { type: "success" });
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
      const { code, data } = await getLoginLogsList(params);
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
