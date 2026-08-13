# 会话交接文档 — 多开配置 & 验证码机制答疑

> 源会话: Claude Code `89e81a2b-dae6-486c-83ab-bfd00dd65090`（2026-08-11）
> 项目: NetworkAuth（网络授权服务，Go + Gin + GORM，前端 Vue）
> 会话性质: 纯咨询答疑，**未修改任何代码**（git 工作区仅新增 `.claude/`）
> 状态: 会话已完整结束（最后一个问题已答复），无未完成任务

---

## 一、会话背景

用户围绕登录系统的 **多开配置**（登录方式 × 多开范围）与 **后台登录验证码** 提出了一系列问题，全部为机制咨询，最终确认了后台验证码为后端校验。以下是完整的问答结论。

---

## 二、核心结论速览

| 问题 | 结论 |
|------|------|
| 登录方式是什么 | `login_type`：0=顶号登录（默认），1=非顶号登录 |
| 多开范围是什么 | `multi_open_scope`：0=单设备（按机器码），1=单IP（按登录IP），2=全部设备（默认，每次会话独立） |
| 全部设备能否一台登满 | **能**。每次登录生成新 token，key 永远不同，一台设备可占满全部上限 |
| 绑定与会话的关系 | **两套独立计数**，但共用同一个 `effectiveMultiOpen` 数字做上限 |
| 绑定是否永久占会话额度 | **否**，绑定不占会话额度 |
| 绑定上限是否受多开数量限制 | **是**，绑定数与会话数共用 `effectiveMultiOpen` |
| 后台验证码是前端还是后端校验 | **后端校验**，抓包绕不过 |

---

## 三、多开机制详解

### 3.1 关键函数 `sessionOpenKey`（`services/member_auth.go:509-521`）

```go
func sessionOpenKey(scope int, machineCode, ip, token string) string {
    switch scope {
    case models.MultiOpenScopeMachine:  // 单设备 → key = "m:机器码"
        ...
    case models.MultiOpenScopeIP:       // 单IP → key = "i:IP"
        ...
    }
    return "t:" + token  // 全部设备 → key = "t:随机token"，每次唯一
}
```

**这个 key 决定"两次登录算不算同一个开"：**

| 范围 | key | 同机器反复登录 |
|------|-----|---------------|
| 单设备 | `m:机器码` | 覆盖旧会话，只占 1 个名额 |
| 单IP | `i:IP` | 同 IP 覆盖，只占 1 个名额 |
| 全部设备 | `t:token`（每次新生成） | 每次都占 1 个新名额 |

### 3.2 登录方式定义（`models/app.go:82-83`）

```go
LoginType int `gorm:"default:0;not null;comment:登陆方式，0=顶号登录，1=非顶号登录" json:"login_type"`
```

### 3.3 登录方式生效点（`services/member_auth.go:418-421`）

```go
if len(distinct) >= maxOpen {       // 会话数达到上限
    if app.LoginType == 1 {          // 非顶号
        return errors.New("已达最大同时在线数")  // → 拒绝
    }
    // 顶号：踢掉最早的「开」直到腾出空位  // → 允许
    for _, k := range order { ... }
}
```

### 3.4 绑定与会话共用同一数字（`services/member_auth.go`）

```go
// 第 336 行：算一次
effMultiOpen := effectiveMultiOpen(db, app, member)

// 第 366 行：绑定门槛
ensureMachineBinding(tx, member.UUID, machineCode, deviceName, effMultiOpen)

// 第 418 行：会话门槛
if len(distinct) >= maxOpen {  // maxOpen 就是 effMultiOpen }
```

`ensureMachineBinding` 内部（第 559 行）：

```go
if int(count) >= multiOpenCount {  // multiOpenCount = effMultiOpen
    return errors.New("机器码未绑定，请先进行机器码转绑")
}
```

### 3.5 登录双关卡流程

```
登录请求
  │
  ├─ ① ensureMachineBinding()  ← 检查 bindings 表，限 effectiveMultiOpen 个不同机器码
  │     ├─ 已绑定 → 放行
  │     ├─ 未绑定 + 绑定量 < 上限 → 新增绑定，放行
  │     └─ 未绑定 + 绑定量 ≥ 上限 → ❌ "机器码未绑定，请先进行机器码转绑"
  │
  └─ ② 会话计数（去重 + 顶号逻辑）← 检查 member_sessions 表
```

**绑定与会话是两条平行线：**

| | 绑定（Binding） | 会话（Session） |
|---|---|---|
| 限制什么 | 几台设备被**授权**登录 | 几个连接**同时在线** |
| 存在哪里 | `bindings` 表 | `member_sessions` 表 |
| 上限 | ≤ effectiveMultiOpen | ≤ effectiveMultiOpen |
| 触发点 | 登录时 `ensureMachineBinding` | 登录时会话去重计数 |

---

## 四、6 种组合场景示例（multi_open_count = 3）

### ① 顶号 + 单设备
3 台不同设备各登 1 次 = 满。再有新设备 → 踢最老的那台。同机器反复登只覆盖自己，不占新坑。

### ② 顶号 + 全部设备（未开机器码验证 MachineVerify=0）
任意设备随便登，只受 3 个会话约束，谁最老踢谁。一台电脑开 4 个窗口 = 第 4 个把第 1 个顶掉。

### ③ 顶号 + 全部设备（开机器码验证 MachineVerify=1）
两层门槛共用数字 3：先过绑定关（限 3 台设备），再过会话关。

### ④ 非顶号 + 单设备 / ⑤ 非顶号 + 全部设备
会话达到上限后第 4 次登录直接拒绝，返回"已达最大同时在线数"。

### ⑥ 单设备 + 机器码验证
一台设备第 1 次登录即完成绑定，之后同机器反复登不新增绑定。

---

## 五、几个容易混淆的点（用户最终确认的理解）

1. **全部设备模式下，同一台设备可以登满上限** —— 对，因为每次登录 token 不同，key 不同。
2. **绑定占一个名额，多开额外占名额** —— 不完全对：绑定和会话是两套独立的限制，各自用同一个数字做上限，但不是共享名额。
3. **一台设备产生绑定就永久占用一个会话额度** —— 否，绑定不占用会话额度。
4. **绑定数量按多开数量限制上限** —— 对，两处共用 `effectiveMultiOpen`。
5. **顶号 + 全部设备 + 多开 3，其他设备不上线时，一台设备可以开 3 个会话** —— 对，3 个不同 token = 3 个独立"开"。

---

## 六、后台登录验证码（会话最后一个问题）

**结论：后端验证，前端绕不过。**

验证位置 `controllers/admin/auth.go:88`，**在查数据库和验密码之前**：

```go
// 第 88-92 行
if !VerifyCaptcha(c, body.Captcha, body.CaptchaToken) {
    response.Error(c, "验证码错误")
    return
}
```

### 三种验证码类型

| 类型 | 验证方式 |
|------|---------|
| image | 传统字符验证码，用户输入文字，后端对照 `CaptchaStore`（内存）验证 |
| slide | 滑块拼图，前端拖完 → 后端验证坐标 → 发一次性 token → 登录时消费 token |
| click | 点字验证码，前端点完 → 后端验证点击位置 → 发一次性 token → 登录时消费 token |

### 为什么抓包绕不过

以 slide/click 为例，前端完成交互后向后端换取**一次性 token**，登录时必须携带该 token 且只能消费一次。抓包直接调登录接口缺少有效 token → 后端拒绝。image 类型则直接对照后端内存中的 `CaptchaStore`，伪造无效。

---

## 七、会话内引用过的关键文件

| 文件 | 位置 | 内容 |
|------|------|------|
| `services/member_auth.go` | 336/366/390-402/418-421/509-521/559 | 多开/绑定/顶号全部逻辑 |
| `models/app.go` | 82-83 | `login_type` 定义 |
| `controllers/admin/auth.go` | 88-92 | 后台登录验证码校验 |
| `models` | `MultiOpenScopeMachine/IP` | 多开范围常量 |

---

## 八、后续建议方向（若继续）

- 如需**拆分绑定与多开的上限数字**（当前共用 `effectiveMultiOpen`），改动点集中在 `services/member_auth.go` 的 `effectiveMultiOpen` 调用处（336/366/418 行），需同步 `models` 与数据库字段。
- 如需调整验证码策略（如增加频率限制、验证码 store 持久化），入口在 `controllers/admin/auth.go` 与 `VerifyCaptcha` 实现。
