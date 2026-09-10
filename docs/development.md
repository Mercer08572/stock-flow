# 开发指南

本文档记录 Stock-Flow 的本地开发工作流。

## 环境要求

- Go 1.25 或更高版本
- PostgreSQL
- golang-migrate
- PostgreSQL 客户端工具，仅在运行 `make schema-dump` 时需要

## 命令

使用项目 Makefile 作为主要命令入口。

```bash
make help
make fmt
make test
make run
make sqlc
```

## 运行时配置

配置只有一个来源：环境变量。API 按以下优先级加载：

```text
export 导出的环境变量  >  .env 文件  >  内置默认值
```

`.env` 位于进程工作目录，使用 `godotenv` 读取：

```bash
cp .env.example .env
go run ./cmd/api
```

优先级规则：

- 导出的变量只要**非空**就一定生效，`.env` 只填补未导出的项。
- 导出为空串（例如 `export DATABASE_URL=`）视为未设置，`.env` 仍会生效。
- `.env` 缺失**不是错误**：生产与 CI 直接注入环境变量即可。
- `ENV_FILE` 显式指定的文件缺失**是错误**，不会被静默忽略。
- `.env` 存在但内容非法**是错误**，不会退回默认值继续启动。
- `.env` 按**进程工作目录**查找；不从项目根目录启动时用 `ENV_FILE` 指定路径：

```bash
ENV_FILE=/etc/stock-flow/.env go run ./cmd/api
```

导出环境变量的示例：

```bash
APP_ENV=production DATABASE_URL=postgres://... ./stock-flow
```

支持的配置项及其环境变量：

```text
APP_ENV               # 默认 development；production 时强制要求 cookie secure
ENV_FILE              # 可选，dotenv 文件路径，默认 ./.env
GIN_MODE              # debug | release | test，默认 debug
HTTP_ADDR             # 默认 :8080
PORT                  # HTTP_ADDR 的别名，仅当 HTTP_ADDR 未设置时生效
DATABASE_URL          # 数据库连接串，必需
SHUTDOWN_TIMEOUT      # 默认 10s
AUTH_ADMIN_SESSION_TTL
AUTH_ADMIN_COOKIE_SAME_SITE
AUTH_ADMIN_COOKIE_SECURE
AUTH_LOGIN_FAILURE_MAX_ATTEMPTS
AUTH_LOGIN_FAILURE_WINDOW
AUTH_LOGIN_LOCKOUT
AUTH_LOGIN_IP_MAX_ATTEMPTS
```

> `HTTP_ADDR` 优先于 `PORT`：只要 `HTTP_ADDR` 有值（无论来自 `.env` 还是导出），
> `PORT` 就会被忽略。

`.env` 与 `.env.local` 这类文件都被 git 忽略，只有 `.env.example` 会入库。

可以查看当前生效值的来源：

```bash
go run ./cmd/config -key env_file       # 实际加载的 dotenv 文件，未加载则为空
go run ./cmd/config -key database_url
go run ./cmd/config -key http_addr
```

数据库迁移命令使用与 API 相同的 Go 配置加载器解析 `database_url`，因此
`.env` 与 `DATABASE_URL` 也能一致地用于 Makefile 迁移命令。

```bash
make migrate-up
make migrate-down
make migrate-version
```

可以使用以下命令检查解析后的迁移数据库 URL：

```bash
make config-database-url
```

## 数据库模式来源

项目保留两种用途不同的数据库模式产物。

`migrations/` 是数据库变更的历史日志，用于升级或回滚实际数据库。

`sql/schema/schema.sql` 是供 sqlc 生成代码使用的当前模式快照。它应描述应用所有预期迁移后的数据库。

简而言之：

```text
migrations = 历史记录
sql/schema/schema.sql = 用于生成代码的当前结构
```

## 更新模式和 sqlc 代码

数据库表发生变更时：

1. 在 `migrations/` 下添加新迁移。
2. 将迁移应用到开发数据库。
3. 刷新 `sql/schema/schema.sql`。
4. 更新 `sql/queries/` 下的查询文件。
5. 重新生成 sqlc 代码。
6. 运行测试。

通常的流程为：

```bash
make migrate-up
make schema-dump
make sqlc
make test
```

## 导出当前 PostgreSQL 模式

PostgreSQL 提供 `pg_dump --schema-only`，用于导出不含表数据的数据库结构。

可选的 Makefile 目标为：

```bash
make schema-dump
```

该目标会调用 `pg_dump`，因此需要 PostgreSQL 客户端工具。

它将当前数据库模式写入：

```text
sql/schema/schema.sql
```

该命令使用以下选项：

```bash
pg_dump --schema-only --no-owner --no-privileges --no-comments --schema=public
```

注意事项：

- 必须通过 PostgreSQL 客户端工具在本地安装 `pg_dump`。
- 需要时，使用 `PG_DUMP=/path/to/pg_dump make schema-dump` 覆盖二进制文件路径。
- 导出前，确保目标数据库已经迁移到预期版本。
- Makefile 会过滤 `\restrict` 等 psql 元命令行，因为 sqlc 需要常规 SQL 输入。
- 导出后始终检查 `sql/schema/schema.sql` 的差异。

## sqlc 布局

sqlc 读取：

```text
sqlc.yaml
sql/schema/schema.sql
sql/queries/*.sql
```

生成的代码放在各模块的 `db` 子包中。

示例：

```text
sql/queries/materials.sql -> internal/material/db
```

生成的 `db` 包是持久化适配器。不得手动编辑生成的文件。

正常的依赖流仍为：

```text
处理器 -> 服务 -> 仓储 -> sqlc db 包 -> PostgreSQL
```

服务层应使用模块自有的业务类型，而不是直接使用 sqlc 行类型。

## 添加新的 sqlc 模块

对于 `warehouse` 等新模块：

1. 在 `sql/queries/warehouses.sql` 中添加查询 SQL。
2. 在 `sqlc.yaml` 中添加新的 `sql:` 配置块。
3. 将 `out` 设置为 `internal/warehouse/db`。
4. 将 `package` 设置为模块专用的包名，例如 `warehousedb`。
5. 运行 `make sqlc`。
6. 仓储代码继续负责将 sqlc 行映射为模块业务类型。

每个模块都应有自己的生成 db 包。除非明确存在共享的持久化边界，否则应避免多个业务模块共用一个生成的 db 包。

## 测试

使用以下命令运行所有测试：

```bash
make test
```

在 Go 无法写入默认构建缓存的沙盒环境中，使用本地缓存路径：

```bash
GOCACHE=/private/tmp/stock-flow-go-build-cache make test
```
