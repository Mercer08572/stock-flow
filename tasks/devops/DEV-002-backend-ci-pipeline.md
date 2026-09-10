# DEV-002 后端接入 GitHub Actions CI

Status: Ready
Owner: coding-agent
Module: devops
Related:
- `.github/workflows/ci.yml`（新增）
- `go.mod`
- `Makefile`

## Background

前端项目 `../stock-flow-admin/` 已有 `.github/workflows/ci.yml`（`format:check`、
`lint`、`typecheck`、`test:run`、`build`、`test:e2e`），但后端 `stock-flow/`
**完全没有 CI**：没有任何 `.github/` 目录。

后果是后端质量门禁只能靠人工记忆执行。实际证据：

- 迁移 `202607130005` 的 admin seed 曾被静默删除（缺陷 R1），迁移元数据
  `202608030007` 的状态字段与实际不符（P1-2），这些都属于**提交时无人拦截**的错误。
- 后端目前有 78 个单元测试且全部通过，但没有任何自动机制保证它们在未来提交中仍然通过。

## Goal

在 `stock-flow/` 建立最小可用的 CI：对 `push` 到 `main` 与所有 `pull_request`
自动执行格式化检查、静态检查、构建与单元测试。

任务完成后：

- 每个 PR 与每次 push 到 `main` 都会触发后端 CI。
- 首次运行必须全绿（不允许提交一个红着的门禁）。
- Go 构建与模块缓存由 `actions/setup-go` 内置缓存管理，不手工折腾缓存路径。

## Non-Goals

- **不接入数据库集成测试**（P3-1 `DB-002` 才建立 `-tags=integration` 基础设施，
  届时再新增独立 job）。
- 不接入 `sqlc` / `swagger` 生成物一致性校验（属 P1-2 / P1-4）。
- 不加部署、发布、镜像构建步骤。
- 不修改 `Makefile` 或任何 Go 源码。
- 不改动前端 CI。

## Scope

允许修改：

- `.github/workflows/ci.yml`（新增）

不应修改：

- 任何 `.go` 文件
- `Makefile`
- `migrations/`
- 前端项目 `../stock-flow-admin/`

## Domain Rules

- CI 检查项必须与本地验证命令一致，避免「本地过、CI 挂」。
- 使用 `go-version-file: go.mod` 读取 Go 版本，避免版本在 CI 与 `go.mod` 之间漂移。
- 权限最小化：仅需 `contents: read`。
- 失败必须给出可定位的信息（gofmt 失败时打印具体文件列表）。

## Implementation Notes

检查项（与开发计划第七章验证命令模板对齐）：

| 步骤 | 命令 | 作用 |
| --- | --- | --- |
| 格式化 | `gofmt -l .` 非空则失败 | 捕获未格式化文件 |
| 静态检查 | `go vet ./...` | 捕获可疑代码 |
| 构建 | `go build ./...` | 捕获编译错误 |
| 单元测试 | `go test ./...` | 捕获逻辑回归 |

要点：

- `go test ./...` 当前**不依赖数据库**（78 个测试全部基于 fake/mock），
  因此不需要 service 容器或 secret，这也是本任务能保持零成本的原因。
- 触发器与前端保持一致：`pull_request`（全部）+ `push` 到 `main`。
- 加 `timeout-minutes` 防止挂死消耗额度。

## Acceptance Criteria

- [ ] `.github/workflows/ci.yml` 存在，且 YAML 合法。
- [ ] 触发条件覆盖 `pull_request` 与 `push` 到 `main`。
- [ ] 包含 `gofmt` / `go vet` / `go build` / `go test` 四项检查。
- [ ] 使用 `actions/setup-go` 且启用内置缓存。
- [ ] 本地预演等价命令全部通过：`gofmt -l .` 无输出、`go vet ./...`、
      `go build ./...`、`go test ./...` 均成功。
- [ ] 推送后 GitHub Actions 首次运行全绿。

## Open Questions

1. 仓库为 **public**，GitHub Actions 对 public 仓库的标准 runner 免费且不计入
   每月分钟数，因此该 CI 不产生费用（详见交付说明）。若仓库将来转为 private，
   需复核额度。
2. 是否把后端 CI 设为 `main` 分支的 required status check（分支保护规则）
   属于仓库设置，需用户在 GitHub 界面操作，本任务不处理。
