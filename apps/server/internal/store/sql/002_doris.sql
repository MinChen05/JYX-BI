-- JYX-BI 事实库（Doris 2.x，库名与 kingdeesync 的 Doris 集群一致，可单独建 rpt 库）
-- Unique Key 模型：重复提交 = 按 key 覆盖。
CREATE DATABASE IF NOT EXISTS rpt;

-- 销售年度预测
CREATE TABLE IF NOT EXISTS rpt.fact_forecast_year (
  year       INT            NOT NULL,
  ftype      VARCHAR(32)    NOT NULL,
  m01        DECIMAL(16,2)  NULL,
  m02        DECIMAL(16,2)  NULL,
  m03        DECIMAL(16,2)  NULL,
  m04        DECIMAL(16,2)  NULL,
  m05        DECIMAL(16,2)  NULL,
  m06        DECIMAL(16,2)  NULL,
  m07        DECIMAL(16,2)  NULL,
  m08        DECIMAL(16,2)  NULL,
  m09        DECIMAL(16,2)  NULL,
  m10        DECIMAL(16,2)  NULL,
  m11        DECIMAL(16,2)  NULL,
  m12        DECIMAL(16,2)  NULL,
  total      DECIMAL(16,2)  NULL,
  updated_at DATETIME       NULL
) UNIQUE KEY(year, ftype)
  DISTRIBUTED BY HASH(year) BUCKETS 1
  PROPERTIES ("replication_num" = "1");

-- 大宗材料价格（列固定 31 天，非当月天数为 NULL）
CREATE TABLE IF NOT EXISTS rpt.fact_material_price (
  biz_date   DATE           NOT NULL,
  material   VARCHAR(64)    NOT NULL,
  category   VARCHAR(32)    NULL,
  avg_price  DECIMAL(14,2)  NULL,
  d01        DECIMAL(14,2)  NULL, d02 DECIMAL(14,2) NULL, d03 DECIMAL(14,2) NULL,
  d04        DECIMAL(14,2)  NULL, d05 DECIMAL(14,2) NULL, d06 DECIMAL(14,2) NULL,
  d07        DECIMAL(14,2)  NULL, d08 DECIMAL(14,2) NULL, d09 DECIMAL(14,2) NULL,
  d10        DECIMAL(14,2)  NULL, d11 DECIMAL(14,2) NULL, d12 DECIMAL(14,2) NULL,
  d13        DECIMAL(14,2)  NULL, d14 DECIMAL(14,2) NULL, d15 DECIMAL(14,2) NULL,
  d16        DECIMAL(14,2)  NULL, d17 DECIMAL(14,2) NULL, d18 DECIMAL(14,2) NULL,
  d19        DECIMAL(14,2)  NULL, d20 DECIMAL(14,2) NULL, d21 DECIMAL(14,2) NULL,
  d22        DECIMAL(14,2)  NULL, d23 DECIMAL(14,2) NULL, d24 DECIMAL(14,2) NULL,
  d25        DECIMAL(14,2)  NULL, d26 DECIMAL(14,2) NULL, d27 DECIMAL(14,2) NULL,
  d28        DECIMAL(14,2)  NULL, d29 DECIMAL(14,2) NULL, d30 DECIMAL(14,2) NULL,
  d31        DECIMAL(14,2)  NULL,
  updated_at DATETIME       NULL
) UNIQUE KEY(biz_date, material)
  DISTRIBUTED BY HASH(biz_date) BUCKETS 4
  PROPERTIES ("replication_num" = "1");

-- 利润分析成本
CREATE TABLE IF NOT EXISTS rpt.fact_profit_cost (
  biz_date   DATE           NOT NULL,
  cust_no    VARCHAR(32)    NOT NULL,
  cust_name  VARCHAR(128)   NULL,
  salesman   VARCHAR(64)    NULL,
  category   VARCHAR(32)    NULL,
  cost       DECIMAL(16,2)  NULL,
  updated_at DATETIME       NULL
) UNIQUE KEY(biz_date, cust_no)
  DISTRIBUTED BY HASH(biz_date) BUCKETS 4
  PROPERTIES ("replication_num" = "1");

-- 行集主数据（dim_material / dim_customer 由 kingdeesync 或手工同步到同集群）
