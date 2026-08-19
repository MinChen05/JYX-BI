# AGENTS.md — AI 代理工作约定

## 构建与验证

- 后端：`cd apps/server && go build ./... && go test ./...`（Go 1.25.7，GOPROXY=goproxy.cn）
- 模板校验：`go run ./cmd/admin validate-tpl -dir ../../templates`（改模板后必跑）
- 前端：`cd apps/web && npm run tsc`（类型检查）；npm 用 .npmrc 里的国内镜像
- 改动模板 YAML 后，必须同时验证：动态列展开、公式、校验规则、doris mapping 四者一致

## 关键约束（不要违反）

1. **Doris 只在提交时写**（`service.Submit → writeDoris`）；草稿/导入只写 MySQL 系统库
2. **导入两阶段**：`ImportFile`（校验+diff，不落业务数据）→ `ConfirmImport`（事务落地）。任何新导入路径必须保持这个结构
3. **导出文件必须带 `_meta` sheet**（pkg/xlsxio），导入时校验模板/版本/参数一致性
4. **公式列与 readonly 列的值永不接受客户端传入**（NormalizePayload 丢弃）
5. 模板 YAML 里 doris mapping 的动态列写法必须是 `"d{day}": "d{day}"`（与列 key 模板同构）
6. 参数进入 SQL 前必须过 `engine.ValidateParams` 白名单格式校验

## 代码组织

- 模板相关（schema/parser/dynamic/formula/validate/engine）全部在 `internal/template`，可独立单测，不依赖 DB
- 网格契约类型在 `internal/engine/grid.go`（GridSpec/RowSpec），与 `packages/rpt-types/src/index.ts` 保持字段一致
- 新增 API：`internal/httpapi/handler.go` 加 handler + `router.go` 挂路由

## 前端坑

- config.ts 必须保留 `initialState: {}`：initial-state 插件是 `enableBy: config`，漏掉这个 key 时 app.tsx 的 `getInitialState` 运行时注册失败（"invalid key getInitialState"），整页白屏
- 改了前端依赖/配置后先 `npx max setup` 重新生成 src/.umi 再跑 tsc
- dev 端口若被占（8001 在 linux10 被容器占用），用 `PORT=8002 npx max dev` 覆盖；新端口要加进 firewalld（`firewall-cmd --permanent --add-port`）Mac 才访问得到

## 提交习惯

- 中文 commit message，conventional 前缀（feat/fix/docs/refactor）
- 不要把 config.ini（含密码）提交进仓库
