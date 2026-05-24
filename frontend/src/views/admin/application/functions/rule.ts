import { reactive } from "vue";
import type { FormRules } from "element-plus";

export const formRules = reactive<FormRules>({
  alias: [
    { required: true, message: "请输入函数别名", trigger: "blur" },
    {
      validator: (rule, value, callback) => {
        if (!/^[a-zA-Z][a-zA-Z0-9]*$/.test(value)) {
          callback(new Error("别名必须以英文字母开头，只能包含数字和英文字母"));
        } else {
          callback();
        }
      },
      trigger: "blur"
    }
  ],
  code: [{ required: true, message: "请输入函数代码", trigger: "blur" }]
});
