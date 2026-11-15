# NetworkAuth 工作流程表

> 本文按「请求进入 → 处理 → 落库」的路径，逐场景列出当前实现的实际步骤。
> 代码位置以函数名为准，便于对照源码。

---

## 一、通用请求流水线（每次调用都走）

所有业务共用一个端点 `POST /api/open`，流程固定在 `controllers/public/api.go: OpenAPIHandler`。

| 步 | 动作 | 校验/处理 | 失败返回 |
|---|---|---|---|
| 1 | 解析信封 | `{app_uuid, api_type, data, timestamp, sign}` JSON | `请求参数错误` |
| 2 | 定位应用 | `WHERE uuid = app_uuid` | `应用不存在` / `APP_NOT_FOUND` |
| 3 | 应用状态 | `app.status = 1` | `应用已停用` / `APP_DISABLED` |
| 4 | **验签** | `SHA256(app_uuid\|api_type\|data\|timestamp\|app_secret)` | `SIGNATURE_INVALID` |
| 5 | 定位接口 | `WHERE api_type = ?`（全站唯一，非按应用） | `接口未配置` |
| 6 | 接口状态 | `api.status = 1` | `接口已停用` |
| 7 | **解密载荷** | 按该接口配置的算法解 `data` | `请求解密失败` |
| 8 | 分发业务 | `dispatch()` → 各 handler | 业务错误 |
| 9 | 加密返回 | 按该接口的返回算法加密 | `返回加密失败` |

> 顺序不可颠倒：**先验签证明调用方身份，再解密**。否则任何人都能用任意应用的配置解密别人载荷。

---

## 二、安装初始化（一次性）

`controllers/install/install.go: InstallSubmitHandler`

| 步 | 动作 | 说明 |
|---|---|---|
| 1 | `database.ReInit()` | 建立连接 |
| 2 | `AutoMigrate(AllModels())` | 建全部表（**新增表必须进 `AllModels()`**，漏了只在运行时 `no such table`） |
| 3 | `SeedDefaultSettings()` | 写 32 项 `account_*` 账号策略 |
| 4 | `SeedDefaultPortalNavigation()` | 门户导航 |
| 5 | `EnsureGlobalAPIs()` | 写 30 条接口定义，默认启用、不加密 |
| 6 | 创建超级管理员 | 事务内 |

---

## 三、账号注册（`21`）

| 步 | 动作 | 校验 |
|---|---|---|
| 1 | 策略检查 | `account_register_enabled` |
| 2 | 邮箱验证码 | `account_email_verify_enabled` 开启时校验 |
| 3 | 卡密注册 | `account_card_register_enabled` 开启时必须带有效未用卡 |
| 4 | 注册限制 | 全站计数：`account_register_limit_*`（**不是按应用**） |
| 5 | 建账号 | `members`，`username`/`email` 全站唯一 |
| 6 | 卡面值发放 | 有 `duration` 加到 `expired_at`，有 `points` 加到 `points`，**两者都有就都发** |
| 7 | 核销卡 | `cards.status = used` |
| 8 | 结算等级 | 按 `total_recharge` 决定 `level_uuid` |

---

## 四、账号登录（`20`）—— 入参是账号密码，不是令牌

> 登录的**凭据永远是账号 + 密码**；令牌是登录成功后的**返回值**，不是登录时提交的东西。

`services/member_auth.go: AccountLogin` → `issueAccountTokenResult`

**入参**：`{ username, password, machine_code, version, device_name }`
**出参**：`LoginResult`，其中 `token` 即账号令牌

| 步 | 动作 | 关键点 |
|---|---|---|
| 1 | 校验账号密码 | `WHERE username = ?` + 加盐哈希比对；失败统一回「账号或密码错误」 |
| 2 | 强制更新门禁 | `checkForceUpdate`，版本过旧直接拒（**在发令牌前**） |
| 3 | 账号状态 | 封停 / 拉黑即拒 |
| 4 | 生成并落库令牌 | `IssueAccountToken`：`token` + `refresh_token`，30 天；**这是出参，不是入参** |
| 5 | 记录登录痕迹 | `last_login_at` / `last_login_ip`；`register_machine` 为空则回填 |
| 6 | 写审计日志 | `AddMemberLog` |
| — | **不做** | ❌ 不扣点 ❌ 不校验绑定 ❌ 不建激活 ❌ 不算并发 |

**返回体**里的 `mode` / `expired_at` / `points` 只是展示用，不代表已获得该应用使用权。

### 令牌的定位

| | 凭据（账号密码 / 卡密） | 令牌 |
|---|---|---|
| 用途 | **证明「你知道密码」**，只能换令牌 | **证明「你已登录过」**，用于后续所有接口 |
| 提交时机 | 只在 `10`/`20` 登录时 | 每次业务请求都要带 |
| 有效期 | 密码本身不变 | 30 天，可刷新/可吊销 |

想强制用户每次启动都输密码：把客户端做成不持久化 token 即可，服务端接口形态不变；
也可以把 `AccountTokenTTL` 调短（`services/account_token.go` 顶部常量）。

---

## 五、卡密登录（`10`）—— 入参是卡号

`services/member_auth.go: CardLogin`

**入参**：`{ card: "卡号", machine_code, version, device_name }`
**出参**：`LoginResult`（同 `20`，`token` 为账号令牌）

| 步 | 动作 | 关键点 |
|---|---|---|
| 1 | 策略检查 | `account_card_login_enabled` |
| 2 | 强制更新门禁 | 在核销卡之前拦，避免「拒登录但卡已用」 |
| 3 | 查卡 | 冻结 → 拒 |
| 4a | **首次使用** | 建账号（`username = 卡号`）→ 发面值（时长/点数按非零项）→ 结算等级 → 核销卡 |
| 4b | **已使用** | 按卡号找已有账号 |
| 5 | 颁发账号令牌 | 同账号登录，**不计费** |

---

## 六、应用激活（`70`）★ 计费唯一入口

`services/entitlements.go: ActivateByToken` → `activation.go: Activate`

### 6.1 事务外前置校验

| 步 | 动作 | 失败码 |
|---|---|---|
| 1 | 校验账号令牌 | `ACCOUNT_TOKEN_INVALID` |
| 2 | 加载应用并检查 | `APP_NOT_FOUND` / `APP_DISABLED` |
| 3 | 应用必须有 `uuid` + `secret`（签名用） | `APP_NOT_FOUND` |
| 4 | 需要设备码时必须有 | `FINGERPRINT_REQUIRED`（`account_machine_verify=1` 且未传） |
| 5 | 账号状态 + 等级 + 会员到期 | `LICENSE_SUSPENDED` / `ENTITLEMENT_REQUIRED` / `LICENSE_EXPIRED` |
| 6 | 黑名单（设备/IP/地区） | 命中即拒（全站生效） |
| 7 | 无设备码时生成 `anon:<随机>` | 不绑设备的应用，每次激活算独立占用 |

### 6.2 事务内（账号行锁保护）

`lockMemberRow` 串行化同一账号，防并发击穿。

| 步 | 动作 | 说明 |
|---|---|---|
| 1 | `lockMemberRow` | 账号级互斥 |
| 2 | 机器码绑定 | `ensureMachineBinding`（`account_machine_verify=1` 时）；超限 → `FINGERPRINT_MISMATCH` |
| 3 | IP 绑定 | `ensureIPBinding`（`account_ip_verify>0` 时） |
| 4 | **幂等查找** | `WHERE app_uuid + member_uuid + fingerprint` |
| 5a | 已存在且**活跃** | 复用：续租约、刷新 IP/版本，**不扣费**，返回 `reused=true` |
| 5b | 已存在但**已死** | 复活：清释放标记 + 按新占用扣费 |
| 5c | **不存在** | `enforceMaxConcurrent` 在线名额检查 → 扣费 → 建激活 |
| 6 | 回写账号登录痕迹 | `last_login_at` / `last_login_ip` |

### 6.3 在线名额控制（`enforceMaxConcurrent`）

上限 = 账号策略「同时在线」+ 会员等级额外并发（下限 1），**账号级、按设备去重**。

| 场景 | 行为 |
|---|---|
| 该设备已在在线名单里 | 直接放行——只是同一台电脑多开一个插件，**不占新名额** |
| 设备数 < 上限 | 放行 |
| 设备数 = 上限 | 按本应用的 `login_type` 处理 |

| `login_type` | 满员时行为 |
|---|---|
| `0` 顶号 | 把**最早那台设备的全部激活**一起下线（整机腾位） |
| `1` 拒绝 | 返回 `MACHINES_LIMIT_EXCEEDED` |
| `2` 手动顶号 | 返回 `code=3` + 在线设备列表（`sessions`） |

> 只有开启 `account_machine_verify` 时才做这个检查：拿不到可靠设备标识时，
> 每次激活都会被当成新设备，反而会把名额瞬间占满。

### 6.4 扣费口径（`chargeActivationOnce`）

| 计费模式 | 激活时 | 扣的是 |
|---|---|---|
| 计时模式 | 只校验等级+有效期 | 不扣 |
| 点数·按次 | 扣 `PointsCostForLevel(level)` | `members.points` |
| 点数·按时（`FROM_CREATION`） | 扣一个周期，写 `activation.period_end` | `members.points` |
| 点数·按时（`FROM_FIRST_PING`） | 不扣，等首次心跳 | — |
| 高级模式 | 等级门禁 + 上述点数口径 | `members.points` |
| 豁免 `free_mode=1` | 跳过全部门禁与扣费 | — |

> ⚠️ 按时周期的 `period_end` **只写激活表**，绝不写 `members.expired_at`
> —— 后者是全站钱包，多应用按时计费若都写它必然互相延长可用期。

---

## 七、心跳 / 续租约（`41`）

`services/member_auth.go: CheckMemberStatus` → `renewLeaseOrImplicit` → `activation.go: RenewLease`

| 步 | 动作 | 说明 |
|---|---|---|
| 1 | 令牌校验 | 只接受激活令牌；账号令牌 → `ACTIVATION_REQUIRED` |
| 2 | 校验激活 | 已释放 → `ACTIVATION_TOKEN_INVALID`；租约过期 → `LEASE_EXPIRED` |
| 3 | 事务 + 行锁 | 同激活串行化 |
| 4 | 门禁复查 | 账号是否被封、等级是否还够 |
| 5 | 按时扣费 | `period_end < now` 时扣下个周期；点数不足 → 置激活为死 |
| 6 | 推进租约 | `lease_expires_at = now + lease_ttl` |
| 7 | 返回 | 最新钱包 + `StatusResult` |


**扣费失败**：激活立即置 `deactivated_at`，客户端下次心跳收到 `ACTIVATION_TOKEN_INVALID`。

---

## 八、应用内功能（`42`/`43`/`44`/`45`/`53`/`54`）

统一走 `services/auth_for_app.go: AuthForApp` → `authActiveMember`

| 步 | 动作 |
|---|---|
| 1 | 尝试激活令牌 → 命中则校验信封 `app_uuid` 与激活所属应用一致（不一致 → `INVALID_REQUEST`） |
| 2 | 未命中则返回 `ACTIVATION_REQUIRED`（不隐式激活） |
| 3 | 校验账号状态正常、应用启用 |
| 4 | 执行具体业务（读变量/执行函数/读写用户数据/扣点） |

**`53 功能扣点`**：条件原子更新 `WHERE uuid=? AND points + 1e-6 >= ?`，`RowsAffected=0` 即点数不足，杜绝并发超扣。
点数支持小数（`float64`），扣减后 `ROUND(..., 6)`：容差保证「余额 0.3 扣 0.1 三次」能扣完，四舍五入避免浮点尾巴累积。

---

## 九、应用释放（`71`）

`services/entitlements.go: DeactivateByToken` → `activation.go: Deactivate`

| 步 | 动作 |
|---|---|
| 1 | `UPDATE app_activations SET deactivated_at=now WHERE token=? AND deactivated_at IS NULL` |
| 2 | 幂等：已释放/不存在也返回 `{released:true}` |
| 3 | 并发名额立即归还 |
| — | 已扣点数不退（按次已消费；按时当前周期仍算已用） |

---

## 十、退出登录（`30`）

`services/member_auth.go: MemberLogout`

| 传入令牌 | 行为 |
|---|---|
| 账号令牌 | `RevokeAccountToken`：删令牌 + **级联把该令牌下所有激活置为已释放** |
| 激活令牌 | 只释放这一个激活（不登出账号） |

---

## 十一、权益清单（`72`）

`services/entitlements.go: EntitlementsByToken` → `ListEntitlements`

| 步 | 动作 |
|---|---|
| 1 | 校验账号令牌 |
| 2 | 账号状态 / 全站黑名单 |
| 3 | 遍历全部启用应用 |
| 4 | 逐个跑门禁预判（不扣费、不建记录） |
| 5 | `usable=1` 才下发 `secret`；不可用的只给 `reason` |
| 6 | 附账号钱包与会员态 |

---

## 十二、充值（`22` / 卡密）与试用（`24`）

| 接口 | 鉴权 | 流程 |
|---|---|---|
| `22` 充值 | 凭据（用户名+卡号） | 策略检查 → 查卡未用 → 按会员等级返利放大面值 → 发时长/点数 → 核销 → 结算等级 |
| `24` 试用 | 凭据（用户名+密码） | 策略检查（开关+额度）→ 领取资格（`trial_claim_mode`）→ 发额度 → 记录已领 |

---

## 十三、转绑（`51`）与手动顶号（`31`）

| 接口 | 鉴权 | 流程 |
|---|---|---|
| `51` 转绑 | **凭据**（卡号即身份，无密码） | 校验转绑开关 → 判断「同设备 no-op / 新增 / 替换」→ 计次 → 超免费次数则扣时长 → 写 `bindings` |
| `31` 手动顶号 | **凭据** | `code=3` 时客户端拿不到令牌，故用凭据验证 → 校验 `session_id` 属于本人且属于本应用 → 释放该激活 |

> 转绑用凭据而非令牌，是为了避免「设备不匹配→登不进→拿不到令牌→无法转绑」的死循环。

---

## 十四、后台定时任务（每分钟一拍，按应用 `clean_interval` 节流）

`services/member_session_cleanup.go: sweepSessions`

| 步 | 动作 | 为什么需要 |
|---|---|---|
| 1 | 回收过期租约 | `lease_expires_at < now` 的激活标为已释放 —— **客户端崩溃不调 71 的唯一兜底** |
| 2 | 清理旧会话 | 过渡期 `member_sessions` 中超 `offline_timeout` 的行 |
| 3 | 物理删除 | 已释放超 7 天的激活（保留期内仍可查历史） |
| 4 | 清理孤儿 | 归属应用已删除的会话/激活 |
| 5 | 清理过期令牌 | 账号令牌 `expires_at < now` |

---

## 十五、后台管理动作

| 动作 | 影响的表 | 说明 |
|---|---|---|
| 删除应用 | 清 `member_sessions` / `app_activations` / `member_logs` / `variables` / `functions` | **绝不删** members / cards / apis / levels（全站资源） |
| 踢下线 | `app_activations` 置 `deactivated_at` | 名额立即回收；同时清旧会话兼容 |
| 仪表盘「在线数」 | `app_activations` 活跃数 | 不再统计登录会话 |
| 在线列表 | 读 `app_activations` | 新增展示 `lease_expires_at` |
| 改密 | 删账号令牌 + 释放全部激活 | 强制所有设备重新登录 |

---

## 十六、令牌矩阵速查

| 令牌 | 可用接口 |
|---|---|
| 不需要令牌（凭据或免登录） | `1`/`2`/`3`/`4`（免登录）；`10`（卡号）、`20`（账号密码）、`21`/`23`/`25`/`26`（注册与找回，用邮箱验证码） |
| **账号令牌** | `22`/`24`/`30`/`40`/`50`/`72` |
| **激活令牌** | `41`/`42`/`43`/`44`/`45`/`53`/`54`/`71` |
| 凭据（密码/卡密） | `31`/`51` |
| 仅签名 | `60`/`61`/`62`（风控，服务端调用） |

**隔离**：账号令牌调应用级接口 → `ACTIVATION_REQUIRED`；激活令牌调账号级接口 → 放行。

---

## 十七、一张图概括

```
         ┌──────────── 一次性 ────────────┐
安装 ──► 建表 + 写入 30 条接口 + 32 项账号策略 + 超管

         ┌──────────── 每次进入应用 ────────────┐
20/10 登录（账号密码 / 卡号）──► 返回账号令牌（不计费）
                 │
                 ├─ 72 清单 ──► 可用插件 + secret（按权限过滤）
                 │
                 └─ 70 激活 ──► ★计费★ 等级/到期/点数/绑定/并发
                                 │
                                 ├─ 41 心跳 ──► 续租约（按时扣周期）
                                 ├─ 42/44/53 ──► 应用数据与功能
                                 └─ 71 释放 ──► 名额立即归还
                                                 │
30 登出 ──────────────────────────────────────┘ 级联失效全部激活

         ┌──────────── 兜底（每分钟） ────────────┐
sweepSessions ──► 回收过期租约 + 清旧会话 + 删 7 天前归档 + 清过期令牌
```
