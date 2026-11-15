# KTRW 外壳 + 插件场景的授权对接方案

> 状态：**NetworkAuth 侧已实施完成**（见 §十三 实施记录）；ktrw-manager 侧对接待开始。解决「主程序是外壳、插件是独立业务单元」时，一号通的计费时机错误问题。
>
> 参考工程：`D:\Synchro\Project\Cheat\ktrw-manager`（v1.7.6，Android 内核级驱动加载器 + `.zex` 插件调度平台）。
>
> **架构全貌见 [architecture.md](./architecture.md)**（本文档是其第六章「会话模型」的详细方案）。
>
> 前置阅读：[plan-unified-account.md](./plan-unified-account.md)（一号通：钱包全局、计费按应用）。

---

## 零、行业标准对标（先读这节）

这套问题在**软件授权（Software Licensing）**领域已有成熟范式。我们不需要发明语义，只需对齐：

### 0.1 我们的设计其实是什么

| 我们的说法 | 行业标准术语 | 标准出处 |
|---|---|---|
| 账号令牌 | **Account Access Token**（长期、可刷新，无 audience 绑定） | OAuth 2.0 |
| 激活 | **Audience-bound Token** / **Machine Activation** | [RFC 8693](https://www.rfc-editor.org/rfc/rfc8693.txt) / [Keygen](https://keygen.sh/docs/activating-machines/) |
| 应用（插件） | **Product** | Keygen |
| 应用的计费策略 | **Policy**（时长/点数/机器数/心跳要求） | Keygen |
| 账号 + 钱包 | **License**（含 entitlement 与 usage） | Keygen / 通用 SaaS |
| 多开数 | **`maxMachines`**（Policy 字段） | Keygen floating license |
| 71 插件退出 | **Deactivation**（`DELETE machine`） | Keygen / [Steam `CancelAuthTicket`](https://partner.steamgames.com/doc/features/auth) |
| 41 心跳 | **Heartbeat ping**（维护 lease） | Keygen `requireHeartbeat` |
| 超时回收 | **Dead machine pruner** | Keygen |
| 72 插件清单 | **Entitlements query** | 通用 SaaS / AWS Marketplace `GetEntitlement` |

**一句话**：我们要做的是 **floating license + heartbeat lease**，这是 Keygen.sh、Cryptlex、LicenseSpring 等商业授权平台的标准模型，也是 Steam 这类"外壳启动子产品"的既成做法。

### 0.2 三个必须吸收的标准机制（原始方案缺的）

#### ① Activation 必须带 TTL（租约），不是永久令牌

Keygen 的 license file 带 `issued` / `expiry` / `ttl` 三个字段，客户端必须尊重 `expiry`：

> `expiry` represents the time at which the license file expires (i.e. `issued` + `ttl`), and `ttl` represents how long, in seconds, the license file is valid for.
> —— [Keygen Licensing API](https://keygen.sh/docs/api/cryptography/)

**影响**：原方案的"激活令牌"是长期有效的，只靠后台定时清理回收名额。标准做法是**租约**：

```
activation.expires_at = now + lease_ttl     (默认 30min，可配)
心跳（41）→ 续期 expires_at，可能轮换 token
到期未续 → 自动失效，名额释放（不依赖客户端调 71）
```

好处：客户端崩溃、被强杀、网络中断，名额都会自然回收，不再需要 `offline_timeout` 扫表兜底（扫表只作为二次清理）。

#### ② 激活必须幂等（fingerprint 维度）

Keygen 的 `POST /machines` 就是"激活"，同一 fingerprint 重复调用返回已有记录。**这是避免重复扣费的关键**：

```
激活（70）：WHERE app_uuid = ? AND member_uuid = ? AND fingerprint = ?
  ├─ 已存在且未过期 → 复用该记录，续租约，**不再扣费**，返回原 activation
  ├─ 已存在但已过期 → 复活该记录，**按新一次占用扣费**
  └─ 不存在 → 检查 maxMachines → 超限则拒绝 → 否则新建 + 扣费
```

原方案没写这条，客户端"启动-崩溃-重启"就会重复扣点。

#### ③ 心跳计时基准要明确（`heartbeatBasis`）

Keygen 有 `FROM_CREATION`（激活即开始计时）与 `FROM_FIRST_PING`（首次心跳开始计时）两种基准。

**这正好对应我们已有的 `points_heartbeat_charge`**：

| 我们的值 | Keygen 对应 | 语义 |
|---|---|---|
| `0` 登录预扣 | `FROM_CREATION` | 激活时立即扣一个周期 |
| `1` 心跳触发 | `FROM_FIRST_PING` | 首次心跳才扣，激活到首次心跳之间免费 |

命名应该向标准看齐，避免自造词。

### 0.3 另一条标准路径：不做设备激活，只做计数租约

Keygen 明确提供了 activation 的简化替代：

> If you're looking for something simpler than machine activation, then you should check out the license resource's usage-related actions. These actions allow you to increment/decrement a license's usage count as an alternative to implementing machine activation.
> —— [Keygen Activating Machines](https://keygen.sh/docs/activating-machines/)

| 路径 | 适用 | 我们的场景 |
|---|---|---|
| **Machine activation**（设备激活） | 需要绑定设备、一机一授权 | ✅ 我们已有 `machine_verify` 与 `bindings`，选这条 |
| **Usage count**（计数租约） | 只限制并发数，不关心是哪台机器 | 可作为 `machine_verify=0` 时的轻量实现 |

**建议**：主路径用 machine activation；当应用 `machine_verify=0` 时退化为纯计数租约（同一个 `activations` 表，`fingerprint` 填空），保持一张表。

### 0.4 标准错误语义

Keygen 用固定的 `code` 字符串表达失败原因，客户端据此分支而不是解析中文。我们应对齐：

| 我们的 `msg` | 建议 `code`（新增机器可读字段） | 标准来源 |
|---|---|---|
| 账号已到期 | `LICENSE_EXPIRED` | Keygen |
| 账号被封停/拉黑 | `LICENSE_SUSPENDED` | Keygen |
| 会员等级不足 | `ENTITLEMENT_REQUIRED` | 通用 |
| 点数不足 | `INSUFFICIENT_CREDITS` | 通用 |
| 机器数超限 | `MACHINES_LIMIT_EXCEEDED` | Keygen |
| 设备指纹不符 | `FINGERPRINT_MISMATCH` | Keygen |
| 租约已过期 | `LEASE_EXPIRED` | Keygen (`HEARTBEAT_DEAD`) |
| 需先进入应用 | `ACTIVATION_REQUIRED` | 通用 |
| 签名失败 | `SIGNATURE_INVALID` | 通用 |

响应体保留 `msg`（中文，可直接展示）+ 新增 `err`（上述机器码）。

### 0.5 与 Steam 模式的印证

Steam 做的是同一件事：

- **客户端登录** → 账号凭据（等价我们的账号令牌）
- **启动游戏** → `GetAuthSessionTicket(appid)` 拿 **per-app ticket**（等价我们的激活）
- **游戏内校验** → 拿着 ticket 向 Steam 后端验证
- **退出游戏** → `CancelAuthTicket` 释放
- **被验证方失效时会收到回调** → `ValidateAuthTicketResponse_t`

> When finished with a Session Ticket, `ISteamUser::CancelAuthTicket` must be called for every handle returned by `ISteamUser::GetAuthSessionTicket`.
> —— [Steamworks: User Authentication and Ownership](https://partner.steamgames.com/doc/features/auth)

**结论：我们的方案方向是对的，只是命名和几个机制细节需要按标准修。**

### 0.6 本方案据此的修订清单

| 项 | 原方案 | 修订为 |
|---|---|---|
| 激活 | 长期令牌 + 定时清理 | **租约**：`expires_at = now + lease_ttl`，心跳续期 |
| 激活 | 每次调用都扣费 | **幂等**：同 `(app, member, fingerprint)` 未过期则复用、不扣费 |
| 释放 | 依赖 71 或超时扫表 | 71 显式释放 + 租约自然到期，扫表降为兜底 |
| 计时基准 | `points_heartbeat_charge` 自造 | 对齐 `heartbeatBasis`：`FROM_CREATION` / `FROM_FIRST_PING` |
| 错误 | 只有中文 `msg` | `msg`（展示用）+ `err`（机器码，标准命名） |
| 表名 | `member_sessions` | **`app_activations`**（语义即 activation，与标准一致） |
| 清单 | 72 插件清单 | 保留，语义对齐 **entitlements** |
| 设备维度 | 未明确 | `fingerprint` 作为激活主键之一；`machine_verify=0` 时填空走计数租约 |

---

## 一、问题：登录即计费与外壳架构冲突

### 1.1 现状流程

当前 `type 20 账号登录` / `type 10 卡密登录` 一条路径做完了四件事：

```
1. 校验账号凭据          ← 账号级
2. 校验机器码/IP 绑定     ← 账号级
3. 按 app_uuid 取计费策略并扣费   ← 应用级 ⚠️
4. 建会话、控多开        ← 应用级 ⚠️
```

在外壳 + 插件模型下：

| 场景 | 实际发生的事 | 问题 |
|---|---|---|
| 用户打开管理器，登录账号 | 必须传某个 `app_uuid` 才能登录 | 外壳不是业务单元，传谁都不对 |
| 登录成功瞬间 | 按那个 `app_uuid` 扣了费 | **还没启动任何插件就被扣费** |
| 用户点开插件 A | 需要再次登录才能拿到 A 的会话 | 重复登录、重复扣费 |
| 用户点开插件 B | 又要登录 | 反复进出反复扣 |
| 管理器界面停留 10 分钟没开插件 | 按时模式仍在扣 | 纯浪费用户的点数 |

根因：**计费绑定在"身份认证"上，而计费的真正语义是"占用某个插件的使用权"。**

### 1.2 本质矛盾

```
身份（我是谁）        —— 全站唯一，一次认证
使用权（我在用哪个插件）—— 每插件独立，按需获取
```

必须拆成两步。

---

## 二、方案：二级令牌

### 2.1 模型

```
┌─────────────────────────────────────────────────┐
│ 账号令牌 AccountSession（全站唯一，不计费）        │
│   token / member_uuid / 设备 / IP / 版本          │
│   "证明你是谁"                                    │
└──────────────────┬──────────────────────────────┘
                   │  插件认证（换取代价：此处计费）
                   ▼
┌─────────────────────────────────────────────────┐
│ 激活 AppSession（每插件一条，按需创建）        │
│   token / account_token / member_uuid / app_uuid │
│   "证明你在用哪个插件、且已付费"                   │
└─────────────────────────────────────────────────┘
```

**计费只发生在「账号令牌 → 激活」这一步**，即用户真正启动某个插件的时候。

### 2.2 生命周期

```
管理器启动
  └─ 20 账号登录 ──────────────► 账号令牌（不计费、不校验绑定、不算多开）
       │
用户点「启动插件 A」
       ├─ 70 插件认证 {token} ──► 激活 A（计费、绑定校验、多开计数）
       │     └─ 41 心跳 ───────► 按时续费 / 保活
       │     └─ 42/44/45 ──────► 插件内数据与功能
       └─ 71 插件退出 ────────► 删除激活 A（释放多开名额，已扣不退）

用户点「启动插件 B」
       └─ 70 插件认证 {token} ──► 激活 B（独立计费、独立多开额度）
```

同一账号可同时持有多个激活，各按自己的计费模式扣费 —— 这正是多插件并行的正常形态。

### 2.3 关键语义

| 问题 | 结论 |
|---|---|
| 计费次数 | 每次「启动插件」算一次（按次模式），不是每次登录 |
| 期间切换插件 | 退出 A 再进 B，A 的周期不退、B 重新开始 |
| 管理器内停留 | 不计费（没有激活） |
| 绑定校验 | 移到插件认证。管理器登录**不校验机器码**，避免外壳本身被绑定限制卡住 |
| 多开计数 | 改为**激活**维度：同账号在同插件的活跃激活数 ≤ 该插件有效多开数 |
| 手动顶号 | 返回的会话列表仍是**本插件**的激活 |
| 账号令牌过期 | 连带失效其下所有激活 |
| 登出（30） | 删除账号令牌 + 其全部激活 |

---

## 三、与 ktrw-manager 的映射

### 3.1 实体对应

| NetworkAuth | ktrw-manager | 说明 |
|---|---|---|
| **应用 `App`** | **插件 `.zex`** | `meta.json` 的 `id`（如 `com.example.esp`）就是 `app_uuid` |
| **外壳 / 宿主** | **管理器 APK + 模块** | 新增 `apps.is_host=1` 的宿主应用，只调账号级接口 |
| `machine_code` | **`KTRW_DEVICE_CODE`** | 现成：`device_fp` 前 16 位 hex，启动插件时注入给插件 |
| 账号登录 | 管理端 WebUI 里登录 | 一次性，取得账号令牌 |
| 插件认证 | 点「启动插件」时管理器代调 | 取得 `app_token` |
| 激活 token | 注入 `KTRW_LICENSE_TOKEN` | 插件可读，做本地校验（可选） |
| 心跳 `41` | **管理器后台轮询代调** | 插件零改动 |

### 3.2 ktrw-manager 的实际执行链

```
WebUI (app/src/main/assets/webroot/app.js)
   │  window.KtrwBridge.exec("ktrw-ctl start-plugin com.example.esp")
   ▼
MainActivity$KtrwBridge（Java）
   │  root Daemon 127.0.0.1:5879
   ▼
ktrw-ctl（/data/adb/modules/ktrw-manager/bin/ktrw-ctl，POSIX shell）
   │  ① 验签（ktrw-loader --ktrw-meta 取签名元数据）
   │  ② 取 targets / caps / type
   │  ③ 注入型：inject_plugin() → ktrw-loader payload.enc --ktrw-inject <pid>
   │     普通型：导出 KTRW_* 环境变量 → nohup ktrw-loader --ktrw-run
   ▼
插件进程（payload）
```

**关键：插件启动是本地 shell 行为，不经过任何网络。** 授权必须由管理器在调用 `start-plugin` **之前**完成，或由 `ktrw-ctl` 自己发起。

### 3.3 现有环境变量（`ktrw-ctl` 已注入给插件）

```
KTRW_DEV / KTRW_BACKEND / KTRW_KMI
KTRW_PLUGIN_ID / KTRW_PLUGIN_DIR / KTRW_PLUGIN_DATA
KTRW_PLUGIN_TARGETS / KTRW_PLUGIN_CAPS / KTRW_PLUGIN_TARGET_UIDS
KTRW_DRIVER_VER
KTRW_DEVICE_FP / KTRW_DEVICE_CODE      ← 设备码，可直接当 machine_code
KTRW_SET_<KEY>                          ← 插件设置（settings.conf → 环境变量）
PLUGIN_DEV / PLUGIN_DATA / PLUGIN_ID / PLUGIN_DIR / PLUGIN_TARGETS
```

**本方案要新增**：`KTRW_LICENSE_TOKEN`、`KTRW_LICENSE_APP`、`KTRW_LICENSE_EXPIRE`。

> ⚠️ **现存缺陷（顺带修）**：`start-plugin` 分支里，注入型插件（`type=inject`）走 `inject_plugin` 后**直接 return**，跳过了后面的 `KTRW_SET_*` 导出与设备码计算。而 demo 插件（`type: inject`）却在读 `KTRW_DEVICE_CODE` 和 `KTRW_SET_fov` —— 实际拿不到。
> 修法：把「设置导出 + 设备码计算」提到 `type` 判断之前，或在 `inject_plugin` 内补同样的注入。

### 3.4 现有授权与新增授权是两层

| 层 | 载体 | 校验者 | 作用 |
|---|---|---|---|
| **L1 框架授权**（已有） | `license.bin`（Ed25519 设备证书） | 驱动 `kb_license_impl.c` + `ktrw-ctl` | 决定「这台设备能不能用这个框架」 |
| **L2 插件授权**（本方案新增） | NetworkAuth 激活 | 管理器（HTTP） | 决定「这个账号能不能用这个插件」 |

两层独立、可叠加：L1 是整机授权，L2 是账号+插件授权。

---

## 四、职责分配（核心决策）

### 4.1 三种可选架构

| 方案 | 做法 | 代价 |
|---|---|---|
| A. 插件自己验证 | 插件声明 `net` cap，启动后自己调授权接口 | 每个插件都要实现登录/心跳；插件可被改；无法统一停用 |
| B. 管理器代验（推荐） | 管理器在 `start-plugin` 前调 70，把 token 注入环境变量；管理器轮询心跳 | 需改管理器（Java + app.js + ktrw-ctl）；**插件零改动** |
| C. ktrw-ctl 内验证 | 在 `ktrw-ctl` 里 curl 调授权接口 | Android 无保证有 curl；shell 里做 HTTPS + 签名困难；无法反映 UI 状态 |

**采纳 B。** 理由：

- 插件开发者不需要对接任何授权逻辑，只读一个环境变量做可选校验；
- 停用/欠费由管理器统一处理（自动 `stop-plugin`），不依赖插件配合；
- 积分/到期展示在管理端，用户体验统一。

### 4.2 管理器代验的完整链路

```
① 登录（一次性）
   WebUI → KtrwBridge.httpPost(/api/open, {api_type:20, app_uuid:host_uuid, ...})
        → 存 account_token 到 localStorage

② 启动插件（每次）
   WebUI 读插件 id → 查后台应用列表映射（id → app_uuid/secret）
   WebUI → KtrwBridge.httpPost(/api/open, {api_type:70, app_uuid:插件id, ...})
        ← { token: appToken, expired_at, points, ... }
   WebUI → KtrwBridge.writeFile(run/<id>.license, appToken, 0600)   ← token 落文件
   WebUI → KtrwBridge.exec("ktrw-ctl start-plugin com.example.esp")
        → ktrw-ctl 注入 KTRW_LICENSE_TOKEN 后启动插件
        → 若返回 ERR（认证失败/点数不足）→ 不启动，弹提示

③ 心跳（管理器后台，每 N 分钟）
   Daemon 遍历运行中插件 → 每个插件用自己的 app_uuid/secret 调 41
        失败 → 自动执行 ktrw-ctl stop-plugin <id>，WebUI 提示「授权失效」

④ 停止插件
   WebUI → ktrw-ctl stop-plugin <id>  → 再调 71 释放激活
```

> ③ 为什么放后台而不是插件里：插件是注入到游戏进程的 payload，不能保证有网络权限（未声明 `net` 的插件连 socket 都被 seccomp 拒），也不能保证活到该心跳的时刻。管理器始终在，更适合做保活与停用。

---

## 五、接口矩阵

| api_type | 接口 | 令牌 | 计费 | 行业对应 | 备注 |
|---|---|---|---|---|---|
| 1 / 2 / 3 / 4 | 公告 / 更新 / 版本 / 卡密信息 | 无 | 否 | — | 免登录 |
| **10** | 卡密登录 | 无（卡密即凭据） | **否** | license key auth | 发**账号令牌** |
| **20** | 账号登录 | 无 | **否** | OAuth password/ROPC | 发**账号令牌** |
| 21 / 23 / 25 / 26 | 注册 / 验证码 / 改密 | 无 | 否 | — | 账号级 |
| 22 / 24 | 充值 / 领试用 | 账号令牌 或 凭据 | 否 | — | 作用于钱包 |
| **70** | **应用激活（新）** | 账号令牌 | **是（幂等）** | Keygen `POST /machines` | 发 **activation_token + 租约** |
| **71** | **应用释放（新）** | 激活令牌 | 否 | Keygen `DELETE /machines` | 立即释放名额 |
| **72** | **权益清单（新）** | 账号令牌 | 否 | Entitlements | 下发 `plugin_id → uuid + secret`，按权限过滤 |
| 30 | 退出登录 | 账号令牌 | 否 | — | 连带失效全部激活 |
| 31 | 手动顶号 | 凭据 | 否 | — | 只踢本应用的激活 |
| 40 | 到期/余额查询 | 账号令牌 | 否 | license validate | 账号级信息 |
| **41** | 心跳 / 续租约 | **激活令牌** | 按时续费 | Keygen heartbeat ping | 续 `lease_expires_at` |
| 42 / 43 / 44 / 45 / 54 | 插件数据 / 变量 / 函数 / 用户数据 | **激活令牌** | 否 | — | 插件级 |
| 50 | 改密码 | 账号令牌 | 否 | — | |
| 51 | 转绑 | 凭据 | 否 | — | 账号级绑定 |
| **53** | 功能扣点 | **激活令牌** | 是（显式） | usage increment | 扣全站钱包 |
| 60 / 61 / 62 | 风控 | 签名 | 否 | — | 服务端调用 |

### 5.1 `70 应用激活`（幂等）

**请求** `data`：`{ "token": "账号令牌", "fingerprint": "KTRW_DEVICE_CODE", "version": "插件版本", "device_name": "机型" }`

信封 `app_uuid` = 该插件的 **`uuid`**（由 `72 权益清单` 下发）。

**语义（按 Keygen machine activation）**：

```
1. 校验账号令牌
2. 校验应用存在、启用、非 is_host
3. 门禁（free / 等级 / 到期 / 点数）
4. 绑定校验（machine_verify / ip_verify）
5. 幂等查找 WITHIN 行锁：
     已有 (app_uuid, member_uuid, fingerprint) 且活跃 → 复用 + 续租约，**不扣费**
     已有但已死（deactivated 或租约过期）→ 复活 + 按新占用扣费
     不存在 → 检查 maxMachines → 超限走顶号/拒绝 → 新建 + 扣费
6. 返回 activation_token + lease_expires_at
```

**成功返回**：

```json
{
  "token": "激活令牌（租约凭证）",
  "expires_at": "2026-11-14T17:05:00+08:00",
  "lease_ttl": 1800,
  "renew_after": 600,
  "reused": true,

  "account_token": "账号令牌（原样带回）",
  "app_uuid": "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B",
  "plugin_id": "com.example.esp",
  "app_name": "透视辅助演示",

  "mode": 1, "free": 0, "required_level": 1,
  "permanent": false,
  "expired_at": "2026-12-01T00:00:00+08:00",
  "points": 480,
  "heartbeat_interval": 10,

  "level": 3, "level_name": "黄金", "level_color": "#E6A23C"
}
```

| 字段 | 说明 |
|---|---|
| `expires_at` / `lease_ttl` | **租约到期时间与时长**。客户端必须在到期前调 `41` 续约 |
| `renew_after` | 建议何时续约（`lease_ttl / 3`），比死等到期更稳 |
| `reused` | `true` 表示复用了已有激活（未重复扣费），便于客户端与后台核对 |

**失败**：`code=1` + `msg`（展示）+ `err`（机器码，见 0.4）；手动顶号满员返回 `code=3` + `sessions`。

### 5.2 `71 应用释放`（deactivation）

请求 `{ "token": "激活令牌" }` → `{ "released": true }`。

- 置 `deactivated_at = now`，**立即释放一个 `maxMachines` 名额**；
- 幂等：已释放/不存在都返回 `released: true`；
- **已扣点数不退**（按次已消费；按时的当前周期仍算已用）—— 与 Keygen deactivation 一致。

### 5.3 `41 心跳 / 续租约`

请求 `{ "token": "激活令牌" }`。

```
1. 校验激活令牌有效且未释放
2. lease_expires_at < now → LEASE_EXPIRED（客户端须重新 70 激活）
3. 门禁复查（账号是否被封、是否到期、点数是否够下个周期）
4. 按时模式：若 period_end < now → 扣下个周期；点数不足 → INSUFFICIENT_CREDITS 并置死
5. 续 lease_expires_at = now + lease_ttl
6. 返回最新钱包与新的 renew_after（可选轮换 token）
```

返回体即 `StatusResult` + `expires_at` / `renew_after`。

> **续约失败的处置**：服务端置激活为死（`deactivated_at = now`），客户端收到 `LEASE_EXPIRED` / `INSUFFICIENT_CREDITS` 后必须停止插件。
> 这与 Steam 的 `ValidateAuthTicketResponse_t` 回调是同一职责 —— 只是我们用**同步返回**代替回调，因为管理器本来就在轮询。

### 5.4 `72 权益清单`（entitlements）

管理器登录后拉一次，拿到「本账号可用插件」的 `plugin_id → uuid + secret` 映射与权益判定。**secret 只在这里下发，不写死在 APK 里。**

**请求**：`{ "token": "账号令牌", "fingerprint": "KTRW_DEVICE_CODE" }`

**返回**：

```json
{
  "plugins": [
    {
      "plugin_id": "com.example.esp",
      "uuid": "3F2A9C8E-1B4D-4E6F-8A2B-9C0D1E2F3A4B",
      "secret": "9F86D081884C7D659A2FEAA0C55AD015",
      "name": "透视辅助演示",
      "version": "2.1.0",
      "min_version": "2.0.0",
      "mode": 2,
      "free": 0,
      "required_level": 2,
      "usable": 1,
      "reason": ""
    },
    {
      "plugin_id": "com.example.aim",
      "uuid": "7B1E4D2A-...",
      "secret": "…",
      "name": "自瞄",
      "usable": 0,
      "reason": "会员等级不足，需要 黄金 及以上"
    }
  ],
  "account": { "username": "a@b.com", "level": 1, "level_name": "默认会员",
               "points": 480, "permanent": false, "expired_at": "..." }
}
```

设计要点：

| 点 | 说明 |
|---|---|
| **只下发有权使用的插件** | 未购买/等级不足的插件不给 `secret`（`usable=0` 时 `secret` 留空），避免逆向 APK 后拿到全部密钥 |
| **`usable` / `reason` 预判** | 管理器据此直接把不可用插件置灰并显示原因，减少无意义的 70 调用 |
| **`min_version`** | 插件强制更新：管理器比对本地插件版本，过旧则拒绝启动并提示更新 `.zex` |
| **清单可缓存** | 建议缓存到本地，失效条件：账号令牌过期 / 手动刷新 / 收到 `code=4`（清单已变更） |
| **`secret` 可能轮换** | 后台重置应用密钥后，旧 secret 立即失效；管理器需重新拉清单。收到签名错误（`code=1` + 「签名校验失败」）时应自动重拉一次 |

> 为什么不复用后台的 `AppsSimpleListHandler`：那是管理端接口（需管理员登录态），且返回全部应用。这里需要的是**面向终端账号、按权限过滤、含 secret** 的清单，语义完全不同。

---

## 六、NetworkAuth 侧改动

### 6.1 应用标识：`uuid` 仍自动生成，另加 `plugin_id`

先说清两件不同的事：

| 标识 | 谁产生 | 用途 | 是否必须随机 |
|---|---|---|---|
| **`uuid`** | 系统生成 | 协议里的 `app_uuid`、签名第一段、数据归属 | 是（天然唯一） |
| **`secret`** | 系统生成 | 签名密钥（32 位大写 hex） | **是，绝不能是插件 id** |
| **`plugin_id`** | 插件作者在 `meta.json` 里定 | 把「后台应用」和「`/data/adb/ktrw-manager/plugins/<id>/`」对上 | 否，反向域名即可 |

**推荐做法：`uuid` 与 `secret` 都保持自动生成，另加 `plugin_id`（唯一索引）做映射。**

理由：插件 id 是**第三方作者自己定的**。一号通若服务多条产品线或多个作者，`com.example.esp` 这类 id 必然撞名；直接拿它当 `uuid` 会破坏「uuid 天然唯一」这条前提，而且插件一旦改 id，授权记录全断。

| 改动 | 位置 |
|---|---|
| `apps` 新增 `plugin_id`，`size:64;uniqueIndex`，允许为空（不接插件系统的应用留空） | `models/app.go` |
| 创建/更新应用时校验 `plugin_id` 只含 `[a-z0-9._-]` | `controllers/admin/app.go` |
| **`uuid` 保持 36 位 UUID 不变，不动校验** | — |

> **例外**：如果确定只有你自己一条产品线、插件 id 全局由你控制，也可以让 `uuid = plugin_id` 省掉这张映射。此时仍需放宽 `uuid` 长度（36 → 64）与格式校验。
> 但既然加 `plugin_id` 的成本只是一个字段 + 一次查表，建议保留 —— 将来接入第二套插件体系时不用返工。

**管理器怎么知道 `plugin_id → uuid + secret`？**

不写死在 APK 里，改为**登录后向后台拉清单**（新增 `type 72`，见 5.3）。好处：

- APK 被逆向只能拿到服务器地址，拿不到任何 secret；
- 只下发**该账号有权使用**的插件（按等级/授权过滤），未购买的插件连 secret 都看不到；
- 插件下线、secret 轮换，客户端下次拉清单自动生效。

### 6.2 数据模型（按 Keygen activation 模型）

#### 新增 `member_account_tokens`（账号令牌）

等价 OAuth 的 Access/Refresh Token，长期、可刷新、不做 audience 绑定。

| 字段 | 说明 |
|---|---|
| `id` / `uuid` | 主键 |
| `token` | 唯一索引 |
| `refresh_token` | 唯一索引，用于续期（账号令牌也应有 TTL，避免永久有效） |
| `member_uuid` | 索引 |
| `fingerprint` / `device_name` / `ip` / `version` | **仅记录，不校验**（绑定校验在激活时做） |
| `expires_at` | 账号令牌过期时间（建议 30 天） |
| `last_active_at` | 账号级接口刷新 |

#### 新增 `app_activations`（应用激活）

这是本方案的核心表，直接对应 Keygen 的 **machine activation**。

| 字段 | 说明 |
|---|---|
| `id` / `uuid` | 主键 |
| `token` | 唯一索引，**激活令牌（短期租约凭证）** |
| `member_uuid` | 索引 |
| `app_uuid` | 索引，哪个应用/插件 |
| `account_uuid` | 关联账号令牌，登出时级联失效 |
| `fingerprint` | **设备指纹**（= `KTRW_DEVICE_CODE`）；`machine_verify=0` 时填空走计数租约 |
| `device_name` / `ip` / `version` | 激活时的采集信息 |
| `lease_expires_at` | **租约到期时间**（`now + lease_ttl`，心跳续期） |
| `charge_basis` | 计费基准：`FROM_CREATION` / `FROM_FIRST_PING`（对应 `points_heartbeat_charge`） |
| `period_end` | **按时模式下当前已付费周期的结束时间** |
| `deactivated_at` | 显式释放时间（NULL = 活跃） |
| `created_at` / `updated_at` | — |

**唯一索引**：`(app_uuid, member_uuid, fingerprint)` —— 这是**幂等激活**的基石：同一账号在同一应用的同一设备只会有一条激活记录。

> **`period_end` 为什么必须在这里**：`members.expired_at` 是全站钱包字段。若插件 A 的按时预扣写它，等于顺手把 B 的可用期也延长了 —— 多插件按时计费会互相污染。按周期记账必须落到**激活级**。

#### `member_sessions` 的处置

现有 `member_sessions` 与新表高度重叠。**建议直接替换**（本版本按全新安装分发，无需迁移）：

- 新表 `app_activations` 承担"激活"的全部职责，并补齐 `fingerprint` / `lease_expires_at` / `period_end`；
- 后台「在线列表」改为读 `app_activations WHERE deactivated_at IS NULL`；
- 旧的 `member_sessions` 从 `AllModels()` 移除。

> ⚠️ 如果不想动现有表，也可以给 `member_sessions` 补 4 个字段（`fingerprint` / `lease_expires_at` / `period_end` / `deactivated_at`）并沿用表名。但既然已经是"激活"语义，改名更清晰，且能避免"会话"与"令牌"两个词混用。

#### `apps` 新增字段

| 字段 | 说明 |
|---|---|
| `is_host` | 宿主应用标记（管理器自身），`is_host=1` 不能作为插件被激活 |
| `lease_ttl_minutes` | **新增**：激活租约时长（默认 30，等于旧 `offline_timeout`，但语义是"租约"而非"离线判定"） |
| `max_machines` | 可复用现有 `multi_open_count`（Keygen 的 `maxMachines`），不必新增 |

#### `member_levels` 可选新增

| 字段 | 说明 |
|---|---|
| `max_concurrent_apps` | 同时活跃的不同应用激活数上限（0=不限） |

### 6.3 服务层

| 文件 | 改动 |
|---|---|
| `services/account_token.go`（新） | `IssueAccountToken` / `AuthAccount(token)` / `RefreshAccountToken` / `RevokeAccountToken` |
| `services/activation.go`（新） | `Activate(accountToken, app, fingerprint, ...)` **幂等**、门禁+计费+绑定+机器数检查；`Deactivate(activationToken)`；`RenewLease(activationToken)`；`PruneDeadActivations()` |
| `services/member_auth.go` | `CardLogin` / `AccountLogin` 只发账号令牌（卸掉计费/绑定/多开）；原 `finishMemberLogin` 拆成 `issueAccountToken` + `activate` |
| `controllers/public/api.go` | 按接口矩阵选择令牌类型；实现隐式激活；新增 70/71 |
| `services/member_session_cleanup.go` | 改为清理「过期账号令牌」与「租约已死的激活」（Keygen 的 dead machine pruner） |
| `controllers/admin/online.go` | 在线列表改读 `app_activations`，展示设备指纹与租约剩余时间 |

### 6.4 激活状态机（实现时按这个写）

```
                     Activate(70)
                          │
     ┌────────────────────┼────────────────────┐
     │                    │                    │
 已存在且活跃          已存在但已死          不存在
     │                    │                    │
 复用 + 续租约        复活 + 按新占用扣费    检查 maxMachines
 **不扣费**               │                    │
     │                    │              ┌─────┴─────┐
     │                    │            未超限       超限
     │                    │              │           │
     │                    │           新建+扣费   MACHINES_LIMIT_EXCEEDED
     │                    │              │        （或顶号/手动顶号）
     └────────────────────┴──────────────┘
                          │
                   返回 activation_token + lease_expires_at
                          │
              ┌───────────┼───────────┐
              │                       │
        RenewLease(41)          Deactivate(71)
        续 lease_expires_at      置 deactivated_at
        可能轮换 token           立即释放名额
              │                       │
              └──────────┬────────────┘
                         │
                  租约到期未续
                  → 自动失效（pruner 兜底）
```

**「已死」的判定**：`deactivated_at IS NOT NULL` 或 `lease_expires_at < now`。

---

## 七、ktrw-manager 侧改动

### 7.1 `KtrwBridge` 增加 HTTP 能力

WebView 是 `file://` 源，直接 `fetch` 会受跨域与混合内容限制。走 Java 桥最稳。

```java
// MainActivity.java（KtrwBridge 内）
@JavascriptInterface
public String httpPost(String url, String body, String headersJson) {
    // HttpURLConnection，5s connect / 10s read
    // 返回 {ok:true,status:200,body:"..."} 或 {ok:false,error:"..."}
}
```

> 也可让 NetworkAuth 加 CORS 允许 `file://` 源 + 允许 `Origin: null`，但 WebView 的 origin 行为在各版本不一致，桥接更可控。

### 7.2 `app.js` 增加登录、拉清单与启动前认证

```js
// ① 登录（一次）
async function login(username, password) {
  const body = envelope(HOST_UUID, HOST_SECRET, 20, { username, password, version: APP_VER });
  const r = await httpPost(API_BASE + "/api/open", body);
  state.accountToken = r.data.token;           // 账号令牌
  localStorage.setItem("na_account", state.accountToken);
  await pullPlugins();                          // 立刻拉清单
}

// ② 拉插件清单（登录后/会话失效/密钥轮换时）
async function pullPlugins() {
  const body = envelope(HOST_UUID, HOST_SECRET, 72,
    { token: state.accountToken, machine_code: state.deviceCode });
  const r = await httpPost(API_BASE + "/api/open", body);
  // { plugin_id, uuid, secret, usable, reason, min_version, ... }
  state.plugins = Object.fromEntries(r.data.plugins.map(p => [p.plugin_id, p]));
  localStorage.setItem("na_plugins", JSON.stringify(state.plugins));
  renderPluginList();                           // 不可用的直接置灰 + 显示 reason
}

// ③ 启动插件（每次）
async function startPlugin(pluginId) {
  const p = state.plugins[pluginId];
  if (!p)                 return toast("未知插件，请刷新清单");
  if (!p.usable)          return toast(p.reason || "无使用权限");
  if (!p.secret)          return toast("该插件未授权，请购买后再试");
  if (versionLt(localVersion(pluginId), p.min_version))
    return toast(`插件版本过低，请更新至 ${p.min_version}`);

  // 用「该插件自己的 uuid + secret」签名
  const body = envelope(p.uuid, p.secret, 70, {
    token: state.accountToken,
    machine_code: state.deviceCode,             // ktrw-ctl device-id 的 CODE
    version: localVersion(pluginId),
    device_name: state.model
  });
  const r = await httpPost(API_BASE + "/api/open", body);
  if (r.code !== 0) {
    if (r.code === 4) { await pullPlugins(); }   // 清单已变更，重拉后重试
    return toast(r.msg);                          // 等级不足/点数不足/未登录…
  }

  const appToken = r.data.token;                  // 激活
  // token 落文件（0600），不走命令行参数，避免 /proc/<pid>/cmdline 泄露
  await writeFile(`/data/adb/ktrw-manager/run/${pluginId}.license`, appToken, 0600);
  const out = await exec(`ktrw-ctl start-plugin ${shellQuote(pluginId)}`);
  if (out.startsWith("ERR")) return toast(out);

  state.appTokens[pluginId] = appToken;
  state.timers[pluginId] = setInterval(() => heartbeat(pluginId),
    r.data.heartbeat_interval * 60000);
}
```

**签名**：`sign = SHA256(app_uuid|api_type|data|timestamp|app_secret)`，`app_uuid` 用**该插件的 `uuid`**（来自清单），`app_secret` 用**该插件的 `secret`**。

- 宿主应用（`HOST_UUID` / `HOST_SECRET`）只用于 `20` / `72` 这类账号级接口；
- 每个插件用自己的 `uuid`/`secret` 签名 `70`/`41`/`42`，应用间身份隔离仍然成立；
- **`secret` 来自 `72` 清单，不写死在 APK**；若后台重置了某应用的密钥，管理器收到签名失败后应自动重拉清单。

### 7.3 `ktrw-ctl` 读取授权文件并统一注入

```sh
start-plugin)
  p="$2"
  # token 由管理器事先写入 run/<id>.license（0600），不走命令行参数
  licfile="$RUN_DIR/$p.license"
  lic=""
  [ -f "$licfile" ] && lic=$(cat "$licfile" 2>/dev/null)
  ...
  # 传输：放文件而非 argv（argv 在 /proc/<pid>/cmdline 可见）
  # 注入（普通型与注入型都要）
  KTRW_LICENSE_TOKEN="$(cat "$licfile" 2>/dev/null)" \
  KTRW_LICENSE_APP="$p" \
  KTRW_DEVICE_CODE="$dev_code" \
  ...
```

> ⚠️ **安全细节**：token **不能**走命令行参数 —— `/proc/<pid>/cmdline` 对同机任意进程可读。正确做法：管理器先把 token 写入 `run/<id>.license`（0600），`start-plugin` 无参调用、自己读文件。

### 7.4 管理器后台心跳

`service.sh` 起一个轻量循环（或放进既有 Daemon）：

```sh
# 每 60s 检查一次各运行中插件
for id in $(list_running_plugins); do
  tok=$(cat "$RUN_DIR/$id.license" 2>/dev/null)
  [ -n "$tok" ] || continue
  resp=$(http_post /api/open "$(envelope "$id" 41 "{\"token\":\"$tok\"}")")
  if [ "$(json_field "$resp" code)" != "0" ]; then
    ktrw-ctl stop-plugin "$id"
    ui_notify "$id 授权失效：$(json_field "$resp" msg)"
  fi
done
```

> HTTP 客户端选择：Android 自带 `toybox wget` 不一定可用；建议由 **Java Daemon 承担**（它已在跑，加一个定时任务即可），shell 只负责 `stop-plugin`。

### 7.5 顺带修复：注入型插件环境变量缺失

统一在 `start-plugin` 里、`type` 分支**之前**完成：

```
1. 验签取 meta
2. 计算 device_fp / dev_code
3. 导出 KTRW_SET_*
4. 读 run/<id>.license → KTRW_LICENSE_*
5. 再按 type 分派 inject_plugin / 普通启动
```

这样注入型插件也能拿到设备码、设置与授权 token（demo 现在读不到，是个既有 bug）。

---

## 八、兼容老客户端：隐式插件认证

老客户端只调 `20 登录` → 直接调 `41 心跳 / 42 数据`。不能让它坏掉。

信封本来就带 `app_uuid`，服务端在需要激活的接口里做：

```
校验 token：
  ├─ 命中激活 → 直接用
  └─ 命中账号令牌 →
        ├─ 信封 app_uuid 是宿主应用或空 → 拒绝「需先进入应用」
        └─ 信封 app_uuid 是真实应用 → 隐式执行插件认证（校验+计费+绑定+多开）
              → 建激活 → 继续处理本次请求
```

行为变化：

| 原行为 | 新行为 |
|---|---|
| 20 登录时扣费 | 20 登录不扣；首次调该应用的 41/42 时扣 |
| 登录后立刻退出 | **不扣费**（对用户有利） |
| 计时从登录开始 | 从首次进入应用开始 |

对单应用客户端差异极小。对 ktrw-manager 这种新对接，建议直接走显式 70/71。

---

## 九、边界与风险

| 风险 | 处置 |
|---|---|
| **token 出现在 `ps`** | 用 `run/<id>.license` 文件（0600）传，不用 argv |
| **WebUI 被改**（APK 可重打包） | 授权是商业授权不是安全边界；root 用户能改任何东西。要更强就靠 L1 `license.bin` + 服务端风控（同账号多设备并发限制） |
| **插件绕过管理器直接 `ktrw-ctl start-plugin`** | 可加：`start-plugin` 校验 `run/<id>.license` 存在且在有效期，否则拒绝。⚠️ 但 root 可自己写这个文件 —— 仍属"防君子" |
| **按时模式跨插件串味** | `charge_period_end` 下沉到会话级（见 6.2） |
| **反复进出刷次数** | 按次模式每次 70 都扣。可加应用级 `reenter_grace_minutes`（同插件 N 分钟内重进不重复扣） |
| **账号令牌泄漏** | 只能换激活，不能直接调插件级接口；配合 `offline_timeout` 兜底 |
| **插件退出不调 71** | 多开名额要等 `offline_timeout`（默认 30 分钟）才释放。管理器 `stop-plugin` 时应同步调 71 |
| **隐式认证并发扣费** | 沿用 `lockMemberRow` 串行化，「是否已有该激活」的查询必须在锁内 |

---

## 十、实施顺序

| 阶段 | 内容 | 预估 |
|---|---|---|
| **S0** | NetworkAuth：`app.uuid` 放宽到 64 / 允许反向域名；加 `is_host` | 0.5 天 |
| **S1** | NetworkAuth：`member_account_tokens` + `app_activations` 两张表（含 `fingerprint` / `lease_expires_at` / `period_end` / `deactivated_at`）、迁移清单、`member_sessions` 退场 | 0.5 天 |
| **S2** | NetworkAuth：`AuthAccount` / `Activate`（幂等）/ `Deactivate` / `RenewLease`；登录卸掉计费与绑定 | 1.5 天 |
| **S3** | NetworkAuth：70/71/72 handler；dispatch 按矩阵路由；隐式激活；标准错误码 `err` | 1 天 |
| **S4** | NetworkAuth：清理任务、在线列表、后台应用表单（宿主开关 / 插件 id） | 0.5 天 |
| **S5** | ktrw-manager：`KtrwBridge.httpPost`；`app.js` 登录 + 拉权益 + 激活 + 按时续约 | 1 天 |
| **S6** | ktrw-manager：`ktrw-ctl` 读授权文件 + 统一环境注入（含修注入型插件的缺失） | 0.5 天 |
| **S7** | 测试：不变量（幂等激活/租约过期/释放名额/隐式兼容）+ 端到端真机 | 1.5 天 |
| **合计** | | **约 7 天** |

### 必测不变量

1. 账号登录后**不扣点、不建激活、不校验绑定**；
2. 同账号在插件 A、B 各建一条激活，各自计费互不影响；
3. 停止 A 后 A 的多开名额立即释放，B 不受影响；
4. **幂等激活**：同 `(app, member, fingerprint)` 重复 70 只扣一次费，返回 `reused: true`；
5. **租约到期**：不调 41 续约，`lease_expires_at` 过后激活自动失效、名额释放；
6. 隐式激活：老客户端 20 + 41，只扣一次；
7. 并发两次 70，只有一个成功扣费建激活（或按 maxMachines 正确计数）；
8. 账号令牌登出 → 其下全部激活一并失效；
9. 宿主应用（`is_host=1`）调 70 被拒；
10. 按时模式下 A 的周期记账不延长 B 的可用期（`period_end` 下沉）；
11. `ktrw-ctl start-plugin` 在 `run/<id>.license` 缺失时拒绝启动（可选加固）。

---

## 十一、与一号通现有设计的衔接

**不改动**：

- 钱包（`expired_at` / `points` / `total_recharge` / `level`）仍全站唯一；
- 卡密、套餐、等级、账号策略仍全站一套；
- 27 条接口定义仍全站唯一、默认启用；
- 计费三模式（会员 / 点数 / 混合）与 `app_level_points` 不变。

**只是把"什么时候触发计费"从登录时机挪到进入插件时机**，并拆出两级令牌。接口编号 `1~62` 全部保留，新增 `70` / `71`。

---

## 十二、待确认

- [ ] **Q1**：管理器是否单独建 `is_host=1` 的宿主应用？（建议是）
- [x] **Q2**：**`uuid` 与 `secret` 均保持系统生成；新增 `apps.plugin_id` 做映射**。插件 id 由第三方作者定义，直接当 `uuid` 会撞名且改 id 即断授权。若确认单一产品线且 id 可控，也可让 `uuid = plugin_id`（需放宽长度与格式校验）。
- [x] **Q3**：**下沉**到 `app_activations.period_end`。`members.expired_at` 是全站钱包，多插件按时计费若写它必然互相污染。
- [x] **Q4**：**不需要额外配置** —— 幂等激活已经覆盖了这个场景：同设备在租约内重进不扣费，租约过期后才算新占用。这比"N 分钟宽限"更准确，也不需要新字段。
- [ ] **Q5**：是否需要 `max_concurrent_apps`（同时开几个**不同**插件的上限）？（建议作为会员等级权益，默认 0=不限）
- [x] **Q6**：**管理器统一代调**。理由：未声明 `net` cap 的插件连 `socket()` 都被 seccomp 拒；管理器始终在线，且能统一处置欠费停用。
- [ ] **Q7**：是否接受「授权由管理器负责、root 可绕过」这个定位？参考 Keygen 也承认这点：客户端只是"尽力而为的执行者"，真正的防线是服务端的并发/设备风控。
- [ ] **Q8**：租约时长 `lease_ttl` 默认多少？（建议 30 分钟，与现有 `offline_timeout` 一致；太短则心跳频繁，太长则崩溃后名额迟迟不释放）
- [ ] **Q9**：`member_sessions` 是**替换**为 `app_activations`，还是补字段沿用旧表名？（建议替换：本版本全新安装无需迁移，且"会话"一词会与令牌混淆）


---

## 十三、实施记录（已完成）

> 本节记录实际落地情况，供后续维护与审计对照。**方案中的 🚧 部分均已实现。**

### 13.1 新增/修改文件

| 文件 | 类型 | 内容 |
|---|---|---|
| `models/member_account_token.go` | 新增 | 账号令牌（token + refresh_token + 30 天 TTL） |
| `models/app_activation.go` | 新增 | 激活（fingerprint / lease_expires_at / period_end / deactivated_at）+ 计费基准常量 |
| `models/errcode.go` | 新增 | 18 个标准错误码 + 中文兜底文案 |
| `models/app.go` | 改 | `PluginID *string`（NULL 允许）、`IsHost`、`LeaseTTLMinutes`、`BeforeSave` 归零空值 |
| `models/api.go` | 改 | `APITypeAppActivate=70` / `AppDeactivate=71` / `Entitlements=72` + 「应用激活」分类 |
| `services/account_token.go` | 新增 | 签发 / 校验 / 刷新（轮换）/ 吊销（级联失效激活）/ 按指纹找活跃激活 |
| `services/activation.go` | 新增 | `Activate`（幂等）/ `Deactivate` / `RenewLease` / `PruneDeadActivations` / 并发控制 |
| `services/activation_errors.go` | 新增 | 带错误码的错误类型 + `ErrorCodeOf` |
| `services/auth_for_app.go` | 新增 | `AuthForApp`（激活令牌优先 / 账号令牌隐式激活）、`AuthForAccount` |
| `services/entitlements.go` | 新增 | 权益清单 + 三个高层入口（供 controller 调用） |
| `services/member_auth.go` | 改 | 登录只发账号令牌；`authActiveMember` 走 `AuthForApp`；心跳走 `RenewLease`；登出走 `RevokeAccountToken`；`KickSession` 改为释放激活 |
| `services/member_data.go` | 改 | 改密后吊销账号令牌与激活 |
| `controllers/public/api.go` | 改 | 70/71/72 三个 handler + dispatch 分支 |
| `controllers/admin/online.go` | 改 | 在线列表改读 `app_activations`，新增 `lease_expires_at` 字段 |
| `controllers/admin/app.go` | 改 | 创建/更新支持 `plugin_id` / `is_host` / `lease_ttl_minutes`；删应用级联清理激活 |
| `controllers/admin/member.go` | 改 | 后台踢下线改为释放激活 |
| `controllers/admin/dashboard.go` | 改 | `online_sessions` 改为活跃激活数 |
| `database/migrate.go` | 改 | `AllModels()` 登记两张新表 |
| `frontend/.../apps/form.vue` | 改 | 新增「外壳 / 插件」区块：插件标识、宿主应用、激活租约 |

### 13.2 测试

新增 `services/activation_test.go`，13 条不变量：

```
TestLoginDoesNotActivateOrCharge           登录不扣点、不建激活、不绑设备
TestActivationChargesPoints                激活才扣费
TestActivationIdempotent                   同指纹重复激活复用且不重复扣费
TestActivationPerFingerprint               不同指纹各自占用、各自扣费
TestActivationAcrossAppsSharesWallet       跨应用各自计费、钱包共享
TestDeactivateReleasesSlot                 释放立即归还名额
TestLeaseExpiryFreesSlot                   租约过期自动回收 + pruner
TestRenewLeaseExtends                      心跳续约（不缩短）
TestHostAppCannotActivate                  宿主应用不可激活、不出现在清单
TestLogoutRevokesActivations               登出级联失效全部激活
TestPerTimeBillingUsesActivationPeriod     按时周期记在激活上，不污染 members.expired_at
TestImplicitActivationChargesOnce          隐式激活只扣一次
TestActivationTokenAppMismatchRejected     激活令牌跨应用被拒
TestEntitlementsHidesSecretWhenUnusable    清单只对可用应用下发 secret
```

既有用例按新语义更新（多开 / 手动顶号 / 绑定 / 点数 / 按时 / 版本记录等 16 处断言），
`go test ./...` 8 个包全绿。

### 13.3 实施中修正的 3 个设计问题

1. **`plugin_id` 唯一索引与空值冲突**：原设计用 `string` + `uniqueIndex`，未接插件的应用都留空 → 第二个应用即撞索引（SQLite/MySQL 都把空串当重复值）。改为 `*string` 存 NULL。
2. **点数门禁位置错误**：原把余额检查放在 `checkActivationGate`，导致「同指纹幂等复用」在余额为 0 时也被拒。**余额检查必须下沉到实际扣费点**（`chargeActivationOnce` / `chargePeriod`），复用路径不扣费也就不该看余额。
3. **测试暴露的隐式激活指纹源**：隐式激活沿用账号令牌记录的 fingerprint。若客户端登录不传 `machine_code`、激活才传，则两者不一致会重复扣费 —— 客户端**应在登录时就传设备指纹**（协议一直有此参数）。

### 13.4 已知边界

- `member_sessions` 表仍在（过渡期兼容），但登录不再写它；代码中对旧表的读已全部改为读 `app_activations`，删除应用时两者都清。
- ktrw-manager 侧改动（`KtrwBridge.httpPost`、`app.js` 登录与激活、`ktrw-ctl` 读授权文件、注入型插件环境变量修复）**尚未开始**，属下一个工程。
- `max_concurrent_apps`（同时开几个不同插件）按方案留为可选扩展，未实现。