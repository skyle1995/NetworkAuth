# 一号通改造：全局单账号 / 单会员体系 / 全局接口

> 状态：**已实施完成**（2026-11-14）。`go build ./...` 与 `go test ./...` 全绿，前端已重建进 `frontend/dist`。
>
> 本文档是**改造记录**（做了什么、为什么、风险如何处置），偏历史与决策过程。
> **只想了解当前架构请直接读 [architecture.md](./architecture.md)**（架构总览，唯一入口）。
> 运维向的逐项开关说明见 [app-settings.md](./app-settings.md)；客户端对接见 [api-guide.md](./api-guide.md)。
>
> **架构定位**：本次为架构级重构，**不做数据迁移，按全新安装设计**。现有 `database.db` 已归档，新库由安装流程从零构建。因此本文不考虑历史数据兼容、字段回填、索引演进与灰度切换。
>
> **适用范围**：
> 1. `members` / `cards` / `card_packages` / `member_levels` / `blacklists` 收敛为全站唯一一套；
> 2. `apis` 从「每应用一套接口条目」收敛为「全局一套接口条目」，应用身份由 `app_uuid + app_secret` 签名区分；
> 3. 全部「账号体系」相关开关（注册、卡密登录、充值、试用、邮箱验证、注册限制）从应用级提升为全局级；
> 4. **计费策略保留在应用级**，并细化为三模式（会员 / 点数 / 混合），混合模式支持按会员等级配置点数消耗。

## 实施摘要

| 项 | 结果 |
|---|---|
| 改动文件 | 45 个已跟踪文件 + 6 个新文件 |
| 新表 | `app_level_points`（混合模式的等级点数） |
| `apps` 表 | 删除 13 个账号策略字段；`operation_mode` → `billing_mode` + `required_level`；新增 `free_mode` 语义扩展 |
| 全局配置 | 14 项账号策略（`settings`） |
| 全局接口 | 27 条，安装时写入，默认启用 |
| 后端验证 | `go build ./...` 通过；`go test ./...` 7 包全绿 |
| 前端验证 | `vue-tsc` 无业务错误；`vite build` 成功（138 文件 / 3.49 MB） |
| 新增回归 | 13 条：7 条一号通不变量 + 6 条全新安装校验（见第十章） |

---

## 一、背景与目标

当前系统是「多应用 + 每应用独立会员池 + 每应用独立接口表 + 每应用独立账号策略」：同一个自然人若使用两个应用，系统里就是两个账号、两份到期时间、两份点数、两个累充等级；后台要为每个应用复制一遍接口配置、加密密钥与账号策略。运营上表现为：

- 用户买的时间/点数不能跨应用使用，需要重复购买；
- 封禁只对单个应用生效，换个应用即绕过；
- 每加一个应用就要重新配置 27 条接口，且各自加密密钥互不相同，对接成本随应用数线性增长；
- 同一个「能否注册」的决策要在每个应用上重复设置一遍，且容易出现应用间不一致。

**目标**：

1. 整个系统只有一套会员/点数体系 —— 一个自然人在全站只有一个账号、一套到期时间与点数余额、一个累充等级、一份全站生效的封禁记录；
2. 整个系统只有一套接口定义 —— 接口条目与加密配置全局唯一，**应用身份只由 `app_uuid` 与应用密钥 `secret` 的签名验证区分**；
3. 整个系统只有一套账号策略 —— 注册、卡密登录、充值、试用、邮箱验证、注册限制全站统一；
4. 表结构按目标形态从零定义，不背历史包袱。

**非目标**（本次不动）：

- 应用自身的载体配置：名称/状态/密钥、版本与更新、公告、`AppData`；
- 应用自身的运行时策略：心跳间隔、离线超时、会话清理、多开范围与数量、机器/IP 验证强度、登录顶号策略；
- 应用级变量/函数（`variables` / `functions`，已有 `app_uuid="0"` 全局机制）。

---

## 二、归属原则（本次改造的判断基准）

后续所有字段的归属，以及未来新增配置项该放哪里，统一按这两条判断：

| 判断 | 归属 |
|---|---|
| 该配置**产生或消耗全局资源**（账号、钱包、卡密、试用额度、封禁记录） | **全局**（`settings`） |
| 该配置只影响**某应用如何使用**（内容分发、会话策略、验证强度、多开数量） | **应用级**（`apps`） |

一句话分界：**应用管「怎么用」，全局管「有多少」与「怎么说」。**

### 2.1 为什么账号策略必须全局

一号通之后，注册产生的是**全局账号**、充值改变的是**全局钱包**、试用发放的是**全局额度**、卡密核销的是**全局卡池**。这些动作的资源都不属于任何单一应用。

若仍用应用级开关控制，会产生三类矛盾：

1. **语义矛盾**：在 A 应用注册的账号能登录 B 应用，但 B 应用却「未开启注册」——开关控制的不是账号，只是入口，容易误解为账号不通用；
2. **一致性问题**：注册限制（每 IP 每日 N 次）按应用分别计数时，用户在 A 应用用完额度后换个应用即可继续注册全局账号，限制形同虚设；
3. **运维负担**：新增应用时要逐项核对 13 个账号策略开关，漏配即产生线上差异。

结论：账号策略与计费策略一样，属于平台级策略，归全局。

---

## 三、改造前的现状诊断

> 本节描述被改造的对象，**不作为兼容目标** —— 现有数据结构将被新库取代。

### 3.1 隔离字段分布

| 表 | 文件 | 隔离字段 | 当前唯一键 |
|---|---|---|---|
| `members` | `models/member.go:51` | `app_uuid` | `idx_member_app_username` = `(app_uuid, username)` |
| `cards` | `models/card.go:50` | `app_uuid` | `idx_card_app_no` = `(app_uuid, card_no)` |
| `card_packages` | `models/card_package.go:35` | `app_uuid` | — |
| `member_levels` | `models/member_level.go:25` | `app_uuid` | — |
| `blacklists` | `models/blacklist.go:38` | `app_uuid` | `idx_blacklist_app_type_value` = `(app_uuid, type, value)` |
| `member_sessions` | `models/member_session.go:25` | `app_uuid` | — |
| `member_logs` | `models/member_log.go:17` | `app_uuid` | — |
| `apis` | `models/api.go:91` | `app_uuid` | `(app_uuid, api_type)` |
| `bindings` | `models/binding.go:38` | **无** | `(member_uuid, type, value)` |
| `variables` / `functions` | `models/variable.go:30` | `app_uuid`，`"0"`=全局 | — |

### 3.2 账号资产全在 `members` 一行上

`ExpiredAt`、`Points`、`TotalRecharge`、`LevelUUID`、`TrialUsed`、`TrialDate`、`MachineRebind*`、`IPRebind*` 全部是 `members` 的列。因此「一号通」的技术本质只有一件事：

> 让 `members` 的一行代表一个自然人，而不是「某应用里的某用户」。

### 3.3 计费规则与账号策略都挂在 `apps` 行上

`apps` 表现有 41 个字段，混合了三类不同性质的内容：

- **应用载体**：名称、状态、密钥、版本、公告、`AppData`；
- **应用运行时**：心跳、离线、多开、机器/IP 验证强度、登录方式；
- **平台级策略（错放）**：计费模式 6 个字段 + 账号策略 13 个字段。

详见 5.1 的字段归属表。

### 3.4 接口体系现状

- 接口类型常量 27 个，定义在 `models/api.go:16-57`，分类清单位于 `GetAPITypes()`；
- 每个应用创建时，`controllers/admin/app.go`（创建应用事务内，约 361-395 行）为该应用插入 27 条 `apis` 记录，默认 `status=0`（禁用）、算法 None；
- 启动时 `server.go:117` 调 `services.EnsureAppAPIs()`，遍历所有应用补齐缺失类型；
- 接口定位：`controllers/public/api.go:105` 执行 `WHERE app_uuid = ? AND api_type = ?`；
- 每条接口独立持有 `SubmitAlgorithm` / `ReturnAlgorithm` 与两套密钥，见 `models/api.go:96-114`；
- 加解密管道：`services/api_codec.go:30` `NewAPICodec(api)`，按接口配置选择 RC4 / RSA / RSA动态 / 易加密；
- 密钥约定见 `services/api_codec.go:18-22`：RC4 用十六进制串，RSA 用 PEM，易加密用逗号分隔整数。

### 3.5 公开 API 协议（不可变）

```
POST /api/open
{ "app_uuid": "...", "api_type": 10, "data": "<密文>", "timestamp": ..., "sign": "..." }
sign = SHA256(app_uuid | api_type | data | timestamp | app_secret) 大写HEX
```

见 `controllers/public/api.go:63`、`services/api_sign.go:28`。`app_uuid` 参与签名，**协议层必须原样保留**。

关键结论：**应用身份认证已由 `app_secret` 承担**。接口条目上的 `app_uuid` 只用于「找配置」，不是安全边界。这正是一号通接口方案可行的技术前提。

### 3.6 改造量实测

| 类别 | 数量 |
|---|---|
| Go：`app_uuid` / `AppUUID` 引用行 | 462 |
| Go：`OperationMode` 引用行 | 55 |
| Go：`app.Register*` / `app.Trial*` / `app.Card*` / `app.Recharge*` / `app.EmailVerify*` 引用行 | 约 40 |
| Go：点数/到期写入点 | 31 |
| Go：`apis` 相关引用行 | 约 45 |
| 前端：`app_uuid` / `appUuid` 引用行 | 95（25 个文件） |

### 3.7 现有数据库的处置

`database.db`（368 KB）、`database.db-shm`、`database.db-wal` 位于项目根目录，均被 `.gitignore` 的 `*.db*` 规则忽略，**未被 git 跟踪**。处理方式见第八章。

---

## 四、目标形态（即当前形态）

### 4.1 全局（一套体系）

**账号与资产**
- 账号身份：`username` 全局唯一，`email` 全局唯一；
- 钱包：到期时间、点数、累计充值、会员等级 —— 全站一份；
- 卡密与套餐：一个卡池，任一应用均可核销；
- 黑名单：机器码 / IP / 地区封禁全站生效；
- 试用领取计数、机器码/IP 转绑计数：全站共享。

**账号策略**
- 注册开关、注册邮箱验证开关、卡密注册开关；
- 注册限制（IP / 设备，每天或永久，次数）；
- 卡密登录开关、卡密充值开关；
- 试用开关、试用额度、试用限制方式、试用领取方案。

**计费策略**
- 计费模式（时长 / 点数）、点数扣费方式、单价与周期参数。

**接口**
- 27 条接口定义与加密配置，安装时自动生成。

### 4.2 仍按应用（应用载体与运行时）

- 应用标识：`UUID`、`Name`、`Status`、`Secret`；
- 内容分发：`Version`、`DownloadType`、`DownloadURL`、`Announcement`、`AppData`；
- 会话策略：`CheckInterval`、`OfflineTimeout`、`CleanInterval`；
- 多开策略：`MultiOpenScope`、`MultiOpenCount`；
- 设备验证强度：`MachineVerify`、`MachineRebind*`、`MachineFreeCount`、`MachineRebindCount`、`MachineRebindDeduct`；
- 网络验证强度：`IPVerify`、`IPRebind*`、`IPFreeCount`、`IPRebindCount`、`IPRebindDeduct`；
- 登录方式：`LoginType`（顶号 / 拒绝 / 手动顶号）；
- 计费例外：`free_mode`（按 D1 的 A 方案）；
- 应用级与全局变量 / 函数。

### 4.3 语义变化清单（无需决策，但需知悉）

| 项 | 旧行为 | 新行为 | 说明 |
|---|---|---|---|
| 注册限制计数 | 按应用分别计数 | **全站计数** | 账号全局后，按应用计数可被「换应用继续注册」绕过 |
| 试用领取 | 按账号 + 应用 | **按账号全站一次** | 防止换应用刷试用 |
| 转绑计数 | 按账号（`members` 行） | **按账号全站** | 字段本就在 `members` 上，行为不变 |
| 邮箱验证码 | 按应用隔离（key 带 `app_uuid`） | **全站共用** | `emailCodeKey` 去掉 `app_uuid`，否则 A 应用发的码在 B 应用不可用 |

### 4.4 `member_sessions` / `member_logs` 的 `app_uuid`

保留，但语义从「归属」改为「来源应用」：

- `member_sessions`：该会话从哪个应用登录 → 支撑后台在线管理、按应用筛选、清理策略；
- `member_logs`：操作发生在哪个应用 → 审计追溯。

---

## 五、决策项（结论见第十三章）

> 编号说明：早期版本中的「同名账号迁移」「灰度开关」「接口配置收敛」「注册限制口径」「试用计数共享」均已随「不做数据迁移」与「账号策略全局化」而取消，无需决策。

### D1 计费模式归属（最关键）

账号只有一份钱包，需要决定「怎么花」由谁定义。

| 方案 | 说明 | 代价 |
|---|---|---|
| A. 全局单模式 + 免费豁免 | 全局一套：时长 或 点数；`apps` 删除全部 6 个计费字段，仅保留 `free_mode` 布尔标记用于「该应用不计费」 | schema 最干净，无继承逻辑。**差异仅限于「收费/免费」，不能让两个应用用不同计费方式** |
| **B. 应用级计费策略（已采纳）** | 钱包（到期时间/点数/等级）全局一份；每个应用自选计费模式，并可要求最低会员等级、按等级配置点数消耗 | schema 多一张 `app_level_points`；代码需按应用解析计费与等级点数 |
| C. 双钱包并行且互不相干 | 时长应用扣 `ExpiredAt`，点数应用扣 `Points`，互不相干 | 用户认知混乱：买的时间在点数应用里用不了，不构成一号通 |

**最终采纳 B，并细化为三模式**（判定依据：钱包全局共享是「一号通」的核心，而「怎么收费」是运营策略，天然需要按产品差异化）：

| 模式 | 值 | 门禁条件 | 扣费 |
|---|---|---|---|
| 会员模式 | 0 | 账号有效（未到期/永久）+ 权限等级 ≥ `required_level` | 不扣点 |
| 点数模式 | 1 | 点数足够 | 每次登录扣 `points_per_login`（或按周期扣） |
| 混合模式 | 2 | 等级 ≥ `required_level` + 点数足够 | 按**该等级在本应用配置的点数**扣（`app_level_points`） |

补充规则：

- `free_mode=1` 为计费豁免，跳过全部门禁与扣费（凌驾于三模式之上）。
- `required_level=1` 表示不限等级，因此「会员模式 + 等级 1」等价于传统免费应用。
- 点数模式与混合模式共用 `points_charge_mode`（按次 / 按时）、`points_period_minutes`、`points_heartbeat_charge`。
- 混合模式下 `points_per_login` / `points_per_period` 退化为**未配置等级的兜底值**。
- 卡密与试用按「面值有哪个就发哪个」：`Points>0` 加点数，`Duration≠0` 加时长，不再依赖模式二选一。

### D2 卡密作用域

| 方案 | 说明 |
|---|---|
| **全局卡池 + `source_app_uuid` 仅作发行渠道标记**（已采纳） | 一张卡可在任意应用登录/充值；`card_no` 全局唯一；`source_app_uuid` 仅用于统计与追溯「这张卡是给哪个应用卖的」，不参与核销判定 |
| 全局卡池且删除 `app_uuid` | 表最干净，但失去渠道归属信息 |
| 严格按应用 | 与一号通目标冲突，不采纳 |

### D3 多开与设备绑定是否跨应用共享

`bindings` 无 `app_uuid`，当前**已是账号级**。

- **绑定额度全局共享**（一个账号最多绑 N 台机器），受「应用多开数 + 会员等级额外多开」共同约束；
- **在线会话按应用独立计数**（A 应用在线 1 台不影响在 B 应用登录）——`member_sessions.app_uuid` 记录来源应用，满员提示与手动顶号均限本应用。

> **已定案（2026-11）**：设备数、在线数、免费转绑次数全部收到**账号级**（`account_max_devices` / `account_max_concurrent` / `account_rebind_free_count`，见 [app-settings.md §6.7](./app-settings.md)），会员等级只做加量。应用的 `multi_open_count` / `multi_open_scope` 已不参与任何判定。在线名额按**设备**去重，同设备多插件不重复占位。

### D4 接口启用策略

`apis.status` 全局化后不再是「按应用开关」。

| 子问题 | 选项 | 建议 |
|---|---|---|
| 4-1 全新安装时默认状态 | 全部启用 / 全部禁用（旧行为） | **全部启用**：接口是协议能力，装完即可联调；否则 27 条要逐个点开 |
| 4-2 应用级接口控制 | 已无必要 —— 账号策略开关已全局化，与接口 status 同层 | **不设应用级控制** |
| 4-3 风控接口（60/61/62） | 全局启用 + `secret` 鉴权 / 默认禁用 | **全局启用**：能力型接口，靠签名鉴权 |

### D5 加密密钥全局共享的接受度

新结构下 27 条接口共用一套密钥，意味着：

- **可接受的部分**：身份认证由 `app_secret` 签名承担，没有某应用的 secret 无法伪造该应用的请求；载荷加密只承担「防抓包 / 提高协议分析成本」的作用；
- **需明确的风险**：逆向任一客户端即可获得全局载荷密钥，从而解析所有应用的明文载荷。隔离等级从「应用级」降为「全站级」。

| 选项 | 说明 |
|---|---|
| **全站共用一套密钥**（推荐，符合本次目标） | 对接简单，客户端与后台各配一次 |
| 保留「每应用一套密钥」 | 与「接口全局化」部分冲突：需新增 `app_api_keys(app_uuid, api_type, ...)` 表，复杂度显著上升 |
| 明文传输 + 仅靠 HTTPS 与签名 | 最简；必要时可全局设 `algorithm=None` |

**建议全站共用一套**。若要求保留应用级密钥隔离，需告知，本方案要增加一张密钥表。

---

## 六、架构设计

### 6.1 `apps` 表字段归属（41 → 22）

| 类别 | 字段 | 处置 |
|---|---|---|
| 应用标识 | `UUID`、`Name`、`Status`、`Secret`、`ID` | 保留 |
| 内容分发 | `Version`、`DownloadType`、`DownloadURL`、`Announcement`、`AppData` | 保留 |
| 会话策略 | `CheckInterval`、`OfflineTimeout`、`CleanInterval` | 保留 |
| 多开策略 | `MultiOpenScope`、`MultiOpenCount` | 保留 |
| 设备验证 | `MachineVerify`、`MachineRebindEnabled`、`MachineRebindLimit`、`MachineFreeCount`、`MachineRebindCount`、`MachineRebindDeduct` | 保留 |
| 网络验证 | `IPVerify`、`IPRebindEnabled`、`IPRebindLimit`、`IPFreeCount`、`IPRebindCount`、`IPRebindDeduct` | 保留 |
| 登录方式 | `LoginType` | 保留 |
| 计费例外 | `FreeMode` | 保留（跳过全部门禁与扣费） |
| **计费策略** | `BillingMode`、`RequiredLevel`（新增）、`PointsChargeMode`、`PointsPerLogin`、`PointsPeriodMinutes`、`PointsPerPeriod`、`PointsHeartbeatCharge` | **保留/新增**（按 D1 的 B 方案：钱包全局，规则按应用） |
| **账号策略** | `RegisterEnabled`、`EmailVerifyEnabled`、`CardRegisterEnabled`、`RegisterLimitEnabled`、`RegisterDeviceLimitEnabled`、`RegisterLimitTime`、`RegisterCount` | **删除 → 全局** |
| **账号策略** | `CardLoginEnabled`、`RechargeEnabled` | **删除 → 全局** |
| **账号策略** | `TrialEnabled`、`TrialLimitTime`、`TrialDuration`、`TrialClaimMode` | **删除 → 全局** |

合计删除 13 个账号策略字段（注册 7 + 卡密 2 + 试用 4），计费字段保留并按三模式扩展：

- 删除 `OperationMode`，改为语义更明确的 `BillingMode`（0 会员 / 1 点数 / 2 混合）
- 新增 `RequiredLevel`（会员/混合模式的最低权限等级，1=不限）
- `PointsPerLogin` / `PointsPerPeriod` 在混合模式下作为未配置等级的兜底值

### 6.2 目标表结构

#### `members`（全站账号）

| 字段 | 说明 |
|---|---|
| `id` / `uuid` | 主键 / 唯一标识 |
| `username` | **唯一索引**（替代原 `(app_uuid, username)`） |
| `email` | 唯一索引（空值重复性按数据库方言处理） |
| `type` / `status` / `password` / `password_salt` | 来源与凭据 |
| `expired_at` / `points` / `total_recharge` / `level_uuid` | **全站唯一钱包** |
| `card_uuid` / `trial_used` / `trial_date` | 卡密来源、试用计数 |
| `machine_rebind_used` / `machine_rebind_date` / `ip_rebind_used` / `ip_rebind_date` | 转绑计数 |
| `register_ip` / `register_machine` / `last_login_at` / `last_login_ip` | 溯源与风控（全站注册限制计数依据） |
| `data` / `remark` / `created_at` / `updated_at` | 业务字段 |

删除：`app_uuid`。

#### `apis`（全站接口，27 行）

| 字段 | 说明 |
|---|---|
| `id` / `uuid` | 主键 / 唯一标识 |
| `api_type` | **唯一索引** |
| `status` | 全站启用状态 |
| `submit_algorithm` / `return_algorithm` | 算法配置 |
| `submit_public_key` / `submit_private_key` / `return_public_key` / `return_private_key` | 密钥 |

删除：`app_uuid`。

#### 其余表

| 表 | 变更 |
|---|---|
| `cards` | `app_uuid` → `source_app_uuid`（发行渠道标记，不参与核销）；`card_no` 改唯一索引 |
| `card_packages` | 删 `app_uuid` |
| `member_levels` | 删 `app_uuid`；等级表全站一份 |
| `blacklists` | 删 `app_uuid`；唯一键改 `(type, value)` |
| `member_sessions` | `app_uuid` 保留，注释改「来源应用」 |
| `member_logs` | `app_uuid` 保留，注释改「事件来源应用」 |
| `bindings` | 不变 |
| `variables` / `functions` | 不变 |
| `settings` | 新增 14 个全局账号策略配置项（见 6.3） |
| `app_level_points` | **新增**：混合模式下 (应用, 会员等级) → 点数消耗，唯一键 `(app_uuid, member_level_uuid)` |

> **无迁移设计优势**：不需要 `operation_mode_override` 这类兼容字段，不存在「默认值 0 既表示时长又表示继承」的哨兵冲突。字段语义一次定义清楚。

### 6.3 全局配置清单

复用 `settings` KV 表（`models/settings.go`）。

**计费策略：不进 settings**。计费按应用配置（`apps.billing_mode` / `required_level` / `points_*`），混合模式的等级点数在 `app_level_points`。原因见 D1：钱包全局，收费规则按产品差异化。

**账号策略**

| Name | 含义 |
|---|---|
| `account_register_enabled` | 账号注册开关 |
| `account_email_verify_enabled` | 注册邮箱验证开关 |
| `account_card_register_enabled` | 卡密注册开关 |
| `account_register_limit_enabled` | IP 注册限制开关 |
| `account_register_device_limit_enabled` | 设备注册限制开关 |
| `account_register_limit_time` | 注册限制窗口：0 每天 / 1 永久（IP 与设备共用） |
| `account_register_count` | 注册次数上限（**全站计数**，默认 3） |
| `account_card_login_enabled` | 卡密登录开关 |
| `account_recharge_enabled` | 卡密充值开关 |
| `account_trial_enabled` | 试用领取开关 |
| `account_trial_limit_time` | 试用限制窗口：0 每天 / 1 永久 |
| `account_trial_points` | 试用发放点数（0=不发放） |
| `account_trial_duration_minutes` | 试用发放在期时长（分钟，0=不发放） |
| `account_trial_claim_mode` | 试用领取方案：0 无限制 / 1 资源耗尽可领 |

> 全部为平台级策略，不带 `app_uuid`。新增应用时无需再逐项配置。
> 试用额度拆成「点数」与「时长」两项后，同一账号可同时获得两种资源，不再依赖「当前是哪种模式」。

实现位置：`models/billing.go`（键名常量 + `GlobalSettingNames()`）、`services/billing.go`（`AccountPolicy` / `GetAccountPolicy` / `SaveAccountPolicy`）。

### 6.4 有效计费解析

按 D1 的 B 方案，解析为「应用行 → `Billing` 结构」，混合模式再叠加等级点数：

```go
// services/billing.go
type Billing struct {
    AppUUID         string
    Free            bool // true=该应用跳过全部门禁与扣费
    Mode            int  // 0 会员 / 1 点数 / 2 混合
    RequiredLevel   int  // 会员/混合模式的最低权限等级（1=不限）
    ChargeMode      int  // 0 按次 / 1 按时
    PerLogin        int
    PeriodMinutes   int
    PerPeriod       int
    HeartbeatCharge int
    // levelPoints 惰性加载的 (等级UUID → 点数) 快照
}

func EffectiveBilling(app *models.App) *Billing
func (b *Billing) RequiresLevel() bool
func (b *Billing) RequiresValidPeriod() bool
func (b *Billing) UsesPoints() bool
func (b *Billing) PointsCostForLevel(levelUUID string, perPeriod bool) int
```

门禁判定集中在 `checkMemberUsable(db, billing, member)`：

1. `Free` → 放行
2. `RequiresLevel()` → `memberPermissionLevel >= RequiredLevel`，否则「会员等级不足」
3. `RequiresValidPeriod()` → 未到期/永久，否则「账号已到期」
4. `UsesPoints()` → 按次模式登录时已扣，不在此拦；按时模式需「仍在周期内」或「余额够下个周期」

扣费集中在 `applyLoginCharge(tx, billing, member)` / `settlePointsTime(tx, billing, member)`，一律用条件原子更新：

```go
res := tx.Model(&models.Member{}).
    Where("uuid = ? AND points >= ?", m.UUID, cost).
    Update("points", gorm.Expr("points - ?", cost))
if res.RowsAffected == 0 { return errors.New("点数不足") }
```

> 全站钱包共享后这是**必须项**：两个应用同时登录会读到同一余额，非原子扣点必然超扣。

替换点（原 55 处 `app.OperationMode` / `Points*` 直读）：`services/member_auth.go`、`member.go`、`member_rebind.go`、`member_risk.go`、`app_info.go`、`member_function.go`、`controllers/public/api.go`。

### 6.5 账号策略接入点

| 原读取处 | 现状 | 目标 |
|---|---|---|
| `AccountRegister`（`member_auth.go:941`） | `app.RegisterEnabled != 1` | `global_settings.RegisterEnabled()` |
| `AccountRegister`（`member_auth.go:956`） | `app.EmailVerifyEnabled == 1` | 全局配置 |
| `AccountRegister`（`member_auth.go:947`） | `app.CardRegisterEnabled == 1` | 全局配置 |
| `enforceRegisterLimit`（`member_auth.go:866`） | 按 `(app_uuid, ip/machine)` 计数 | **全站计数**：`COUNT(members WHERE register_ip = ? AND created_at >= ?)` |
| `CardLogin`（`member_auth.go:296`） | `app.CardLoginEnabled != 1` | 全局配置 |
| `RechargeByCard`（`member_auth.go:1174`） | `app.RechargeEnabled != 1` | 全局配置 |
| `ClaimTrial`（`member_auth.go:1070`） | `app.TrialEnabled` / `TrialDuration` / `TrialClaimMode` / `TrialLimitTime` | 全部全局配置 |

> 注册限制改为全站计数后，**不需要新增 `registration_events` 流水表** —— 直接查 `members` 的 `register_ip` / `register_machine` + `created_at` 窗口即可，与现有实现同构，只是去掉 `app_uuid` 条件。

### 6.6 鉴权链路改造

| 函数 | 现状 | 目标 |
|---|---|---|
| `authMemberByToken`（`member_auth.go:698`） | `WHERE app_uuid = ? AND token = ?` | `WHERE token = ?` |
| `authActiveMember`（`member_auth.go:718`） | 同上 + 取应用 | 同上，应用仅取策略 |
| `authMemberByCredential`（`member_auth.go:739`） | `WHERE app_uuid = ? AND username = ?` | `WHERE username = ?` |
| `findMemberByUsername`（`member_risk.go:22`） | 同上 | 同上 |
| `loadEnabledApp` | 不变 | 不变（校验应用启用，取 secret） |

### 6.7 扣费原子化（必须改）

现状是「读-改-写」，全局共享后同一账号被两个应用同时心跳即为真并发。统一改为条件原子更新：

```go
res := tx.Model(&models.Member{}).
    Where("uuid = ? AND points >= ?", uuid, cost).
    Update("points", gorm.Expr("points - ?", cost))
if res.RowsAffected == 0 {
    return errors.New("点数不足")
}
```

涉及 6 处：`applyLoginCharge`、`settlePointsTime`、`DeductPoints`、`RechargeByCard` 点数分支、`riskDeductMember`、后台 `RechargeMemberPoints` / `DeductMemberPoints`。

同时把 `lockMemberRow`（`member_auth.go:634`）的适用范围从「登录事务」扩展到「所有改动钱包的事务」。

### 6.8 全局接口体系

#### 6.8.1 数据形态

见 6.2 的 `apis` 定义：27 行、`api_type` 唯一、无 `app_uuid`。

#### 6.8.2 安装与补齐

| 场景 | 位置 | 动作 |
|---|---|---|
| 全新安装 | `controllers/install/install.go`（`AutoMigrate` 与 `SeedDefaultSettings` 之后） | 调 `services.EnsureGlobalAPIs()` 插入 27 条，`status` 按 D4 决定 |
| 服务启动 | `server.go:117`（原 `EnsureAppAPIs` 调用点） | 改为 `EnsureGlobalAPIs()`：按 `api_type` 补齐缺失项，幂等兜底 |
| 新建应用 | `controllers/admin/app.go`（创建应用事务内，约 361-395 行） | **删除整段「为应用创建所有默认接口」逻辑** |

```go
// EnsureGlobalAPIs 补齐全局缺失的接口条目（幂等）。
// 不再遍历应用；每条 api_type 全站仅一条。
func EnsureGlobalAPIs() {
    // 1. 读取已有 api_type
    // 2. 对 GetDefaultAPITypes() 做差集，插入缺失项（status 按 D4）
    // 3. purgeLegacyRebindAPI(db) 保留
}
```

#### 6.8.3 请求链路

```
1. 解析信封 → 取 app_uuid / api_type / data / timestamp / sign
2. 查应用：WHERE uuid = app_uuid，校验存在且 status=1     ← 取 secret，应用身份的唯一来源
3. 验签：SHA256(app_uuid|api_type|data|timestamp|app_secret) ← 应用鉴权（机制不变）
4. 查接口：WHERE api_type = ?，校验 status=1               ← 去掉 app_uuid 条件
5. 解密 data（NewAPICodec）
6. dispatch 业务（读全局账号策略 + 全局计费 + 应用运行时策略）
7. 加密返回
```

**安全要点**：第 3 步必须早于第 5 步 —— 先用 `app_secret` 证明调用方身份，再解密载荷。现有顺序已满足，改造时不要打乱。

可选优化：接口配置是全站热点数据，可加进程内缓存（`api_type → *models.API`），后台保存时失效。

#### 6.8.4 后台管理

| Handler | 现状 | 目标 |
|---|---|---|
| `APIListHandler`（`admin/api.go:32`） | `app_uuid` 筛选 + 聚合应用名 | 去掉应用维度；直接列出 27 条；保留 `api_type` 筛选 |
| `APIExportKeysHandler`（`admin/api.go:216`） | 导出「某应用 + 该应用全部接口密钥」 | 保留按应用导出（`app` 段含 uuid/secret），`interfaces` 段改为「全局共享配置」，响应标注 `shared: true` |
| 接口更新 Handler | 按 id 更新单条 | 不变，只是不再校验归属应用 |

#### 6.8.5 删除应用的影响

`controllers/admin/app.go:22 cascadeDeleteAppData` 当前删除列表含 `&models.Member{}`、`&models.Card{}`、`&models.API{}`。新结构下：

- **账号、卡密、接口一律不删**；
- 仅删除该应用的 `sessions` / `logs` / `variables` / `functions`；
- 详见第七章高危 1。

---

## 七、实施记录

各阶段均已完成。下表列出**实际落地位置**，便于后续排查与二次开发。

| 阶段 | 实际改动文件 | 关键内容 |
|---|---|---|
| P1 数据层 | `models/member.go`、`card.go`、`card_package.go`、`member_level.go`、`blacklist.go`、`api.go`、`app.go`、`member_session.go`、`member_log.go` | 删 `app_uuid`；单列唯一索引；`apps` 删 13 项账号策略字段；`OperationMode` → `BillingMode` + `RequiredLevel` |
| P1 新增 | `models/billing.go`、`models/app_level_point.go`、`database/migrate.go`、`database/settings.go` | 三模式常量 + 14 项账号策略键名；等级点数表；`AllModels` 登记；默认值种子 |
| P2 全局接口 | `services/api_seed.go`、`cmd/server.go`、`controllers/install/install.go`、`controllers/admin/api.go`、`controllers/admin/app.go`、`controllers/public/api.go` | `EnsureGlobalAPIs()`；安装与启动双入口；接口查询去 `app_uuid`；删「建应用插 27 条」；级联删除只留会话/日志/变量/函数 |
| P3 服务层 | `services/billing.go`（新）、`member_auth.go`、`member_level.go`、`email_verify.go`、`blacklist.go`、`card.go`、`card_package.go`、`member.go`、`member_risk.go`、`member_data.go`、`member_function.go`、`member_rebind.go`、`app_info.go` | 鉴权去 `app_uuid`；策略改读全局；原子扣点；按等级取价 |
| P4 公开 API | `controllers/public/api.go` | `api_type` 单条件查询；`config` 合并应用级与全局级；`LoginResult` / `StatusResult` 增加 `free` |
| P5 后台 API | `controllers/admin/member.go`、`card.go`、`card_package.go`、`member_level.go`、`blacklist.go`、`app.go`、`server/admin.go` | 去应用维度；注册配置改全局读写；新增 `/apps/get_level_points`、`/apps/save_level_points` |
| P6 前端 | `api/admin/app.ts`、`views/admin/application/apps/form.vue`、`register-config.vue`、`level-points.vue`（新）、`index.vue` | 三模式表单；全局账号策略面板；等级点数矩阵；重建 `frontend/dist` |
| P7/P8 | `services/unified_account_test.go`（新）、`public_api_smoke_test.go`、`app_cascade_test.go`、`card_smoke_test.go`、`member_smoke_test.go`、`public/api_test.go` | 按新语义调整既有用例 + 7 条不变量测试 |

### P0 基线 ✅

1. `git tag pre-unified`；
2. 现有 `database.db*` 归档到 `backup/pre-unified/`（仅本地，已被 gitignore，不影响仓库）；
3. `go test ./...` 确认基线可运行（`services/public_api_smoke_test.go` 为回归基线，需按其语义调整而非原样通过）；
4. 记录接口类型清单（27 个）与现有 `apps` 41 个字段取值，用于核对 6.1 的归属表。

### P1 数据层 ✅

- 按 6.2 重写模型定义：删除 `app_uuid` 相关字段与复合唯一索引，改为单一唯一索引；
- `apps` 按 6.1 删除 19 个字段，新增 `free_mode`（按 D1）；
- 新增 `services/global_settings.go`（6.3 的 19 个配置项）；
- `database/settings.go` 的 `SeedDefaultSettings` 增加全局计费与账号策略默认值；
- `AllModels()` 清单同步；
- **无迁移函数**：不做 `MigrateToUnified`、不做索引演进、不做字段回填。

### P2 全局接口体系 ✅

- `services/api_seed.go` 重写为 `EnsureGlobalAPIs()`；
- `server.go:117` 调用点改名；
- `controllers/install/install.go` 安装流程加入 seed；
- `controllers/admin/app.go`：删除「创建应用时插入 27 条接口」整段；级联删除列表移除 `Member` / `Card` / `API`；
- `controllers/public/api.go:105`：接口查询去掉 `app_uuid`；
- `controllers/admin/api.go`：列表/导出 Handler 去应用维度；
- 修正 `controllers/admin/app_cascade_test.go` 断言。

### P3 账号与会员服务层 ✅

- `services/member_auth.go`：鉴权链路去 `app_uuid`；扣费原子化；**账号策略改读全局配置**（6.5 的 7 处）；`enforceRegisterLimit` 改全站计数；
- `services/member_level.go`：`resolveLevelUUID` / `defaultLevelDisplay` / `settleMemberLevel` / `memberLevelInfo` / `ListMemberLevels` / `SaveMemberLevel` 去 `appUUID`；
- `services/email_verify.go`：`emailCodeKey` / `emailResetCodeKey` 去 `app_uuid`；开关读全局配置；
- `services/blacklist.go`：`CheckBlacklist` / `existsBlacklist` / `AddBlacklistManual` / `ListBlacklist` 去 `appUUID`；
- `services/card.go` / `card_package.go`：制卡与套餐去 `appUUID`；
- `services/member.go`：`CreateMember`、`BatchRecharge`、`UpdateMemberProfile` 去按应用分支；
- `services/member_risk.go` / `member_data.go` / `member_function.go` / `member_rebind.go`：定位账号的查询去掉 `app_uuid`；
- 新增 `services/billing.go`（6.4）。

### P4 公开 API ✅

- 协议零改动：`app_uuid` 仍在信封、仍在签名；
- `dispatch` 中 `appUUID` 从「账号定位参数」降级为「策略上下文」；
- `LoginResult.Mode` / `StatusResult.Mode` 返回全局有效模式，字段名与类型不变。

### P5 后台 API ✅

- `controllers/admin/member.go`：`app_uuid` 由必填降为可选筛选；列表去掉「按应用批量查模式」逻辑；
- `card.go` / `card_package.go` / `member_level.go` / `blacklist.go` / `member_log.go` / `online.go`：同上口径；
- `controllers/admin/settings.go`：新增**计费策略 + 账号策略**两组配置的读写接口；
- `controllers/admin/app.go`：应用编辑接口移除 19 个字段。

### P6 前端 ✅

25 个文件，另加接口页与设置页改造：

- `views/admin/application/members/`、`cards/`、`levels/`、`packages/`：页面内「应用下拉」由必选改为可选筛选，空值即「全部」；
- `views/admin/application/apis/index.vue`：去掉应用选择，直接展示全局 27 条；建议从「应用管理」分组移到独立菜单；
- `views/admin/system/settings/`：新增两组区块 —— **计费策略**（模式、点数参数）与**账号策略**（13 项开关与参数）；
- `views/admin/application/apps/form.vue`、`multi-config.vue`、`register-config.vue`、`bind-config.vue`：移除 19 个字段，`register-config.vue` 可能整体废弃；
- `api/admin/api.ts`：去掉 `app_uuid` 参数；
- `api/admin/member.ts` / `card.ts` / `cardPackage.ts` / `memberLevel.ts`：`app_uuid` 参数改可选；
- `api/admin/settings.ts`：新增两组配置的读写；
- **重新构建 `frontend/dist`**，否则 Go 嵌入的静态资源仍是旧版。

### P7 高危修正 ✅

见第八章逐条处置结果。

### P8 验收 ✅

**已通过**

- `go build ./...` 通过；
- `go test ./...` 7 个包全绿（`controllers/admin`、`controllers/public`、`middleware`、`models`、`server`、`services`、`utils`）；
- 前端 `vue-tsc --noEmit` 无业务类型错误（仅模板自带的 `iconifyIconOffline` 提示）；`vite build` 成功产出 `frontend/dist`（138 文件 / 3.49 MB）。

**新增不变量测试**（`services/unified_account_test.go`，全部通过）

| 用例 | 锁住的行为 |
|---|---|
| `TestWalletSharedAcrossApps` | 在 A 应用扣点后，B 应用读到同一份已减少的余额 |
| `TestDeductPointsNeverOverdraws` | 连续扣点只有余额允许的次数成功，最终余额为 0 不为负 |
| `TestCardRedeemAcrossApps` | A 渠道发行的卡可在 B 应用核销；同一张卡不可二次使用 |
| `TestAppSecretIsolatesSignature` | 用 A 的 secret 签的串在 B 上验签失败 |
| `TestEnsureGlobalAPIsCreatesOnePerType` | `apis` 行数 = 接口类型数；二次调用幂等；默认全部启用 |
| `TestHybridModeUsesLevelPoints` | 混合模式按等级取价，未配置的等级回退默认；等级不足被拒 |
| `TestFreeModeSkipsAllGates` | `free_mode=1` 时过期/无点/无等级账号仍可用，且不扣点 |

**既有用例按新语义调整**

| 文件 | 调整 |
|---|---|
| `controllers/admin/app_cascade_test.go` | 断言重心反转：账号/卡密/接口**必须存活**，只清会话/日志/变量/函数 |
| `models/card_smoke_test.go` | 唯一索引改为单列（`card_no` / `username` / `api_type`），并新增 `api_type` 唯一断言 |
| `services/public_api_smoke_test.go` | 账号策略改走全局 `setPolicy` 辅助；卡密/等级/套餐字面量去 `app_uuid`；套餐面值校验替代模式匹配校验 |
| `services/member_smoke_test.go` | `CreateMember` 去 `appUUID`；建号改为双钱包参数 |
| `controllers/public/api_test.go` | 接口条目去 `app_uuid`；应用不再带账号策略字段 |

**待人工回归**（自动化覆盖不到）

全新安装 → 创建应用 → 卡密登录 → 充值 → 心跳扣费 → 转绑 → 拉黑 → 后台改名 → 后台解绑；客户端联调重点验证 `billing_mode` / `free` 与混合模式的等级点数。

---

## 八、高危清单与处置结果

### 1. 删应用会清空全站数据（核弹级）

改造前 `cascadeDeleteAppData` 的删除列表包含 `Member` / `Card` / `API`：

```go
// 改造前（危险）
derived := []any{
    &models.MemberSession{}, &models.MemberLog{},
    &models.Member{},        // ← 全局化后等于清库
    &models.Card{},          // ← 全局卡池一并清空
    &models.API{},           // ← 全局接口定义一并清空
    &models.Variable{}, &models.Function{},
}
```

去掉 `app_uuid` 后该语句要么报错，要么被误改成无条件删除。

**实际处置**：

```go
// 现状：只删该应用专属数据
derived := []any{
    &models.MemberSession{}, // 该应用发起的会话
    &models.MemberLog{},     // 该应用产生的审计日志
    &models.Variable{},      // 应用级变量
    &models.Function{},      // 应用级函数
}
```

账号、卡密、接口**一律不删**；`app_cascade_test.go` 已重写为「共享资产必须存活」的断言。

### 2. 全局点数后并发超扣

见 6.7。这是全新安装后**上线即存在**的问题，不是偶发 —— 两个应用同时在线就必然触发。

**实际处置**：`applyLoginCharge` / `settlePointsTime` / `DeductPoints` / `RechargeByCard` / `ClaimTrial` / `BatchRecharge` 全部改为条件原子更新：

```go
res := tx.Model(&models.Member{}).
    Where("uuid = ? AND points >= ?", m.UUID, cost).
    Update("points", gorm.Expr("points - ?", cost))
if res.RowsAffected == 0 { return errors.New("点数不足") }
```

`TestDeductPointsNeverOverdraws` 验证余额不会被扣成负数。

### 3. 安装流程遗漏 seed（无迁移兜底，风险升级）

只有安装流程这一个入口。若 `install.go` 忘记调用 `EnsureGlobalAPIs()`，新装系统会「有表无数据」，后台接口页空白、`/api/open` 全部返回「接口未配置」，且**无历史数据可参照**。

**实际处置**：
- `controllers/install/install.go` 在 `AutoMigrate` → `SeedDefaultSettings` → `SeedDefaultPortalNavigation` 之后调用 `services.EnsureGlobalAPIs()`；
- `cmd/server.go` 启动路径保留 `EnsureGlobalAPIs()` 作为幂等兜底；
- `TestEnsureGlobalAPIsCreatesOnePerType` 断言行数等于接口类型数且幂等；
- **新增 `controllers/install/install_smoke_test.go`**：在空内存库上完整复现安装步骤，逐项校验接口数量、策略默认值、`apps` 表结构、全局表无 `app_uuid`、唯一索引、`api_type` 直查。

> 这条高危在实施中**确实命中过一次**：新表 `app_level_points` 漏了往 `database.AllModels()` 里登记，`AutoMigrate` 不建表，混合模式一用就 `no such table`。因为单元测试自己手动 `AutoMigrate` 了该表，问题被掩盖到写安装回归测试才暴露。教训：**新增表必须同时进 `AllModels()`**，且安装测试不能复用业务测试的建表逻辑。

### 4. 账号策略改为全局后，误以为仍有应用级控制

`apps` 表删除 13 个账号策略字段后，任何残留的 `app.RegisterEnabled` / `app.TrialEnabled` 等读取点会**编译失败**（这是好事，能强制暴露全部接入点）。

**实际处置**：全部改为 `GetAccountPolicy()` 读取，**无任何留桩**。已 grep 确认这些字段名在 `.go` 源码中零残留（仅作为响应字段名与 JS 上下文 key 保留，用于前端兼容）。

### 5. 注册限制改全站计数后的行为变化

旧系统：同一 IP 可在每个应用各注册 N 个，全站最多 N×应用数。
新系统：同一 IP 全站最多注册 N 个。

**实际处置**：属预期变化（见 4.3）。默认值 `account_register_count` 已从 1 调整为 **3**，避免「注册一个号后再也注册不了」；前端策略面板对此有加粗提示。

### 6. 唯一索引定义遗漏

改为全新安装后不再需要 DROP 旧索引，但**新索引必须一次定义正确**：`members.username`、`cards.card_no`、`apis.api_type`、`blacklists (type, value)`。

**实际处置**：`models/card_smoke_test.go` 逐条验证 `card_no` / `username` / `api_type` 的重复插入会报错；`blacklists` 的 `(type, value)` 复合唯一在建表时定义。

### 7. `mode` 字段语义变化

客户端据 `mode` 走不同计费分支。最终采纳 D1-B（应用级三模式），故 `mode` 的**值域**从「0 时长 / 1 点数 / 2 免费」变为「0 会员 / 1 点数 / 2 混合」，并新增独立布尔 `free`。

**实际处置**：
- 响应体保留 `mode` 字段名与整型类型，但语义按新值域；
- `config` 中同时给出 `billing_mode`（新名）与 `operation_mode`（旧名，值相同）以便平滑过渡；
- 新增 `free` 布尔，客户端应优先用 `free` 判断「是否计费」，再用 `billing_mode` 决定显示到期还是点数；
- ⚠️ **旧客户端若把 `mode==2` 当免费，会在混合模式应用上误判**，需同步发版。这一点已在 `docs/api-guide.md` 显著标注。

### 8. `app_uuid="0"` 全局变量/函数

该机制已存在且工作正常，本次**不动**。它解决「配置共享」，与「账号共享」是两件事，不要混改。

### 9. 误把接口 `status` 当作应用级开关

全局化后 `apis.status=0` 会让**所有**应用该接口不可用。后台文案需明确写成「全站生效」，避免运维误操作。

**实际处置**：`services/api_seed.go` 的注释与接口页说明均标注「全站生效」；默认全部启用，避免运维为「开功能」而误关全局接口。

### 10. 接口密钥泄露面扩大

见 D5。全站共用密钥后，任一客户端的密钥泄露即等同于全站载荷密钥泄露。

**实际处置**：`docs/api-guide.md` 已改写「接口条目全站唯一 / 应用身份由 sign 区分」；导出接口响应标注 `shared: true`，提示 27 条共用一套密钥。

### 11. 旧库残留导致误连（已不存在）

归档旧 `database.db` 后，若运维忘记改配置或误把旧文件放回，服务会连上结构不匹配的库。`AutoMigrate` 会试图在新旧混杂的表上加索引。

**结论**：旧 `database.db` 已删除，本版本按**全新安装**分发，不存在误连旧库的路径，故**不加结构自检**。
若将来需要在旧库上原地升级，再补 `AutoMigrate` 后的列/索引存在性校验即可。

---

## 九、全新安装与初始化

### 9.1 安装流程顺序（已实现）

`controllers/install/install.go` 中，`AutoMigrate` 之后依次执行：

```
1. database.AutoMigrate()            → 按 6.2 建表（含 app_level_points）
2. database.SeedDefaultSettings()    → 写入 14 项账号策略默认值
3. database.SeedDefaultPortalNavigation()
4. services.EnsureGlobalAPIs()       → 插入 27 条全局接口，默认启用
5. 创建/更新超级管理员账户
```

> 第 4 步为本次新增。当前与建表在同一事务内，避免「表建好、接口没生成」的中间态。
> 注意：安装流程**不创建默认应用**。全新安装后 `apps` 为空、27 条接口已就绪 —— 先配接口，再加应用。

### 9.2 旧数据库处置（已执行）

| 项 | 动作 |
|---|---|
| `database.db` / `-shm` / `-wal` | 已移动到 `_backup/NetworkAuth-pre-unified-<时间戳>/`，**未删除**（保留旧数据以备取证） |
| `config.json` 的 `database.sqlite.path` | 保持不变；新库由安装流程重新创建同名文件 |
| git | 无需处理（`*.db*` 已被 .gitignore 忽略） |
| 代码回滚锚点 | `git tag pre-unified` |

### 9.3 安装后自检清单

| 检查项 | 期望 | 验证方式 |
|---|---|---|
| `apis` 行数 | = `len(GetDefaultAPITypes())` | 后台接口页显示条数；或直接查表 |
| `apis.api_type` 唯一 | 无重复 | 唯一索引已定义，重复插入会报错 |
| `apis.status` | 全部 = 1 | 装完即可联调 |
| `settings` 中 14 项 `account_*` | 存在且有值 | 后台「账号策略(全站)」面板可读到默认值 |
| `members` / `cards` / `card_packages` / `member_levels` / `blacklists` | 存在且**不含** `app_uuid` 列 | — |
| `cards` | 含 `source_app_uuid` 列 | — |
| `apps` | 含 `billing_mode` / `required_level` / `free_mode`；不含 13 项账号策略字段 | — |
| `app_level_points` | **表存在**，唯一键 `(app_uuid, member_level_uuid)` | 已在 `AllModels()` 中登记；`TestFreshInstallUniqueIndexes` 覆盖 |
| 后台应用编辑弹窗 | 显示「计费豁免 / 计费模式 / 要求等级 / 扣费参数」，不再有注册/试用/卡密开关 | — |
| `POST /api/open` 传不存在的 `app_uuid` | 返回「应用不存在」（而非「接口未配置」） | 说明应用校验先于接口查询 |

---

## 十、测试与验收

| 层次 | 实际内容 |
|---|---|
| 单元 | 计费解析（三模式 + 免费豁免）、账号策略读取、原子扣点边界、等级点数取值、`EnsureGlobalAPIs` 幂等 |
| 集成 | `services/public_api_smoke_test.go` 按新语义调整后全量通过；`services/unified_account_test.go` 7 条不变量用例 |
| 安装 | `controllers/install/install_smoke_test.go`：空库复现安装全流程，6 条断言（接口数/策略默认值/apps 结构/全局表无 app_uuid/唯一索引/api_type 直查） |
| 手工 | 真实客户端跑完整生命周期（未自动化） |
| 兼容 | 协议与字段名不变；`mode` 值域与 `free` 需客户端同步 |

跑法：

```bash
go build ./...          # 编译
go test ./...           # 全部测试
go test ./services/ -run TestWalletShared -v   # 单条不变量

cd frontend && pnpm install && pnpm build      # 重建 dist（改前端后必须）
```

---

## 十一、回滚

因为不涉及数据迁移，回滚比较简单：

1. 代码：`git revert` 或切回 `pre-unified` tag；
2. 数据：停服 → 删除新建的 `database.db*` → 从 `_backup/NetworkAuth-pre-unified-<时间戳>/` 取回旧库 → 重启旧版本；
3. 前端：`git checkout pre-unified -- frontend/src && pnpm build`（`dist` 是 gitignore 的，必须重建）；
4. 无「部分回滚」问题 —— 新旧库结构完全不兼容，只能整体切换。

---

## 十二、工作量

### 预估 vs 实际

| 阶段 | 预估人日 | 说明 |
|---|---|---|
| P0 基线 + P1 数据层 | 1.0 | 模型重写 + 新表 + 种子 |
| P2 全局接口体系 | 0.75 | seed 重写 + 调用点 + 级联删除修正 |
| P3 账号与会员服务层 | 2.25 | 最大头：13 个 service 文件 |
| P4 公开 API + P5 后台 API | 1.5 | 接口查询 + config 合并 + 策略读写改全局 |
| P6 前端 + 重建 dist | 2.0 | 3 个组件重写/新建 + 依赖补齐 + 构建 |
| P7 高危修正 + P8 验收 | 1.5 | 原子扣点 + 测试调整 + 7 条新用例 |
| **合计** | **约 9 人日** | |

实际实施中的额外开销（未计入上表）：

- **计费模型中途改向**：初版按「全局单模式」实现，后确认改为「应用级三模式」，需要回退 `apps` 计费字段并新增等级点数表，约多半天；
- **前端依赖不全**：`node_modules` 缺 `vite-plugin-cdn-import` 等包，需先 `pnpm install` 才能构建；
- **同步盘写冲突**：项目位于 OneDrive 同步目录，`write` 偶发 `ReplaceFileW EIO`，需先删文件再写。

风险如预期集中在 P3（并发扣点 + 策略迁移）与 P2（接口收敛为单一入口），两者均已通过测试锁定。

---

## 十三、决策结论

全部决策项已确认并落地。

- [x] **D1**：采纳 **B（应用级计费策略）**，并细化为三模式：会员 / 点数 / 混合。钱包（到期时间、点数、等级）全局共享，收费规则按应用。
- [x] **D2**：采纳 **全局卡池 + `source_app_uuid` 仅作发行渠道标记**。一张卡可在任意应用登录/充值；`card_no` 全局唯一；渠道字段不参与核销判定。
- [x] **D3**：采纳 **绑定全局共享 + 会话按应用独立计数**。绑定额度受「应用多开数 + 会员等级额外多开」约束；满员提示与手动顶号限本应用。
- [x] **D4**：采纳 **接口默认全部启用、不设应用级控制**。27 条装完即用，靠 `status` 全站开关。
- [x] **D5**：采纳 **加密密钥全站共用一套**。风险已接受：身份认证由 `app_secret` 签名承担，载荷加密仅作防抓包与提高分析成本；隔离等级从应用级降为全站级。
- [x] **账号策略全局化**：13 项全部迁移到 `settings`（键名前缀 `account_`）。
- [x] **注册限制全站计数**：同 IP/设备全站最多注册 N 个。默认值已调整为 **3**。
- [x] **客户端**：`mode` 值域变化 + 新增 `free`，**需同步发版**（已在 `api-guide.md` 标注）。
- [x] **旧库**：`database.db*` 已归档到 `_backup/`，不做数据保留承诺。
- [ ] **环境**：生产使用 SQLite 还是 MySQL 尚未确认。⚠️ 两者在「唯一索引 + 空值唯一性」行为上有差异（MySQL 允许多个 NULL，SQLite 同样允许，但 `emial` 为空串时表现不同），上线前需在实际数据库上跑一遍 `go test ./models/`。

### 遗留待办

| 项 | 优先级 | 说明 |
|---|---|---|
| 客户端发版适配 `billing_mode` / `free` | 高 | 旧客户端把 `mode==2` 当免费，遇混合模式会误判 |
| ~~旧库误连的结构自检~~ | ~~中~~ | **已不需要**：旧 `database.db` 已删除，本版本按全新安装分发，不存在误连旧库的路径 |
| 接口配置进程内缓存 | 低 | `api_type → *models.API` 热点读，当前每次查库 |
| 等级点数配置的缓存失效 | 低 | `Billing.loadLevelPoints()` 每次登录查一次该应用配置，可加进程缓存 + 后台保存时失效 |
| MySQL 环境回归 | 中 | 见上方「环境」项 |
