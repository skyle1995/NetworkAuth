# NetworkAuth 客户端对接文档（完整版）

面向客户端（软件端）对接 NetworkAuth 服务端的公开 API。所有业务接口都走**同一个端点** `POST /api/open`，靠请求体里的 `api_type` 区分。

> 后台「应用设置」每一项开关的作用与生效逻辑，见 [app-settings.md](./app-settings.md)。

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
| `api_type` | int | 接口类型编号（明文，见第六节） |
| `data` | string | 业务参数密文（按该接口「提交算法」加密；不加密时为参数 JSON 原文） |
| `timestamp` | int64 | Unix 秒级时间戳（明文，防重放，与服务器时间偏差需 ≤ 300 秒） |
| `sign` | string | 请求签名（明文，大写十六进制，见第三节） |

**响应**：

- 成功：`{ "code": 0, "data": "<按该接口"返回算法"加密的结果密文>" }`
- 失败：`{ "code": 1, "msg": "错误信息（明文，可直接展示给用户）" }`
- 强制更新拦截（仅登录接口 `10`/`20`）：`{ "code": 2, "msg": "...", "update": { download_type, need_update, latest_version, download_url } }`——客户端须据此弹更新引导，升级后再登录（`update` 为明文，便于未解密即可展示）。

> 每个应用创建时会自动生成全部接口记录，但**默认禁用且不加密**。对接前需在后台「接口设置」里逐个**启用**并按需配置算法与密钥（可用「批量设置算法」一次性设置并生成密钥，用「导出密钥」导出对接所需的全部密钥与服务器地址）。

---

## 二、限流

`/api/open` 按来源 IP 限流 **120 次/分钟**。登录/心跳/扣点等高频接口请合理控制频率。

---

## 三、签名算法（sign）

```
sign = SHA256( app_uuid | api_type | data | timestamp | app_secret ) 的大写十六进制
```

- `|` 为字面竖线分隔符；`api_type`、`timestamp` 按十进制字符串拼接；`data` 用信封里的密文串（不加密时即明文 JSON）。
- `app_secret` 为应用密钥（后台「导出密钥」可获取），**只参与本地计算，绝不上传**。
- 服务端用同一算法重算比对（常量时间比较），并校验 `timestamp` 在 ±300 秒窗口内。

**示例（伪代码）**
```
data      = encryptSubmit(jsonParams)          // 见第四节；不加密时 = jsonParams
timestamp = 1720512000
raw       = app_uuid + "|" + "20" + "|" + data + "|" + "1720512000" + "|" + app_secret
sign      = HEX_UPPER( SHA256(raw) )
```

---

## 四、加解密（每接口独立配置）

每个接口可分别配置**提交算法**（客户端→服务端）和**返回算法**（服务端→客户端），互不影响。取值：

| 值 | 算法 | 密文编码 | 密钥格式 | 客户端用哪把钥匙 |
|---|---|---|---|---|
| 0 | 不加密 | 明文 | 无 | 无 |
| 1 | RC4 | base64 | `private_key`=16 进制密钥串 | 提交/返回都用该密钥 |
| 2 | RSA | base64（分块 OAEP） | PEM | **提交**用 `submit_public_key` 加密；**返回**用 `return_private_key` 解密 |
| 3 | RSA（动态） | base64 | PEM | 提交/返回用对应的动态公私钥 |
| 4 | 易加密 | base64 | `private_key`=逗号分隔整数 | 提交/返回都用该密钥 |

**方向约定（重要）**：
- 提交方向：服务端用 `submit_*` 密钥**解密**你上报的 `data`。RSA 提交时服务端持**私钥**，故客户端用 `submit_public_key` **加密**。
- 返回方向：服务端用 `return_*` 密钥**加密**结果。RSA 返回时服务端持**公钥**，故客户端用 `return_private_key` **解密**。

> 安全建议：RC4、易加密为弱加密/混淆，RSA（动态）为 PKCS#1 v1.5，均不建议用于强机密场景；如需加密优先选 **RSA（标准，OAEP）**。联调阶段可先用「不加密」跑通签名与流程。

---

## 五、通用约定

- **登录态**：需要登录的接口都要在 `data` 里带登录返回的 `token`。
- **运营模式（app 级）**：`mode=0` 时长模式（看 `expired_at`），`mode=1` 点数模式（看 `points`），`mode=2` 免费模式。多数返回体同时给出 `mode`，客户端据此显示到期或余额。
  - **免费模式（`mode=2`）**：不计费。账号即便**已到期 / 无点数也可正常登录使用**；心跳（`41`）照常做令牌与账号状态校验，但**不扣任何费用、不因到期拒绝**；转绑不扣费；`53 功能扣点`返回「当前应用非点数模式」。客户端应忽略 `expired_at` / `points`，视为始终可用。已充值的时长/点数仍保留，只是不再消耗。
- **心跳与离线**：登录成功返回 `heartbeat_interval`（分钟）。客户端应按该间隔周期性调用 `41 检测账号状态`（或 `40 获取到期`）以刷新在线；超过应用配置的「自动离线时长」未心跳，会话会被后台清理（掉线）。
- **时间字段**：`expired_at` 为 RFC3339 时间；永久有效对应 `permanent=true`（时间为 2099 年）。

---

## 六、接口清单

> 下表「登录」列表示是否需要 `token`。请求参数指 `data` 解密后的 JSON 字段；返回字段指结果密文解密后的 JSON。

### 基础信息（免登录）

#### `1` 获取程序公告（客户端启动初始化入口）
- 请求：无
- 返回：公告 + 应用能力开关 + 运营模式 + 更新策略,客户端**开机调一次即可拿全初始化信息**,据此渲染登录/注册界面（是否显示验证码框、是否有卡密登录、按模式显示到期或点数等）。
```json
{
  "title": "应用名",
  "version": "1.0.0",
  "content": "公告内容",
  "config": {
    "operation_mode": 1,          // 0时长/1点数/2免费
    "points_charge_mode": 1,      // 0按次/1按时
    "points_heartbeat_charge": 0, // 按时:0登录预扣/1心跳触发
    "card_login_enabled": 1,      // 是否开放卡密登录
    "register_enabled": 1,        // 是否开放账号注册
    "email_verify_enabled": 1,    // 注册是否需要邮箱验证码（=1 则需先调 23 取码）
    "card_register_enabled": 0,   // 注册是否必须提交卡密（=1 则注册须带 card，注册即核销并发放卡面值）
    "register_device_required": 1,// 注册是否必须提交设备码（=1 则注册须带 machine_code）
    "recharge_enabled": 1,        // 是否开放卡密充值
    "trial_enabled": 1,           // 是否开放领取试用
    "machine_verify": 1,          // 机器码验证 0关/1开
    "ip_verify": 0,               // IP验证 0关/1开/2市/3省
    "multi_open_scope": 2,        // 多开范围 0单机/1单IP/2全部
    "multi_open_count": 1         // 多开数量
  },
  "rebind": {                     // 换绑能力（machine=机器码，ip=IP）
    "machine": { "enabled": 1, "limit": 0, "count": 3, "deduct": 60 },
    "ip":      { "enabled": 0, "limit": 0, "count": 0, "deduct": 0 }
    // enabled 0关/1开；limit 0每天/1永久；count 次数；deduct 每次换绑扣除分钟
  },
  "interfaces": {                 // 各接口启用状态 api_type -> 1启用/0禁用
    "1": 1, "10": 1, "20": 1, "21": 1, "23": 1, "30": 1, "40": 1, "41": 1
  },
  "update": { "download_type": 0, "download_url": "" }
  // download_type 更新方式：0不启用/1强制更新/2自由更新（=1 即强制）
}
```
> 客户端**开机调一次 type 1** 即可拿到：能力开关(`config`)、换绑能力(`rebind`)、以及**每个接口的启用状态**(`interfaces`)，据此渲染界面、决定哪些功能可用，无需再逐个探测。
> **注册要不要验证码**：看 `config.email_verify_enabled`——为 `1` 时注册前需先调 `23` 发验证码、注册时带上 `code`。
> **注册要不要设备码**：看 `config.register_device_required`——为 `1` 时注册须带 `machine_code`，否则被拒。
> **注册要不要卡密**：看 `config.card_register_enabled`——为 `1` 时注册须带有效 `card`，注册成功即核销该卡并按面值发放时长/点数。
> **换绑是否开放**：看 `rebind.machine.enabled` / `rebind.ip.enabled`。

#### `2` 获取更新地址
- 请求：无
- 返回：`{ download_type, download_url }`（`download_type` **更新方式**：0 不启用 / 1 强制更新 / 2 自由更新）

#### `3` 检测最新版本
- 请求：`{ version: "客户端当前版本" }`
- 返回：`{ latest_version, need_update(bool), download_type, download_url }`
  - `download_type`：更新方式（0 不启用 / 1 强制更新 / 2 自由更新）。**是否必须更新看 `download_type==1`**。
  - ⚠️ 语义已变更：原「强制更新开关(`force_update`) + 更新方式(自动/手动)」已合并为单一 `download_type` 三态。`force_update` 字段**已移除**，不再下发；客户端改用 `download_type`（=1 强制、=2 自由）判断，拿到 `download_url` 自行处理下载。

#### `4` 获取卡密信息
- 请求：`{ card: "卡号" }`
- 返回：`{ card_no, status(0未用/1已用/2冻结), status_text, mode, duration, points, used_at }`
  （时长模式看 `duration` 分钟，点数模式看 `points`）

### 卡密登录

#### `10` 卡密登录
- 触发条件：应用「卡密登录」开启。首次用某卡登录会**自动创建绑定该卡的账号**（用户名=卡号）并核销该卡。
- 请求：`{ card: "卡号", machine_code: "机器码", version: "客户端版本" }`
  - `version`：**必传**，客户端当前版本。缺失直接拒绝登录（「请提供客户端版本号」）。用于更新判断，并记入在线会话供后台在线列表查看。
- 成功返回（`LoginResult`）：`{ token, username, type, mode, permanent, expired_at, points, heartbeat_interval, update? }`
- **强制更新（`download_type=1`）且版本过旧 → 直接拒绝登录**：不发 token、不核销卡、不建号、不开会话；返回 **`code=2`** 的失败响应：
  ```json
  { "code": 2, "msg": "客户端版本过低，请更新至 X 后再登录",
    "update": { "download_type": 1, "need_update": true, "latest_version": "X", "download_url": "..." } }
  ```
  客户端收到 `code=2` 即弹强制更新引导（`download_url`），升级后再登录。
- **自由更新（`download_type=2`）**：登录**照常成功**，`LoginResult.update` 带 `need_update` 标记，客户端**提示**可更新但不阻断。
- **`update` 对象**（成功响应，仅更新方式非「不启用」时出现）：`{ download_type, need_update, latest_version, download_url }`。强制更新成功登录时 `need_update` 必为 `false`（否则已被 `code=2` 拒绝）。
- 未提交 `version`（视为最旧）：强制更新下会被 `code=2` 拒绝。**心跳不做版本判断**（避免打断使用），仅登录判定一次。

### 账号体系

#### `21` 账号注册（邮箱即账号）
- 触发条件：应用「账号注册」开启；开启邮箱验证时需先调 `23` 拿验证码。
- 请求：`{ email, password, code, card, machine_code }`（未开邮箱验证时 `code` 可空；未开卡密注册时 `card` 可空）
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`
- 说明：注册成功**不下发 token**（无试用/卡密时账号初始即过期），需再登录/充值/领试用。
- **卡密注册**：开启**卡密注册**（`config.card_register_enabled=1`）时，`card` **必传**且须为本应用**未使用**的有效卡，否则返回「请提供注册卡密」「卡号不存在」「该卡已被使用」等。注册成功即核销该卡，并按运营模式把卡面值发放给新账号（时长模式发到期时长、点数模式发点数）。
- **注册限制**：可按 IP 和/或设备限流（后台「注册设置」分别开关，共用限制时间/次数）。开启**设备注册限制**时，`machine_code` **必传**，否则返回「注册需提供设备码」；换 IP 不能绕过设备限制。

#### `23` 发送注册验证码
- 触发条件：应用「账号注册」且「邮箱验证」均开启，且已配置 SMTP 与 Redis。
- 请求：`{ email }`
- 返回：`{ ... }`（发送结果；失败以明文 `msg` 返回，如「发送过于频繁」）

#### `24` 领取试用
- 触发条件：应用「领取试用」开启。**两种模式都支持**：时长模式发放试用时长（分钟），点数模式发放试用点数；受「每天/永久」领取次数限制。
- **领取方案**（应用「试用领取设置」的 `trial_claim_mode`）：
  - `0` 无限制：满足次数限制即可领取。
  - `1` 到期可领（默认）：仅账号资源已耗尽才可领——时长模式须**已到期**（永久账号不可领）、点数模式须**点数为 0**、免费模式恒可用故不可领；否则返回「账号仍可用，无需领取试用」。防止有效期内反复叠加。
- 请求：`{ username, password }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `25` 发送找回密码验证码
- 请求：`{ email }`（邮箱须为本应用**已注册**账号）
- 返回：`{ message }`；依赖 SMTP 配置 + Redis。验证码 10 分钟有效、60 秒冷却。

#### `26` 找回密码（忘记密码，无需登录）
- 请求：`{ email, code, new_password }`
- 返回：`{ message: "密码重置成功，请用新密码登录" }`
- 说明：先调 `25` 取码,再带 `code` + 新密码重置。**不需要 token/旧密码**,专供忘记密码的用户;重置成功后该账号全部会话被清空。

#### `22` 账号充值（用卡为账号充值）
- 触发条件：应用「卡密充值」开启。按运营模式给账号加时长或加点数。
- 请求：`{ username, card }`
- 返回（`StatusResult`）

#### `20` 账号登录
- 请求：`{ username, password, machine_code, version: "客户端版本" }`（`version` **必传**，同 `10`）
- 返回（`LoginResult`）：同 `10`（含 `update?` 对象）

### 登出

#### `30` 退出登录
- 请求：`{ token }`
- 返回：`{ message: "已退出登录" }`

### 状态查询与数据（需登录）

#### `40` 获取到期时间
- 请求：`{ token }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `41` 检测账号状态（心跳）
- 请求：`{ token, no_charge?: bool }`
- 返回（`StatusResult`，含 `heartbeat_interval`）：既是心跳也是状态查询，返回用户基本信息 + 心跳间隔,客户端可据返回的 `heartbeat_interval` **动态调整**下次心跳时间。点数「按时」模式会在此结算并顺延周期。账号被封停/拉黑/到期/点数耗尽时返回异常，客户端据此登出。
- **`no_charge` 参数**（仅点数-按时模式有效）：点数-按时模式下心跳**默认按周期扣费**；对免费功能传 `no_charge:true` 可**跳过本次扣费**（点数不变、仍可用）。免费模式/时长模式/按次模式忽略该参数。可实现「功能A免费 / 功能B计费」：功能A的心跳传 `no_charge:true`，功能B的心跳照常（默认扣）。
  - ⚠️ 语义相较旧版**已反转**：旧字段 `charge`（默认不扣、传 `true` 才扣）已废弃为 `no_charge`（默认扣、传 `true` 才不扣）。升级需同步调整客户端心跳参数。

#### `42` 获取程序数据
- 请求：`{ token }`
- 返回：`{ data(应用公共数据), user_data(该用户独有数据) }`

#### `43` 获取变量数据
- 请求：`{ token, alias: "变量别名" }`
- 返回：`{ alias, data }`（别名限本应用或全局）

#### `44` 执行远程函数（服务端沙箱）
- 请求：`{ token, alias: "函数别名", params: <任意JSON> }`
- 返回：`{ result: <函数 return 值> }`
- 说明：函数代码存于服务端、在 goja 沙箱执行（客户端看不到逻辑）。沙箱内提供只读 `getUser()`/`getApp()`；无网络/文件；单次执行 3 秒超时。

#### `45` 获取账号数据
- 请求：`{ token }`
- 返回：`{ user_data: "当前用户的独有数据" }`
- 说明：读取当前登录用户的专属数据块（存档/配置等）。`42` 也会顺带返回 `user_data`，本接口只返回它、语义更清晰。

### 用户操作（需登录）

#### `50` 修改账号密码
- 请求：`{ token, old_password, new_password }`
- 返回：`{ ... }`；仅注册账号支持（卡密账号无密码）。

#### `51` 转绑（机器码 + IP 统一，原 51/52 已合并为一个接口）
- 请求：`{ username, password, machine_code }`
  - `username`/`password`：账号凭据（**卡密账号用卡号作 username、password 可空**）
  - `machine_code`：仅当开启「机器码转绑」时必传，为新机器码
- **凭凭据鉴权，不需登录令牌** —— 这样设备/IP 对不上、登不进的用户也能转绑（避免死循环）。
- 行为：按应用配置转绑已开启的维度 —— 机器码转绑替换为 `machine_code`；IP 转绑以服务端识别的**当前请求 IP** 为准（客户端须**从新 IP 调用**）。两维度都开则依次转绑、各自独立计次/扣费。
- 返回（`StatusResult`）；受各自「转绑开关/免费次数/次数上限/扣费」约束。**免费模式下转绑一律不扣费**（仍照常计次）。
- 后台「接口设置」里只需启用**一个「转绑」**接口。**统一用类型号 `51`**；原「IP转绑(52)」已彻底移除（历史记录会在服务启动时自动清理），客户端不要再调 `52`。

#### `53` 功能扣点（点数模式）
- 请求：`{ token, points: 扣除点数 }`
- 返回（`StatusResult`）；原子扣减，余额不足则失败。
- 仅**点数模式**可用；时长模式/免费模式调用返回「当前应用非点数模式」。

#### `54` 设置账号数据
- 请求：`{ token, data: "要保存的数据字符串" }`
- 返回：`{ message: "保存成功" }`
- 说明：**覆盖式**写入当前登录用户的专属数据块（配合 `45` 读取）。单次最大 64KB。多为存档/云配置等场景。

### 风控操作（作者/服务端侧调用）

> ⚠️ 这三个接口仅凭应用签名鉴权、无用户 token，等价于"持有 app_secret 即可操作任意账号"。**请只从你自己的可信服务端调用，切勿在分发给终端的客户端里内置**。

#### `60` 封停用户
- 请求：`{ username }` → 返回：`{ username, status }`

#### `61` 拉黑账号
- 请求：`{ username }` → 返回：`{ username, status }`

#### `62` 扣除资源
- 请求：`{ username, minutes: 数量 }`
- 返回：点数模式 `{ username, points }`；时长模式 `{ username, expired_at }`
  （`minutes` 字段在点数模式下按点数扣减）

---

## 七、返回体字段字典

**LoginResult（登录）**

| 字段 | 说明 |
|---|---|
| `token` | 会话令牌，后续需登录接口都要带 |
| `username` | 用户名（卡密账号为卡号） |
| `type` | 来源类型：0 注册账号 / 1 卡密账号 |
| `mode` | 运营模式：0 时长 / 1 点数 / 2 免费 |
| `permanent` | 是否永久有效 |
| `expired_at` | 到期时间（时长模式） |
| `points` | 点数余额（点数模式） |
| `heartbeat_interval` | 心跳间隔（分钟） |
| `total_recharge` | 累计充值金额（单位：**分**，展示时 ÷100 为元） |
| `level_name` | 会员等级名，空字符串 = 默认「免费账号」 |
| `rebate_rate` | 当前等级充值返利比例（%），0 = 无返利 |
| `update` | 更新判断结果，仅更新方式开启时出现：`{ download_type, need_update, latest_version, download_url }` |

**StatusResult（状态/到期/充值/转绑/扣点等）**

| 字段 | 说明 |
|---|---|
| `username` / `type` | 用户名 / 来源类型：0 注册 / 1 卡密 |
| `status` | 状态：0 封停 / 1 正常 / 2 黑名单 |
| `mode` / `permanent` / `expired_at` / `points` | 同 LoginResult |
| `heartbeat_interval` | 心跳间隔（分钟）——每次心跳都会返回，客户端可据此**动态调整**心跳频率 |

---

## 八、使用示例

### 8.1 一次完整请求（不加密，可照抄验证）

以「卡密登录」`api_type=10` 为例，接口提交/返回算法都设为 **0 不加密**。

假设：
```
app_uuid   = 3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B
app_secret = A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6
```

**第 1 步：拼参数 JSON（这就是 data，因为不加密）**
```json
{"card":"KM-8QWE-2RTY-6UIO","machine_code":"PC-DEMO-001"}
```

**第 2 步：算签名**。取 `timestamp = 1720512000`，拼接：
```
3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B|10|{"card":"KM-8QWE-2RTY-6UIO","machine_code":"PC-DEMO-001"}|1720512000|A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6
```
对上面整串做 SHA256 再转大写十六进制，得到：
```
548A15D4FB8F3A3243E2C6A4DC64C010BA872257FA709FDBF3FFB596AA884985
```
> 可自行验证：`printf '%s' '<上面整串>' | shasum -a 256` 结果应一致。**注意签名用的 `data` 必须和请求体里发送的 `data` 逐字节相同。**

**第 3 步：发请求**
```bash
curl -X POST https://your-domain.com/api/open \
  -H "Content-Type: application/json" \
  -d '{
    "app_uuid": "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B",
    "api_type": 10,
    "data": "{\"card\":\"KM-8QWE-2RTY-6UIO\",\"machine_code\":\"PC-DEMO-001\"}",
    "timestamp": 1720512000,
    "sign": "548A15D4FB8F3A3243E2C6A4DC64C010BA872257FA709FDBF3FFB596AA884985"
  }'
```

**第 4 步：解析响应**（不加密时 `data` 就是明文 JSON）
```json
{ "code": 0, "data": "{\"token\":\"9f2c...\",\"username\":\"KM-8QWE-2RTY-6UIO\",\"type\":1,\"mode\":0,\"permanent\":false,\"expired_at\":\"2025-08-01T12:00:00+08:00\",\"points\":0,\"heartbeat_interval\":10}" }
```
拿到 `token` 与 `heartbeat_interval`，后续用 `token` 调需登录接口，并按 `heartbeat_interval` 分钟周期调 `41` 心跳。

### 8.2 加密接口的差别

若该接口配了加密，只需把第 1 步的 `data` 换成密文，其余（用最终 `data` 参与签名、请求体、解密响应）完全不变：
```
不加密：data = JSON
RC4   ：data = base64( RC4(JSON, hex解码(private_key)) )
易加密 ：data = base64( Easy(JSON, private_key的逗号整数数组) )
RSA   ：data = base64( RSA_OAEP(JSON, submit_public_key) )        # 超长自动分块
```
响应解密方向相反：`结果 = 解密(resp.data, 接口返回算法, 返回侧密钥)`（RSA 用 `return_private_key`）。

### 8.3 代码示例

**Python**
```python
import time, json, hashlib, requests

BASE = "https://your-domain.com/api/open"
APP_UUID = "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B"
APP_SECRET = "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6"

def make_sign(app_uuid, api_type, data, ts, secret):
    raw = f"{app_uuid}|{api_type}|{data}|{ts}|{secret}"
    return hashlib.sha256(raw.encode()).hexdigest().upper()

# encrypt/decrypt 默认不加密；加密接口传入对应实现即可
def call(api_type, params, encrypt=lambda s: s, decrypt=lambda s: s):
    data = encrypt(json.dumps(params, separators=(",", ":")))
    ts = int(time.time())
    body = {
        "app_uuid": APP_UUID, "api_type": api_type, "data": data,
        "timestamp": ts,
        "sign": make_sign(APP_UUID, api_type, data, ts, APP_SECRET),
    }
    resp = requests.post(BASE, json=body, timeout=10).json()
    if resp["code"] != 0:
        raise RuntimeError(resp["msg"])
    return json.loads(decrypt(resp["data"]))

# 卡密登录 -> 心跳
r = call(10, {"card": "KM-8QWE-2RTY-6UIO", "machine_code": "PC-DEMO-001"})
token = r["token"]
print("到期:", r.get("expired_at"), "心跳(分):", r["heartbeat_interval"])
call(41, {"token": token})
```

**C#**
```csharp
using System.Net.Http;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

const string BASE = "https://your-domain.com/api/open";
const string APP_UUID = "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B";
const string APP_SECRET = "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6";

static string MakeSign(string appUuid, int apiType, string data, long ts, string secret) {
    var raw = $"{appUuid}|{apiType}|{data}|{ts}|{secret}";
    using var sha = SHA256.Create();
    return Convert.ToHexString(sha.ComputeHash(Encoding.UTF8.GetBytes(raw))); // 已是大写
}

static async Task<JsonElement> Call(int apiType, object p) {
    string data = JsonSerializer.Serialize(p);            // 不加密；加密则替换为密文
    long ts = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
    var body = new {
        app_uuid = APP_UUID, api_type = apiType, data,
        timestamp = ts, sign = MakeSign(APP_UUID, apiType, data, ts, APP_SECRET)
    };
    using var http = new HttpClient();
    var res = await http.PostAsync(BASE,
        new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json"));
    var root = JsonDocument.Parse(await res.Content.ReadAsStringAsync()).RootElement;
    if (root.GetProperty("code").GetInt32() != 0)
        throw new Exception(root.GetProperty("msg").GetString());
    return JsonDocument.Parse(root.GetProperty("data").GetString()).RootElement; // 不加密时
}

var login = await Call(10, new { card = "KM-8QWE-2RTY-6UIO", machine_code = "PC-DEMO-001" });
var token = login.GetProperty("token").GetString();
await Call(41, new { token });
```

**JavaScript（Node 18+）**
```js
import crypto from "node:crypto";

const BASE = "https://your-domain.com/api/open";
const APP_UUID = "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B";
const APP_SECRET = "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6";

const makeSign = (appUuid, apiType, data, ts, secret) =>
  crypto.createHash("sha256")
    .update(`${appUuid}|${apiType}|${data}|${ts}|${secret}`)
    .digest("hex").toUpperCase();

async function call(apiType, params) {
  const data = JSON.stringify(params);                 // 不加密；加密则替换为密文
  const ts = Math.floor(Date.now() / 1000);
  const body = { app_uuid: APP_UUID, api_type: apiType, data, timestamp: ts,
                 sign: makeSign(APP_UUID, apiType, data, ts, APP_SECRET) };
  const r = await fetch(BASE, { method: "POST",
    headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const resp = await r.json();
  if (resp.code !== 0) throw new Error(resp.msg);
  return JSON.parse(resp.data);                         // 不加密时 data 即明文JSON
}

const login = await call(10, { card: "KM-8QWE-2RTY-6UIO", machine_code: "PC-DEMO-001" });
await call(41, { token: login.token });
```

### 8.4 典型流程（伪代码）

```text
# 1) 启动：检测更新 / 公告（免登录）
call(3, {version: localVersion})        -> 若 need_update 则提示/更新
call(1, {})                             -> 展示公告

# 2) 登录（卡密 或 账号）
resp = call(10, {card, machine_code})   # 或 call(20, {username, password, machine_code})
token = resp.token
heartbeat = resp.heartbeat_interval

# 3) 使用中：按 heartbeat 周期心跳；按需取数据/扣点
every(heartbeat minutes): call(41, {token})   # 掉线/封停/到期 -> 登出
call(42, {token}); call(43, {token, alias})
call(44, {token, alias, params})
call(53, {token, points})               # 点数模式功能扣点

# 4) 退出
call(30, {token})
```

---

## 九、错误处理

- 传输层出错统一 `{ code:1, msg }`，`msg` 为可直接展示的中文（如「应用已停用」「接口已停用」「签名校验失败」「请求已过期，请校准时间」「请求解密失败」「会话无效或已被顶号」「账号状态异常」「点数不足」等）。
- 常见排查：
  - `签名校验失败`：检查拼接顺序/大小写、`data` 是否用的是最终密文、`app_secret` 是否正确。
  - `请求已过期`：客户端与服务器时间偏差超过 300 秒，校准本地时间。
  - `接口未配置/已停用`：后台「接口设置」里启用对应 `api_type`。
  - `请求解密失败`：客户端加密算法/密钥与后台该接口配置不一致。

---

## 十、附录：算法密钥格式

| 算法 | 公钥 | 私钥 | 密文 |
|---|---|---|---|
| 不加密 | — | — | 明文 |
| RC4 | — | 16 进制字符串 | base64 |
| RSA（标准） | PEM | PEM | base64（>117B 自动分块，OAEP） |
| RSA（动态） | PEM | PEM | base64 |
| 易加密 | — | 逗号分隔整数 | base64 |

> 全部密钥可在后台「接口设置 → 导出密钥」一键导出为 JSON（含 `server.api_endpoint`、`app.uuid/secret` 及每个接口的算法与四类密钥），交给对接开发直接使用。

---

## 十一、算法 SDK（参考实现）

以下为各加密算法的**确切规格**与 **Python 参考实现**（依赖 `cryptography` 库）。其它语言按同一规格移植即可（RC4/易加密逻辑与语言无关，RSA 用各平台自带库）。

**方向与密钥**（务必对应）：提交(客户端→服务端)用 `submit_*`，返回(服务端→客户端)用 `return_*`；RSA 提交用 `submit_public_key` 加密、返回用 `return_private_key` 解密；RC4/易加密两个方向都用各自的 `private_key`。

### 各算法规格
| 算法 | 密钥（来自导出的 private_key/public_key） | 密文编码 | 说明 |
|---|---|---|---|
| 0 不加密 | 无 | 明文 | `data` = JSON 原文 |
| 1 RC4 | private_key = 16 进制串 | base64 | **标准 RC4**，密钥=hex 解码后的字节 |
| 2 RSA | PEM 公/私钥 | base64 | **OAEP + SHA-256 + MGF1(SHA-256)，label 空**；明文超长按 `keySize-66` 分块，密文按 `keySize` 分块拼接 |
| 3 RSA动态 | PEM 公/私钥 | base64 | PKCS#1 v1.5 外层 + 内层动态 XOR（见文末，弱加密，不建议） |
| 4 易加密 | private_key = 逗号分隔整数 | base64 | 自定义：`(字节-207) ^ key[i]`，带符号 16 进制、逗号拼接后 base64 |

### Python 参考实现

```python
import base64, hashlib, json, time, requests
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives import hashes, serialization

# ---------- 签名 ----------
def make_sign(app_uuid, api_type, data, ts, secret):
    return hashlib.sha256(f"{app_uuid}|{api_type}|{data}|{ts}|{secret}".encode()).hexdigest().upper()

# ---------- 1 RC4（标准RC4；key为16进制串；输出base64） ----------
def _rc4(key: bytes, data: bytes) -> bytes:
    S = list(range(256)); j = 0
    for i in range(256):
        j = (j + S[i] + key[i % len(key)]) % 256
        S[i], S[j] = S[j], S[i]
    out = bytearray(); i = j = 0
    for b in data:
        i = (i + 1) % 256; j = (j + S[i]) % 256
        S[i], S[j] = S[j], S[i]
        out.append(b ^ S[(S[i] + S[j]) % 256])
    return bytes(out)

def rc4_encrypt(plain, hex_key): return base64.b64encode(_rc4(bytes.fromhex(hex_key), plain.encode())).decode()
def rc4_decrypt(ciph, hex_key):  return _rc4(bytes.fromhex(hex_key), base64.b64decode(ciph)).decode()

# ---------- 4 易加密（key为逗号分隔整数；输出base64） ----------
def _easy_key(key_str): return [int(x) for x in key_str.split(",") if x.strip() != ""]

def easy_encrypt(plain, key_str):
    key = _easy_key(key_str); n = len(key); parts = []
    for i, b in enumerate(plain.encode()):          # 按 UTF-8 字节
        v = (b - 207) ^ key[i % n]                  # 负数用两补码（与Go一致）
        parts.append(("-" + format(-v, "x")) if v < 0 else format(v, "x"))
    return base64.b64encode((",".join(parts) + ",").encode()).decode()  # 注意结尾逗号

def easy_decrypt(ciph, key_str):
    key = _easy_key(key_str); n = len(key)
    out = bytearray()
    for i, part in enumerate(base64.b64decode(ciph).decode().split(",")):
        if part == "": continue
        neg = part.startswith("-")
        d = -int(part[1:], 16) if neg else int(part, 16)
        out.append(((d ^ key[i % n]) + 207) & 0xFF)
    return out.decode("utf-8", "ignore")

# ---------- 2 RSA（OAEP-SHA256，分块） ----------
_OAEP = padding.OAEP(mgf=padding.MGF1(hashes.SHA256()), algorithm=hashes.SHA256(), label=None)

def rsa_encrypt(plain, public_pem):        # 提交方向：用 submit_public_key
    pub = serialization.load_pem_public_key(public_pem.encode())
    ksz = pub.key_size // 8; blk = ksz - 2*32 - 2
    d = plain.encode(); out = b""
    for i in range(0, len(d), blk):
        out += pub.encrypt(d[i:i+blk], _OAEP)
    return base64.b64encode(out).decode()

def rsa_decrypt(ciph, private_pem):        # 返回方向：用 return_private_key
    prv = serialization.load_pem_private_key(private_pem.encode(), password=None)
    ksz = prv.key_size // 8; d = base64.b64decode(ciph); out = b""
    for i in range(0, len(d), ksz):
        out += prv.decrypt(d[i:i+ksz], _OAEP)
    return out.decode()

# ---------- 统一调用（按接口算法装配 data/解析 resp.data） ----------
BASE = "https://your-domain.com/api/open"
APP_UUID = "你的应用UUID"
APP_SECRET = "你的应用密钥"

def call(api_type, params, submit=("none", ""), ret=("none", "")):
    # submit/ret 形如 ("rc4", hexkey) / ("easy", "1,2,3") / ("rsa", pem) / ("none","")
    plain = json.dumps(params, separators=(",", ":"))
    algo, key = submit
    data = {"none": lambda: plain, "rc4": lambda: rc4_encrypt(plain, key),
            "easy": lambda: easy_encrypt(plain, key), "rsa": lambda: rsa_encrypt(plain, key)}[algo]()
    ts = int(time.time())
    body = {"app_uuid": APP_UUID, "api_type": api_type, "data": data,
            "timestamp": ts, "sign": make_sign(APP_UUID, api_type, data, ts, APP_SECRET)}
    resp = requests.post(BASE, json=body, timeout=10).json()
    if resp["code"] != 0: raise RuntimeError(resp["msg"])
    algo, key = ret
    return json.loads({"none": lambda: resp["data"], "rc4": lambda: rc4_decrypt(resp["data"], key),
        "easy": lambda: easy_decrypt(resp["data"], key), "rsa": lambda: rsa_decrypt(resp["data"], key)}[algo]())

# 例：不加密卡密登录
# print(call(10, {"card": "KM-xxxx", "machine_code": "PC-1"}))
```

### RSA 动态（algorithm 3）规格
弱加密、不建议使用；如需对接，流程为：客户端生成一段随机动态密钥 `keys`（每字节非 0）→ 对明文按 `keys` 循环 XOR → 拼成 `[1字节keys长度][keys][XOR后的密文]` → 用 RSA **PKCS#1 v1.5** 公钥加密 → base64。解密相反（RSA 私钥解密后，读首字节得 keys 长度，取出 keys，再 XOR 还原）。需要该算法的完整参考代码可单独索取。

> 其它语言（C# / JS / 易语言 / C++）版本可按上表规格移植；需要我直接给某语言的成品 SDK，告诉我目标语言即可。
