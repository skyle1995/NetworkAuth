import { reactive, ref, onMounted, h } from "vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import { ElTag, ElMessageBox } from "element-plus";
import editForm from "./form.vue";
import {
  getCardPackages,
  saveCardPackage,
  deleteCardPackage
} from "@/api/admin/cardPackage";
import { getAppsSimple } from "@/api/admin/app";

export function useCardPackage() {
  const form = reactive({
    app_uuid: ""
  });

  const dataList = ref([]);
  const loading = ref(true);
  const apps = ref([]);

  const columns: TableColumnList = [
    { label: "ID", prop: "id", width: 70 },
    { label: "套餐名称", prop: "name", minWidth: 140 },
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
      label: "类型",
      prop: "type",
      width: 90,
      cellRenderer: ({ row }) =>
        h(
          ElTag,
          { type: row.type === 1 ? "warning" : "primary", effect: "light" },
          () => (row.type === 1 ? "点数" : "时长")
        )
    },
    {
      label: "面值",
      prop: "duration",
      minWidth: 110,
      cellRenderer: ({ row }) =>
        row.type === 1
          ? `${row.points} 点`
          : row.duration === -1
            ? "永久"
            : `${row.duration} 分钟`
    },
    {
      label: "售价",
      prop: "price",
      width: 100,
      cellRenderer: ({ row }) => `${(row.price / 100).toFixed(2)} 元`
    },
    { label: "排序", prop: "sort", width: 80 },
    {
      label: "状态",
      prop: "status",
      width: 90,
      cellRenderer: ({ row }) =>
        h(
          ElTag,
          { type: row.status === 1 ? "success" : "info", effect: "light" },
          () => (row.status === 1 ? "启用" : "禁用")
        )
    },
    { label: "备注", prop: "remark", minWidth: 120 },
    { label: "操作", fixed: "right", width: 160, slot: "operation" }
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
      const { code, data } = await getCardPackages({ app_uuid: form.app_uuid });
      if (code === 0) {
        dataList.value = data || [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  }

  // 售价在表单里以「元」编辑，提交时换算为整数「分」
  function openDialog(row?: any) {
    const dialogFormRef = ref();
    const isEdit = !!row;
    addDialog({
      title: isEdit ? "编辑套餐" : "新增套餐",
      props: {
        formInline: {
          uuid: row?.uuid ?? "",
          app_uuid: row?.app_uuid ?? form.app_uuid ?? "",
          name: row?.name ?? "",
          type: row?.type ?? 0,
          duration: row?.duration ?? 43200,
          points: row?.points ?? 100,
          price_yuan: row ? row.price / 100 : 10,
          sort: row?.sort ?? 0,
          status: row?.status ?? 1,
          remark: row?.remark ?? ""
        },
        apps: apps.value
      },
      width: "520px",
      draggable: true,
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
          label: "保存",
          type: "primary",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => {
            const inst = dialogFormRef.value;
            if (!inst) return;
            inst.getRef().validate(async valid => {
              if (!valid) return;
              const cur = options.props.formInline;
              try {
                const { code, msg } = await saveCardPackage({
                  uuid: cur.uuid,
                  app_uuid: cur.app_uuid,
                  name: cur.name,
                  type: cur.type,
                  duration: cur.duration,
                  points: cur.points,
                  price: Math.round(cur.price_yuan * 100),
                  sort: cur.sort,
                  status: cur.status,
                  remark: cur.remark
                });
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
            });
          }
        }
      ]
    });
  }

  async function handleDelete(row: any) {
    try {
      await ElMessageBox.confirm(
        `确认删除套餐「${row.name}」？已售出的卡密不受影响（面值已快照）。`,
        "提示",
        { type: "warning" }
      );
    } catch {
      return;
    }
    try {
      const { code, msg } = await deleteCardPackage({ uuid: row.uuid });
      if (code === 0) {
        message("删除成功", { type: "success" });
        onSearch();
      } else {
        message(msg || "删除失败", { type: "error" });
      }
    } catch (e) {
      console.error(e);
    }
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
    apps,
    onSearch,
    openDialog,
    handleDelete
  };
}
