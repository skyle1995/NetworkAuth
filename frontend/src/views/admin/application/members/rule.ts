import { reactive } from "vue";
import type { FormRules } from "element-plus";

export const formRules = reactive(<FormRules>{
  app_uuid: [{ required: true, message: "请选择所属应用", trigger: "change" }],
  username: [
    { required: true, message: "请输入用户名", trigger: "blur" },
    { min: 2, max: 64, message: "用户名长度为 2 ~ 64 位", trigger: "blur" }
  ],
  password: [
    { required: true, message: "请输入密码", trigger: "blur" },
    { min: 6, max: 64, message: "密码长度为 6 ~ 64 位", trigger: "blur" }
  ]
});
