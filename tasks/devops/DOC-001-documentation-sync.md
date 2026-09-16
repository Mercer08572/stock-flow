# DOC-001 Documentation Sync

Status: Done
Owner: coding-agent
Module: devops
Related:
- README.md
- .env.example
- openapi/swagger.json
- cmd/api/main.go
- tasks/warehouse/WH-002-data-consistency-risk.md
- ../stock-flow-admin/README.md
- ../stock-flow-admin/.env.example
- ../stock-flow-admin/vite.config.ts

## Background

计划文档 P1-3 与 M1 要求「文档描述与 `go test`、实际路由、`.env` 三者一致」，但核查发现：

- 后端 README 把**库存余额**列为"规划中"，而 `/api/v1/inventory/stocks` 三个只读接口早已实现并有 handler 测试；
- 认证模块（`/api/v1/auth/**`）、物料单位换算、库存路径都未出现在 README 路径表；
- 目录树缺 `cmd/admin/`，也没有任何"首次部署如何创建管理员"的说明（这正是 R1 的部署侧成因）；
- 前端 README 与 `.env.example` 都写代理到 `http://localhost:8080`，而 `../stock-flow/.env.example`
  提供的是 `HTTP_ADDR=:8181`，前端仓库自身的 `.env` 也已经改成 8181——照文档跑必然连不上；
- `openapi/swagger.json` **完全没有 `securityDefinitions`**，Swagger UI 无法表达 47 个受保护
  operation 的鉴权方式；
- `WH-002` 的 Background 仍描述"删除时没有检查库存引用"，实际上该保护已由 `ac6a0d3` 实现。

## Goal

让新人和 agent 只读文档就能跑通「建库 → 迁移 → 创建管理员 → 登录」，并让文档中的端口、
路由、鉴权方式与代码/配置保持一致。

## Non-Goals

- 不改动任何运行时业务逻辑。
- 不手工编辑生成物（`openapi/*` 由 `make swagger` 产出）。
- 不为 47 个 operation 逐个补 `@Security` 注解（已有 `securityDefinitions` 后 UI 可表达鉴权方式，
  逐条注解留待后续需要时再做）。
- 不新建 WH-003；WH-002 剩余的唯一验收项（数据库集成测试）移交计划文档 P3-7。

## Scope

允许修改：

- `README.md`、`.env.example`（如端口说明需要）
- `cmd/api/main.go`（仅注释块）、`openapi/docs.go`、`openapi/swagger.json`、`openapi/swagger.yaml`
- `tasks/warehouse/WH-002-data-consistency-risk.md`
- `../stock-flow-admin/README.md`、`../stock-flow-admin/.env.example`、`../stock-flow-admin/vite.config.ts`
  （**跨项目变更**：本任务显式声明同时涉及前后端文档与前端代理默认值）

不应修改：

- 任何 handler / service / repository、路由表、响应格式
- 迁移文件

## Domain Rules

- 部署文档必须与迁移实际行为一致：引导管理员由迁移创建，口令由 `cmd/admin init` 写入，命令不可重复执行。
- 端口只有一处"事实来源"：后端 `.env.example`（`:8181`）与前端 `VITE_API_PROXY_TARGET`。
  代码内置默认 `:8080`（`internal/shared/config`）必须被如实说明，不能被描述成"默认端口"。
- 环境文件加载顺序按 Vite 原生规则描述，不发明新规则。
- Swagger 由注解生成；安全定义写在 `cmd/api/main.go` 的通用注解块中。

## Acceptance Criteria

- [x] 后端 README「当前能力」含认证、库存余额（只读）、单位换算、仓库停用，并说明库存写操作仍未实现
- [x] 后端 README 路径表覆盖 `/api/v1/auth/**`、`/api/v1/inventory/stocks*`、
      `/api/v1/materials/:id/unit-conversions`、`/api/v1/warehouses/:id/disable`
- [x] 后端 README 目录树含 `cmd/admin/`，并有「初始化管理员」章节（命令、环境变量、重复执行行为、失败含义）
- [x] 后端 README 中所有 `:8080` 的表述改为"内置默认"并给出 `.env.example` 的 `:8181`
- [x] `openapi/swagger.json` 含 `securityDefinitions.AdminSession` 与 `securityDefinitions.APIAppCredentials`
- [x] WH-002 状态为 `Done`，Background 注明删除保护已实现，剩余验收项移交 P3-7
- [x] 前端 README / `.env.example` / `vite.config.ts` 的代理目标统一为 `http://localhost:8181`，
      并说明 `.env` 与 `.env.local` 的加载关系
- [x] 前端 `pnpm check` 通过

## 验证记录（2026-09-16）

```bash
cd stock-flow
make swagger && python3 -c "import json;print(list(json.load(open('openapi/swagger.json'))['securityDefinitions']))"
# ['APIAppCredentials', 'AdminSession']
grep -rn "8080" README.md     # 仅剩「内置默认」这一处说明
cd ../stock-flow-admin
grep -rn "8080" README.md .env.example vite.config.ts   # 无输出
pnpm --config.store-dir=/tmp/agent-cache/pnpm-store run check
```

## Open Questions

- 47 个受保护 operation 目前只有全局 securityDefinitions，没有逐条 `@Security`；
  Swagger UI 的 "Authorize" 与 Try it out 已可用，但单条 operation 的鉴权要求不可区分。
- 后端内置默认端口 `:8080` 与 `.env.example` 的 `:8181` 不一致属于历史遗留；
  本次以文档说明对齐，未经确认不擅自改动代码默认值。
