# DB-001 Migration Guardrails

Status: Done
Owner: coding-agent
Module: database
Related:
- migrations/
- migrations/AGENTS.md
- tasks/auth/AU-003-admin-bootstrap.md

## Background

风险 R1 的根因不只是"少了管理员种子"，而是**已应用迁移可以被悄悄重写**：

- `202607130005_create_auth_tables.up.sql` 在 `a3fcea1` 中被删除了一段种子 INSERT，
  文件仍标记 `applied`，没有任何机制发现这次改写。
- `202608030007_simplify_inventory_reservation_status` 的 `-- status:` 元数据仍写 `pending`，
  但该迁移早已在数据库中应用；元数据与事实不符，会误导下一个改 schema 的人。

`migrations/AGENTS.md` 原文只有一句"已经 applied 的迁移不得重写"，既没有说明违反的后果，
也没有登记任何例外渠道。

## Goal

- 迁移元数据与数据库实际状态一致。
- 迁移规范中把"已 applied 迁移不可变"写成可执行的硬性约束，并说明违反后果与审查红线。
- 明确登记早期开发阶段允许的**唯一例外**及其影响面。

## Non-Goals

- 不新增或改写任何 DDL / DML。
- 不引入 CI 校验脚本（用户本次明确选择"只做规范文档 + 元数据修正"；
  机器拦截可作为后续任务，届时需解决浅克隆下的 base 引用问题）。
- 不重构迁移目录结构或文件命名。

## Scope

允许修改：

- `migrations/202608030007_simplify_inventory_reservation_status.up.sql`
- `migrations/202608030007_simplify_inventory_reservation_status.down.sql`
- `migrations/AGENTS.md`

不应修改：

- 其它迁移文件（`202607130005` 的种子恢复由 `AU-003` 单独提交并单独登记例外）
- `sql/`、`internal/`、`pkg/`

## Domain Rules

- `-- status:` 只能是 `pending` 或 `applied`，且必须与事实一致：只要在任一目标库执行过，就必须写 `applied`。
- 已 `applied` 的迁移文件不可修改、删除、重命名，也不得把语句挪到别的迁移文件。
- 任何结构变更或数据修复必须新增迁移文件。
- 例外只能由用户显式批准，并且必须写进 `migrations/AGENTS.md` 与对应任务文档。

## Acceptance Criteria

- [x] `202608030007` 的 up / down 均为 `-- status: applied`，目录内 8 个迁移文件无 `pending`
- [x] `migrations/AGENTS.md` 含「迁移不可变性（硬性约束）」一节，包含违反后果与代码审查红线
- [x] `migrations/AGENTS.md` 登记了 `202607130005` 这一个例外及其影响（旧库不会补种）
- [x] 未改动任何 DDL / DML 语句
- [x] `make migrate-version` 行为不变（元数据只是注释，无需数据库即可确认无语句变化）

## 验证记录（2026-09-16）

```bash
cd stock-flow
grep -rn "^-- status:" migrations/*.sql    # 全部为 applied
git diff --stat migrations/202608030007_*  # 仅第 2 行注释变化
```

- 需要数据库的 `make migrate-version` / 新库 `make migrate-up` 本地无法执行（无 PostgreSQL），
  由 CI 的集成测试 job 覆盖（该 job 会对空库执行全量迁移）。
