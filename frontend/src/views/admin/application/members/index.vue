<script setup lang="ts">
import { ref, computed } from "vue";
import { useMember } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { ElMessageBox, ElMessage } from "element-plus";
import { batchDeleteMembers, setMemberStatus } from "@/api/admin/member";

defineOptions({
  name: "Members"
});

const formRef = ref();
const tableRef = ref();

const {
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
  openProfileDialog,
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
} = useMember();

const selectedRows = ref<any[]>([]);
const selectedNum = computed(() => selectedRows.value.length);
const selectedIds = computed(() => selectedRows.value.map(r => r.id));

function handleSelectionChange(val: any[]) {
  selectedRows.value = val;
}

// 行内“更多”下拉命令分发
function onRowCommand(command: string, row: any) {
  switch (command) {
    case "detail":
      openDetailDialog(row);
      break;
    case "resetPwd":
      handleResetPassword(row);
      break;
    case "remark":
      handleUpdateRemark(row);
      break;
    case "editProfile":
      openProfileDialog(row);
      break;
    case "bindings":
      openBindingsDialog(row);
      break;
    case "userData":
      openDataDialog(row);
      break;
    case "normal":
      handleSetStatus(row, 1);
      break;
    case "disable":
      handleSetStatus(row, 0);
      break;
    case "black":
      openBlacklistDialog(row);
      break;
    case "unblack":
      handleSetStatus(row, 1);
      break;
    case "delete":
      handleDelete(row);
      break;
  }
}

async function onBatchStatus(status: number) {
  if (selectedNum.value === 0) return;
  const { code, msg } = await setMemberStatus({
    ids: selectedIds.value,
    status
  });
  if (code === 0) {
    ElMessage.success("操作成功");
    onSearch();
  } else {
    ElMessage.error(msg || "操作失败");
  }
}

async function onBatchDel() {
  if (selectedNum.value === 0) return;
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedNum.value} 个用户吗？<br><span style="color:red;font-size:12px;">将同时清除其绑定记录，且不可恢复！</span>`,
      "提示",
      { type: "warning", dangerouslyUseHTMLString: true }
    );
    const { code, msg } = await batchDeleteMembers({ ids: selectedIds.value });
    if (code === 0) {
      ElMessage.success("批量删除成功");
      onSearch();
    } else {
      ElMessage.error(msg || "批量删除失败");
    }
  } catch (e) {
    // cancelled
  }
}
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="用户名" prop="search">
        <el-input
          v-model="form.search"
          placeholder="精确用户名"
          clearable
          class="w-[160px]!"
          @keyup.enter="onSearch"
        />
      </el-form-item>

      <el-form-item label="所属应用" prop="app_uuid">
        <el-select
          v-model="form.app_uuid"
          placeholder="请选择应用"
          clearable
          class="w-[160px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option
            v-for="app in apps"
            :key="app.uuid"
            :label="app.name"
            :value="app.uuid"
          />
        </el-select>
      </el-form-item>

      <el-form-item label="类型" prop="type">
        <el-select
          v-model="form.type"
          placeholder="全部"
          clearable
          class="w-[130px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option label="注册账号" :value="0" />
          <el-option label="卡密账号" :value="1" />
        </el-select>
      </el-form-item>

      <el-form-item label="状态" prop="status">
        <el-select
          v-model="form.status"
          placeholder="全部"
          clearable
          class="w-[130px]!"
          @change="onSearch"
        >
          <el-option label="全部" value="" />
          <el-option label="正常" :value="1" />
          <el-option label="已封停" :value="0" />
          <el-option label="黑名单" :value="2" />
        </el-select>
      </el-form-item>

      <el-form-item>
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:search')"
          :loading="loading"
          @click="onSearch"
        >
          搜索
        </el-button>
        <el-button
          :icon="useRenderIcon('ep:refresh')"
          @click="resetFormSearch(formRef)"
        >
          重置
        </el-button>
      </el-form-item>
    </el-form>

    <el-card shadow="never" class="table-wrapper mt-4">
      <div class="toolbar mb-4 px-2 overflow-x-auto whitespace-nowrap pb-2">
        <el-button
          type="primary"
          :icon="useRenderIcon('ep:plus')"
          @click="openCreateDialog"
        >
          新增用户
        </el-button>
        <el-dropdown :disabled="selectedNum === 0" @command="onBatchStatus">
          <el-button type="warning" :disabled="selectedNum === 0">
            批量状态<el-icon class="el-icon--right"
              ><component :is="useRenderIcon('ep:arrow-down')"
            /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item :command="1">设为正常</el-dropdown-item>
              <el-dropdown-item :command="0">封停账号</el-dropdown-item>
              <el-dropdown-item :command="2">拉黑账号</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button
          type="success"
          :icon="useRenderIcon('ep:timer')"
          @click="openBatchRechargeDialog(selectedIds, selectedNum)"
        >
          批量加时/点
        </el-button>
        <el-button
          type="danger"
          :icon="useRenderIcon('ep:delete')"
          :disabled="selectedNum === 0"
          @click="onBatchDel"
        >
          批量删除
        </el-button>
      </div>

      <PureTableBar title="终端账号管理" :columns="columns" @refresh="onSearch">
        <template v-slot="{ size, dynamicColumns }">
          <pure-table
            ref="tableRef"
            row-key="id"
            align-whole="center"
            header-align="center"
            table-layout="auto"
            show-overflow-tooltip
            border
            :loading="loading"
            :size="size"
            :data="dataList"
            :columns="dynamicColumns"
            :header-cell-style="{
              background: 'var(--el-fill-color-light)',
              color: 'var(--el-text-color-primary)'
            }"
            class="w-full"
            @selection-change="handleSelectionChange"
          >
            <template #operation="{ row }">
              <div class="flex items-center justify-center">
                <el-button
                  class="reset-margin"
                  link
                  type="primary"
                  :size="size"
                  :icon="useRenderIcon('ep:wallet')"
                  @click="openDurationDialog(row, 'recharge')"
                >
                  充值
                </el-button>
                <el-button
                  class="reset-margin ml-2"
                  link
                  type="warning"
                  :size="size"
                  :icon="useRenderIcon('ep:remove')"
                  @click="openDurationDialog(row, 'deduct')"
                >
                  扣减
                </el-button>
                <el-dropdown
                  class="ml-2"
                  @command="cmd => onRowCommand(cmd, row)"
                >
                  <el-button
                    class="reset-margin"
                    link
                    type="primary"
                    :size="size"
                  >
                    更多
                    <el-icon class="el-icon--right">
                      <component :is="useRenderIcon('ep:arrow-down')" />
                    </el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="detail">详情</el-dropdown-item>
                      <el-dropdown-item command="resetPwd"
                        >重置密码</el-dropdown-item
                      >
                      <el-dropdown-item command="remark"
                        >修改备注</el-dropdown-item
                      >
                      <el-dropdown-item command="editProfile"
                        >编辑账号</el-dropdown-item
                      >
                      <el-dropdown-item command="bindings"
                        >绑定信息</el-dropdown-item
                      >
                      <el-dropdown-item command="userData"
                        >用户数据</el-dropdown-item
                      >
                      <el-dropdown-item
                        v-if="row.status === 0"
                        command="normal"
                        divided
                        >设为正常</el-dropdown-item
                      >
                      <el-dropdown-item
                        v-if="row.status === 1"
                        command="disable"
                        divided
                        >封停账号</el-dropdown-item
                      >
                      <el-dropdown-item v-if="row.status !== 2" command="black"
                        >拉黑账号</el-dropdown-item
                      >
                      <el-dropdown-item v-else command="unblack" divided
                        >解除拉黑</el-dropdown-item
                      >
                      <el-dropdown-item command="delete" divided
                        >删除</el-dropdown-item
                      >
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </pure-table>
          <div class="flex mt-4 w-full overflow-x-auto">
            <div class="ml-auto shrink-0">
              <el-pagination
                v-model:current-page="pagination.currentPage"
                v-model:page-size="pagination.pageSize"
                :page-sizes="[10, 20, 30, 50, 100, 200, 500, 1000]"
                :background="true"
                layout="total, sizes, prev, pager, next, jumper"
                :total="pagination.total"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </div>
        </template>
      </PureTableBar>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
:deep(.el-dropdown-menu__item i) {
  margin: 0;
}

// 操作列“更多”：点击展开后触发按钮保留 :focus-visible，
// 会残留一圈蓝色 outline（看起来像蓝色背景），这里去掉
:deep(.el-dropdown:focus-visible),
:deep(.el-dropdown .el-button:focus-visible) {
  outline: none;
}

.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}

.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
