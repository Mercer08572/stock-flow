# Stock-Flow

Stock-Flow 是一个面向库存管理场景的后端 API 服务。项目采用 Go、Gin、PostgreSQL、pgx、sqlc 和 golang-migrate 构建，整体架构是模块化单体，业务代码遵循 DDD Lite 与 Clean Architecture 的分层约束。

这个仓库只包含后端服务。前端可以使用 Vue 3、TypeScript、Naive UI、AG Grid、Pinia 和 Vue Router 独立实现。

## 当前能力

已实现的主数据能力：

- 计量单位管理：`/api/v1/units`
- 物料分类管理：`/api/v1/material-categories`
- 物料管理：`/api/v1/materials`
- SKU 管理：`/api/v1/skus`
- 仓库管理：`/api/v1/warehouses`
- 健康检查：`/api/v1/health`

规划中的库存能力：

- 库存余额
- 批次库存
- 库存预留
- 库存流水
- 库存操作幂等
- FIFO 分配

## 技术栈

- Go 1.22+
- Gin
- PostgreSQL
- pgx
- sqlc
- golang-migrate
- Makefile

## 快速开始

### 1. 准备依赖

本地需要安装：

- Go 1.22 或更高版本
- PostgreSQL
- golang-migrate
- PostgreSQL 客户端工具，只有执行 `make schema-dump` 时才需要 `pg_dump`

`sqlc` 通过 Makefile 使用 `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate` 执行，不需要提前全局安装。

### 2. 准备配置

复制一份开发环境配置：

```bash
cp configs/development.example.yaml configs/development.yaml
```

按本机 PostgreSQL 修改 `database_url`：

```yaml
app_env: development
gin_mode: debug
http_addr: ":8080"
database_url: "postgres://postgres:postgres@localhost:5432/stock_flow_dev?sslmode=disable"
shutdown_timeout: "10s"
```

配置加载优先级：

```text
默认值 < 配置文件 < 环境变量
```

常用环境变量：

```text
APP_ENV
CONFIG_FILE
GIN_MODE
HTTP_ADDR
PORT
DATABASE_URL
SHUTDOWN_TIMEOUT
```

### 3. 创建数据库并执行迁移

先创建本地数据库，例如：

```bash
createdb stock_flow_dev
```

执行迁移：

```bash
make migrate-up
```

查看迁移版本：

```bash
make migrate-version
```

### 4. 启动服务

```bash
make run
```

默认监听地址是 `:8080`。启动后可以检查服务状态：

```bash
curl http://localhost:8080/api/v1/health
```

返回格式示例：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "status": "ok",
    "service": "stock-flow"
  },
  "trace_id": "req_xxx",
  "timestamp": 1672531200000
}
```

## 常用命令

```bash
make help              # 查看可用命令
make fmt               # 格式化 Go 代码
make test              # 运行测试
make run               # 启动 API 服务
make sqlc              # 生成 sqlc 代码
make migrate-up        # 执行数据库迁移
make migrate-down      # 回滚最后一个迁移
make migrate-version   # 查看当前迁移版本
make schema-dump       # 导出当前数据库 schema 给 sqlc 使用
```

如果当前环境无法写入 Go 默认构建缓存，可以指定本地缓存目录：

```bash
GOCACHE=/private/tmp/stock-flow-go-build-cache make test
```

## API 约定

所有 API 都以 `/api/v1` 为基础路径，资源名使用复数名词。常规资源遵循：

- `GET /resources`：列表
- `GET /resources/:id`：详情
- `POST /resources`：创建
- `PUT /resources/:id`：更新
- `DELETE /resources/:id`：软删除

当前资源路径：

| 模块 | 路径 | 说明 |
| --- | --- | --- |
| health | `/api/v1/health` | 健康检查 |
| unit | `/api/v1/units` | 计量单位 |
| material category | `/api/v1/material-categories` | 物料分类 |
| material | `/api/v1/materials` | 物料 |
| sku | `/api/v1/skus` | SKU |
| warehouse | `/api/v1/warehouses` | 仓库 |
| warehouse | `/api/v1/warehouses/:id/disable` | 禁用仓库 |

所有响应都通过 `pkg/response` 返回统一结构：

```json
{
  "code": 200,
  "message": "success",
  "data": {},
  "trace_id": "req_abc123xyz",
  "timestamp": 1672531200000
}
```

常用业务响应码：

| code | 含义 |
| --- | --- |
| 200 | 成功 |
| 1001 | 请求参数错误 |
| 1004 | 资源不存在 |
| 1009 | 资源冲突 |
| 1500 | 服务内部错误 |

## 架构说明

Stock-Flow 是模块化单体。每个业务模块拥有自己的模型、服务、仓储和 sqlc 生成代码。

核心分层必须保持：

```text
HTTP Request -> Handler -> Service -> Repository -> PostgreSQL
```

主要原则：

- 业务逻辑放在 Service 或领域对象中。
- Repository 只负责持久化，不承载业务规则。
- 事务由应用服务层管理。
- 不使用 ORM。
- 每一层优先定义接口，再写实现。
- 依赖通过 `NewXxx(dep)` 构造函数注入。
- 禁止用全局变量传递依赖。
- 跨模块访问必须通过应用服务或稳定契约，不能直接访问其他模块的 repository。

模块边界关系：

```text
material -> sku -> inventory(warehouse, batch)
```

说明：

- `material` 负责物料主数据、分类和基础单位。
- `sku` 负责库存可管理的具体 SKU 定义。
- `warehouse` 负责仓库主数据，不负责库存数量。
- `inventory` 负责库存余额、批次、预留和库存流水。

更多设计文档见：

- `docs/development.md`
- `docs/architecture/module-boundaries.md`
- `docs/architecture/dependency-rules.md`
- `docs/architecture/transaction-rules.md`
- `docs/database/schema-design.md`

## 数据库与 sqlc 工作流

项目中有两类 schema 文件：

```text
migrations/            # 数据库变更历史，用于真实数据库升级和回滚
sql/schema/schema.sql  # 当前数据库结构快照，用于 sqlc 代码生成
```

当表结构变化时，推荐流程是：

```bash
make migrate-up
make schema-dump
make sqlc
make test
```

新增迁移时请遵守：

- 文件名格式：`<12位编号>_<description>.up.sql` 和 `<12位编号>_<description>.down.sql`
- 主键使用 `BIGSERIAL`
- 外键使用 `BIGINT`
- 文本使用 `TEXT`
- 金额、库存数量和换算系数使用 `NUMERIC`
- 时间使用 `TIMESTAMPTZ`
- 枚举值使用 `TEXT + CHECK`，不使用 PostgreSQL ENUM
- 软删除字段使用 `deleted_at TIMESTAMPTZ NULL`

## 目录结构

```text
stock-flow/
├── cmd/
│   ├── api/                 # API 服务入口
│   └── config/              # 配置辅助命令
├── configs/                 # 配置模板
├── docs/                    # 架构、开发和数据库文档
├── internal/
│   ├── material/            # 物料、分类、单位模块
│   ├── sku/                 # SKU 模块
│   ├── inventory/           # 库存模块文档与后续实现位置
│   ├── warehouse/           # 仓库模块
│   └── shared/              # 配置、数据库、HTTP、健康检查等共享能力
├── migrations/              # 数据库迁移
├── pkg/
│   └── response/            # 统一响应结构
├── sql/
│   ├── queries/             # sqlc 查询
│   └── schema/              # sqlc schema 快照
└── tasks/                   # 面向 coding agent 的任务规格
```

## 贡献指南

在实现新功能前，请先阅读：

1. `AGENTS.md`
2. 相关模块下的 `AGENTS.md`
3. 相关 `tasks/` 文档
4. 相关源码、migration、sqlc query 和测试

提交代码前建议执行：

```bash
make fmt
make test
```

如果修改了数据库结构，请额外执行：

```bash
make migrate-up
make schema-dump
make sqlc
make test
```

## License

This project is licensed under the Apache License 2.0. See `LICENSE` for details.
