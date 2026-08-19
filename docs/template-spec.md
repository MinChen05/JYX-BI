# 模板规范 v1（rpt/v1）

报表模板 = 一份 YAML，定义"这张表长什么样、怎么校验、往哪写、推给谁"。
新增报表 = 在 `templates/` 加一份 YAML，前端零改动。

## 完整字段

```yaml
apiVersion: rpt/v1        # 固定
kind: Report              # 固定
metadata:
  code: my_report         # 全局唯一，URL 用
  name: 我的报表           # 显示名
  version: 1              # 改结构时 +1（旧导出文件会被拒绝导入）
spec:
  params:                 # 表单参数（URL query 传入）
    - {key: biz_date, label: 业务日期, type: month, required: true}
    # type: month(YYYY-MM) | date(YYYY-MM-DD) | text
  rows:                   # 行集
    source: sql           # sql | static
    query: "SELECT ..."   # source=sql 时执行；{param.key} 会替换（过白名单校验）
    editable_rows: false  # true=允许 Web 端增删行
    static_rows: []       # source=static 时的行
  columns:
    - {key: a, label: A列, type: money}          # 可编辑
    - {key: b, label: B列, type: text, readonly: true}  # 只读（预填/展示）
    - {key: c, label: C列, type: money, readonly: true, formula: "sum(a01..a12)"}  # 公式列
    - {key: day, type: money,                                    # 动态列
       dynamic: {expr: "days(param.biz_date)", key: "d{day:02}", label: "{biz_date:MM}-{day:02}"}}
    # type: text | money | int | date | month | auto(序号,自动)
    # 动态列 v1 仅支持 days(param.<monthKey>)，key/label 支持 {day} {day:02} {paramKey} {paramKey:MM}
  validation:
    - {cols: ["m01..m12"], rule: "v >= 0"}   # v=单元格值；可引用同行其他列
    - {cols: ["cost"], rule: "required"}
    - {rule: "unique(year, ftype)"}          # 表级唯一约束（不配 cols）
    # cols 支持: 精确 key / a..b 范围(保留补位) / prefix.* 通配
  import:
    mode: overwrite
    match_keys: [material]      # 行匹配键（缺省取前 2 个 readonly 列）
    on_unmatched: report        # report=报告忽略 | reject=拒绝 | (add 未实现)
  export:
    layout: {freeze_header: true, number_format: "#,##0"}  # 支持 #,##0 / #,##0.00
  submit:
    doris:
      table: rpt.fact_xxx
      mapping:                  # 网格列 → Doris 列；特殊源:
        material: material      #   "param.biz_date": 取参数值（月份自动转月初日期）
        "d{day}": "d{day}"      #   动态列模板（与列 key 模板同构）
    lock_after: true
  push:
    - {channel: email, to: a@b.com, on: submit}   # 带 xlsx 附件
    - {channel: dingtalk, to: group, on: submit}  # 群机器人
```

## 行为约定

1. **rowKey**：优先由 `match_keys` 的值拼接（`值1|值2`）；取不到时退化为行位置 `r0001`。新增行（editable_rows）用客户端生成的临时 key，提交时服务端按 match_keys 归一。
2. **草稿**：只存可编辑列的值；公式列/只读列每次打开时服务端重算。
3. **月份写 Doris DATE 列**：`2026-01` 自动转 `2026-01-01`。
4. **重复提交**：Doris Unique Key upsert，幂等。
5. **提交 = 全量校验通过 + MySQL 事务（状态+快照）+ Doris upsert + 异步推送**；校验失败整体拒绝。
6. **覆盖导入**：文件 `_meta` 的 模板/版本/参数 必须与当前请求完全一致；行按 match_keys 匹配；未匹配行按策略处理；确认前可反复查看 diff。
7. **导出打印检测**：`GET /selfcheck` = 导出→重解析→逐格 diff，输出保真报告。

## 新增报表 checklist

- [ ] 建 `templates/<code>.yaml`
- [ ] `go run ./cmd/admin validate-tpl -dir ../../templates` 通过
- [ ] Doris 建对应 fact 表（Unique Key 与 on_conflict 语义一致）
- [ ] 如需行集主数据，确认 Doris 里 dim 表已就位（kingdeesync 同步）
- [ ] 导出→改一格→覆盖导入 手工走一遍
