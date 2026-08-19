# kingdee-rpt 金蝶报表平台

金蝶 ERP 数据的**填报 + 复杂报表**平台：配置驱动的通用填报引擎，覆盖"纯内容填报、预填部分编辑、导出→本地编辑→覆盖导入、定时推送"四类场景。

## 定位（三层分工）

| 层 | 项目 | 职责 |
|---|---|---|
| 数据管道 | [kingdeesync](https://github.com/MinChen05/kingdeesync) | 金蝶 → Doris 同步 |
| 看（分析） | DataEase | 仪表板 / 大屏 / 自助分析 / 移动端 |
| 出 + 录（本项目） | kingdee-rpt | 复杂报表、填报、覆盖导入、定时推送 |

三者共用同一个 Doris 集群，口径一致。

## 架构

```
web (React + AntD Pro + Handsontable)
  └─ REST ─► server (Go + Gin + GORM)
                 ├─ template/   模板引擎：YAML 解析 / 动态列 / 公式(expr) / 校验规则
                 ├─ engine/     网格装配：行集(预填) × 列(动态) × 草稿值
                 ├─ store/      MySQL(状态/审计) + Doris(Unique Key upsert)
                 ├─ service/    草稿/校验/提交/撤回/导入/导出/自检
                 └─ push/       邮件(附件) / 钉钉(加签)
templates/*.yaml   报表模板 = 代码版本化的"设计器"
```

核心原则：
- **模板即配置**：新增报表 = 加一份 YAML，前端零改动
- **Doris 只在提交时写**：草稿/导入只动 MySQL 系统库，单一事实来源
- **导入两阶段**：上传→校验+diff（不落库）→确认→事务落地
- **文件自带身份**：导出 xlsx 内嵌隐藏 `_meta` sheet（模板/版本/参数），防旧文件覆盖新结构

## 快速开始

```bash
# 1. 配置
cp packages/rpt-config/config.example.ini apps/server/config.ini  # 填 DSN/SMTP/钉钉

# 2. 建库（首次）
mysql  < apps/server/internal/store/sql/001_mysql.sql
mysql -h <doris-fe> -P 9030 < apps/server/internal/store/sql/002_doris.sql

# 3. 校验模板
cd apps/server && go run ./cmd/admin validate-tpl -dir ../../templates

# 4. 起服务
go run ./cmd/server -config config.ini        # :8090

# 5. 前端（开发）
cd apps/web && npm install && npm run dev     # :8001, /api 代理到 :8090
```

## 开发

```bash
make test      # Go 单测（模板引擎/公式/校验/xlsx 往返）
make dev       # 起 server
make build     # 前端构建 + Go 单二进制（embed web/dist）
make docker    # 构建镜像
```

模板规范见 [docs/template-spec.md](docs/template-spec.md)。

## 目录

```
apps/server/    Go 后端（cmd/server + cmd/admin + internal/*）
apps/web/       React 前端（AntD Pro 6 + Handsontable）
packages/rpt-types/   前后端共享 TS 类型
packages/rpt-config/  配置样例
templates/      报表模板 YAML（新增报表在这里加文件）
deploy/         Dockerfile / nginx 示例
```
