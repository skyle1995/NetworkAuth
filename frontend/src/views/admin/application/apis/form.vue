<script setup lang="ts">
import { ref } from "vue";
import { formRules } from "./rule";

export interface FormProps {
  formInline: {
    id: number;
    app_uuid: string;
    api_type: number;
    api_type_name: string;
    app_name: string;
    submit_algorithm: number;
    submit_public_key: string;
    submit_private_key: string;
    return_algorithm: number;
    return_public_key: string;
    return_private_key: string;
    status: number;
  };
}

const props = withDefaults(defineProps<FormProps>(), {
  formInline: () => ({
    id: 0,
    app_uuid: "",
    api_type: 0,
    api_type_name: "",
    app_name: "",
    submit_algorithm: 0,
    submit_public_key: "",
    submit_private_key: "",
    return_algorithm: 0,
    return_public_key: "",
    return_private_key: "",
    status: 1
  })
});

const ruleFormRef = ref();
const newFormInline = ref(props.formInline);

// 定义算法选项
const algorithms = [
  { value: 0, label: "不加密" },
  { value: 1, label: "RC4 加密" },
  { value: 2, label: "RSA 加密" },
  { value: 4, label: "易加密" }
];

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
        <el-form-item label="状态" prop="status">
          <el-switch
            v-model="newFormInline.status"
            :active-value="1"
            :inactive-value="0"
            active-text="启用"
            inactive-text="禁用"
            inline-prompt
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row>
      <el-col :span="24">
        <el-form-item label="所属应用" prop="app_name">
          <el-input v-model="newFormInline.app_name" disabled />
        </el-form-item>
      </el-col>
      <el-col :span="24">
        <el-form-item label="接口类型" prop="api_type_name">
          <el-input v-model="newFormInline.api_type_name" disabled />
        </el-form-item>
      </el-col>
    </el-row>

    <el-divider content-position="left">提交数据加密配置</el-divider>

    <el-row>
      <el-col :span="24">
        <el-form-item label="加密算法" prop="submit_algorithm">
          <el-select
            v-model="newFormInline.submit_algorithm"
            placeholder="请选择提交数据加密算法"
            class="w-full"
          >
            <el-option
              v-for="item in algorithms"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.submit_algorithm === 2" class="mt-4">
      <el-col :span="24">
        <el-form-item label="公钥" prop="submit_public_key">
          <el-input
            v-model="newFormInline.submit_public_key"
            type="textarea"
            :rows="4"
            placeholder="请输入 RSA 公钥"
          />
        </el-form-item>
      </el-col>
      <el-col :span="24">
        <el-form-item label="私钥" prop="submit_private_key">
          <el-input
            v-model="newFormInline.submit_private_key"
            type="textarea"
            :rows="4"
            placeholder="请输入 RSA 私钥"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.submit_algorithm === 1" class="mt-4">
      <el-col :span="24">
        <el-form-item label="RC4 密钥" prop="submit_private_key">
          <el-input
            v-model="newFormInline.submit_private_key"
            placeholder="请输入 RC4 密钥（16位十六进制字符）"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.submit_algorithm === 4" class="mt-4">
      <el-col :span="24">
        <el-form-item label="易加密密钥" prop="submit_private_key">
          <el-input
            v-model="newFormInline.submit_private_key"
            placeholder="易加密密钥（15-30位整数数组，逗号分隔）"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-divider content-position="left">返回数据加密配置</el-divider>

    <el-row>
      <el-col :span="24">
        <el-form-item label="加密算法" prop="return_algorithm">
          <el-select
            v-model="newFormInline.return_algorithm"
            placeholder="请选择返回数据加密算法"
            class="w-full"
          >
            <el-option
              v-for="item in algorithms"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.return_algorithm === 2" class="mt-4">
      <el-col :span="24">
        <el-form-item label="公钥" prop="return_public_key">
          <el-input
            v-model="newFormInline.return_public_key"
            type="textarea"
            :rows="4"
            placeholder="请输入 RSA 公钥"
          />
        </el-form-item>
      </el-col>
      <el-col :span="24">
        <el-form-item label="私钥" prop="return_private_key">
          <el-input
            v-model="newFormInline.return_private_key"
            type="textarea"
            :rows="4"
            placeholder="请输入 RSA 私钥"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.return_algorithm === 1" class="mt-4">
      <el-col :span="24">
        <el-form-item label="RC4 密钥" prop="return_private_key">
          <el-input
            v-model="newFormInline.return_private_key"
            placeholder="请输入 RC4 密钥（16位十六进制字符）"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row v-if="newFormInline.return_algorithm === 4" class="mt-4">
      <el-col :span="24">
        <el-form-item label="易加密密钥" prop="return_private_key">
          <el-input
            v-model="newFormInline.return_private_key"
            placeholder="易加密密钥（15-30位整数数组，逗号分隔）"
          />
        </el-form-item>
      </el-col>
    </el-row>
  </el-form>
</template>
