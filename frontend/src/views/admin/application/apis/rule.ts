import { reactive } from "vue";
import type { FormRules } from "element-plus";

export const formRules = reactive(<FormRules>{
  submit_algorithm: [
    { required: true, message: "请选择提交数据加密算法", trigger: "change" }
  ],
  return_algorithm: [
    { required: true, message: "请选择返回数据加密算法", trigger: "change" }
  ]
});
