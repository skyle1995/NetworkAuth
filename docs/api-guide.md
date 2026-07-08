# NetworkAuth 客户端对接文档

面向客户端（软件端）对接 NetworkAuth 服务端的公开 API。所有业务接口都走**同一个端点**，靠请求体里的 `api_type` 区分。

---

## 一、统一入口

```
POST /api/open
Content-Type: application/json
```

请求体（信封）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `app_uuid` | string | 应用 UUID（明文，用于路由与定位接口配置） |
| `api_type` | int | 接口类型编号（明文，见第五节） |
| `data` | string | 业务参数密文（按该接口"提交算法"加密；不加密时为参数 JSON 原文） |
| `timestamp` | int64 | Unix 秒级时间戳（明文，防重放，与服务器时间偏差需 ≤ 300 秒） |
| `sign` | string | 请求签名（明文，大写十六进制） |

**响应**：

- 成功：`{ "code": 0, "data": "<按该接口'返回算法'加密的结果密文>" }`
- 失败：`{ "code": 1, "msg": "错误信息（明文，可直接展示给用户）" }`

> 提示：每个应用创建时会自动生成全部接口记录，但**默认禁用且不加密**。对接前需在后台「接口设置」里逐个**启用**并按需配置加密算法与密钥。

---

## 二、签名（必填）

```
sign = SHA256( app_uuid + "|" + api_type + "|" + data + "|" + timestamp + "|" + app_secret )
```

- 结果转**大写十六进制**字符串。
- `app_secret` 是应用密钥（后台「应用管理」可见/重置），**不上网传输**，客户端内置。
- 服务端用同样规则重算比对（常量时间比较），并校验 `timestamp` 在 ±300 秒内。

签名作用：防篡改（改任一字段签名即变）、防重放（时间戳过期拒绝）、鉴权（算得出正确签名 = 持有密钥）。

---

## 三、加解密

每个接口在后台独立配置**提交算法**（客户端→服务端）与**返回算法**（服务端→客户端），可选：

| 值 | 算法 | 密钥形态 |
|---|---|---|
| 0 | 不加密 | 无（`data` 直接是 JSON 文本） |
| 1 | RC4 | 16 进制密钥 |
| 2 | RSA | 公钥/私钥 PEM |
| 3 | RSA 动态 | 公钥/私钥 PEM |
| 4 | 易加密 | 逗号分隔整数 |

**方向约定**：
- 提交：客户端用**公钥加密**，服务端用私钥解密（RC4/易加密为对称密钥，两向同一把）。
- 返回：服务端用**公钥加密**，客户端用私钥解密。

对接联调建议：先把接口设为**不加密**跑通业务，再逐个接入加密算法。

---

## 四、调用流程（以卡密登录为例）

1. 组装业务参数：`{"card":"KM-XXXX","machine_code":"MC-1"}`
2. 按接口提交算法加密 → 得到 `data`（不加密则为 JSON 原文）
3. 取当前时间戳 `timestamp`，计算 `sign`
4. `POST /api/open`，body = `{app_uuid, api_type:10, data, timestamp, sign}`
5. 收到 `{code:0, data:"<密文>"}`，按返回算法解密 → 业务结果 JSON

---

## 五、接口清单

> 下列"参数/返回"均指**解密后的 JSON**（即 `data` 明文内容）。`token` 为登录后获得的会话令牌。

### 基础信息（无需登录）

| type | 名称 | 参数 | 返回 |
|---|---|---|---|
| 1 | 获取公告 | 无 | `{title, version, content}` |
| 2 | 获取更新地址 | 无 | `{download_type, download_url}` |
| 3 | 检测版本 | `{version}` | `{latest_version, need_update, force_update, download_type, download_url}` |
| 4 | 获取卡密信息 | `{card}` | `{card_no, status, status_text, duration, used_at}` |

### 登录与账号

| type | 名称 | 参数 | 返回 |
|---|---|---|---|
| 10 | 卡密登录 | `{card, machine_code}` | 见「登录结果」 |
| 20 | 账号登录 | `{username, password, machine_code}` | 见「登录结果」 |
| 21 | 账号注册（邮箱即账号） | `{email, password, code}` | 见「状态结果」（不含 token） |
| 23 | 发送注册验证码 | `{email}` | `{message}` |
| 24 | 领取试用 | `{username, password}` | 见「状态结果」 |
| 22 | 卡密充值 | `{username, card}` | 见「状态结果」 |
| 30 | 退出登录 | `{token}` | `{message}` |

### 状态与数据（需登录）

| type | 名称 | 参数 | 返回 |
|---|---|---|---|
| 40 | 获取到期/余额 | `{token}` | 见「状态结果」（不因已到期/点数耗尽而报错） |
| 41 | 检测状态/心跳 | `{token}` | 见「状态结果」（按时点数模式在此续扣周期） |
| 42 | 获取程序数据 | `{token}` | `{data}` |
| 43 | 获取变量数据 | `{token, alias}` | `{alias, data}` |
| 44 | 执行远程函数 | `{token, alias, params}` | `{result}`（服务端沙箱执行，见第六节） |

### 用户自助操作（需登录）

| type | 名称 | 参数 | 返回 |
|---|---|---|---|
| 50 | 修改密码 | `{token, old_password, new_password}` | `{message}`（改密后需重新登录） |
| 51 | 机器码转绑 | `{token, machine_code}` | 见「状态结果」 |
| 52 | IP 转绑 | `{token, ip}` | 见「状态结果」 |
| 53 | 功能扣点（点数模式） | `{token, points}` | 见「状态结果」 |

### 风控操作（作者侧，靠签名鉴权，建议服务端调用）

| type | 名称 | 参数 | 返回 |
|---|---|---|---|
| 60 | 封停用户 | `{username}` | `{username, status}` |
| 61 | 加入黑名单 | `{username}` | `{username, status}` |
| 62 | 扣除时间/点数 | `{username, minutes}` | `{username, expired_at}` 或 `{username, points}` |

#### 登录结果

```json
{
  "token": "会话令牌",
  "username": "用户名/卡号",
  "type": 0,          // 0=注册账号 1=卡密账号
  "mode": 0,          // 0=时长模式 1=点数模式
  "permanent": false, // 是否永久
  "expired_at": "2026-01-01T00:00:00Z", // 时长模式有效
  "points": 0         // 点数模式有效
}
```

#### 状态结果

```json
{
  "username": "…",
  "status": 1,        // 0=封停 1=正常 2=黑名单
  "mode": 0,
  "permanent": false,
  "expired_at": "…",
  "points": 0
}
```

---

## 六、运营模式

应用可选**时长模式**或**点数模式**（后台「应用编辑 → 运营模式」）：

- **时长模式**：账号按到期时间计费。卡为时长卡，充值加时长。
- **点数模式**：账号按点数余额计费。卡为点数卡，充值加点数。扣费方式分：
  - **按次**：每次登录扣固定点数（type 10/20 登录时扣）。
  - **按时**：预扣费，每 N 分钟扣 M 点；登录预扣一个周期，心跳（type 41）到期自动续扣。
  - **功能扣点**：客户端用 type 53 为高级功能显式扣点。

客户端可从登录/状态返回的 `mode` 字段识别当前模式，据此展示"到期时间"或"点数余额"。

---

## 七、远程函数（type 44，防破解）

函数代码存于服务端，在 **goja（JS 引擎）沙箱**内执行；客户端只传参数、收结果，**看不到代码逻辑**。

- 后台「公共函数」里，函数代码写成 `function(params){ ... }` 的**函数体**，用 `return` 返回结果。
- 客户端请求：`{token, alias:"函数别名", params:{...任意JSON...}}`
- 服务端执行 `fn(params)`，返回 `{result: <返回值>}`。
- 限制：单次执行超时 3 秒（防死循环）；沙箱无文件/网络/require 能力。

**示例**：后台函数 `calc` 代码：

```js
return { sum: params.a + params.b, vip: params.a > 10 };
```

客户端调用 `{token, alias:"calc", params:{a:20, b:5}}` → 返回 `{result:{sum:25, vip:true}}`。

---

## 八、错误处理

- 业务错误统一返回明文 `{code:1, msg:"..."}`，`msg` 可直接提示用户（如"卡号不存在""点数不足""账号已到期""签名校验失败""会话无效或已被顶号"）。
- 成功恒为 `{code:0, data:"<密文>"}`。

---

## 九、伪代码（客户端）

```text
function callOpenAPI(apiType, params):
    dataPlain = toJSON(params)
    data      = encryptSubmit(dataPlain)          // 按接口提交算法，不加密则原样
    ts        = nowUnixSeconds()
    sign      = upperHex(SHA256(appUUID +"|"+ apiType +"|"+ data +"|"+ ts +"|"+ appSecret))
    resp      = httpPost("/api/open", { app_uuid: appUUID, api_type: apiType,
                                        data: data, timestamp: ts, sign: sign })
    if resp.code != 0: throw resp.msg
    return fromJSON(decryptReturn(resp.data))     // 按接口返回算法解密
```
