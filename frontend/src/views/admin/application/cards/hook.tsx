import { reactive, ref, onMounted, h } from "vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import { ElTag } from "element-plus";
import type { PaginationProps } from "@pureadmin/table";
import editForm from "./form.vue";
import {
  getCards,
  createCards,
  freezeCards,
  unfreezeCards
} from "@/api/admin/card";
import { getAppsSimple } from "@/api/admin/app";

// 卡密状态：0未使用 1已使用 2已冻结
const STATUS_META: Record<number, { text: string; type: any }> = {
  0: { text: "未使用", type: "primary" },
  1: { text: "已使用", type: "success" },
  2: { text: "已冻结", type: "danger" }
};

export function useCard() {
  const form = reactive({
    search: "",
    app_uuid: "",
    status: "",
    batch_no: ""
  });

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
      align: "center"
    },
    {
      label: "ID",
      prop: "id",
      width: 70
    },
    {
      label: "卡号",
      prop: "card_no",
      minWidth: 200
    },
    {
      label: "所属应用",
      prop: "app_uuid",
      minWidth: 140,
      cellRenderer: ({ row }) => {
        const app = apps.value.find(a => a.uuid === row.app_uuid);
        return app ? app.name : "未知应用";
      }
    },
    {
      label: "面值",
      prop: "duration_text",
      minWidth: 100,
      cellRenderer: ({ row }) =>
        row.points > 0 ? `${row.points} 点` : row.duration_text
    },
    {
      label: "状态",
      prop: "status",
      minWidth: 90,
      cellRenderer: ({ row }) => {
        const meta = STATUS_META[row.status] ?? { text: "未知", type: "info" };
        return h(ElTag, { type: meta.type, effect: "light" }, () => meta.text);
      }
    },
    {
      label: "批次号",
      prop: "batch_no",
      minWidth: 140
    },
    {
      label: "核销时间",
      prop: "used_at",
      minWidth: 160,
      cellRenderer: ({ row }) => row.used_at || "—"
    },
    {
      label: "备注",
      prop: "remark",
      minWidth: 120
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
      const { code, data, count } = await getCards({
        page: pagination.currentPage,
        limit: pagination.pageSize,
        search: form.search,
        app_uuid: form.app_uuid,
        status: form.status,
        batch_no: form.batch_no
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

  function openCreateDialog() {
    const dialogFormRef = ref();
    addDialog({
      title: "批量制卡",
      props: {
        formInline: {
          app_uuid: form.app_uuid || "",
          prefix: "",
          length: 16,
          count: 10,
          duration_value: 30,
          duration_unit: "day",
          points: 10,
          remark: ""
        },
        apps: apps.value
      },
      width: "520px",
      draggable: true,
      fullscreenIcon: true,
      closeOnClickModal: false,
      contentRenderer: () => h(editForm, { ref: dialogFormRef } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => {
            options.visible = false;
          }
        },
        {
          label: "生成",
          type: "primary",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => {
            const formRefInstance = dialogFormRef.value;
            if (!formRefInstance) return;
            formRefInstance.getRef().validate(async valid => {
              if (!valid) return;
              try {
                const curData = options.props.formInline;
                const { code, msg, data } = await createCards(curData);
                if (code === 0) {
                  message(`制卡成功，共生成 ${data?.count ?? ""} 张`, {
                    type: "success"
                  });
                  options.visible = false;
                  onSearch();
                } else {
                  message(msg || "制卡失败", { type: "error" });
                }
              } catch (e) {
                console.error(e);
              }
            });
          }
        }
      ]
    });
  }

  async function handleFreeze(row) {
    try {
      const { code, msg } = await freezeCards({ ids: [row.id] });
      if (code === 0) {
        message("冻结成功", { type: "success" });
        onSearch();
      } else {
        message(msg || "冻结失败", { type: "error" });
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function handleUnfreeze(row) {
    try {
      const { code, msg } = await unfreezeCards({ ids: [row.id] });
      if (code === 0) {
        message("解冻成功", { type: "success" });
        onSearch();
      } else {
        message(msg || "解冻失败", { type: "error" });
      }
    } catch (e) {
      console.error(e);
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

  return {
    form,
    loading,
    columns,
    dataList,
    pagination,
    apps,
    onSearch,
    resetFormSearch,
    openCreateDialog,
    handleFreeze,
    handleUnfreeze,
    handleSizeChange,
    handleCurrentChange
  };
}
