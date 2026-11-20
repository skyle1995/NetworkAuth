# NetworkAuth 客户端对接文档（完整版）

面向客户端（软件端）对接 NetworkAuth 服务端的公开 API。所有业务接口都走**同一个端点** `POST /api/open`，靠请求体里的 `api_type` 区分。

> 后台「应用设置」每一项开关的作用与生效逻辑，见 [app-settings.md](./app-settings.md)。
>
> 🔑 **所有客户端都必须读** [应用激活与两级授权](#应用激活与两级授权所有接入方必读)：登录只拿授权码，用授权码换取某个应用的访问权，计费发生在换取的那一刻。

---

## 一、统一入口

```
POST /api/open
Content-Type: application/json
```

请求体（信封）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `app_uuid` | string | 应用 UUID（明文）。**仅作应用身份标识与计费策略上下文**——接口配置已全站统一，不再由它定位 |
| `api_type` | int | 接口类型编号（明文，见第六节） |
| `data` | string | 业务参数密文（按该接口「提交算法」加密；不加密时为参数 JSON 原文） |
| `timestamp` | int64 | Unix 秒级时间戳（与服务器时间偏差需 ≤ 300 秒；窗口内尚不去重） |
| `sign` | string | 请求签名（明文，大写十六进制，见第三节）。**这是应用身份的唯一凭据** |

**响应**：

- 成功：`{ "code": 0, "data": "<按该接口"返回算法"加密的结果密文>" }`
- 失败：`{ "code": 1, "msg": "错误信息（明文，可直接展示给用户）" }`
- 强制更新拦截（仅登录接口 `10`/`20`）：`{ "code": 2, "msg": "...", "update": { download_type, need_update, latest_version, download_url } }`——客户端须据此弹更新引导，升级后再登录（`update` 为明文，便于未解密即可展示）。

> **新安装默认值**：新建接口默认提交、返回均为 Curve25519 Box。每个 `api_type` 独立生成服务端密钥对；重启不更换已有配置。客户端按 `api_type` 取得可信的 `submit_public_key`。接口导出不含应用凭据，但部分算法的对接密钥仍需保密。生产环境仍须使用 HTTPS。
> **应用身份由 `sign` 区分**：服务端先按 `app_uuid` 取出该应用的 `secret` 重算签名，签名不符直接拒绝。因此「同一套接口配置」不会让 A 应用能冒充 B 应用。

---

## 二、限流

`/api/open` 按来源 IP 限流 **120 次/分钟**。登录/心跳/扣点等高频接口请合理控制频率。

> **来源 IP 如何识别**：服务端默认只取 TCP 直连对端地址，**不信任任何 `X-Forwarded-For`**（防伪造）。若服务端部署在反向代理之后，须在 `config.json` 的 `server.trusted_proxies` 配置代理白名单，否则所有客户端共享代理 IP，限流会被误触发、IP 相关功能（`ip_verify` 绑定验证、接口 `51` IP 转绑、注册限流）全部失真。配置方法见 README「反向代理与客户端 IP 识别」。

---

## 三、签名算法（sign）

```
sign = SHA256( app_uuid | api_type | data | timestamp | app_secret ) 的大写十六进制
```

- `|` 为字面竖线分隔符；`api_type`、`timestamp` 按十进制字符串拼接；`data` 用信封里的密文串（不加密时即明文 JSON）。
- `app_secret` 为对应应用单独配置的密钥，不包含在接口导出中，**只参与本地计算，绝不上传**。
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
| 5 | Curve25519 Box | 标准 base64 | 32 字节提交公钥，以 hex 编码 | 每次请求生成新的临时密钥对和随机 24 字节 nonce；用临时私钥、接口公钥和 nonce 执行 NaCl `box`，按 `nonce ‖ 临时公钥(32字节) ‖ 带认证标签的密文` 拼接后编码 |

**Box 返回协议 v1**（仅在 `return_algorithm=5` 时使用，要求 `submit_algorithm=5`）：

1. 客户端保留本次请求的临时私钥和请求 nonce。外层 `{ "code": 0, "data": "..." }` 只表示密文信封，不代表业务成功。
2. 标准 Base64 解码 `data`，取前 24 字节为响应 nonce，余下为带认证标签的密文。用响应 nonce、本次临时私钥和接口提交公钥执行 `box.Open`。不另发返回密钥。
3. 解密结果必须依次为 ASCII `NetworkAuth/response/v1`、一个零字节、本次请求的 24 字节原始 nonce、UTF-8 JSON。验证认证标签、方向前缀和请求 nonce 后才可解析 JSON；不匹配一律拒绝。
4. 内层成功为 `{ "code": 0, "data": <业务结果> }`；业务错误保留原 `code/msg/err/update/sessions` 语义。请求校验或解密失败等协议错误可能只有明文错误提示，不得据此接受授权、执行升级或处理其他业务指令。每次请求只接受一次有效响应。

**方向约定（重要）**：
- 提交方向：服务端用 `submit_*` 密钥**解密**你上报的 `data`。RSA 提交时服务端持**私钥**，故客户端用 `submit_public_key` **加密**。
- 返回方向：服务端用 `return_*` 密钥**加密**结果。RSA 返回时服务端持**公钥**，故客户端用 `return_private_key` **解密**。

> 安全建议：RC4、易加密为弱加密/混淆，RSA（动态）为 PKCS#1 v1.5，均不建议用于强机密场景；如需加密优先选 **RSA（标准，OAEP）**。联调阶段可先用「不加密」跑通签名与流程。

---

## 五、通用约定

- **登录态**：需要鉴权的接口都要在 `data` 里带 `token`。令牌分两级（详见[应用激活与两级授权](#应用激活与两级授权所有接入方必读)）：`10`/`20` 登录拿**账号令牌**（证明你是谁），`70` 激活拿**激活令牌**（证明你被授权用这个应用）。账号令牌全站通用——在 A 应用登录拿到的账号令牌，可直接用于换取 B 应用的访问权。
- **钱包是全站共享的**：账号只有一份 `expired_at`（到期时间）、一份 `points`（点数）、一份 `level`（会员等级）。在任一应用消费，其它应用立刻可见。这是「一号通」的定义。
- **计费策略（app 级，三模式）**：`config.billing_mode` 决定门禁——

  | `billing_mode` | 含义 | 客户端判断 |
  |---|---|---|
  | `0` | 计时模式 | 看 `expired_at`（`permanent=true` 为永久）+ `level ≥ config.required_level` |
  | `1` | 点数模式 | 看 `points`；`points_charge_mode` 决定按次/按时 |
  | `2` | 高级模式 | **同时**看 `level ≥ config.required_level` 和 `points` |

  - **计费豁免 `config.free=1`**：该应用不计费。账号即便**已到期 / 无点数 / 等级不足也可正常登录使用**；心跳（`41`）照常做令牌与账号状态校验，但**不扣任何费用**；转绑不扣费；`53 功能扣点`返回「当前非点数计费模式」。客户端应忽略 `expired_at` / `points` / `level`，视为始终可用。
    - ⚠️ **计时模式才校验到期时间**。点数/高级模式只看等级与点数，所以时长到期的账号仍能在点数应用里继续消费点数额度。
- **心跳与离线**：登录成功返回 `heartbeat_interval`（分钟）。客户端应按该间隔周期性调用 `41 检测账号状态`（或 `40 获取到期`）以刷新在线；超过应用配置的「自动离线时长」未心跳，会话会被后台清理（掉线）。
- **同时在线与顶号**：在线名额是**账号级**资源，按**设备**计数——同一台电脑开多个插件只占 1 个名额（详见[应用激活与两级授权](#应用激活与两级授权所有接入方必读)）。上限 = 账号策略的「同时在线」+ 会员等级额外并发；超限时按 `login_type`（0 顶掉最早设备 / 1 拒绝 / 2 让用户手选）处理。
  - **按应用独立计数**：同一账号在 A 应用开 1 台、在 B 应用开 1 台都算正常，互不挤占名额；满员提示与手动顶号的 `sessions` 列表**只含本应用**的会话。
  - 同一个「开」重登先清掉自己的旧会话（不占新名额）；不同「开」未达上限则各自在线；**达到上限后**才按登录方式处理——顶号（`login_type=0`）踢掉最早的「开」，非顶号（`=1`）拒绝并返回「已达最大同时在线数」，手动顶号（`=2`）返回 `code=3` + 在线会话列表 `sessions`，客户端弹窗让用户选择踢出哪台（用 `31` 接口）再重试。
- **设备绑定是全账号级的**：账号在所有应用共用同一批绑定记录，上限按「当前登录应用」的有效多开数校验。转绑计次同样全站累计。
- **设备名**：登录 `device_name` 由客户端采集提交（`macOS 15.1`/`Windows 11`/`Android 14`/`iOS 18.1`），写入会话与设备绑定，供后台在线列表、换绑设备列表（`51`）展示。
- **时间字段**：`expired_at` 为 RFC3339 时间；永久有效对应 `permanent=true`（时间为 2099 年）。

---

## 六、接口清单

> 请求参数指 `data` 解密后的 JSON 字段；返回字段指结果密文解密后的 JSON。
> 「鉴权」列说明该接口需要哪种令牌：**免** = 不需要；**账号** = 登录拿的账号令牌；**激活** = `70` 换的激活令牌；**凭据** = 用户名密码/卡号（用于设备对不上时自救）。

| 类型 | 名称 | 鉴权 | 章节 |
|---|---|---|---|
| `1` | 获取程序公告 | 免 | [跳转](#1-获取程序公告客户端启动初始化入口) |
| `2` | 获取更新地址 | 免 | [跳转](#2-获取更新地址) |
| `3` | 检测最新版本 | 免 | [跳转](#3-检测最新版本) |
| `4` | 获取卡密信息 | 免 | [跳转](#4-获取卡密信息) |
| `10` | 卡密登录 | 卡号 | [跳转](#10-卡密登录) |
| `20` | 账号登录 | 账号密码 | [跳转](#20-账号登录) |
| `21` | 账号注册 | 免 | [跳转](#21-账号注册邮箱即账号) |
| `22` | 账号充值 | 凭据 | [跳转](#22-账号充值用卡为账号充值) |
| `23` | 发送注册验证码 | 免 | [跳转](#23-发送注册验证码) |
| `24` | 领取试用 | 凭据 | [跳转](#24-领取试用) |
| `25` | 发送找回密码验证码 | 免 | [跳转](#25-发送找回密码验证码) |
| `26` | 找回密码 | 免 | [跳转](#26-找回密码忘记密码无需登录) |
| `30` | 退出登录 | 账号 | [跳转](#30-退出登录) |
| `31` | 踢下线（手动顶号） | 凭据 | [跳转](#31-踢下线手动顶号) |
| `40` | 获取到期时间 | 账号 | [跳转](#40-获取到期时间) |
| `41` | 心跳 / 检测状态 | 激活 | [跳转](#41-检测账号状态心跳) |
| `42` | 获取程序数据 | 激活 | [跳转](#42-获取程序数据) |
| `43` | 获取变量数据 | 激活 | [跳转](#43-获取变量数据) |
| `44` | 执行远程函数 | 激活 | [跳转](#44-执行远程函数服务端沙箱) |
| `45` | 获取账号数据 | 激活 | [跳转](#45-获取账号数据) |
| `50` | 修改账号密码 | 账号 | [跳转](#50-修改账号密码) |
| `51` | 转绑 | 凭据 | [跳转](#51-转绑机器码ip) |
| `53` | 功能扣点 | 激活 | [跳转](#53-功能扣点) |
| `54` | 设置账号数据 | 激活 | [跳转](#54-设置账号数据) |
| `60` | 封停用户 | 公开入口拒绝 | [跳转](#60-封停用户) |
| `61` | 添加黑名单 | 公开入口拒绝 | [跳转](#61-添加黑名单) |
| `62` | 扣除时间 | 公开入口拒绝 | [跳转](#62-扣除时间) |
| **`70`** | **应用激活** | **账号** | [跳转](#70-应用激活) |
| **`71`** | **应用释放** | **激活** | [跳转](#71-应用释放) |
| **`72`** | **权益清单** | **账号** | [跳转](#72-权益清单) |

### 基础信息（免登录）

#### `1` 获取程序公告（客户端启动初始化入口）
- 请求：无
- 返回：公告 + 应用能力开关 + 计费模式 + 更新策略,客户端**开机调一次即可拿全初始化信息**,据此渲染登录/注册界面（是否显示验证码框、是否有卡密登录、按模式显示到期或点数等）。
```json
{
  "title": "应用名",
  "version": "1.0.0",
  "content": "公告内容",
  "config": {
    "free": 0,                    // 计费豁免：1=本应用免费（忽略等级/到期/点数）
    "time_enabled": true,         // 计时开关：要求账号有效期（卖时长）
    "points_enabled": false,      // 点数开关：按点数扣费
    "level_gate": false,          // 会员限制开关：要求等级 ≥ required_level
    "billing_mode": 1,            // 0会员/1点数/2混合
    "required_level": 1,          // 要求的最低会员权限等级（1=不限；仅会员/高级模式生效）
    "points_charge_mode": 1,      // 0按次/1按时
    "points_heartbeat_charge": 0, // 按时:0登录预扣/1心跳触发
    "card_login_enabled": 1,      // 是否开放卡密登录（全局策略）
    "register_enabled": 1,        // 是否开放账号注册（全局策略）
    "email_verify_enabled": 1,    // 注册是否需要邮箱验证码（=1 则需先调 23 取码）
    "card_register_enabled": 0,   // 注册是否必须提交卡密（=1 则注册须带 card，注册即核销）
    "register_device_required": 1,// 注册是否必须提交设备码（=1 则注册须带 machine_code）
    "recharge_enabled": 1,        // 是否开放卡密充值（全局策略）
    "trial_enabled": 1,           // 是否开放领取试用（全局策略）
    "machine_verify": 0,          // 设备校验 0关/1开（账号策略）
    "ip_verify": 0,               // IP校验 0关/1精确/2市/3省（账号策略）
    "login_type": 0               // 账号在线名额满时的策略 0顶号/1拒绝/2手动选择
  },
  "rebind": {                     // 换绑能力：设备与 IP 各一套独立额度（账号策略）
    "machine": { "enabled": 1, "limit": 1, "free_count": 1, "max_devices": 3, "count": 0, "deduct_minutes": 0, "deduct_points": 0 },
    "ip":      { "enabled": 1, "limit": 1, "free_count": 2, "count": 0, "deduct_minutes": 0, "deduct_points": 0 }
    // enabled 0关/1开；limit 0每天/1永久；free_count 每窗口免费次数；
    // count 每窗口次数上限（0=不限）；deduct 超免费后每次扣除量
  },
  "interfaces": {                 // 各接口启用状态 api_type -> 1启用/0禁用（全站统一）
    "1": 1, "10": 1, "20": 1, "21": 1, "23": 1, "30": 1, "40": 1, "41": 1
  },
  "update": { "download_type": 0, "download_url": "" }
  // download_type 更新方式：0不启用/1强制更新/2自由更新（=1 即强制）
}
```
> 客户端**开机调一次 type 1** 即可拿到：能力开关(`config`)、换绑能力(`rebind`)、以及**每个接口的启用状态**(`interfaces`)，据此渲染界面、决定哪些功能可用，无需再逐个探测。
> **`config` 混了两类开关**：`billing_mode` / `required_level` / `free` / `points_*` / `login_type` 是**本应用**的；`machine_verify` / `ip_verify` 与下列账号策略共 14 项是**全站统一**的（所有应用返回相同值）。`card_login_enabled` / `register_enabled` / `email_verify_enabled` / `card_register_enabled` / `register_device_required` / `recharge_enabled` / `trial_enabled` 是**全站账号策略**（所有应用返回相同值）。
> **注册要不要验证码**：看 `config.email_verify_enabled`——为 `1` 时注册前需先调 `23` 发验证码、注册时带上 `code`。
> **注册要不要设备码**：看 `config.register_device_required`——为 `1` 时注册须带 `machine_code`，否则被拒。
> **注册要不要卡密**：看 `config.card_register_enabled`——为 `1` 时注册须带有效 `card`，注册成功即核销该卡并按面值发放（点数与时长可同时发放）。
> **换绑是否开放**：看 `rebind.machine.enabled` / `rebind.ip.enabled`。
> **显示到期还是点数**：`free=1` 两者都不显示；否则 `billing_mode=0` 显示到期、`=1` 显示点数、`=2` 两者都显示（等级 + 点数）。

#### `2` 获取更新地址
- 请求：无
- 返回：`{ download_type, download_url }`（`download_type` **更新方式**：0 不启用 / 1 强制更新 / 2 自由更新）

#### `3` 检测最新版本

**两种查询方式，由是否传 `plugin_id` 决定：**

| 用途 | 请求 | 比较对象 |
|---|---|---|
| 宿主/模块自检 | `{ version }` | 请求签名对应的应用 |
| **插件自检（推荐）** | `{ version, plugin_id }` | `plugin_id` 映射到的应用（`apps.plugin_id`） |

- 请求：`{ version: "当前版本", plugin_id?: "com.example.esp" }`
  - `plugin_id` 可选。传了则按插件查它自己的版本列表；**不传则沿用旧语义**（查签名应用），老客户端行为不变。
  - ⚠️ 不传 `plugin_id` 时，宿主应用与各插件版本不同会**必然误报**：例如插件 2.1.0、宿主 3.0.1，会把 2.1.0 判成需要更新。
- 返回：`{ latest_version, need_update(bool), download_type, download_url, update_announcement, plugin_id?, plugin_found? }`
  - 传了 `plugin_id` 时额外返回 `plugin_id` 与 `plugin_found`：
    - `plugin_found=true`：后台存在该插件对应的应用（且已启用），`latest_version` 取自它的版本列表。
    - `plugin_found=false`：后台没登记这个插件，`need_update` 恒为 false、`download_type=0`。**这是正常的「无更新」结果，不是错误**，客户端不要弹更新框。
  - `download_type`：更新方式（0 不启用 / 1 强制更新 / 2 自由更新）。**是否必须更新看 `download_type==1`**。
  - ⚠️ 语义已变更：原「强制更新开关(`force_update`) + 更新方式(自动/手动)」已合并为单一 `download_type` 三态。`force_update` 字段**已移除**，不再下发；客户端改用 `download_type`（=1 强制、=2 自由）判断，拿到 `download_url` 自行处理下载。

#### `4` 获取卡密信息
- 请求：`{ card: "卡号" }`
- 返回：`{ card_no, status(0未用/1已用/2冻结), status_text, mode, duration, points, used_at }`
  - `duration` 与 `points` **可能同时非 0**（一张卡可同时含时长与点数），客户端按非 0 项各自展示。
  - `mode` 为本应用的 `billing_mode`，仅供展示参考。

### 卡密登录

#### `10` 卡密登录
- 触发条件：应用「卡密登录」开启。首次用某卡登录会**自动创建绑定该卡的账号**（用户名=卡号）并核销该卡。
- 请求：`{ card: "卡号", machine_code: "机器码", version: "客户端版本", device_name: "设备名" }`
  - `version`：**必传**，客户端当前版本。缺失直接拒绝登录（「请提供客户端版本号」）。用于更新判断，并记入在线会话供后台在线列表查看。
  - `device_name`：设备名称,客户端采集拼好提交（`macOS 15.1`/`Windows 11`/`Android 14`/`iOS 18.1`）。写入会话与设备绑定,供在线列表、换绑设备列表展示。
- 成功返回（`LoginResult`）：`{ token, username, type, permanent, expired_at, points, total_recharge, level, level_name, level_color, rebate_rate, heartbeat_interval, max_devices, bound_devices, bound_ips, machine_rebind_*, ip_rebind_*, update? }`
  - 返回体里**不含** `mode`/`free`：计费模式与计费豁免是**应用级**属性，账号跨应用，已从账号结果中移除。要判断「本应用怎么计费」请用 `1` 配置的 `billing_mode`/`free`，要判断「某个插件的计费」请用 `72` 权益清单里每一项的 `mode`/`billing`。`level*` 为全站会员等级，高级模式的门槛判定依据。
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
- 触发条件：**全站账号策略**的「账号注册」开启；开启邮箱验证时需先调 `23` 拿验证码。
- 请求：`{ email, password, code, card, machine_code }`（未开邮箱验证时 `code` 可空；未开卡密注册时 `card` 可空）
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`
- 说明：注册成功**不下发 token**（无试用/卡密时账号初始即过期），需再登录/充值/领试用。
- **卡密注册**：开启**卡密注册**（`config.card_register_enabled=1`）时，`card` **必传**且须为**全站未使用**的有效卡，否则返回「请提供注册卡密」「卡号不存在」「该卡已被使用」等。注册成功即核销该卡，并按卡面值发放：`points>0` 加点数、`duration≠0` 加时长（两者都有则都发）。
- **注册限制**：可按 IP 和/或设备限流（全站账号策略，共用限制窗口/次数）。⚠️ **计数是全站的**：同一 IP/设备全站最多注册 `account_register_count` 个（默认 3），换应用不能重置。

#### `23` 发送注册验证码
- 触发条件：应用「账号注册」且「邮箱验证」均开启，且已配置 SMTP 与 Redis。
- 请求：`{ email }`
- 返回：`{ ... }`（发送结果；失败以明文 `msg` 返回，如「发送过于频繁」）

#### `24` 领取试用
- 触发条件：**全站账号策略**的「领取试用」开启，且试用点数/时长至少配置一项。额度可同时含两种资源，按配置各自发放。
- **领取方案**（全站策略 `account_trial_claim_mode`）：
  - `0` 无限制：满足次数限制即可领取。
  - `1` 资源耗尽可领（默认）：仅当**到期已过且点数为 0** 才可领（两个钱包都空才算「已耗尽」）；否则返回「账号仍可用，无需领取试用」。防止有效期内反复叠加。
- 请求：`{ username, password }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `25` 发送找回密码验证码
- 请求：`{ email }`（邮箱须为本应用**已注册**账号）
- 返回：`{ message }`；依赖 SMTP 配置 + Redis。验证码 10 分钟有效、60 秒冷却。

#### `26` 找回密码（忘记密码，无需登录）
- 请求：`{ email, code, new_password }`
- 返回：`{ message: "密码重置成功，请用新密码登录" }`
- 说明：先调 `25` 取码，再带 `code` + `new_password` 重置。无需 token/旧密码；成功后在同一事务中吊销该账号全部账号令牌、刷新令牌及激活。验证码最多允许五次错误尝试，成功消费使用 Redis 原子比较删除。

#### `22` 账号充值（用卡为账号充值）
- 触发条件：**全站账号策略**的「卡密充值」开启。按卡面值给账号加时长和/或加点数（`points>0` 加点、`duration≠0` 加时长，可同时）。
- 请求：`{ username, card }`
- 返回（`StatusResult`）

#### `20` 账号登录
- 请求：`{ username, password, machine_code, version: "客户端版本" }`（`version` **必传**，同 `10`）
- 返回（`LoginResult`）：同 `10`（含 `update?` 对象）

### 登出

#### `30` 退出登录
- 请求：`{ token }`
- 返回：`{ message: "已退出登录" }`
- 账号令牌退出仅吊销当前登录及其名下激活，其他设备保留。使用激活令牌时只释放当前应用实例，信封应用必须与令牌所属应用一致。

#### `31` 踢下线（手动顶号）
- 用途：`login_type=2`（手动顶号）满员时，登录返回 `code=3` 并附会话列表 `sessions`，用户选一台后调用本接口踢下线，再重试登录。
- 鉴权：满员时客户端拿不到 `token`，故用**凭据**验证——注册账号 `{ username, password }`，卡密账号 `{ username: 卡号 }`（密码留空）。仅能踢出**本人账号**的会话。
- 请求：`{ username, password?, session_id }`（`session_id` 为 `code=3` 返回列表中的 `id`）
- 返回：`{ session_id, message: "已下线" }`

### 应用激活与两级授权（所有接入方必读）

> **类比 OAuth 2.0**：`20`/`10` 登录相当于拿**授权码**；`70` 激活相当于用授权码去换**某个资源的 access token**。
> 授权码只证明「你是谁」，access token 才证明「你被授权使用这个应用」。

#### 为什么要分两级

不管客户端是「一个独立程序」还是「一个管理器装多个插件」，用户的行为都是两步：

1. 先登录 —— 证明他是哪个账号
2. 再决定用哪个应用 —— 这时才知道该按**哪个应用的规则**收费

如果把计费绑在登录上，就会出现「一登录就得按某个应用收钱，而用户还没决定要开什么」的问题。

所以：

- **登录（`10`/`20`）**：只发账号令牌，不扣费、不占并发、不绑设备
- **激活（`70`）**：用账号令牌换该应用的访问权，**这一步才计费、才绑设备、才占名额**
- **心跳（`41`）**：续期，按时计费模式在此扣下一个周期
- **释放（`71`）**：归还名额

服务端将缺失授权期限及到达截止时刻的租约视为失效。续期在账号锁内重新读取激活、账号和应用状态；已释放、令牌已轮换、账号异常或应用停用时拒绝续期，不推进期限或结算费用。公开心跳及兼容心跳入口均要求激活令牌属于当前应用，账号令牌必须先调用 `70`。

这些规则控制服务端访问权；客户端必须自行处理续期失败，不能将服务端拒绝等同于已经停止本地功能。

#### 令牌两级

| 令牌 | 从哪来 | 证明什么 | 有效期 | 能调什么 |
|---|---|---|---|---|
| **账号令牌** `account_token` | `10`/`20` 登录 | 你是哪个账号 | 30 天，可刷新可吊销 | 账号级：`22` `24` `30` `40` `50` `72` |
| **激活令牌** `activation_token` | `70` 应用激活 | 你被授权使用**这个应用** | 授权期限（默认 30 分钟，心跳自动续） | 应用级：`41` `42` `43` `44` `45` `53` `54` `71` |

两个令牌**不能互换用途**：拿激活令牌调账号级接口会被放行（等价于账号令牌），但拿账号令牌调应用级接口会返回 `ACTIVATION_REQUIRED`——必须先调 `70` 换激活令牌。

#### `70` 应用激活

- 鉴权：**账号令牌**（放在 `data.token`）
- 信封 `app_uuid` = **要进入的那个应用**的 UUID
- 请求：

```json
{ "token": "账号令牌", "fingerprint": "设备码", "version": "2.1.0", "device_name": "Windows 11" }
```

  - `fingerprint`：设备码，推荐直接用平台设备标识。应用开了「机器码验证」时**必填**。
  - `version`：客户端版本，缺省取应用当前版本。
  - `device_name`：仅用于后台展示。

- 返回：

```json
{
  "token": "激活令牌",
  "expires_at": "2026-11-15T00:35:00+08:00",
  "lease_ttl": 1800,
  "renew_after": 600,
  "reused": false,
  "app_uuid": "...",
  "app_name": "透视",
  "username": "a@b.com",
  "type": 0,
  "mode": 2,
  "free": 0,
  "required_level": 2,
  "permanent": false,
  "expired_at": "2026-12-01T00:00:00+08:00",
  "points": 480,
  "heartbeat_interval": 10,
  "total_recharge": 1000,
  "max_devices": 3,
  "bound_devices": [
    { "machine_code": "MC-A1B2C3", "device_name": "台式机", "bound_at": "2026-11-15 10:00:00" }
  ],
  "bound_ips": ["1.2.3.4"],
  "machine_rebind_free_left": 1,
  "machine_rebind_used": 0,
  "machine_rebind_max_count": 0,
  "machine_rebind_limit_time": 1,
  "ip_rebind_free_left": 2,
  "ip_rebind_used": 0,
  "ip_rebind_max_count": 0,
  "ip_rebind_limit_time": 1,
  "level": 3,
  "level_name": "黄金",
  "level_color": "#E6A23C",
  "rebate_rate": 10
}
```

| 字段 | 说明 |
|---|---|
| `token` | **激活令牌**，后续应用级接口都用它 |
| `expires_at` / `lease_ttl` / `renew_after` | 授权到期时间 / 有效秒数 / 建议续期时刻（秒） |
| `reused` | `true` = 复用了已有激活，**本次没有扣费** |
| `mode` / `free` / `required_level` | 该应用的计费模式 / 是否免费 / 要求等级 |
| `points` / `expired_at` / `permanent` | 账号当前钱包状态（激活扣费后的最新值） |
| `level*` / `rebate_rate` / `total_recharge` | 会员等级信息（高级模式下影响扣点数量） |
| `max_devices` / `bound_devices` / `bound_ips` | 账号的绑定白名单：可绑设备数、已绑设备、已绑 IP |
| `machine_rebind_*` / `ip_rebind_*` | 设备与 IP 各自的转绑额度：`_free_left` 剩余免费次数、`_used` 本窗口已用、`_max_count` 上限（0=不限）、`_limit_time` 0每天重置/1永久 |

- **幂等**：同一 `(应用, 账号, fingerprint)` 重复调用会复用已有激活，返回 `reused: true` 且**不重复扣费**。
  客户端崩溃重启、用户反复开关同一插件，只要设备码不变就不用担心重复消费。

#### `71` 应用释放

- 鉴权：**激活令牌**
- 请求：`{ token }` → 返回 `{ released: true }`
- 信封应用须与激活所属应用一致，不接受账号令牌；已释放但仍存在的激活可幂等释放。已扣点数不退（按次已消费；按时模式的当前周期仍算已用）。
- **建议在应用退出时必调**。不调也能靠授权到期自动回收，但要等满一个授权期限（默认 30 分钟）才腾出并发名额。

#### `72` 权益清单
- 鉴权：**账号令牌**
- 请求：`{ token, fingerprint }`
- 返回：

```json
{
  "plugins": [
    {
      "uuid": "...",
      "secret": "...",                // 仅 usable=1 时下发
      "name": "透视",
      "version": "2.1.0",
      // 三个正交开关：真正的门禁依据，可任意组合
      "time_enabled": false,          // 要求账号有效期（卖时长）
      "points_enabled": true,         // 按点数扣费
      "level_gate": true,             // 要求会员等级 ≥ required_level

      "mode": 2,                      // 兼容展示：由上面三个开关推导（0 计时/1 点数/2 混合）
      "free": false,

      "required_level": 2,            // 本应用要求的最低权限等级（1=不限）
      "required_level_name": "黄金",
      "my_level": 2,
      "my_level_name": "黄金",
      "level_ok": true,

      "permanent": false,
      "expired_at": "2026-12-01T00:00:00+08:00",
      "period_ok": true,

      "charge_mode": 1,               // 0 按次 / 1 按时
      "points_needed": 0.5,           // 启动一次 / 一个周期需要多少点
      "points_balance": 480,
      "points_enough": true,
      "blocked_by_points": false,

      "usable": 1,                    // 1=可用 / 0=不可用
      "reason": "",
      "deny_code": "",
      "billing": { "time_enabled": false, "points_enabled": true, "level_gate": true,
                   "mode": 2, "mode_name": "高级模式", "scope": 1, "scope_name": "账号合并",
                   "charge_mode": 1, "charge_mode_name": "按时扣点",
                   "per_login": 1, "period_minutes": 60, "per_period": 1, "heartbeat_charge": 0 }
    },
    {
      "uuid": "...",
      "name": "自瞄",
      "required_level": 3,
      "required_level_name": "铂金",
      "my_level": 2,
      "my_level_name": "黄金",
      "level_ok": false,
      "usable": 0,
      "deny_code": "ENTITLEMENT_REQUIRED",
      "reason": "会员等级不足，需要 铂金 及以上"
    },
    {
      "uuid": "...",
      "name": "穿墙",
      "mode": 1,
      "charge_mode": 0,
      "points_needed": 2,
      "points_balance": 0.5,
      "points_enough": false,
      "blocked_by_points": true,
      "usable": 0,
      "deny_code": "INSUFFICIENT_CREDITS",
      "reason": "点数不足：本次需 2.000 点，当前 0.500 点"
    }
  ],
  "account": {
    "username": "a@b.com",
    "level": 2,
    "level_name": "黄金",
    "level_color": "#E6A23C",
    "points": 480,
    "total_recharge": 10000,
    "rebate_rate": 10,
    "permanent": false,
    "expired_at": "2026-12-01T00:00:00+08:00",
    "max_devices": 3,
    "bound_devices": [
      { "machine_code": "MC-A1B2C3", "device_name": "台式机", "bound_at": "2026-11-15 10:00:00" }
    ],
    "bound_ips": ["1.2.3.4"],
    "machine_rebind_free_left": 1,
    "machine_rebind_used": 0,
    "machine_rebind_max_count": 0,
    "machine_rebind_limit_time": 1,
    "ip_rebind_free_left": 2,
    "ip_rebind_used": 0,
    "ip_rebind_max_count": 0,
    "ip_rebind_limit_time": 1
  }
}
```

> **`deny_code` 与激活错误码同源**，客户端可直接按码分支处理，不必解析中文：
> `ENTITLEMENT_REQUIRED`（等级不足）/ `INSUFFICIENT_CREDITS`（点数不足）/
> `LICENSE_EXPIRED`（已过期）/ `LICENSE_SUSPENDED`（已封禁/黑名单）。
>
> **`points_needed` 与实扣同源**：按时模式取 `PointsPerPeriodForLevel`（该等级在本应用的点数，
> 未配置则回退应用默认周期扣点），按次模式取 `PointsCostForLevel`，与 `70` 激活扣的是同一个值。
>
> **`blocked_by_points` 是独立标记**：余额不足时 `usable=0` 且该字段为 `true`，
> 客户端据此提示「充值即可用」，而不是笼统的「不可用」。
>
> **`secret` 只在 `usable=1` 时下发**：逆向客户端也拿不到无权限应用的签发密钥。

- `uuid` **就是**你要传给 `70` 信封 `app_uuid` 的那个值，客户端直接用它对应到自己的插件，不需要额外映射字段。
- `secret` **只对 `usable=1` 的应用下发**；不可用的应用返回空 —— 避免客户端被逆向时泄露全部应用的签名密钥。
- 客户端可用 `usable` / `reason` 直接把不可用项置灰，省掉一次注定失败的 `70` 调用。

#### 令牌用途隔离

应用级接口（`41`-`45`、`53`、`54`）**只接受激活令牌**。用账号令牌直接调用会返回
`ACTIVATION_REQUIRED`，且不会产生任何扣费——服务端不会替客户端"顺手"激活。

账号级接口（`40`、`72`、`30`、`50`）接受账号令牌；激活令牌也放行（等价于所属账号）。

#### 计费规则速查

| 应用模式 | 激活时检查 | 扣费时机 |
|---|---|---|
| 计时模式 `0` | 等级 ≥ `required_level` **且** 未到期 | 不扣费 |
| 点数·按次 `1` | 无 | 每次激活扣一次 |
| 点数·按时 `1` | 无 | 激活扣一个周期（`FROM_FIRST_PING` 则首次心跳才扣），之后每次心跳续期时扣 |
| 高级模式 `2` | 等级 ≥ `required_level` | 同点数模式，但单价由**该等级**在**该应用**的配置决定 |
| 免费 `free_mode=1` | 全跳过 | 不扣费 |

> **点数支持小数**：所有点数配置（登录扣点 / 周期扣点 / 等级点数 / 卡密面值）都可以填 `0.5`、`1.5` 这类小数，
> 余额与扣费按 6 位精度存取，累减不会因为浮点误差扣不动。客户端展示时建议按 3 位小数格式化。

**高级模式的等级单价**（后台「应用程序 → 更多 → 等级点数」）：

| 配置 | 含义 |
|---|---|
| 设为 `0` | 该等级**完全免费**（不扣点） |
| 设为 `N > 0` | 该等级每次（或每周期）扣 N 点 |
| 不配置 | 跟随应用默认点数（按次取「登录扣点」，按时取「周期扣点」） |

#### 完整时序

```
① 登录        20 { username, password, machine_code }   → account_token     不计费、不占名额
② 拉清单      72 { token: account_token, fingerprint }   → 可用应用 + secret
③ 进入应用    70 { token: account_token, fingerprint }   → activation_token  ★计费、绑设备、占名额
             信封 app_uuid = 该应用的 uuid
④ 运行中      41 { token: activation_token }             → 续期（按时模式扣下个周期）
              42/44/45 { token: activation_token }       → 应用数据与功能
⑤ 退出应用    71 { token: activation_token }             → 立即归还名额
⑥ 退出程序    30 { token: account_token }                → 仅失效当前账号令牌名下的激活
```

> 如果客户端只是「一个程序 = 一个应用」，可以跳过 ②，登录后直接 ③。

### 状态查询与数据（需激活令牌）

#### `40` 获取到期时间
- 请求：`{ token }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `41` 检测账号状态（心跳）
- 请求：`{ token }`
- 返回（`StatusResult`，含 `heartbeat_interval`）：既是心跳也是状态查询，返回用户基本信息 + 心跳间隔 + **会员权限信息**（`level`/`level_name`/`rebate_rate`/`total_recharge`）。客户端可据 `heartbeat_interval` **动态调整**下次心跳，并用 `level` **实时同步权限**——管理员中途改了等级，下次心跳即生效，不会继续用旧权限。点数「按时」模式会在此结算并顺延周期。账号被封停/拉黑/到期/点数耗尽时返回异常，客户端据此登出。

#### `42` 获取程序数据
- 请求：`{ token }`
- 返回：`{ data(应用公共数据), user_data(该用户独有数据) }`

#### `43` 获取变量数据
- 请求：`{ token, alias: "变量别名" }`
- 说明：优先查询当前应用变量；普通变量可回落全局。`zex_key` 只允许当前应用配置，不回落全局；部署前须将已有全局包密钥迁移为应用级配置。
  这样包密钥、接口地址这类配置只需维护一处，改一次全站生效。
- 返回：`{ alias, data }`

#### `44` 执行远程函数（服务端沙箱）
- 请求：`{ token, alias: "函数别名", params: <任意JSON> }`
- 返回：`{ result: <函数 return 值> }`
- 说明：函数代码存于服务端、在 goja 沙箱执行（客户端看不到逻辑）。沙箱内提供只读 `getUser()`/`getApp()`；无网络/文件；单次执行 3 秒超时。函数同样是**全站共用**资源，`alias` 全局唯一。

#### `45` 获取账号数据
- 请求：`{ token }`
- 返回：`{ user_data: "当前用户的独有数据" }`
- 说明：读取当前登录用户的专属数据块（存档/配置等）。`42` 也会顺带返回 `user_data`，本接口只返回它、语义更清晰。

### 用户操作（需激活令牌）

#### `50` 修改账号密码
- 请求：`{ token, old_password, new_password }`
- 返回：`{ ... }`；仅注册账号支持（卡密账号无密码）。

#### `51` 转绑（机器码 + IP 统一，原 51/52 已合并为一个接口）
- 请求：`{ username, password, machine_code, replace_machine, device_name }`
  - `username`/`password`：账号凭据（**卡密账号用卡号作 username、password 可空**）
  - `machine_code`：新机器码。**不传（空）→ 返回当前绑定设备列表**（见下），不做任何变更
  - `replace_machine`：设备已达多开上限时，要**替换掉的旧设备号**（从设备列表里选一个）
  - `device_name`：新设备名称（客户端采集，如 `macOS 15.1`）
- **凭凭据鉴权，不需登录令牌** —— 这样设备/IP 对不上、登不进的用户也能转绑（避免死循环）。
- **机器码转绑（多设备感知）**：
  - **未提交 `machine_code`** → 返回绑定设备列表:`{ devices: [{ machine_code, device_name, bound_at }] }`，供用户挑选要替换的设备。
  - 提交 `machine_code`：目标已绑定 → 幂等放行；**未达有效多开上限** → 直接新增该设备；**已满** → 须带 `replace_machine` 精确替换那一台（设备配额=1 且仅 1 个绑定时自动替换）。
  - **有效多开 = 应用多开数 + 会员等级额外多开**；免费换绑次数 = 应用免费次数 + 会员等级赠送。
- IP 转绑：以服务端识别的**当前请求 IP** 为准（单值替换，客户端须**从新 IP 调用**）。两维度都开则依次处理、各自独立计次/扣费。
- 返回：设备列表（未带 `machine_code` 时）或 `StatusResult`。受「转绑开关/免费次数/次数上限/扣费」约束。**`free=1` 的应用下转绑一律不扣费**（仍照常计次）。⚠️ 转绑计数**全站累计**，在 A 应用用掉的次数在 B 应用同样算已用。
- 后台「接口设置」里只需启用**一个「转绑」**接口。**统一用类型号 `51`**；原「IP转绑(52)」已彻底移除（历史记录会在服务启动时自动清理），客户端不要再调 `52`。

#### `53` 功能扣点（点数模式）
- 请求：`{ token, points: 扣除点数 }`
- 返回（`StatusResult`）；原子扣减，余额不足则失败。
- 仅**点数模式或高级模式**可用；计时模式/`free=1` 调用返回「当前非点数计费模式」。

#### `54` 设置账号数据
- 请求：`{ token, data: "要保存的数据字符串" }`
- 返回：`{ message: "保存成功" }`
- 说明：**覆盖式**写入当前登录用户的专属数据块（配合 `45` 读取）。单次最大 64KB。多为存档/云配置等场景。

### 风控操作（作者/服务端侧调用）

> `POST /api/open` 已拒绝 60/61/62。客户端应用密钥不具备管理员权限；管理操作使用现有后台鉴权入口。

#### `60` 封停用户
- 公开入口返回业务错误；后台封停使用 `POST /api/admin/member/set_status`。

#### `61` 拉黑账号
- 公开入口返回业务错误；后台账号状态管理使用 `POST /api/admin/member/set_status`，黑名单记录管理使用现有后台黑名单入口。

#### `62` 扣除资源
- 公开入口返回业务错误；后台资源扣减使用 `POST /api/admin/member/deduct`。后台接口沿用自己的参数契约与管理员鉴权，不能直接转发旧公开请求。

---

## 七、返回体字段字典

**LoginResult（登录）**

| 字段 | 说明 |
|---|---|
| `token` | 登录返回的是**账号令牌**（30 天，当前公开接口未提供刷新流程）；`70` 返回的是**激活令牌**（授权期限，心跳续） |
| `username` | 用户名（卡密账号为卡号） |
| `type` | 来源类型：0 注册账号 / 1 卡密账号 |
| `mode` | 本应用计费模式：0 计时 / 1 点数 / 2 高级 |
| `permanent` | 是否永久有效（`expired_at` 为 2099 年） |
| `expired_at` | 账号到期时间（全站共享；计时模式据此判定） |
| `points` | 点数余额（全站共享；点数/高级模式据此判定） |
| `heartbeat_interval` | 心跳间隔（分钟） |
| `total_recharge` | 累计充值金额（单位：**分**，展示时 ÷100 为元） |
| `free` | 本应用是否计费豁免（1=免费）。为 1 时客户端应忽略 `expired_at` / `points` / `level`，视为始终可用 |
| `level` | 权限等级数值，**由后台自定义**（每个等级填一个整数），**数值越大权限越高**。**默认档 = 1**（账号未升到任何等级时）。客户端据此做**权限门槛**：如「`level >= 3` 才能用某功能」「不同 `level` 解锁不同菜单/资源」等，具体阈值与用途由你的业务约定 |
| `level_name` | 会员等级名。默认档为该应用 `level=1` 等级的名称，未配置则「默认会员」（不会为空） |
| `level_color` | 等级显示颜色（hex，如 `#E6A23C`）。默认档为灰 `#909399`；后台创建/编辑等级可自定义。客户端可用它渲染等级标识 |
| `rebate_rate` | 当前等级充值返利比例（%），0 = 无返利 |
| `update` | 更新判断结果，仅更新方式开启时出现：`{ download_type, need_update, latest_version, download_url }` |

**StatusResult（状态/到期/充值/转绑/扣点等）**

| 字段 | 说明 |
|---|---|
| `username` / `type` | 用户名 / 来源类型：0 注册 / 1 卡密 |
| `status` | 状态：0 封停 / 1 正常 / 2 黑名单 |
| `mode` / `free` / `permanent` / `expired_at` / `points` | 同 LoginResult |
| `heartbeat_interval` | 心跳间隔（分钟）——每次心跳都会返回，客户端可据此**动态调整**心跳频率 |
| `level` / `level_name` / `level_color` / `rebate_rate` / `total_recharge` | 会员信息，同 LoginResult。**心跳（`41`）等状态返回也实时下发**，管理员中途改等级后客户端下次心跳即同步，避免继续用旧权限 |

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
登录返回的 `token` 是**账号令牌**（不计费）。要用应用级接口，需先调 `70` 用它换**激活令牌**；之后按 `heartbeat_interval` 分钟周期调 `41` 心跳（同时自动续期）。

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

#### 形态 A：一个程序 = 一个应用

```text
# 1) 启动：检测更新 / 公告（免登录）
call(3, {version: localVersion})                  # 宿主自身版本（比签名应用）
call(3, {version: pluginVersion, plugin_id: pid}) # 插件自检：比该插件自己的版本列表
                                                  # plugin_found=false 视为无更新
call(1, {})                        -> 展示公告

# 2) 登录：只拿授权码，不计费、不占名额
resp = call(10, {card, machine_code})    # 或 call(20, {username, password, machine_code})
accountToken = resp.token
heartbeat    = resp.heartbeat_interval

# 3) 换取本应用的访问权 ★计费发生在这里★
act = call(70, {token: accountToken, fingerprint: machine_code})
appToken = act.token            # 激活令牌，后续应用级接口都用它
leaseSec = act.lease_ttl        # 授权期限（秒）

# 4) 使用中：按 heartbeat 周期心跳（同时自动续期）
every(heartbeat minutes): call(41, {token: appToken})
call(42, {token: appToken})
call(43, {token: appToken, alias})
call(44, {token: appToken, alias, params})
call(53, {token: appToken, points})     # 显式功能扣点

# 5) 退出
call(71, {token: appToken})     # 立即归还并发名额
call(30, {token: accountToken}) # 登出当前登录（失效该令牌名下的激活）
```

> 老客户端可以跳过第 3 步，直接用 `accountToken` 调 `41`/`42`——服务端会自动补一次激活（幂等，不重复扣费）。
> 但**第 5 步的 `71` 建议加上**，否则名额要等授权期限到期才释放。

#### 形态 B：一个管理器 + 多个插件

```text
# 1) 启动、2) 登录 —— 同形态 A

# 3) 拉权益清单，决定界面上哪些插件可用
list = call(72, {token: accountToken, fingerprint: machine_code})
for p in list.plugins:
    if p.usable == 0: 置灰该插件，悬浮显示 p.reason

# 3.5) 逐个插件检查更新（免登录即可，必须在登录前也能用）
u = call(3, {version: p.localVersion, plugin_id: p.plugin_id})
if u.plugin_found and u.need_update: 标记该插件「有更新」

# 4) 用户点开某个插件 → 用「该插件的 uuid」激活
act = call(70, {token: accountToken, fingerprint: machine_code},
           信封 app_uuid = p.uuid)
appToken = act.token

# 5) 插件运行中 —— 同形态 A 的第 4 步

# 6) 插件关闭 → 立刻释放，让别的设备能进
call(71, {token: appToken})
```

> 清单里的 `uuid` 就是激活时要传的 `app_uuid`，客户端不需要自己维护映射表。

---

## 九、错误处理

- 传输层出错统一 `{ code:1, msg, err? }`：`msg` 为可直接展示的中文；`err` 是机器可读的标准错误码（可能缺省）。

- **请按 `err` 分支，不要解析 `msg`**（中文会改，多语言也不适用）：

  | `err` | 含义 | 客户端应对 |
  |---|---|---|
  | `SIGNATURE_INVALID` | 签名校验失败 | 检查拼接顺序/大小写/secret，重试无意义 |
  | `ACCOUNT_TOKEN_INVALID` | 账号令牌无效或过期 | 重新登录（`20`/`10`） |
  | `ACTIVATION_REQUIRED` | 用账号令牌直接调了应用级接口 | 先调 `70` 应用激活 |
  | `ACTIVATION_TOKEN_INVALID` | 激活令牌无效/已被释放 | 重新 `70` 激活 |
  | `LEASE_EXPIRED` | 租约已过期 | 重新 `70` 激活（会重新计费） |
  | `LICENSE_EXPIRED` | 账号已到期 | 提示续费 |
  | `LICENSE_SUSPENDED` | 账号被封停/拉黑 | 提示联系客服 |
  | `ENTITLEMENT_REQUIRED` | 会员等级不足 | 提示升级等级 |
  | `INSUFFICIENT_CREDITS` | 点数不足 | 提示充值 |
  | `MACHINES_LIMIT_EXCEEDED` | 账号在线设备数已达上限 | 提示在其他设备退出，或升级等级 |
  | `FINGERPRINT_MISMATCH` | 设备未在绑定范围 | 引导走 `51` 转绑 |
  | `FINGERPRINT_REQUIRED` | 该应用要求设备码但未提供 | 补上 `fingerprint` |
  | `APP_NOT_FOUND` / `APP_DISABLED` | 应用不存在/已停用 | 检查 `app_uuid` 或联系作者 |
  | `INVALID_REQUEST` | 参数缺失/格式错误 | 修请求体 |
  | `ACCOUNT_POLICY_DENIED` | 被全站策略关闭（注册/试用等） | 提示对应功能未开放 |
- 专用拦截码（非失败，客户端据此走特殊流程）：
  - `code:2` 强制更新拦截：登录被拒并附 `update` 对象（`download_type`/`latest_version`/`download_url`）。
  - `code:3` 手动顶号满员：在线数已满并附 `sessions` 数组（每项含 `id`/`device_name`/`machine_code`/`ip`/`province`/`city`/`version`/`last_active_at`/`created_at`），客户端弹窗让用户选一台后用 `31 踢下线` 再重试登录。
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

> 后台「接口设置 → 导出密钥」导出接口地址、算法与客户端所需的接口密钥，不含应用列表、UUID 或应用密钥。Box 的服务端公钥是 `submit_public_key`，双向 Box 无需单独的返回公钥。

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
