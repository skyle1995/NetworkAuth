import dayjs from "dayjs";
import { message } from "@/utils/message";
import {
  getApiList,
  updateApi,
  updateApiStatus,
  getApiTypes,
  generateApiKeys
} from "@/api/admin/api";
import { getAppsList } from "@/api/admin/app";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw, h } from "vue";
import { ElMessageBox } from "element-plus";
import { addDialog } from "@/components/ReDialog";
import editForm from "./form.vue";

export function useApi() {
  const form = reactive({
    app_uuid: "",
    api_type: ""
  });

  const dataList = ref([]);
  const appList = ref([]);
  const apiTypes = ref([]);
  const loading = ref(true);

  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 30,
    currentPage: 1,
    background: true
  });

  const columns: TableColumnList = [
    {
      label: "ID",
      prop: "id",
      width: 100
    },
    {
      label: "所属应用",
      prop: "app_name",
      minWidth: 120
    },
    {
      label: "接口类型",
      prop: "api_type_name",
      minWidth: 120
    },
    {
      label: "提交数据算法",
      prop: "algorithm_names.submit",
      minWidth: 120
    },
    {
      label: "返回数据算法",
      prop: "algorithm_names.return",
      minWidth: 120
    },
    {
      label: "状态",
      prop: "status",
      width: 100,
      align: "center",
      cellRenderer: ({ row }) => (
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
      label: "更新时间",
      prop: "updated_at",
      width: 160,
      formatter: ({ updated_at }) =>
        dayjs(updated_at).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "操作",
      fixed: "right",
      width: 120,
      slot: "operation"
    }
  ];

  async function handleStatusChange(row: any, val: number) {
    try {
      const { code, msg } = await updateApiStatus({ id: row.id, status: val });
      if (code === 0) {
        message(`已${val === 1 ? "启用" : "禁用"}接口配置`, {
          type: "success"
        });
      } else {
        row.status = val === 1 ? 0 : 1;
        message(msg || "操作失败", { type: "error" });
      }
    } catch (e) {
      row.status = val === 1 ? 0 : 1;
      message("操作失败", { type: "error" });
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

  async function onSearch() {
    loading.value = true;
    const params = {
      ...toRaw(form),
      page: pagination.currentPage,
      limit: pagination.pageSize
    };

    try {
      const { data, code, count } = (await getApiList(params)) as any;
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

  async function fetchApps() {
    try {
      const { data, code } = (await getAppsList({
        page: 1,
        limit: 1000
      })) as any;
      if (code === 0) {
        appList.value = data || [];
      }
    } catch (e) {
      console.error("Failed to fetch apps:", e);
    }
  }

  async function fetchApiTypes() {
    try {
      const { data, code } = (await getApiTypes()) as any;
      if (code === 0 && Array.isArray(data)) {
        apiTypes.value = data.map((item: any) => ({
          value: item.value,
          label: item.name
        }));
      }
    } catch (e) {
      console.error("Failed to fetch API types:", e);
    }
  }

  const resetForm = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  function openDialog(title = "修改接口配置", row?: any) {
    const dialogFormRef = ref();
    addDialog({
      title,
      props: {
        formInline: {
          id: row?.id ?? 0,
          uuid: row?.uuid ?? "",
          app_uuid: row?.app_uuid ?? "",
          api_type: row?.api_type ?? 0,
          api_type_name: row?.api_type_name ?? "",
          app_name: row?.app_name ?? "",
          submit_algorithm: row?.submit_algorithm ?? 0,
          submit_public_key: row?.submit_public_key ?? "",
          submit_private_key: row?.submit_private_key ?? "",
          return_algorithm: row?.return_algorithm ?? 0,
          return_public_key: row?.return_public_key ?? "",
          return_private_key: row?.return_private_key ?? "",
          status: row?.status ?? 1
        }
      },
      width: "460px",
      draggable: true,
      fullscreenIcon: true,
      closeOnClickModal: false,
      contentRenderer: () => h(editForm, { ref: dialogFormRef } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options, index } }) => {
            try {
              await ElMessageBox.confirm(
                `确认关闭当前表单吗？<br><span style="color:red;font-size:12px;">注意：直接关闭将不会保存已经修改的数据！</span>`,
                "提示",
                {
                  type: "warning",
                  dangerouslyUseHTMLString: true,
                  confirmButtonText: "确认",
                  cancelButtonText: "取消"
                }
              );
              options.visible = false;
            } catch (e) {
              // 用户点击了取消，不执行任何操作
            }
          }
        },
        {
          label: "一键生成密钥",
          type: "warning",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            const curData = options.props.formInline as any;
            let successCount = 0;

            // 提交数据加密生成
            if (curData.submit_algorithm > 0) {
              try {
                const { code, data } = await generateApiKeys({
                  side: "submit",
                  algorithm: curData.submit_algorithm
                });
                if (code === 0) {
                  curData.submit_public_key = data.public_key || "";
                  curData.submit_private_key = data.private_key || "";
                  successCount++;
                }
              } catch (e) {}
            }

            // 返回数据加密生成
            if (curData.return_algorithm > 0) {
              try {
                const { code, data } = await generateApiKeys({
                  side: "return",
                  algorithm: curData.return_algorithm
                });
                if (code === 0) {
                  curData.return_public_key = data.public_key || "";
                  curData.return_private_key = data.private_key || "";
                  successCount++;
                }
              } catch (e) {}
            }

            if (successCount > 0) {
              message("密钥生成成功", { type: "success" });
            } else if (
              curData.submit_algorithm === 0 &&
              curData.return_algorithm === 0
            ) {
              message("当前均为不加密算法，无需生成密钥", { type: "info" });
            }
          }
        },
        {
          label: "保存",
          type: "primary",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options, index } }) => {
            const formRef = dialogFormRef.value;
            if (!formRef) {
              handleSave();
              return;
            }
            formRef.getRef().validate(valid => {
              if (valid) {
                handleSave();
              }
            });

            async function handleSave() {
              try {
                const { code, msg } = await updateApi(options.props.formInline);
                if (code === 0) {
                  message("保存成功", { type: "success" });
                  options.visible = false;
                  onSearch();
                } else {
                  message(msg || "保存失败", { type: "error" });
                }
              } catch (e) {
                console.error(e);
              }
            }
          }
        }
      ]
    });
  }

  onMounted(() => {
    fetchApps();
    fetchApiTypes();
    onSearch();
  });

  return {
    form,
    loading,
    columns,
    dataList,
    appList,
    apiTypes,
    pagination,
    onSearch,
    resetForm,
    openDialog,
    handleSizeChange,
    handleCurrentChange
  };
}
