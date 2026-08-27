# 壁画清洗许可门

`mural-conservation-gate` 是面向历史建筑保护现场的壁画清洗试验治理系统。它把病害基线、互斥试验/对照分区、不可变清洗方案修订、多轮观察证据、确定性风险阻断、复核冻结和施工许可串成一条可审计链路，避免未验证方案直接扩大到正式作业面。

浏览器工作台由 Go 服务直接提供，所有写操作通过同源 JSON API 完成，并要求 `Idempotency-Key` 与 `expectedVersion`。业务数据存入本地 SQLite，基线复测、方案修订、试验轮次、冻结清单与许可均保留不可变版本；许可可根据冻结方案与逐条观察证据摘要独立验真。

工作台首页提供稳定游标分页的案卷队列，可按状态、建筑或遗址名称、壁画空间位置检索，并显示当前基线、流程就绪度、开放阻断和许可有效期。选择案卷后仍进入同一详情、阶段表单与审计时间线。

## 构建与测试

```text
go build ./cmd/muralgate
go test ./...
```

## 运行

默认仅监听高位回环地址：

```text
go run ./cmd/muralgate
```

显式指定监听地址与数据库：

```text
go run ./cmd/muralgate -addr=127.0.0.1:19081 -db=muralgate.db
```

也可设置 `PORT`（仅端口号），服务会绑定 `127.0.0.1:<PORT>`。出于现场数据安全考虑，`-addr` 拒绝非回环 IP。启动后访问 `http://127.0.0.1:19081/`。

## 完整自检

以下命令使用内存 SQLite，启动真实回环 HTTP 监听，调用完整业务 API、验真许可、检查审计时间线，然后在限定时间内退出：

```text
go run ./cmd/muralgate -selfcheck -addr=127.0.0.1:19081 -timeout=15s
```

## 主要业务顺序

先建立案卷并提交完整基线；随后先划定对照区，再划定与其不重叠的试验区。保存候选方案修订后，使用成对批量录入在一次事务中提交同轮对照区与关联试验区观察。系统依据方案阈值、当前基线、趋势和对照差异生成阻断项。

现场条件变化时可追加更晚且确有变化的基线复测，原评估转为可查询的历史结论，必须在新基线下重新取得成对证据。若出现阻断，先登记责任人、参数调整目标、指定复验分区和期限；只有新方案按计划修改且目标规则在指定分区成对复验通过，系统才自动销项。复核员横向比较各候选最新修订并指定唯一入围方案后，方可冻结。项目负责人再签发具有范围、限制、有效期和校验摘要的许可；验真回执逐项核对许可字段、冻结清单、方案材料参数和每条观察证据，并可从工作台下载 JSON。

## 主要扩展接口

- `GET /api/cases`：案卷队列，支持 `status`、`keyword`、`cursor` 和 `limit`。
- `POST /api/cases/{caseID}/baseline`：首次基线或带 `reason` 的复测版本。
- `POST /api/cases/{caseID}/observation-batches`：对照区与一个或多个关联试验区的原子成对观察。
- `GET /api/cases/{caseID}/candidates` 与 `POST /api/cases/{caseID}/candidate-selection`：候选横向比较和唯一入围。
- `POST /api/cases/{caseID}/remediations` 与 `GET /api/remediations`：风险整改计划及按责任人、逾期、严重度检索。
- `GET /api/permits/{permitNumber}/verify`：带逐项校验结果和回执摘要的只读许可验真。
