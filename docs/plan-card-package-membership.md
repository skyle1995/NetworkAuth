# 计划书：卡密套餐 / 会员制（累充自动升级 + 充值返利）

> 状态：**方案已定稿**，尚未实施。两项决策已确认：累充**按价格**累计、返利**先返利再升级**。

## 一、目标

1. **卡密套餐**：制卡不再手填时长/点数，改为**选套餐**，面值由套餐决定（时长卡或点卡）。
2. **会员制**：账号有会员等级，权益为**充值返利**（按比例多返时长/点数）。
3. **累充自动升级**：累计充值达到等级门槛，自动升级。

## 二、现状摸底

- `Card`：`Duration`(分钟，`-1`=永久) / `Points` / `Status` / `BatchNo` / `UsedByMember` …
- 制卡：`services.BatchCreateCards(appUUID, prefix, randomLen, count, durationMinutes, points, remark)` —— 手动指定面值
- **卡密面值在 3 处被消费**（`services/member_auth.go`）：
  1. `CardLogin` 首次激活建号
  2. `AccountRegister` 卡密注册
  3. `RechargeByCard` 充值
- 应用是**单一运营模式**（时长 / 点数 / 免费），面值语义随模式而定

## 三、数据模型

### 新增：`CardPackage`（卡密套餐 = 售卖单元）

| 字段 | 说明 |
|---|---|
| `UUID` / `AppUUID` | 归属应用 |
| `Name` | 套餐名（如「月卡」「1000点」） |
| `Type` | 0=时长 / 1=点数 |
| `Duration` | 面值分钟数，`-1`=永久（Type=0 用） |
| `Points` | 面值点数（Type=1 用） |
| `Price` | **售价，整数「分」**（累充计量口径）。前端以「元」输入展示（×100 / ÷100） |
| `Status` / `Sort` / `Remark` | 启用禁用 / 排序 / 备注 |

> 金额一律用**整数分**存储（`Price` / `TotalRecharge` / `Threshold`），避免浮点误差导致门槛比较出错。

### 新增：`MemberLevel`（会员等级）

| 字段 | 说明 |
|---|---|
| `UUID` / `AppUUID` | 归属应用 |
| `Name` | 等级名（如「白银」「黄金」） |
| `Threshold` | **累充金额门槛（分）**，达到即升级 |
| `RebateRate` | **充值返利比例（%）**，如 `10` = 多返 10% |
| `Sort` / `Status` | 排序 / 启用禁用 |

> **默认等级「免费账号」**：所有账号默认无等级（`Member.LevelUUID` 为空），即「免费账号」，返利 0。
> 该默认等级**不落库、不可配置**，后台展示为「免费账号」；白银/黄金等其他等级由管理员**手动添加**，靠累充门槛自动升上去。

### 改动：`Card`

| 字段 | 说明 |
|---|---|
| `PackageUUID` | 来源套餐，仅作追溯 |
| `Price` | **售价快照（分）**，制卡时从套餐复制；消费时按此累加累充 |

### 改动：`Member`

| 字段 | 说明 |
|---|---|
| `TotalRecharge` | **累计充值金额（分）** |
| `LevelUUID` | 当前会员等级 |

## 四、关键设计：面值与售价都**快照**，不是引用

制卡时把套餐的 `Duration/Points/Price` **一并复制进 Card**，`PackageUUID` 只用于追溯来源。

理由：

1. 套餐后续改面值/改价/改名，**已售出的卡不受影响** —— 否则老卡会凭空变值、累充金额也跟着变，是事故。
2. 三处消费逻辑读的仍是 `card.Duration` / `card.Points`，**零改动**；累充读 `card.Price`。

若改成「Card 只存 PackageUUID、消费时查套餐取面值/售价」，则三处消费逻辑全要改，且历史卡会被套餐变更污染。**维持快照。**

## 五、充值返利

- 点数模式：实发 = `面值 + 面值 × RebateRate / 100`
- 时长模式：实发分钟 = `面值 + 面值 × RebateRate / 100`
- **永久卡（`-1`）不返利**（已永久，返利无意义）
- 取整：向下取整

**结算顺序**（已定：先返利再升级）：按**充值前**的等级算返利 → 累加 `TotalRecharge` → 再结算升级（新等级从**下次**充值起生效）。

**生效范围**：三个消费点（充值 / 卡密登录首次激活 / 卡密注册）统一走同一套结算函数。新号等级最低、返利 0，因此激活与注册天然无返利，无需特判。

## 六、累充自动升级

三处消费点统一调用一个结算：

```
TotalRecharge += card.Price          // 售价快照，单位分
LevelUUID = 该应用中 Threshold <= TotalRecharge 的最高等级
```

**只升不降**（累充只增不减）。旧卡 / 手动制卡 `Price=0`，累充 +0，不影响等级。

## 七、制卡改造

- `BatchCreateCards`：入参 `durationMinutes / points` → **`packageUUID`**，内部查套餐做**面值 + 售价**快照
- 制卡表单：时长/点数输入框 → **套餐下拉**（剩：套餐 + 数量 + 前缀 + 随机长度 + 备注）
- 卡密列表：加「套餐」列

## 八、改动清单

| 层 | 文件 | 改动 |
|---|---|---|
| 模型 | `models/card_package.go`（新） | `CardPackage` |
| 模型 | `models/member_level.go`（新） | `MemberLevel` |
| 模型 | `models/card.go` | +`PackageUUID` |
| 模型 | `models/member.go` | +`TotalRecharge` / +`LevelUUID` |
| 服务 | `services/card_package.go`（新） | 套餐 CRUD |
| 服务 | `services/member_level.go`（新） | 等级 CRUD + `settleMemberLevel()` 累充升级 + 返利计算 |
| 服务 | `services/card.go` | `BatchCreateCards` 改为按套餐制卡 |
| 服务 | `services/member_auth.go` | 三处消费点接入「返利 + 累充升级」结算 |
| 后台 | `controllers/admin/card_package.go`（新） | 套餐管理接口 |
| 后台 | `controllers/admin/member_level.go`（新） | 等级管理接口 |
| 后台 | `controllers/admin/card.go` | 制卡入参改 `package_uuid` |
| 后台 | `controllers/admin/member.go` | 成员列表/详情返回等级与累充 |
| 路由 | `server/admin.go` | 注册套餐 / 等级路由 |
| 前端 | 套餐管理页（新） | CRUD |
| 前端 | 等级管理页（新） | CRUD |
| 前端 | 卡密制卡表单 | 改套餐下拉；列表加「套餐」列 |
| 前端 | 成员列表/表单 | 显示等级与累充 |
| 文档 | `docs/app-settings.md` | 套餐 / 等级 / 返利说明 |
| 测试 | smoke test | 套餐制卡快照、返利计算、累充跨级升级、永久卡不返利、旧卡兼容 |

## 九、已确认的决策

1. **累充按价格累计** —— `CardPackage` 加 `Price`，`Card` 快照 `Price`，`Member.TotalRecharge` 累计金额，`MemberLevel.Threshold` 为金额门槛。金额统一用整数**分**。
2. **先返利再升级** —— 按充值前等级算返利，之后再累充结算升级，新等级下次充值生效。

## 十、已定默认（有异议再改）

- 所有账号默认为**「免费账号」**（无等级记录、返利 0），其他等级需管理员**手动添加**
- 旧卡（手动制、无套餐）：**保留原面值照常可用**，累充按 0 计
- 套餐类型必须与应用**运营模式一致**，制卡时只列匹配的套餐
- 套餐**不直接绑定等级**，只靠累充门槛升级
- 等级**只升不降**
