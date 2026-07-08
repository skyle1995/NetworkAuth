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
  // 点数模式下面值点数必填且需 >=1（该表单项仅在点数模式渲染时规则才生效）
  points: [
    { required: true, message: "请输入面值点数", trigger: "blur" },
    { type: "number", min: 1, message: "面值点数需大于等于 1", trigger: "blur" }
  ],
  prefix: [
    {
      pattern: /^[A-Za-z0-9]*$/,
      message: "前缀只能包含字母和数字",
      trigger: "blur"
    }
  ]
});
