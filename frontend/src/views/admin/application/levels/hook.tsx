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

// ============================================================================
// 会员等级（全站一套）
// ============================================================================
//
// 等级属于**全站会员体系**：按累计充值金额升级，权益是充值返利、额外多开、赠送换绑。
// 它与「应用」没有归属关系 —— 任何应用都可以要求"等级 ≥ N"，但等级本身只有一套。
//
// 历史沿革：早期实现把等级挂在应用下（`app_uuid` 必填、列表有"所属应用"列），
// 一号通改造后后端 `member_levels` 表已无 `app_uuid` 字段，
// 前端的应用选择器与筛选已全部移除。

export function useMemberLevel() {
  // 保留一个空的查询表单：本页已无筛选条件，但 el-form 仍需要一个 model
  const form = reactive({});

  const dataList = ref([]);
  const loading = ref(true);

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
      label: "额外设备",
      prop: "extra_devices",
      width: 95,
      cellRenderer: ({ row }) => `+${row.extra_devices}`
    },
    {
      label: "额外并发",
      prop: "extra_concurrent",
      width: 95,
      cellRenderer: ({ row }) => `+${row.extra_concurrent}`
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

  async function onSearch() {
    loading.value = true;
    try {
      const { code, data } = await getMemberLevels({});
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
          name: row?.name ?? "",
          level: row?.level ?? 1,
          color: row?.color || "#909399",
          threshold_yuan: row ? row.threshold / 100 : 0,
          rebate_rate: row?.rebate_rate ?? 0,
          extra_devices: row?.extra_devices ?? 0,
          extra_concurrent: row?.extra_concurrent ?? 0,
          extra_rebind_count: row?.extra_rebind_count ?? 0,
          sort: row?.sort ?? 0,
          status: row?.status ?? 1,
          remark: row?.remark ?? ""
        }
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
                  name: cur.name,
                  level: cur.level,
                  color: cur.color,
                  threshold: Math.round(cur.threshold_yuan * 100),
                  rebate_rate: cur.rebate_rate,
                  extra_devices: cur.extra_devices,
            extra_concurrent: cur.extra_concurrent,
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

  onMounted(onSearch);

  return {
    form,
    loading,
    columns,
    dataList,
    onSearch,
    openDialog,
    handleDelete
  };
}
