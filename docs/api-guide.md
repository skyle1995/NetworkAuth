# NetworkAuth 客户端对接文档（完整版）

面向客户端（软件端）对接 NetworkAuth 服务端的公开 API。所有业务接口都走**同一个端点** `POST /api/open`，靠请求体里的 `api_type` 区分。

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
- **运营模式（app 级）**：`mode=0` 时长模式（看 `expired_at`），`mode=1` 点数模式（看 `points`）。多数返回体同时给出 `mode`，客户端据此显示到期或余额。
- **心跳与离线**：登录成功返回 `heartbeat_interval`（分钟）。客户端应按该间隔周期性调用 `41 检测账号状态`（或 `40 获取到期`）以刷新在线；超过应用配置的「自动离线时长」未心跳，会话会被后台清理（掉线）。
- **时间字段**：`expired_at` 为 RFC3339 时间；永久有效对应 `permanent=true`（时间为 2099 年）。

---

## 六、接口清单

> 下表「登录」列表示是否需要 `token`。请求参数指 `data` 解密后的 JSON 字段；返回字段指结果密文解密后的 JSON。

### 基础信息（免登录）

#### `1` 获取程序公告
- 请求：无
- 返回：`{ title, version, content }`

#### `2` 获取更新地址
- 请求：无
- 返回：`{ download_type, download_url }`（`download_type`：0 不启用 / 1 自动更新 / 2 手动下载）

#### `3` 检测最新版本
- 请求：`{ version: "客户端当前版本" }`
- 返回：`{ latest_version, need_update(bool), force_update(bool), download_type, download_url }`

#### `4` 获取卡密信息
- 请求：`{ card: "卡号" }`
- 返回：`{ card_no, status(0未用/1已用/2冻结), status_text, mode, duration, points, used_at }`
  （时长模式看 `duration` 分钟，点数模式看 `points`）

### 卡密登录

#### `10` 卡密登录
- 触发条件：应用「卡密登录」开启。首次用某卡登录会**自动创建绑定该卡的账号**（用户名=卡号）并核销该卡。
- 请求：`{ card: "卡号", machine_code: "机器码" }`
- 返回（`LoginResult`）：`{ token, username, type, mode, permanent, expired_at, points, heartbeat_interval }`

### 账号体系

#### `21` 账号注册（邮箱即账号）
- 触发条件：应用「账号注册」开启；开启邮箱验证时需先调 `23` 拿验证码。
- 请求：`{ email, password, code }`（未开邮箱验证时 `code` 可空）
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`
- 说明：注册成功**不下发 token**（无试用时账号初始即过期），需再登录/充值/领试用。

#### `23` 发送注册验证码
- 触发条件：应用「账号注册」且「邮箱验证」均开启，且已配置 SMTP 与 Redis。
- 请求：`{ email }`
- 返回：`{ ... }`（发送结果；失败以明文 `msg` 返回，如「发送过于频繁」）

#### `24` 领取试用
- 触发条件：应用「领取试用」开启。**两种模式都支持**：时长模式发放试用时长（分钟），点数模式发放试用点数；受「每天/永久」领取次数限制。
- 请求：`{ username, password }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `22` 账号充值（用卡为账号充值）
- 触发条件：应用「卡密充值」开启。按运营模式给账号加时长或加点数。
- 请求：`{ username, card }`
- 返回（`StatusResult`）

#### `20` 账号登录
- 请求：`{ username, password, machine_code }`
- 返回（`LoginResult`）：同 `10`

### 登出

#### `30` 退出登录
- 请求：`{ token }`
- 返回：`{ message: "已退出登录" }`

### 状态查询与数据（需登录）

#### `40` 获取到期时间
- 请求：`{ token }`
- 返回（`StatusResult`）：`{ username, status, mode, permanent, expired_at, points }`

#### `41` 检测账号状态（心跳）
- 请求：`{ token }`
- 返回（`StatusResult`，含 `heartbeat_interval`）：既是心跳也是状态查询，返回用户基本信息 + 心跳间隔，客户端可据返回的 `heartbeat_interval` **动态调整**下次心跳时间。点数「按时」模式会在此结算并顺延周期。账号被封停/拉黑/到期/点数耗尽时返回异常，客户端据此登出。

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

#### `51` 机器码转绑
- 请求：`{ token, machine_code: "新机器码" }`
- 返回（`StatusResult`）；受「机器码重绑」开关、免费次数、扣费（时长扣分钟/点数扣点）约束。

#### `52` IP 转绑
- 请求：`{ token }`（以服务端识别的客户端 IP 为准）
- 返回（`StatusResult`）；约束同上。

#### `53` 功能扣点（点数模式）
- 请求：`{ token, points: 扣除点数 }`
- 返回（`StatusResult`）；原子扣减，余额不足则失败。

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
| `mode` | 运营模式：0 时长 / 1 点数 |
| `permanent` | 是否永久有效 |
| `expired_at` | 到期时间（时长模式） |
| `points` | 点数余额（点数模式） |
| `heartbeat_interval` | 心跳间隔（分钟） |

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
