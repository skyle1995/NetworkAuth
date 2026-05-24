<script setup lang="ts">
import { getConfig } from "@/config";
import {
  Monitor,
  Connection,
  Key,
  DataLine,
  SetUp,
  Lock
} from "@element-plus/icons-vue";

defineOptions({
  name: "HomeMain"
});

const features = [
  {
    title: "应用接入管理",
    icon: Monitor,
    description:
      "围绕应用维度统一维护接入配置、授权标识与版本策略，为每个项目提供独立的网络鉴权入口，帮助业务系统快速接入并集中管理运行参数。",
    color: "#409eff"
  },
  {
    title: "接口与能力编排",
    icon: Connection,
    description:
      "将开放接口、逻辑函数与变量配置统一纳入平台编排体系，支持按应用动态组合输出能力，满足多项目、多渠道下的接口管理与逻辑复用需求。",
    color: "#67c23a"
  },
  {
    title: "动态变量下发",
    icon: Key,
    description:
      "支持将关键变量、鉴权参数与业务配置按应用进行动态下发，客户端可按需请求最新配置内容，减少硬编码带来的维护成本与版本切换风险。",
    color: "#e6a23c"
  },
  {
    title: "函数逻辑控制",
    icon: SetUp,
    description:
      "通过后台集中维护函数开关、逻辑节点与执行规则，让服务端能够在不重新发版的前提下灵活调整部分业务流程，实现更高效的远程控制能力。",
    color: "#f56c6c"
  },
  {
    title: "运行状态与数据观测",
    icon: DataLine,
    description:
      "提供应用、接口与配置项的统一观测视角，便于运营和开发快速确认配置是否生效、调用是否正常，并为后续扩展统计与监控能力保留空间。",
    color: "#909399"
  },
  {
    title: "安全控制与审计",
    icon: Lock,
    description:
      "结合登录日志、操作日志与后台权限体系，对网络授权链路形成可追溯审计能力，帮助平台在动态分发与远程控制场景下保持稳定与安全。",
    color: "#8e44ad"
  }
];
</script>

<template>
  <div class="home-container">
    <div class="hero-section">
      <h1 class="hero-title">{{ getConfig()?.Title || "加载中..." }}</h1>
      <p class="hero-subtitle">
        {{
          getConfig()?.Description ||
          "网络授权服务（NetworkAuth）专注于应用鉴权、接口管理与动态逻辑分发，为业务系统提供统一的远程配置与控制能力。"
        }}
      </p>
    </div>

    <div class="features-section">
      <h2 class="section-title">核心功能特性</h2>
      <div class="features-grid">
        <el-card
          v-for="(item, index) in features"
          :key="index"
          class="feature-card"
          shadow="hover"
        >
          <div
            class="feature-icon"
            :style="{ color: item.color, backgroundColor: `${item.color}15` }"
          >
            <el-icon :size="32"><component :is="item.icon" /></el-icon>
          </div>
          <h3 class="feature-title">{{ item.title }}</h3>
          <p class="feature-desc">{{ item.description }}</p>
        </el-card>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.home-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
}

.hero-section {
  text-align: center;
  width: 100%;
  padding: 20px 0 40px;
}

.hero-title {
  margin-bottom: 20px;
  font-size: 48px;
  font-weight: 800;
  background: linear-gradient(
    135deg,
    var(--el-color-primary),
    var(--el-color-primary-light-3)
  );
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.hero-subtitle {
  max-width: 600px;
  margin-left: auto;
  margin-right: auto;
  font-size: 20px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.features-section {
  width: 100%;
  padding-bottom: 60px;
}

.section-title {
  position: relative;
  margin-bottom: 50px;
  text-align: center;
  font-size: 32px;
  font-weight: bold;
  color: var(--el-text-color-primary);

  &::after {
    content: "";
    position: absolute;
    bottom: -15px;
    left: 50%;
    transform: translateX(-50%);
    width: 60px;
    height: 4px;
    border-radius: 2px;
    background-color: var(--el-color-primary);
  }
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 30px;
  width: 100%;
}

.feature-card {
  height: 100%;
  border: none;
  border-radius: 12px;
  background-color: var(--el-bg-color-overlay);
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease;

  &:hover {
    transform: translateY(-5px);
    box-shadow: 0 10px 20px rgba(0, 0, 0, 0.08) !important;
  }

  :deep(.el-card__body) {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    padding: 30px;
  }
}

.feature-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  margin-bottom: 24px;
  border-radius: 16px;
  transition: transform 0.3s ease;

  .feature-card:hover & {
    transform: scale(1.1);
  }
}

.feature-title {
  margin-bottom: 16px;
  font-size: 20px;
  font-weight: bold;
  color: var(--el-text-color-primary);
}

.feature-desc {
  margin: 0;
  font-size: 15px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}

@media screen and (max-width: 768px) {
  .hero-title {
    font-size: 36px;
  }

  .hero-subtitle {
    padding: 0 20px;
    font-size: 16px;
  }

  .features-grid {
    grid-template-columns: 1fr;
  }
}
</style>
