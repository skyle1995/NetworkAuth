<script setup lang="ts">
import { ref, reactive, onBeforeUnmount } from "vue";
import { getSlideCaptcha, verifySlideCaptcha } from "@/api/admin/user";

// 校验通过后向父组件抛出一次性令牌
const emit = defineEmits<{ (e: "success", token: string): void }>();

type SlideData = {
  id: string;
  master_image: string;
  tile_image: string;
  master_width: number;
  master_height: number;
  tile_x: number;
  tile_y: number;
  tile_width: number;
  tile_height: number;
};

const HANDLE_W = 40; // 滑块把手宽度(px)
const verified = ref(false); // 控件是否已通过验证
const visible = ref(false); // 弹窗是否打开
const loading = ref(false);
const failed = ref(false);
const status = ref<"idle" | "moving" | "success" | "fail">("idle");
const data = ref<SlideData | null>(null);

// 弹窗内拼图按后端基准宽(300)原尺寸渲染，坐标 1:1，无需缩放
const drag = reactive({ active: false, startX: 0, handleX: 0, tileX: 0 });

// 打开弹窗并出题（未验证时点击控件触发）
function open() {
  if (verified.value) return;
  visible.value = true;
  load();
}

// 供父组件在登录失败/令牌失效时重置控件
function reset() {
  verified.value = false;
  visible.value = false;
  status.value = "idle";
  drag.handleX = 0;
}
defineExpose({ reset });

async function load() {
  status.value = "idle";
  loading.value = true;
  failed.value = false;
  data.value = null;
  drag.active = false;
  drag.handleX = 0;
  try {
    const res = await getSlideCaptcha();
    if (res.code === 0 && res.data) {
      data.value = res.data as SlideData;
      drag.tileX = res.data.tile_x;
    } else {
      failed.value = true;
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function clientX(e: MouseEvent | TouchEvent): number {
  return "touches" in e ? e.touches[0].clientX : (e as MouseEvent).clientX;
}

function onDown(e: MouseEvent | TouchEvent) {
  if (!data.value || status.value === "success" || loading.value) return;
  drag.active = true;
  drag.startX = clientX(e);
  status.value = "moving";
  window.addEventListener("mousemove", onMove);
  window.addEventListener("mouseup", onUp);
  window.addEventListener("touchmove", onMove, { passive: false });
  window.addEventListener("touchend", onUp);
}

function onMove(e: MouseEvent | TouchEvent) {
  if (!drag.active || !data.value) return;
  if ("preventDefault" in e) e.preventDefault();
  const d = data.value;
  const max = d.master_width - HANDLE_W;
  let dx = clientX(e) - drag.startX;
  dx = Math.max(0, Math.min(dx, max));
  drag.handleX = dx;
  drag.tileX = Math.max(0, Math.min(d.tile_x + dx, d.master_width - d.tile_width));
}

async function onUp() {
  window.removeEventListener("mousemove", onMove);
  window.removeEventListener("mouseup", onUp);
  window.removeEventListener("touchmove", onMove);
  window.removeEventListener("touchend", onUp);
  if (!drag.active || !data.value) return;
  drag.active = false;
  try {
    const res = await verifySlideCaptcha({
      id: data.value.id,
      x: Math.round(drag.tileX)
    });
    if (res.code === 0 && res.data?.token) {
      status.value = "success";
      verified.value = true;
      emit("success", res.data.token);
      // 通过后短暂停留展示"验证成功"再关闭弹窗
      setTimeout(() => (visible.value = false), 500);
    } else {
      status.value = "fail";
      setTimeout(load, 600);
    }
  } catch {
    status.value = "fail";
    setTimeout(load, 600);
  }
}

onBeforeUnmount(onUp);
</script>

<template>
  <!-- 控件：点击进行验证 / 已验证 -->
  <div class="captcha-control" :class="{ verified }" @click="open">
    <span class="cc-main">
      <span v-if="verified" class="cc-check">
        <svg viewBox="0 0 24 24" width="17" height="17">
          <path
            fill="currentColor"
            d="M9 16.17 4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"
          />
        </svg>
      </span>
      <span v-else class="cc-ring" />
      <span class="cc-text">{{ verified ? "验证成功" : "点击进行验证" }}</span>
    </span>
    <span class="cc-badge">
      <svg viewBox="0 0 24 24" width="14" height="14">
        <path
          fill="currentColor"
          d="M12 1 3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5zm-2 16-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9z"
        />
      </svg>
      安全验证
    </span>
  </div>

  <!-- 弹窗滑块 -->
  <el-dialog
    v-model="visible"
    title="安全验证"
    width="344px"
    align-center
    append-to-body
    :close-on-click-modal="false"
  >
    <div class="puzzle-wrap">
      <div
        class="slide-img"
        :style="{
          width: (data ? data.master_width : 300) + 'px',
          height: (data ? data.master_height : 180) + 'px'
        }"
      >
        <template v-if="data">
          <img :src="data.master_image" class="master" draggable="false" />
          <img
            :src="data.tile_image"
            class="tile"
            draggable="false"
            :style="{ left: drag.tileX + 'px', top: data.tile_y + 'px' }"
          />
        </template>
        <div v-else class="loading-tip">
          {{ loading ? "加载中…" : failed ? "加载失败" : "" }}
        </div>
        <div class="refresh" title="换一张" @click="load">↻</div>
      </div>

      <div class="slide-track" :class="status">
        <span v-if="status === 'success'" class="tip ok">验证通过</span>
        <span v-else-if="failed" class="tip err" @click="load"
          >加载失败，点击刷新</span
        >
        <span v-else class="tip">向右拖动滑块完成拼图</span>
        <div
          class="handle"
          :style="{ left: drag.handleX + 'px' }"
          @mousedown="onDown"
          @touchstart="onDown"
        >
          <span v-if="status === 'success'">✓</span>
          <span v-else-if="status === 'fail'">✕</span>
          <span v-else>➜</span>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
/* 控件 */
.captcha-control {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 44px;
  padding: 0 14px;
  overflow: hidden;
  font-size: 14px;
  color: var(--el-text-color-regular);
  cursor: pointer;
  background: var(--el-fill-color-blank);
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  box-shadow: 0 1px 2px rgb(0 0 0 / 4%);
  transition:
    border-color 0.25s,
    box-shadow 0.25s,
    background 0.25s;
}
.captcha-control:hover {
  border-color: var(--el-color-primary);
  box-shadow: 0 4px 12px
    color-mix(in srgb, var(--el-color-primary) 22%, transparent);
}
/* 未验证时一道柔和的微光扫过，提示可点击 */
.captcha-control:not(.verified)::before {
  position: absolute;
  inset: 0;
  content: "";
  background: linear-gradient(
    100deg,
    transparent 30%,
    color-mix(in srgb, var(--el-color-primary) 14%, transparent) 50%,
    transparent 70%
  );
  transform: translateX(-100%);
  animation: cc-shimmer 2.6s ease-in-out infinite;
}
@keyframes cc-shimmer {
  0% {
    transform: translateX(-100%);
  }
  60%,
  100% {
    transform: translateX(100%);
  }
}
.captcha-control.verified {
  color: var(--el-color-success);
  cursor: default;
  background: var(--el-color-success-light-9);
  border-color: var(--el-color-success-light-5);
  box-shadow: none;
}

.cc-main {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
}
.cc-text {
  font-weight: 500;
}
/* 未验证：脉冲圆环 */
.cc-ring {
  position: relative;
  width: 16px;
  height: 16px;
  margin-right: 10px;
  border: 2px solid var(--el-color-primary);
  border-radius: 50%;
}
.cc-ring::after {
  position: absolute;
  inset: -2px;
  content: "";
  border: 2px solid var(--el-color-primary);
  border-radius: 50%;
  opacity: 0.6;
  animation: cc-pulse 1.6s ease-out infinite;
}
@keyframes cc-pulse {
  0% {
    opacity: 0.6;
    transform: scale(1);
  }
  100% {
    opacity: 0;
    transform: scale(1.9);
  }
}
/* 已验证：对勾弹入 */
.cc-check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-right: 10px;
  color: #fff;
  background: var(--el-color-success);
  border-radius: 50%;
  animation: cc-pop 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes cc-pop {
  0% {
    transform: scale(0);
  }
  100% {
    transform: scale(1);
  }
}
/* 右侧安全徽标 */
.cc-badge {
  position: relative;
  z-index: 1;
  display: flex;
  gap: 4px;
  align-items: center;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.captcha-control.verified .cc-badge {
  color: var(--el-color-success);
}

/* 弹窗内拼图 */
.puzzle-wrap {
  user-select: none;
}
.slide-img {
  position: relative;
  margin: 0 auto;
  overflow: hidden;
  background: #f2f3f5;
  border-radius: 4px;
}
.slide-img .master {
  display: block;
  width: 100%;
  height: 100%;
}
.slide-img .tile {
  position: absolute;
  cursor: grab;
}
.loading-tip {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 13px;
  color: #909399;
}
.refresh {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 24px;
  height: 24px;
  line-height: 24px;
  color: #fff;
  text-align: center;
  cursor: pointer;
  background: rgb(0 0 0 / 35%);
  border-radius: 4px;
}
.slide-track {
  position: relative;
  height: 40px;
  margin-top: 12px;
  line-height: 40px;
  color: #909399;
  text-align: center;
  background: #f2f3f5;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.slide-track.success {
  color: #529b2e;
  background: #f0f9eb;
  border-color: #a4da89;
}
.slide-track.fail {
  color: #c45656;
  background: #fef0f0;
  border-color: #f9a7a7;
}
.tip {
  font-size: 13px;
}
.tip.err {
  color: #c45656;
  cursor: pointer;
}
.handle {
  position: absolute;
  top: -1px;
  left: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  color: #fff;
  cursor: grab;
  background: var(--el-color-primary, #409eff);
  border-radius: 4px;
}
.slide-track.success .handle {
  background: #67c23a;
}
.slide-track.fail .handle {
  background: #f56c6c;
}
</style>
