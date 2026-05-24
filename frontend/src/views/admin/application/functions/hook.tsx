import { reactive, ref, onMounted, h } from "vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import editForm from "./form.vue";
import type { PaginationProps } from "@pureadmin/table";
import {
  getFunctions,
  createFunction,
  updateFunction,
  deleteFunction,
  batchDeleteFunctions
} from "@/api/admin/function";
import { getAppsSimple } from "@/api/admin/app";
import { ElMessageBox } from "element-plus";

export function useFunction() {
  const form = reactive({
    search: "",
    app_uuid: ""
  });

  const formRef = ref();
  const dataList = ref([]);
  const loading = ref(true);
  const apps = ref([]);

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
      width: 80
    },
    {
      label: "函数别名",
      prop: "alias",
      minWidth: 120
    },
    {
      label: "函数编号",
      prop: "number",
      minWidth: 140
    },
    {
      label: "所属应用",
      prop: "app_uuid",
      minWidth: 150,
      cellRenderer: ({ row }) => {
        if (row.app_uuid === "0" || !row.app_uuid) {
          return "全局函数";
        }
        const app = apps.value.find(a => a.uuid === row.app_uuid);
        return app ? app.name : "未知应用";
      }
    },
    {
      label: "备注说明",
      prop: "remark",
      minWidth: 150
    },
    {
      label: "创建时间",
      prop: "created_at",
      minWidth: 160
    },
    {
      label: "操作",
      fixed: "right",
      width: 160,
      slot: "operation"
    }
  ];

  async function fetchApps() {
    try {
      const { code, data } = await getAppsSimple();
      if (code === 0 && Array.isArray(data)) {
        apps.value = data;
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function onSearch() {
    loading.value = true;
    try {
      const { code, data, count } = await getFunctions({
        page: pagination.currentPage,
        limit: pagination.pageSize,
        search: form.search,
        app_uuid: form.app_uuid
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

  const resetFormSearch = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  function openDialog(title = "新增", row?: any) {
    const dialogFormRef = ref();
    addDialog({
      title: `${title}函数`,
      props: {
        formInline: {
          id: row?.id,
          uuid: row?.uuid ?? "",
          alias: row?.alias ?? "",
          app_uuid: row?.app_uuid ?? "0",
          code: row?.code ?? "",
          remark: row?.remark ?? ""
        },
        apps: apps.value
      },
      width: "600px",
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
          label: "保存",
          type: "primary",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options, index } }) => {
            const formRefInstance = dialogFormRef.value;
            if (!formRefInstance) {
              handleSave();
              return;
            }
            formRefInstance.getRef().validate(valid => {
              if (valid) {
                handleSave();
              }
            });

            async function handleSave() {
              try {
                const curData = options.props.formInline;
                let res;
                if (curData.id) {
                  res = await updateFunction(curData);
                } else {
                  res = await createFunction(curData);
                }

                if (res.code === 0) {
                  message("保存成功", { type: "success" });
                  options.visible = false;
                  onSearch();
                } else {
                  message(res.msg || "保存失败", { type: "error" });
                }
              } catch (e) {
                console.error(e);
                message("保存失败", { type: "error" });
              }
            }
          }
        }
      ]
    });
  }

  async function handleDelete(row) {
    try {
      await ElMessageBox.confirm(
        `确认删除函数 <strong style="color:red">${row.alias}</strong> 吗？<br><span style="color:red;font-size:12px;">注意：此操作不可恢复！</span>`,
        "提示",
        {
          type: "warning",
          dangerouslyUseHTMLString: true
        }
      );
      const { code, msg } = await deleteFunction({ id: row.id });
      if (code === 0) {
        message("删除成功", { type: "success" });
        onSearch();
      } else {
        message(msg || "删除失败", { type: "error" });
      }
    } catch (e) {
      if (e !== "cancel") {
        console.error(e);
        message("删除失败", { type: "error" });
      }
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

  function handleSelectionChange(val) {
    // expose selection to index
  }

  onMounted(() => {
    fetchApps();
    onSearch();
  });

  return {
    form,
    loading,
    columns,
    dataList,
    pagination,
    apps,
    onSearch,
    resetFormSearch,
    openDialog,
    handleDelete,
    handleSizeChange,
    handleCurrentChange,
    handleSelectionChange
  };
}
