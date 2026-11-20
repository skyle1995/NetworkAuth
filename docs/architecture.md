# NetworkAuth 架构总览

> **唯一入口文档。** 想理解这套系统「数据归谁、什么时候收费、谁能用什么」，看这一份就够。
>
> 细节与决策过程在子文档：
> - [plan-unified-account.md](./plan-unified-account.md) — 一号通改造的完整设计与实施记录（**已实施**）
> - [plan-host-plugin-session.md](./plan-host-plugin-session.md) — 外壳 + 插件二级会话方案（**待实施**）
> - [flows.md](./flows.md) — **各场景工作流程表**（请求流水线 / 登录 / 激活 / 心跳 / 清理）\n> - [app-settings.md](./app-settings.md) — 后台每个开关的逐项说明（运维向）
> - [api-guide.md](./api-guide.md) — 客户端对接协议
> - [plan-card-package-membership.md](./plan-card-package-membership.md) — 卡密套餐与会员制

**状态图例**：✅ 已实施并在测试中锁定 ｜ 🚧 已设计待实施 ｜ 💡 可选扩展

---

## 一、一句话概括

**钱包全局唯一，收费规则按应用，占用时机按会话。**

```
一个自然人 ── 全站一个账号 ──┬── 一份到期时间
                              ├── 一份点数
                              ├── 一个会员等级
                              └── 一份封禁/绑定记录

同一个账号 ── 进入不同应用 ──┬── 应用 A：计时模式，要黄金等级
                              ├── 应用 B：点数模式，每次登录扣 2 点
                              └── 应用 C：高级模式，钻石等级不扣点，白银扣 10 点
```

---

## 二、架构图

```
┌──────────────────────────────────────────────────────────────┐
│ 客户端层                                                       │
│  独立客户端 ─────────┐                                         │
│  外壳（管理器） ──┬── 插件 A（.zex）                            │
│                  ├── 插件 B（.zex）                            │
│                  └── 插件 C（.zex）                            │
└──────────────────────────────┬───────────────────────────────┘
                               │ POST /api/open  （单端点，api_type 区分）
                               │ 信封：app_uuid + api_type + data + timestamp + sign
                               │ sign = SHA256(app_uuid|api_type|data|ts|app_secret)
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 接入层：应用身份 → 接口定义 → 解密 → 分发 → 加密返回            │
│   应用：WHERE uuid = ?  校验 status     ← secret 是唯一身份凭据 │
│   接口：WHERE api_type = ? 校验 status  ← 全站唯一，不再按应用   │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 业务层                                                         │
│  ┌────────────────┐  ┌────────────────┐  ┌─────────────────┐ │
│  │ 账号令牌 ✅     │  │ 激活/租约 ✅    │  │ 计费引擎 ✅      │ │
│  │ 全站一次，不计费 │→│ 每应用一条，计费 │  │ 三开关+等级点数  │ │
│  └────────────────┘  └────────────────┘  └─────────────────┘ │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 数据层                                                         │
│  全局（全站一份）        按应用（每应用一行）   按激活（运行时）  │
│  members   wallet        apps 行              account_tokens   │
│  cards / packages        app_level_points     app_activations  │
│  member_levels           bindings                              │
│  apis / settings                                               │
│  variables / functions   ← 全站共用，无应用归属                 │
└──────────────────────────────────────────────────────────────┘
```

---

## 三、数据归属：一条判断标准

> **产生或消耗「全局资源」的配置 → 全局；决定「这个应用怎么被使用」的 → 应用。**

### 3.1 全局（`settings` 表 + 独立管理页）✅

| 资源 | 表 | 说明 |
|---|---|---|
| 账号 | `members` | `username` / `email` 全站唯一 |
| 钱包 | `members` 行内 | `expired_at` 到期、`points` 点数、`total_recharge` 累充、`level_uuid` 等级 |
| 会员等级 | `member_levels` | 按累充升级，权益：返利 / 额外设备 / 额外并发 / 赠送换绑 / 高级模式点数折扣 |
| 卡密 | `cards` | 全站卡池，任意卡可在任意应用核销（无应用归属） |
| 卡密套餐 | `card_packages` | 制卡时面值与售价**快照**进卡 |
| 接口定义 | `apis` | 每个 `api_type` 恰好一条，安装即写入，默认启用 |
| 账号策略 | `settings` | 32 项 `account_*`：注册/邮箱验证/卡密注册/卡密登录/卡密充值/试用/注册限制/设备与IP绑定转绑/配额 |
| 黑名单 | `blacklists` | 按设备/IP/地区，全站生效 |
| 绑定 | `bindings` | 无 `app_uuid`，一个账号所有应用共用一批绑定 |
| 公共变量 | `variables` | 全站共用，`alias` 全局唯一，无 `app_uuid` |
| 公共函数 | `functions` | 全站共用，`alias` 全局唯一，无 `app_uuid` |

### 3.2 按应用（`apps` 行）✅

| 类别 | 字段 |
|---|---|
| 应用身份 | `uuid` + `secret`（secret 是唯一身份凭据）+ `lease_ttl_minutes` + `plugin_id` ✅ |
| 内容分发 | `announcement` / `app_data`；版本与下载地址移到 `app_versions`（见 3.4） |
| 会话策略 | `check_interval` / `offline_timeout` / `clean_interval` |
| 满员策略 | `login_type`（在线名额满时进入本应用怎么处理） |
| 绑定验证 | 账号策略：`account_machine_verify` / `account_ip_verify` + 设备、IP 两套转绑额度 |
| **计费策略** | `free_mode` / `bill_time_enabled` / `bill_points_enabled` / `level_gate_enabled` / `required_level` / `points_*` |
| 等级点数 | `app_level_points`（`(应用, 等级) → 点数`） |

### 3.3 按会话（运行时）✅

| 表 | 说明 | 行业对应 |
|---|---|---|
| `member_account_tokens` ✅ | 账号令牌：证明"你是谁"，全站一次，不计费，30 天可刷新 | OAuth Access/Refresh Token |
| `app_activations` ✅ | 应用激活：证明"你在用哪个应用且已付费"，含 `fingerprint` / `lease_expires_at` / `period_end` / `online_at` / `deactivated_at` | Keygen machine activation |

### 3.4 版本列表（`app_versions`）✅

版本不再是 `apps` 上的三个列，而是**一行一个版本**，支持保留历史、逐版本启用：

| 字段 | 说明 |
|---|---|
| `app_uuid` | 所属应用 |
| `version` | 版本号（如 `3.0.2`） |
| `download_type` | 该版本的更新方式：0 不启用 / 1 强制 / 2 自由 |
| `download_url` | 下载地址，留空表示只提示更新、需手动安装 |
| `changelog` | 本次更新公告，随 `2`/`3`/登录 `update` 下发 |
| `enabled` | 是否启用 |
| `published_at` | 发布时间，取「已启用中的最高版本」作为当前版本 |

**更新检查只读版本列表**：先在所有 `enabled=1` 的记录里取版本号最高的那条，
再用它的 `download_type` 决定强制或自由更新；列表为空即视为「没有可更新版本」。

**插件与应用的对应**：`apps.plugin_id`（唯一索引）把本地插件 ID 映射到应用行。
版本仍按应用维护，所以「插件版本」= 该应用版本列表里的最高版本。
`3 检测最新版本` 传 `plugin_id` 时按它定位应用；不传则查请求签名对应的应用
（宿主自检用，插件自检必须传，否则会拿插件版本与宿主版本比对而误报）。

---

## 四、账号与钱包（一号通核心）✅

- 一个自然人全站只有一个账号（`username` / `email` 唯一）；
- 到期时间、点数、累充、会员等级**所有应用共享同一份**；
- 封禁、黑名单、转绑计数、注册限制计数**全站累计**；
- 删除应用**不会**删除账号、卡密、接口（只清该应用的会话/日志/变量/函数）。

**钱包共享的直接后果**：用户在 A 应用花掉的点数，B 应用立刻看得到余额减少。这是定义，不是 bug。

---

## 五、计费模型：三个正交开关 + 等级点数 ✅

每个应用三个**彼此独立、可任意组合**的开关（`free_mode=1` 时全部跳过）：

| 开关 | 字段 | 命中时的门禁 |
|---|---|---|
| **计时模式** | `bill_time_enabled` | 账号必须有效（未到期或永久） |
| **点数模式** | `bill_points_enabled` | 按 `points_charge_mode` 扣点（按次 / 按周期） |
| **会员限制** | `level_gate_enabled` | 会员等级 ≥ `required_level` |

组合语义：

| 计时 | 点数 | 含义 |
|---|---|---|
| ✅ | ❌ | 卖时长，到期即不可用 |
| ❌ | ✅ | 纯点数，账号到期也能靠点数继续用 |
| ✅ | ✅ | 双重门禁：既不能过期，也要扣点 |
| ❌ | ❌ | 不按时长/点数收费（仍可叠加会员限制） |

`billing_mode` 字段保留为**兼容展示值**，由 `models.DeriveBillingMode(计时, 点数, 会员限制)` 推导，
保存时回写；**判定逻辑一律读三个开关**，不要再读它。

### 5.1 关键细节

- 门禁真实入口是 `checkActivationGate` / `checkMemberUsable`，两者都只读三个开关：
  `RequiresLevel()` / `RequiresValidPeriod()` / `UsesPoints()`；
- **点数门禁不在预检里**，而在扣费处：幂等复用同一激活不扣费，也就不该被余额拦住；
  72 权益清单会用同一套单价函数提前给出 `points_needed` / `blocked_by_points`；
- 点数与计时可同时开：先判到期（`LICENSE_EXPIRED`），再判余额（`INSUFFICIENT_CREDITS`），
  不会出现「钱扣了但不让用」；
- 点数计费共用 `points_charge_mode`（按次 / 按周期）、`points_period_minutes`、`points_heartbeat_charge`；
- `required_level=1` 等价于「不限等级」；
- **等级点数对任何按点数计费的应用都生效**（不再限定旧「高级模式」）：配置在
  `app_level_points`（唯一键 `(app_uuid, member_level_uuid)`），未配置的等级回退
  `points_per_login` / `points_per_period`；**配置 0 = 该等级不扣点**。

### 5.2 事务与连接约定（务必遵守）

SQLite 连接池配置为 **单连接**（`database.go: SetMaxOpenConns(1)`）。因此：

> **事务内部禁止调用 `database.GetDB()`** —— 会去抢那条已被当前事务占用的唯一连接，
> 形成互等死锁：请求挂死、事务永不提交、SQLite 写锁不释放。

落地点：计费单价是**惰性**加载的（`Billing.loadLevelPoints`）。所有在事务内取价的入口
（`chargeActivationOnce` / `chargePeriod` / `chargePeriodMerged` / `checkMemberUsable`）
必须先 `billing = billing.WithDB(tx)`，让惰性查询走事务句柄。
回归用例：`services/single_conn_regression_test.go`。

### 5.3 卡密与试用：面值有哪个就发哪个

| 卡面值 | 发放 |
|---|---|
| `points > 0` | 累加到点数钱包 |
| `duration ≠ 0` | 累加到期时间（`-1` = 永久） |
| 两者都有 | **两边都发** |

试用同理：`account_trial_points` 与 `account_trial_duration_minutes` 各自独立，都为 0 视为未开启。

> 旧版「时长模式发时长、点数模式发点数」的二选一逻辑已移除。一张卡可同时卖「30 天 + 500 点」。

---

## 六、会话模型：二级会话 ✅

> 解决「外壳 + 插件」场景：登录时没有应用上下文，但计费需要应用上下文。
>
> **对齐行业标准**：本模型即软件授权领域的 **floating license + heartbeat lease**（[Keygen](https://keygen.sh/docs/choosing-a-licensing-model/floating-licenses/) 的标准做法），令牌分层遵循 **OAuth 2.0 / RFC 8693 audience 绑定**，生命周期对齐 **Steam `GetAuthSessionTicket` / `CancelAuthTicket`**。

### 6.1 两级令牌

```
账号令牌 Account Token ── 20/10 登录获得 ── 长期(30d)、可刷新、无 audience
        │                                    不计费、不校验绑定、不算在线名额
        │
        │ 70 应用激活（信封 app_uuid = 要进的应用）← 计费、绑定、机器数检查都只在这里
        ▼
激活 Activation ── 绑定 (app_uuid, member_uuid, fingerprint) ── 带租约
        │         ├── activation_token  短期凭证
        │         ├── lease_expires_at  租约到期（默认 30min）
        │         └── period_end        按时模式当前已付费周期
        │
        ├── 41 心跳 = 续租约（可能轮换 token），按时模式扣下个周期
        │
        └── 71 释放 = deactivation → 立即归还 machine 名额
              租约到期未续 → 自动失效（pruner 兜底）
```

**三个标准机制（缺一不可）**：

| 机制 | 做法 | 为什么 |
|---|---|---|
| **租约（TTL）** | `lease_expires_at = now + ttl`，心跳续期 | 客户端崩溃/被强杀时名额自然回收，不依赖它调 71 |
| **幂等激活** | 唯一键 `(app_uuid, member_uuid, fingerprint)`；活跃则复用**不重复扣费** | 「启动-崩溃-重启」不该反复扣点 |
| **显式释放** | `71` 置 `deactivated_at`，立即还名额 | 用户体验：不必等 30 分钟 |

### 6.2 计费时机变化

| 原行为 | 新行为 |
|---|---|
| 登录时扣费 | 登录不扣；首次激活应用时扣 |
| 登录后立刻退出 | 不扣费（对用户有利） |
| 计时从登录开始 | 从激活开始（`heartbeatBasis` = `FROM_CREATION` / `FROM_FIRST_PING`） |
| 外壳内停留 | 不计费 |
| 同设备崩溃重启 | 复用激活，**不重复扣费** |

### 6.3 令牌矩阵 ✅

| 令牌 | 可用接口 |
|---|---|
| 无 | 1/2/3/4（公告/更新/版本/卡密信息）、10/20/21/23/25/26（登录注册） |
| 账号令牌 | 22/24（充值/试用）、30（登出）、40（余额查询）、50（改密）、**72（权益清单）** |
| 激活令牌 | **41（心跳/续租）**、42/43/44/45/54（应用数据/变量/函数）、**53（扣点）** |
| 凭据（密码/卡密） | 51（转绑）、31（手动顶号） |

### 6.4 令牌用途隔离 ✅

| 令牌 | 可调接口 | 不能做什么 |
|---|---|---|
| 账号令牌（`20`/`10` 签发） | `40` `72` `30` `50` 等账号级 | 直接调 `41`-`45`/`53`/`54` → `ACTIVATION_REQUIRED` |
| 激活令牌（`70` 签发） | 全部应用级 + 账号级 | 跨应用使用 → `TOKEN_APP_MISMATCH` |

服务端**不做隐式激活**：应用级接口收到账号令牌时直接拒绝，不替客户端扣费。
这样"什么时候开始计费"完全由客户端显式调用 `70` 决定，不存在静默扣钱。
### 6.5 在线名额与绑定归属 ✅

| 项 | 归属 | 行业对应 |
|---|---|---|
| 并发计数 | **账号维度、按设备去重**：同账号的活跃设备数 ≤ 账号策略「同时在线」+ 等级额外并发 | Keygen `maxMachines` |
| 手动顶号列表 | 满员时列出**账号在各设备**的激活（整台设备一组） | — |
| 设备/IP 绑定 | **账号级**（全站共用 `bindings`），校验时机移到激活 | machine fingerprint |
| 转绑计数 | **账号级**，全站累计 | — |
| 同设备开几个插件 | **不限制**：同一台电脑开多个插件只占 1 个在线名额 | — |
| 名额回收 | 71 显式释放 + 租约到期 + pruner 兜底 | deactivation + dead machine pruner |

### 6.6 标准错误码 ✅

响应体保留中文 `msg`（可直接展示）+ 新增机器可读 `err`（客户端分支用）：

| `err` | 触发 |
|---|---|
| `LICENSE_EXPIRED` / `LICENSE_SUSPENDED` | 账号到期 / 被封停拉黑 |
| `ENTITLEMENT_REQUIRED` | 会员等级不足 |
| `INSUFFICIENT_CREDITS` | 点数不足 |
| `MACHINES_LIMIT_EXCEEDED` | 账号在线设备数超限 |
| `FINGERPRINT_MISMATCH` | 设备指纹不符（绑定校验失败） |
| `LEASE_EXPIRED` | 租约已过期，需重新激活 |
| `ACTIVATION_REQUIRED` | 需先激活才能调该接口 |
| `SIGNATURE_INVALID` | 签名校验失败 |

命名对齐 [Keygen 的错误语义](https://keygen.sh/docs/api/)，不在错误处理上自造词。

---

## 七、应用标识与密钥 ✅

| 标识 | 谁产生 | 用途 |
|---|---|---|
| `uuid` | 系统生成（UUID v4） | 协议 `app_uuid`、签名第一段 |
| `secret` | 系统生成（32 位 hex） | **签名密钥，唯一身份凭据** |

- **`secret` 绝不用插件 id** —— 必须随机；
- **`secret` 不下发到客户端包里**，改由 `72 权益清单` 按账号权限下发，避免逆向 APK 拿到全部密钥；
- 应用身份隔离靠签名：用 A 的 secret 签的串在 B 上验签必然失败（有测试锁定）。

---

## 八、接口体系 ✅

- 所有业务走**单一端点** `POST /api/open`，靠 `api_type` 区分；
- **接口定义全站唯一**：27 条，安装时写入，默认启用且不加密，装完即可联调；
- 请求链路固定顺序：**先验签（证明调用方身份）→ 再解密载荷**，不可颠倒；
- 新增 ✅：`70 应用激活`（幂等）、`71 应用释放`（deactivation）、`72 权益清单`（entitlements）。共 30 条。

完整清单与参数见 [api-guide.md](./api-guide.md)。

---

## 九、账号策略（全站）✅

32 项 `account_*`，入口在应用列表「更多 → 账号策略(全站)」。

| 分组 | 键名 |
|---|---|
| 卡密登录/充值 | `account_card_login_enabled` / `account_recharge_enabled` |
| 注册 | `account_register_enabled` / `account_email_verify_enabled` / `account_card_register_enabled` |
| 注册限制 | `account_register_limit_enabled` / `account_register_device_limit_enabled` / `account_register_limit_time` / `account_register_count` |
| 试用 | `account_trial_enabled` / `account_trial_limit_time` / `account_trial_claim_mode` / `account_trial_points` / `account_trial_duration_minutes` |

⚠️ **注册次数是全站计数**，默认 3。填 1 会导致同一 IP 注册一个号后再也注册不了。

---

## 十、安装与初始化 ✅

```
1. AutoMigrate            建表（含 app_level_points）
2. SeedDefaultSettings    写 32 项 account_* 默认值
3. SeedDefaultPortalNavigation
4. EnsureGlobalAPIs       写 27 条接口，默认启用
5. 创建超级管理员
```

- **不做数据迁移**：按全新安装分发，旧库归档弃用；
- 安装流程是唯一数据入口，`server.go` 启动路径保留 `EnsureGlobalAPIs()` 作幂等兜底；
- ⚠️ **新增表必须同时进 `database.AllModels()`** —— 漏了不会被编译发现，只在运行时 `no such table`（实施中真实踩过）。

---

## 十一、被测试锁住的不变量

这些行为有自动化回归，改动时会被拦住：

| 不变量 | 测试 |
|---|---|
| 钱包跨应用共享（A 扣点后 B 读到新余额） | `TestWalletSharedAcrossApps` |
| 扣点不透支（条件原子更新） | `TestDeductPointsNeverOverdraws` |
| 卡密跨应用核销、不可二次使用 | `TestCardRedeemAcrossApps` |
| 应用 secret 隔离（A 的签名在 B 无效） | `TestAppSecretIsolatesSignature` |
| 接口每条 `api_type` 唯一、幂等、默认启用 | `TestEnsureGlobalAPIsCreatesOnePerType` |
| 高级模式按等级取价、未配置回退、等级不足被拒 | `TestHybridModeUsesLevelPoints` |
| 豁免模式跳过全部门禁且不扣点 | `TestFreeModeSkipsAllGates` |
| 删应用不删账号/卡密/接口 | `app_cascade_test.go` |
| 全新安装后表、策略、接口、索引齐备 | `install_smoke_test.go`（6 条） |

✅ 分层授权已锁定（`services/activation_test.go`，12 条）：登录不计费 / 激活才扣费 / 同指纹幂等 / 跨应用各自计费 / 释放立即归还名额 / 租约过期自动回收 / 心跳续约 / 登出级联失效 / 按时周期不串味 / 令牌用途隔离 / 令牌跨应用被拒 / 清单仅对可用应用下发 secret。

---

## 十二、代码地图

| 关注点 | 文件 |
|---|---|
| 计费三模式、等级点数 | `models/billing.go`、`models/app_level_point.go`、`services/billing.go` |
| 全局接口种子 | `services/api_seed.go` |
| 鉴权、扣费、注册、试用、充值 | `services/member_auth.go` |
| 账号令牌 / 应用激活 | `services/account_token.go`、`services/activation.go`、`services/auth_for_app.go`、`services/entitlements.go` |
| 公开 API 分发 | `controllers/public/api.go` |
| 安装流程 | `controllers/install/install.go` |
| 表清单 | `database/migrate.go`（`AllModels()`） |

---

## 十三、遗留与待办

| 项 | 优先级 | 说明 |
|---|---|---|
| ~~二级会话实施~~ | ~~高~~ | ✅ 已完成，见 [plan-host-plugin-session.md](./plan-host-plugin-session.md) §十三 |
| 客户端适配 `billing_mode` / `free` | 高 | 旧客户端把 `mode==2` 当免费，遇高级模式会误判 |
| MySQL 环境回归 | 中 | 唯一索引与空值唯一性行为与 SQLite 有差异 |
| 接口配置进程内缓存 | 低 | `api_type → *models.API` 热点读 |
| 等级点数缓存失效 | 低 | 每次登录查一次该应用配置，可加缓存 |
| `max_concurrent_apps` | 低 | 外壳场景防「一账号开满所有插件」 |
