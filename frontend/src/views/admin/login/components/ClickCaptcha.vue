<script setup lang="ts">
import { ref, onBeforeUnmount } from "vue";
import { getClickCaptcha, verifyClickCaptcha } from "@/api/admin/user";

const emit = defineEmits<{ (e: "success", token: string): void }>();

type ClickData = {
  id: string;
  master_image: string;
  thumb_image: string;
  master_width: number;
  master_height: number;
  dot_count: number;
};

const verified = ref(false);
const visible = ref(false);
const loading = ref(false);
const failed = ref(false);
const status = ref<"idle" | "success" | "fail">("idle");
const data = ref<ClickData | null>(null);
const points = ref<{ x: number; y: number }[]>([]);

function open() {
  if (verified.value) return;
  visible.value = true;
  load();
}

function reset() {
  verified.value = false;
  visible.value = false;
  status.value = "idle";
  points.value = [];
}
defineExpose({ reset });

async function load() {
  status.value = "idle";
  loading.value = true;
  failed.value = false;
  data.value = null;
  points.value = [];
  try {
    const res = await getClickCaptcha();
    if (res.code === 0 && res.data) {
      data.value = res.data as ClickData;
    } else {
      failed.value = true;
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function onImgClick(e: MouseEvent) {
  if (!data.value || status.value === "success") return;
  // 图片按基准宽(300)原尺寸渲染，offsetX/Y 即图片像素坐标
  const x = Math.round(e.offsetX);
  const y = Math.round(e.offsetY);
  points.value.push({ x, y });
  if (points.value.length >= data.value.dot_count) {
    await submit();
  }
}

async function submit() {
  if (!data.value) return;
  try {
    const res = await verifyClickCaptcha({
      id: data.value.id,
      points: points.value
    });
    if (res.code === 0 && res.data?.token) {
      status.value = "success";
      verified.value = true;
      emit("success", res.data.token);
      setTimeout(() => (visible.value = false), 500);
    } else {
      status.value = "fail";
      setTimeout(load, 700);
    }
  } catch {
    status.value = "fail";
    setTimeout(load, 700);
  }
}

onBeforeUnmount(() => (visible.value = false));
</script>

<template>
  <!-- 控件 -->
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

  <!-- 弹窗：依次点击文字 -->
  <el-dialog
    v-model="visible"
    title="安全验证"
    width="344px"
    align-center
    append-to-body
    :close-on-click-modal="false"
  >
    <div class="click-wrap">
      <div class="hint">
        <span>请依次点击：</span>
        <img
          v-if="data"
          :src="data.thumb_image"
          class="thumb"
          draggable="false"
        />
        <span class="refresh-btn" title="换一张" @click="load">↻</span>
      </div>

      <div
        class="click-img"
        :class="status"
        :style="{
          width: (data ? data.master_width : 300) + 'px',
          height: (data ? data.master_height : 200) + 'px'
        }"
      >
        <template v-if="data">
          <img
            :src="data.master_image"
            class="master"
            draggable="false"
            @click="onImgClick"
          />
          <!-- 已点击的序号标记 -->
          <span
            v-for="(p, i) in points"
            :key="i"
            class="marker"
            :style="{ left: p.x + 'px', top: p.y + 'px' }"
            >{{ i + 1 }}</span
          >
        </template>
        <div v-else class="loading-tip">
          {{ loading ? "加载中…" : failed ? "加载失败，点击刷新" : "" }}
        </div>
      </div>

      <div class="foot">
        <span v-if="status === 'success'" class="ok">验证通过</span>
        <span v-else-if="status === 'fail'" class="err">验证失败，请重试</span>
        <span v-else class="tip"
          >已点击 {{ points.length }} / {{ data ? data.dot_count : "-" }}</span
        >
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
/* 控件（与滑块一致） */
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

/* 弹窗内点击图 */
.click-wrap {
  user-select: none;
}
.hint {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.hint .thumb {
  height: 28px;
  margin-left: 6px;
  background: #fff;
  border-radius: 3px;
}
.refresh-btn {
  margin-left: auto;
  font-size: 16px;
  color: var(--el-text-color-secondary);
  cursor: pointer;
}
.click-img {
  position: relative;
  margin: 0 auto;
  overflow: hidden;
  border: 2px solid transparent;
  border-radius: 4px;
}
.click-img.success {
  border-color: var(--el-color-success);
}
.click-img.fail {
  border-color: var(--el-color-danger);
}
.click-img .master {
  display: block;
  width: 100%;
  height: 100%;
  cursor: pointer;
}
.marker {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  font-size: 13px;
  color: #fff;
  pointer-events: none;
  background: var(--el-color-primary);
  border: 2px solid #fff;
  border-radius: 50%;
  transform: translate(-50%, -50%);
}
.loading-tip {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 13px;
  color: #909399;
}
.foot {
  margin-top: 10px;
  font-size: 13px;
  text-align: center;
}
.foot .ok {
  color: var(--el-color-success);
}
.foot .err {
  color: var(--el-color-danger);
}
.foot .tip {
  color: var(--el-text-color-secondary);
}
</style>
