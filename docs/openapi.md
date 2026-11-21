# 对外 API（API 密钥）

供脚本、CI、第三方系统调用的一组管理接口。与客户端协议（`/api/open`，走应用签名）不同，
这一组走 **API 密钥**，用于运维自动化：建应用、调计费、维护变量/函数、管理终端账号。

- 基址：`/api/openapi`
- 鉴权：请求头 `X-API-Key: sk_xxx`（也支持 `Authorization: Bearer sk_xxx`）
- 响应：`{ "code": 0, "msg": "success", "data": ... }`，`code != 0` 表示失败，同时返回对应 HTTP 状态码

---

## 一、密钥与能力（scope）

后台「开发配置 → API 密钥」里创建与分配能力。密钥只在创建/重置时明文显示一次。

| scope | 含义 |
|---|---|
| `api:read` | **通用只读**：放行所有 `*:read` |
| `api:write` | **通用写入**：放行所有 `*:write` |
| `app:read` | 应用：列表/详情 |
| `app:write` | 应用：创建、删除、重置密钥 |
| `billing:write` | 应用：调整计费策略 |
| `var:read` / `var:write` | 变量：读取 / 增删改 |
| `fn:read` / `fn:write` | 函数：读取 / 增删改 |
| `member:read` / `member:write` | 终端账号：读取 / 增删改、充值扣减 |

**兜底规则**：没有精确能力时按后缀兜底——有 `api:read` 即可用所有读取接口，
有 `api:write` 即可用所有写入接口。所以老密钥（只发了通用档）不会因为细分能力而失效；
新密钥建议按最小权限分配。

自检：

```bash
curl -H "X-API-Key: sk_xxx" http://127.0.0.1:8080/api/openapi/me
# {"code":0,"msg":"success","data":{"name":"ci","scopes":["app:read","billing:write"],"expire_at":""}}
```

---

## 二、应用

### `GET /api/openapi/apps` — 列表（`app:read`）

查询参数：`search`（名称或 uuid 精确）、`status`、`page`、`limit`（默认 1/20，上限 200）

```json
{ "list": [ { "uuid": "...", "name": "嘎嘎射击辅助", "plugin_id": "com.gaga.esp",
              "status": 1, "free_mode": 0,
              "bill_time_enabled": 1, "bill_points_enabled": 0, "level_gate_enabled": 0,
              "required_level": 1, "billing_scope": 0, "points_charge_mode": 0,
              "points_per_login": 1, "points_per_period": 1, "points_period_minutes": 60,
              "lease_ttl_minutes": 30, "created_at": "2026-11-20 12:00:00" } ],
  "total": 1, "page": 1, "limit": 20 }
```

> 列表与详情**不返回 `secret`**；密钥只在创建与重置接口返回一次。

### `GET /api/openapi/apps/:uuid` — 详情（`app:read`）

### `POST /api/openapi/apps` — 创建（`app:write`）

```json
{ "name": "新应用", "plugin_id": "com.example.new",
  "bill_time_enabled": true, "bill_points_enabled": false,
  "level_gate_enabled": false, "required_level": 1,
  "free_mode": 0, "status": 1, "lease_ttl_minutes": 30 }
```

- `plugin_id` 唯一，重复会 400
- 三个计费开关都不传时默认**计时模式**
- 返回 `{ "app": {...}, "secret": "..." }`，`secret` 只此一次

### `PUT /api/openapi/apps/:uuid/billing` — 调整计费（`billing:write`）

字段都可选，只改提交的项：

```json
{ "bill_time_enabled": false, "bill_points_enabled": true,
  "level_gate_enabled": true, "required_level": 2,
  "billing_scope": 1, "points_charge_mode": 1,
  "points_per_period": 2.5, "points_period_minutes": 30,
  "points_per_login": 1, "points_heartbeat_charge": 0, "free_mode": 0 }
```

三个开关的语义见 [architecture.md §5](architecture.md)。`billing_mode` 由开关自动推导，无需传。

### `POST /api/openapi/apps/:uuid/reset-secret` — 重置签名密钥（`app:write`）

返回新的 `secret`，旧值立即失效。

### `DELETE /api/openapi/apps/:uuid` — 删除应用（`app:write`）

删除应用行，并级联清理**该应用专属**的数据：

| 会被删除 | 不会被删除 |
|---|---|
| 激活/租约 `app_activations` | 终端账号 `members` |
| 该应用产生的账号日志 `member_logs` | 卡密 `cards` |
| 版本列表 `app_versions` | 接口定义 `apis`（全站唯一） |
| 等级点数 `app_level_points` | 变量 `variables` |
| 账号周期账本 `app_account_billings` | 函数 `functions` |

整个过程在一个事务里完成：任一步失败全部回滚。

```json
{ "code": 0, "msg": "已删除", "data": { "uuid": "..." } }
```

---

## 三、变量

`app_uuid` 为空表示全站变量；同一应用内 `alias` 唯一。

| 方法 | 路径 | 能力 | 说明 |
|---|---|---|---|
| GET | `/variables` | `var:read` | `app_uuid=global` 查全局；`alias`/`search` 过滤 |
| GET | `/variables/:uuid` | `var:read` | |
| POST | `/variables` | `var:write` | `{ alias, data, remark?, app_uuid? }` |
| PUT | `/variables/:uuid` | `var:write` | `{ data?, remark?, app_uuid? }`，**alias 不可改** |
| DELETE | `/variables/:uuid` | `var:write` | |

```bash
curl -X POST http://127.0.0.1:8080/api/openapi/variables \
  -H "X-API-Key: sk_xxx" -H "Content-Type: application/json" \
  -d '{"alias":"zex_key","data":"1a2b3c","app_uuid":""}'
```

---

## 四、函数

函数全站共用、`alias` 全局唯一，在 goja 沙箱执行（客户端 `44` 调用）。

| 方法 | 路径 | 能力 |
|---|---|---|
| GET | `/functions` | `fn:read` |
| GET | `/functions/:uuid` | `fn:read` |
| POST | `/functions` | `fn:write` |
| PUT | `/functions/:uuid` | `fn:write` |
| DELETE | `/functions/:uuid` | `fn:write` |

创建：`{ "alias": "calc", "code": "return a+b;", "remark": "..." }`；更新只能改 `code` / `remark`。

---

## 五、终端账号

账号全站唯一，接口一律按 **UUID** 定位（不暴露自增 id）。

| 方法 | 路径 | 能力 | 说明 |
|---|---|---|---|
| GET | `/members` | `member:read` | `search`（用户名/邮箱模糊）、`status`、`level_uuid`、分页 |
| GET | `/members/:uuid` | `member:read` | |
| POST | `/members` | `member:write` | `{ username, password, duration_value?, duration_unit?, points?, remark? }` |
| PUT | `/members/:uuid` | `member:write` | `{ points?, permanent?, expired_at?, total_recharge?, remark? }`（points 为**绝对值**） |
| POST | `/members/:uuid/status` | `member:write` | `{ status }` 0 封停 / 1 正常 / 2 黑名单 |
| POST | `/members/:uuid/points` | `member:write` | `{ points, mode? }`，`mode=add`（默认）/ `sub` |
| POST | `/members/:uuid/time` | `member:write` | `{ value, unit, mode?, permanent? }` |
| POST | `/members/:uuid/password` | `member:write` | `{ password }` |
| DELETE | `/members/:uuid` | `member:write` | 删除账号 |

`unit` 取值与卡密一致：`minute` / `hour` / `day` / `month` / `year` / `permanent`。

账号返回体：

```json
{ "uuid": "...", "username": "user@test.com", "email": "",
  "type": 0, "status": 1, "status_text": "正常",
  "points": 8, "permanent": false, "expired_at": "2026-12-20 12:00:00",
  "total_recharge": 0, "level_uuid": "...", "level": 1, "level_name": "普通会员",
  "remark": "", "created_at": "2026-11-20 12:00:00" }
```

> `permanent=true` 时 `expired_at` 返回字符串 `"permanent"`。

---

## 六、错误处理

| HTTP | 场景 |
|---|---|
| 401 | 缺少密钥 / 密钥无效 |
| 403 | 密钥禁用、过期，或缺少对应 scope |
| 400 | 参数校验失败（含别名重复、应用不存在） |
| 404 | 资源不存在 |
| 500 | 服务端错误 |

```json
{ "code": 403, "msg": "API 密钥无此权限" }
```

所有写操作都会记入后台「日志管理 → 操作日志」，操作者显示为 `apikey:<密钥名称>`，便于审计。

---

## 七、典型脚本

```bash
KEY=sk_xxx
BASE=http://127.0.0.1:8080/api/openapi

# 建应用
curl -s -X POST $BASE/apps -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"name":"自动化测试","plugin_id":"com.example.auto"}'

# 切成纯点数（每 30 分钟 2.5 点）
curl -s -X PUT $BASE/apps/<uuid>/billing -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"bill_time_enabled":false,"bill_points_enabled":true,
       "points_charge_mode":1,"points_per_period":2.5,"points_period_minutes":30}'

# 给账号加 100 点
curl -s -X POST $BASE/members/<uuid>/points -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"points":100}'
```
