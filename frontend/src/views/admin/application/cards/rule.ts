import { reactive } from "vue";
import type { FormRules } from "element-plus";

export const formRules = reactive(<FormRules>{
  app_uuid: [{ required: true, message: "请选择所属应用", trigger: "change" }],
  count: [
    { required: true, message: "请输入生成数量", trigger: "blur" },
    {
      type: "number",
      min: 1,
      max: 10000,
      message: "生成数量需在 1 ~ 10000 之间",
      trigger: "blur"
    }
  ],
  duration_value: [
    {
      required: true,
      message: "请输入时长数值",
      trigger: "blur"
    }
  ],
  prefix: [
    {
      pattern: /^[A-Za-z0-9]*$/,
      message: "前缀只能包含字母和数字",
      trigger: "blur"
    }
  ]
});
