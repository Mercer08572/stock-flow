# 依赖规则

Stock-Flow 遵循严格的三层请求流：

```text
HTTP 请求 -> 处理器 -> 服务 -> 仓储 -> PostgreSQL
```

依赖必须仅指向一个方向：

```text
处理器 -> 服务 -> 仓储
```

## 处理器层

- 处理 HTTP 请求和响应。
- 解析路径、查询参数和请求体数据。
- 仅执行请求级校验。
- 调用服务接口。
- 错误必须交给 `internal/shared/httperr.Write` 统一写出，禁止在处理器里手写
  「错误 -> 状态码」的映射 switch。信封本身仍由 `pkg/response` 生成。
- 不得包含业务逻辑。
- 不得直接调用仓储。

## 错误处理规则

- 领域错误在模块的 `errors.go` 用 `pkg/apperr` 声明，错误自带 HTTP 状态与业务错误码：

  ```go
  ErrDuplicateCode = apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "warehouse code already exists")
  ```

- 错误码表集中在 `pkg/apperr/codes.go`。必须遵守两条规则：一码只对应一个 HTTP 状态；
  已发布的码字符串不得修改（它是对外契约）。
- 全项目只有一个错误出口 `internal/shared/httperr.Write`：它负责把错误写成 `pkg/response`
  的统一信封。模块不得再各写一份 `writeError`。
- 需要携带响应头的错误（如登录限流的 `Retry-After`）实现 `Headers() map[string]string` 即可。
- 仅用于 CLI、不经 HTTP 出口的错误保持普通 `errors.New`。
- 业务错误码与前端文案的对应关系见 `stock-flow-admin/src/api/error-messages.ts`。

## 服务层

- 负责应用用例和业务编排。
- 调用仓储接口进行持久化。
- 协调跨模块的应用服务调用。
- 变更库存时强制校验仓库等库存维度。
- 强制库存变更操作具备幂等性。
- 在用例需要原子变更时管理事务。
- 不得依赖 Gin 或 HTTP 特有类型。

## 仓储层

- 仅负责持久化逻辑。
- 使用 `pgx` 和 `sqlc` 生成的查询。
- 将数据库记录映射为领域或应用数据结构。
- 服务层要求时，将仓库标识符作为库存记录和变动记录的一部分持久化。
- 服务层要求时，持久化幂等键和变动记录。
- 不得包含业务规则。
- 不得调用处理器或服务。
- 不得管理应用工作流。

## 接口优先规则

每一层都必须先公开接口，再提供实现。

依赖必须通过构造函数注入：

```go
func NewMaterialService(repo MaterialRepository) MaterialService {
    return &materialService{repo: repo}
}
```

不得使用全局变量在各层之间传递依赖。
