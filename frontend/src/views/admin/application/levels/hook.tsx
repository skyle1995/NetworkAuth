import { reactive, ref, onMounted, h } from "vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import { ElTag, ElMessageBox } from "element-plus";
import editForm from "./form.vue";
import {
  getMemberLevels,
  saveMemberLevel,
  deleteMemberLevel
} from "@/api/admin/memberLevel";
import { getAppsSimple } from "@/api/admin/app";

export function useMemberLevel() {
  const form = reactive({
    app_uuid: ""
  });

  const dataList = ref([]);
  const loading = ref(true);
  const apps = ref([]);

  const columns: TableColumnList = [
    { label: "ID", prop: "id", width: 70 },
    {
      label: "等级名称",
      prop: "name",
      minWidth: 140,
      cellRenderer: ({ row }) =>
        h(
          ElTag,
          {
            effect: "plain",
            style: {
              color: row.color || "#909399",
              borderColor: row.color || "#909399"
            }
          },
          () => row.name
        )
    },
    { label: "权限等级", prop: "level", width: 90 },
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
      label: "累充门槛",
      prop: "threshold",
      minWidth: 110,
      cellRenderer: ({ row }) => `${(row.threshold / 100).toFixed(2)} 元`
    },
    {
      label: "充值返利",
      prop: "rebate_rate",
      width: 90,
      cellRenderer: ({ row }) => `${row.rebate_rate}%`
    },
    {
      label: "额外多开",
      prop: "extra_multi_open",
      width: 90,
      cellRenderer: ({ row }) => `+${row.extra_multi_open}`
    },
    {
      label: "赠送换绑",
      prop: "extra_rebind_count",
      width: 90,
      cellRenderer: ({ row }) => `+${row.extra_rebind_count}`
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
      const { code, data } = await getMemberLevels({ app_uuid: form.app_uuid });
      if (code === 0) {
        dataList.value = data || [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  }

  // 累充门槛在表单里以「元」编辑，提交时换算为整数「分」
  function openDialog(row?: any) {
    const dialogFormRef = ref();
    const isEdit = !!row;
    addDialog({
      title: isEdit ? "编辑会员等级" : "新增会员等级",
      props: {
        formInline: {
          uuid: row?.uuid ?? "",
          app_uuid: row?.app_uuid ?? form.app_uuid ?? "",
          name: row?.name ?? "",
          level: row?.level ?? 1,
          color: row?.color || "#909399",
          threshold_yuan: row ? row.threshold / 100 : 0,
          rebate_rate: row?.rebate_rate ?? 0,
          extra_multi_open: row?.extra_multi_open ?? 0,
          extra_rebind_count: row?.extra_rebind_count ?? 0,
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
                const { code, msg } = await saveMemberLevel({
                  uuid: cur.uuid,
                  app_uuid: cur.app_uuid,
                  name: cur.name,
                  level: cur.level,
                  color: cur.color,
                  threshold: Math.round(cur.threshold_yuan * 100),
                  rebate_rate: cur.rebate_rate,
                  extra_multi_open: cur.extra_multi_open,
                  extra_rebind_count: cur.extra_rebind_count,
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
        `确认删除等级「${row.name}」？该等级下的账号将被清除等级归属。`,
        "提示",
        { type: "warning" }
      );
    } catch {
      return;
    }
    try {
      const { code, msg } = await deleteMemberLevel({ uuid: row.uuid });
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
