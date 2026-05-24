<script setup lang="ts">
import { ref, reactive, watch, nextTick } from "vue";
import { formRules } from "./rule";
import Codemirror from "codemirror-editor-vue3";
import type { Editor, EditorConfiguration } from "codemirror";

import "codemirror/lib/codemirror.css";
import "codemirror/theme/material-darker.css";
import "codemirror/addon/hint/show-hint.css";
import "codemirror/addon/hint/show-hint";
import "codemirror/addon/hint/javascript-hint.js";
import "codemirror/mode/javascript/javascript.js";
import "codemirror/addon/display/autorefresh.js";

import { useDark } from "@pureadmin/utils";

export interface FormProps {
  formInline: {
    id?: number;
    uuid?: string;
    alias: string;
    app_uuid: string;
    code: string;
    remark: string;
  };
  apps: Array<{ id: number; uuid: string; name: string }>;
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    alias: "",
    app_uuid: "0",
    code: "",
    remark: ""
  }),
  apps: () => []
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);
const { isDark } = useDark();

const cminstance = ref<Editor | null>(null);
const cmOptions: EditorConfiguration = reactive({
  mode: "javascript",
  theme: isDark.value ? "material-darker" : "default",
  tabSize: 4,
  readOnly: false,
  autofocus: true,
  autoRefresh: true,
  lineNumbers: true,
  lineWiseCopyCut: true,
  gutters: ["CodeMirror-lint-markers"],
  lint: true,
  extraKeys: {
    Ctrl: "autocomplete",
    Tab: "autocomplete"
  },
  hintOptions: {
    completeSingle: false
  }
});

const onReady = (cm: Editor) => {
  cminstance.value = cm;
  cm.on("keypress", () => cm.showHint());
};

watch(
  () => isDark.value,
  async newVal => {
    await nextTick();
    newVal
      ? cminstance.value.setOption("theme", "material-darker")
      : cminstance.value.setOption("theme", "default");
  }
);

function getRef() {
  return ruleFormRef.value;
}

defineExpose({ getRef });
</script>

<template>
  <el-form
    ref="ruleFormRef"
    :model="newFormInline"
    :rules="formRules"
    label-width="100px"
  >
    <el-row>
      <el-col :span="24">
        <el-form-item label="函数别名" prop="alias">
          <el-input
            v-model="newFormInline.alias"
            placeholder="请输入函数别名 (字母开头，字母数字组合)"
            :disabled="!!newFormInline.id"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="所属应用" prop="app_uuid">
          <el-select
            v-model="newFormInline.app_uuid"
            placeholder="请选择所属应用 (默认全局函数)"
            class="w-full"
            clearable
          >
            <el-option label="全局函数" value="0" />
            <el-option
              v-for="app in apps"
              :key="app.uuid"
              :label="`${app.name} (ID: ${app.id})`"
              :value="app.uuid"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="函数代码" prop="code">
          <Codemirror
            v-model:value="newFormInline.code"
            :options="cmOptions"
            :border="true"
            placeholder="请输入函数代码"
            width="100%"
            height="200px"
            @ready="onReady"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="备注说明" prop="remark">
          <el-input
            v-model="newFormInline.remark"
            type="textarea"
            :rows="3"
            placeholder="请输入备注说明"
          />
        </el-form-item>
      </el-col>
    </el-row>
  </el-form>
</template>

<style lang="scss" scoped>
.codemirror-container.bordered {
  border: 1px solid var(--pure-border-color);
}

:deep(.CodeMirror) {
  line-height: 1.5;
  text-align: left;
  font-family: Consolas, "Courier New", monospace;
}
</style>
