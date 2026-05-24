<script setup lang="ts">
import { useRouter } from "vue-router";
import noServer from "@/assets/status/500.svg?component";

defineOptions({
  name: "503"
});

const router = useRouter();
</script>

<template>
  <div
    class="flex flex-col md:flex-row justify-center items-center min-h-full w-full p-4 md:p-0"
  >
    <noServer />
    <div class="mt-8 md:ml-12 md:mt-0 text-center md:text-left">
      <p
        v-motion
        class="font-medium text-4xl mb-4! dark:text-white"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 80
          }
        }"
      >
        503
      </p>
      <p
        v-motion
        class="text-xl mb-4! text-gray-500"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 120
          }
        }"
      >
        系统正在进行升级维护。<br />
        请稍后访问，给您带来的不便敬请谅解。
      </p>
    </div>
  </div>
</template>

<style scoped>
.main-content {
  margin: 0 !important;
}
</style>
