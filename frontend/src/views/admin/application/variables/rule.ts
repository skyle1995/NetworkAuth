import { reactive } from "vue";
import type { FormRules } from "element-plus";

export const formRules = reactive(<FormRules>{
  alias: [
    { required: true, message: "变量别名不能为空", trigger: "blur" },
    {
      pattern: /^[a-zA-Z][a-zA-Z0-9]*$/,
      message: "别名必须以英文字母开头，只能包含数字和英文字母",
      trigger: "blur"
    }
  ]
});
