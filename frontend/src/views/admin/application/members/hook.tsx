import { reactive, ref, onMounted, h } from "vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import { ElMessageBox, ElTag } from "element-plus";
import type { PaginationProps } from "@pureadmin/table";
import editForm from "./form.vue";
import durationForm from "./durationForm.vue";
import dataForm from "./dataForm.vue";
import memberDetail from "./memberDetail.vue";
import bindingsView from "./bindingsView.vue";
import blacklistForm from "./blacklistForm.vue";
import batchRechargeForm from "./batchRechargeForm.vue";
import {
  getMembers,
  createMember,
  rechargeMember,
  deductMember,
  resetMemberPassword,
  updateMemberRemark,
  getMemberData,
  updateMemberData,
  setMemberStatus,
  getMemberBindings,
  clearMemberBindings,
  blacklistMember,
  batchRechargeMembers,
  batchDeleteMembers
} from "@/api/admin/member";
import { getAppsSimple } from "@/api/admin/app";

// 来源类型：0注册 1卡密
const TYPE_META: Record<number, { text: string; type: any }> = {
  0: { text: "注册账号", type: "primary" },
  1: { text: "卡密账号", type: "warning" }
};
// 状态：0封停 1正常 2黑名单
const STATUS_META: Record<number, { text: string; type: any }> = {
  0: { text: "已封停", type: "info" },
  1: { text: "正常", type: "success" },
  2: { text: "黑名单", type: "danger" }
};

export function useMember() {
  const form = reactive({
    search: "",
    app_uuid: "",
    type: "",
    status: ""
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
    { type: "selection", width: 55, align: "center" },
    { label: "ID", prop: "id", width: 70 },
    { label: "用户名", prop: "username", minWidth: 140 },
    {
      label: "所属应用",
      prop: "app_uuid",
      minWidth: 130,
      cellRenderer: ({ row }) => {
        const app = apps.value.find(a => a.uuid === row.app_uuid);
        return app ? app.name : "未知应用";
      }
    },
    {
      label: "类型",
      prop: "type",
      minWidth: 100,
      cellRenderer: ({ row }) => {
        const meta = TYPE_META[row.type] ?? { text: "未知", type: "info" };
        return h(ElTag, { type: meta.type, effect: "light" }, () => meta.text);
      }
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
      label: "额度(到期/余额)",
      prop: "expired_at",
      minWidth: 160,
      cellRenderer: ({ row }) =>
        row.mode === 1 ? `${row.points} 点` : row.expired_at
    },
    {
      label: "最近登录",
      prop: "last_login_at",
      minWidth: 160,
      cellRenderer: ({ row }) => row.last_login_at || "—"
    },
    { label: "备注", prop: "remark", minWidth: 120 },
    { label: "创建时间", prop: "created_at", minWidth: 160 },
    { label: "操作", fixed: "right", width: 260, slot: "operation" }
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
      const { code, data, count } = await getMembers({
        page: pagination.currentPage,
        limit: pagination.pageSize,
        search: form.search,
        app_uuid: form.app_uuid,
        type: form.type,
        status: form.status
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
      title: "新增终端账号",
      props: {
        formInline: {
          app_uuid: form.app_uuid || "",
          username: "",
          password: "",
          duration_value: 30,
          duration_unit: "day",
          points: 10,
          remark: ""
        },
        apps: apps.value
      },
      width: "600px",
      draggable: true,
      closeOnClickModal: false,
      contentRenderer: () => h(editForm, { ref: dialogFormRef } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
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
              const { code, msg } = await createMember(
                options.props.formInline
              );
              if (code === 0) {
                message("创建成功", { type: "success" });
                options.visible = false;
                onSearch();
              } else {
                message(msg || "创建失败", { type: "error" });
              }
            });
          }
        }
      ]
    });
  }

  // 充值 / 扣减共用弹窗；按运营模式切换时长/点数输入
  function openDurationDialog(row: any, mode: "recharge" | "deduct") {
    const dialogFormRef = ref();
    const isRecharge = mode === "recharge";
    const pointsMode = row.mode === 1;
    const deductLabel = pointsMode ? "扣点" : "扣时";
    addDialog({
      title: `${isRecharge ? "充值" : deductLabel} - ${row.username}`,
      props: {
        formInline: { duration_value: 30, duration_unit: "day", points: 10 },
        allowPermanent: isRecharge && !pointsMode,
        pointsMode
      },
      width: "420px",
      draggable: true,
      closeOnClickModal: false,
      contentRenderer: () => h(durationForm, { ref: dialogFormRef } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
        },
        {
          label: "确定",
          type: "primary",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            const payload = { id: row.id, ...options.props.formInline };
            const { code, msg } = isRecharge
              ? await rechargeMember(payload)
              : await deductMember(payload);
            if (code === 0) {
              message(isRecharge ? "充值成功" : `${deductLabel}成功`, {
                type: "success"
              });
              options.visible = false;
              onSearch();
            } else {
              message(msg || "操作失败", { type: "error" });
            }
          }
        }
      ]
    });
  }

  async function handleResetPassword(row: any) {
    try {
      const { value } = await ElMessageBox.prompt(
        `为用户 ${row.username} 设置新密码`,
        "重置密码",
        {
          confirmButtonText: "确定",
          cancelButtonText: "取消",
          inputType: "password",
          inputValidator: (v: string) =>
            v && v.length >= 6 ? true : "密码至少 6 位"
        }
      );
      const { code, msg } = await resetMemberPassword({
        id: row.id,
        password: value
      });
      if (code === 0) {
        message("密码已重置", { type: "success" });
      } else {
        message(msg || "重置失败", { type: "error" });
      }
    } catch {
      // cancelled
    }
  }

  async function handleUpdateRemark(row: any) {
    try {
      const { value } = await ElMessageBox.prompt(
        `修改用户 ${row.username} 的备注`,
        "修改备注",
        {
          confirmButtonText: "保存",
          cancelButtonText: "取消",
          inputValue: row.remark || "",
          inputType: "textarea"
        }
      );
      const { code, msg } = await updateMemberRemark({
        id: row.id,
        remark: value ?? ""
      });
      if (code === 0) {
        message("备注已更新", { type: "success" });
        onSearch();
      } else {
        message(msg || "更新失败", { type: "error" });
      }
    } catch {
      // cancelled
    }
  }

  async function handleSetStatus(row: any, status: number) {
    const { code, msg } = await setMemberStatus({ ids: [row.id], status });
    if (code === 0) {
      message("操作成功", { type: "success" });
      onSearch();
    } else {
      message(msg || "操作失败", { type: "error" });
    }
  }

  async function openBindingsDialog(row: any) {
    const { code, data } = await getMemberBindings({ member_uuid: row.uuid });
    const bindings = ref(code === 0 ? data || [] : []);
    addDialog({
      title: `绑定信息 - ${row.username}`,
      width: "720px",
      draggable: true,
      closeOnClickModal: false,
      contentRenderer: () => h(bindingsView, { bindings: bindings.value }),
      footerButtons: [
        {
          label: "关闭",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
        },
        {
          label: "清空绑定",
          type: "danger",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            try {
              await ElMessageBox.confirm(
                `确认清空 ${row.username} 的全部机器码/IP 绑定吗？`,
                "提示",
                { type: "warning" }
              );
              const { code, msg } = await clearMemberBindings({
                uuid: row.uuid
              });
              if (code === 0) {
                message("已清空绑定", { type: "success" });
                options.visible = false;
              } else {
                message(msg || "解绑失败", { type: "error" });
              }
            } catch {
              // cancelled
            }
          }
        }
      ]
    });
  }

  function openDetailDialog(row: any) {
    addDialog({
      title: `用户详情 - ${row.username}`,
      width: "640px",
      draggable: true,
      closeOnClickModal: false,
      hideFooter: true,
      contentRenderer: () => h(memberDetail, { row })
    });
  }

  async function openDataDialog(row: any) {
    const { code, data } = await getMemberData({ id: row.id });
    const dialogFormRef = ref();
    addDialog({
      title: `用户数据 - ${row.username}`,
      props: { formInline: { data: code === 0 ? data?.data || "" : "" } },
      width: "560px",
      draggable: true,
      closeOnClickModal: false,
      contentRenderer: () => h(dataForm, { ref: dialogFormRef } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
        },
        {
          label: "保存",
          type: "primary",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            const { code, msg } = await updateMemberData({
              id: row.id,
              data: options.props.formInline.data
            });
            if (code === 0) {
              message("已保存", { type: "success" });
              options.visible = false;
            } else {
              message(msg || "保存失败", { type: "error" });
            }
          }
        }
      ]
    });
  }

  // 批量加时/加点（维护补偿）：选中账号或全体
  function openBatchRechargeDialog(selectedIds: number[], selectedNum: number) {
    addDialog({
      title: "批量加时/点",
      width: "560px",
      draggable: true,
      closeOnClickModal: false,
      props: {
        formInline: {
          scope: selectedNum > 0 ? "selected" : "all",
          app_uuid: "",
          duration_value: 1,
          duration_unit: "day",
          points: 0,
          selectedNum
        }
      },
      contentRenderer: () => h(batchRechargeForm, { apps: apps.value } as any),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
        },
        {
          label: "确认",
          type: "primary",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            const f = (options.props as any).formInline;
            const hasTime = f.duration_value > 0;
            const hasPoints = f.points > 0;
            if (!hasTime && !hasPoints) {
              message("请填写要增加的时长或点数", { type: "warning" });
              return;
            }
            const payload: any = {
              duration_value: hasTime ? f.duration_value : 0,
              duration_unit: f.duration_unit,
              points: hasPoints ? f.points : 0
            };
            if (f.scope === "all") {
              payload.all = true;
              payload.app_uuid = f.app_uuid;
              try {
                await ElMessageBox.confirm(
                  f.app_uuid
                    ? "确认为该应用下的全体账号批量加时/点吗？"
                    : "确认为『所有应用的全体账号』批量加时/点吗？此操作影响范围很大。",
                  "提示",
                  { type: "warning" }
                );
              } catch {
                return;
              }
            } else {
              if (!selectedIds.length) {
                message("请先勾选账号，或改为『全体账号』", { type: "warning" });
                return;
              }
              payload.ids = selectedIds;
            }
            const { code, msg, data } = await batchRechargeMembers(payload);
            if (code === 0) {
              message(
                `操作完成（命中${data?.total ?? 0}：加时${data?.time_added ?? 0}/加点${data?.points_added ?? 0}/永久跳过${data?.skip_permanent ?? 0}）`,
                { type: "success", duration: 4000 }
              );
              options.visible = false;
              onSearch();
            } else {
              message(msg || "操作失败", { type: "error" });
            }
          }
        }
      ]
    });
  }

  // 拉黑账号：弹窗多选，可附带拉黑 设备/IP/地区
  function openBlacklistDialog(row: any) {
    addDialog({
      title: `拉黑账号 - ${row.username}`,
      width: "520px",
      draggable: true,
      closeOnClickModal: false,
      props: {
        formInline: {
          username: row.username,
          blacklist_device: false,
          blacklist_ip: false,
          blacklist_region: false
        }
      },
      contentRenderer: () => h(blacklistForm),
      footerButtons: [
        {
          label: "取消",
          text: true,
          bg: true,
          btnClick: ({ dialog: { options } }) => (options.visible = false)
        },
        {
          label: "确认拉黑",
          type: "danger",
          text: true,
          bg: true,
          btnClick: async ({ dialog: { options } }) => {
            const f = (options.props as any).formInline;
            const { code, msg, data } = await blacklistMember({
              id: row.id,
              blacklist_device: f.blacklist_device,
              blacklist_ip: f.blacklist_ip,
              blacklist_region: f.blacklist_region
            });
            if (code === 0) {
              const extra =
                data && (data.device || data.ip || data.region)
                  ? `（设备+${data.device} / IP+${data.ip} / 地区+${data.region}）`
                  : "";
              message(`已拉黑${extra}`, { type: "success" });
              options.visible = false;
              onSearch();
            } else {
              message(msg || "拉黑失败", { type: "error" });
            }
          }
        }
      ]
    });
  }

  async function handleDelete(row: any) {
    try {
      await ElMessageBox.confirm(
        `确认删除用户 <strong style="color:red">${row.username}</strong> 吗？<br><span style="color:red;font-size:12px;">将同时清除其绑定记录，且不可恢复！</span>`,
        "提示",
        { type: "warning", dangerouslyUseHTMLString: true }
      );
      const { code, msg } = await batchDeleteMembers({ ids: [row.id] });
      if (code === 0) {
        message("删除成功", { type: "success" });
        onSearch();
      } else {
        message(msg || "删除失败", { type: "error" });
      }
    } catch {
      // cancelled
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
    openDurationDialog,
    handleResetPassword,
    handleUpdateRemark,
    handleSetStatus,
    openBindingsDialog,
    openDataDialog,
    openDetailDialog,
    openBlacklistDialog,
    openBatchRechargeDialog,
    handleDelete,
    handleSizeChange,
    handleCurrentChange
  };
}
