# 数据库迁移规范

> 本文件适用于 `migrations/` 目录。
> 在此目录中工作时，应同时阅读根目录的 `AGENTS.md` 和本文件。

## 文件命名

```
<number>_<description>.up.sql # 正向迁移（应用变更）
<number>_<description>.down.sql # 反向迁移（还原变更）
```

- 编号：共 12 位，前八位表示年月日，后四位为序号（`202601010001`）。
- 描述：使用 snake_case。
- 下一个序号 = 当前最大序号 + 1。

## 迁移元数据

每个 `*.up.sql` 文件都必须以元数据注释块开头：

```sql
-- 迁移元数据
-- status: pending
-- description: 简短的迁移描述
```

`status` 由人工维护，且只能是 `pending` 或 `applied`。
迁移在目标数据库中执行前使用 `pending`。执行后，在进行下一次模式变更之前，将文件更新为 `applied`。尚处于 pending 状态的迁移可以直接修改，无需创建另一个迁移文件；已经 applied 的迁移不得重写。

`status` 必须与事实一致：只要某个迁移已经在任何目标数据库中执行过，其 `*.up.sql` 文件就必须写 `applied`。
文件里留着 `pending` 而数据库里已经应用，会直接误导下一个改 schema 的人。

## 迁移不可变性（硬性约束）

- **已 `applied` 的迁移是不可变历史**：禁止修改、删除、重命名其中的任何内容（包括 DDL、DML、注释、元数据以外的一切），
  也禁止把原本在 A 文件里的语句挪到 B 文件。
- 任何结构变更或数据修复都必须**新增迁移文件**，由新文件表达差异；即使在本地看起来"只是改一行"也一样。
- 已应用迁移被重写时，**已经迁移过的数据库不会重跑它**，于是新环境与旧环境的结构/数据就此分叉，
  且这种分叉不会报错、只会以"某些环境缺数据/缺列"的形式在很久之后暴露。
  历史事故：管理员种子在 `a3fcea1` 被从 `202607130005` 中删除，导致所有全新库都没有 admin 行、`stock-flow-admin init` 永远失败。
- **代码审查红线**：审查迁移改动时，第一件事是确认改动只涉及"新增文件"，或仅涉及 `pending` 文件。

### 唯一例外：早期开发阶段的引导数据集中维护

项目早期（尚无生产部署）允许在**用户显式批准**的前提下，把引导数据补回既有迁移文件，以便初始状态集中在一处。当前仅有一例：

- `202607130005_create_auth_tables.up.sql`：恢复管理员种子行（不可登录的占位 `password_hash` + 由 `202607140006` 提供的 `password_initialized = FALSE` 默认值）。
  已应用该迁移的旧库**不会**补上这一行，需要手工 `INSERT` 或重建库。

除该例外之外，一律按上面的不可变性约束执行；新的例外必须写进本节，并在任务文档中记录批准来源。

## 创建新表时必须包含基础字段

```sql
id         BIGSERIAL    PRIMARY KEY,
created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
deleted_at TIMESTAMPTZ  NULL      -- 软删除，NULL 表示未删除
```

## PostgreSQL 类型规范

| 用途 | 使用 | 禁止使用 |
|---|---|---|
| 主键/外键 | `BIGSERIAL` / `BIGINT` | `INT` / `SERIAL` |
| 文本（名称、编码） | `TEXT` | `VARCHAR(n)` |
| 小数/金额 | `NUMERIC(p, s)` | `FLOAT` / `REAL` |
| 时间戳 | `TIMESTAMPTZ` | `TIMESTAMP`（不含时区）|
| 枚举值 | `TEXT` + `CHECK` 约束 | PostgreSQL `ENUM` 类型 |
| 布尔值 | `BOOLEAN` | |

## 示例：20260101001_create_materials_table

**up.sql**

```sql
CREATE TABLE materials (
    id          BIGSERIAL    PRIMARY KEY,
    code        TEXT         NOT NULL UNIQUE,
    name        TEXT         NOT NULL,
    description TEXT,
    unit        TEXT         NOT NULL,
    category    TEXT,
    status      TEXT         NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'inactive')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ  NULL
);

CREATE INDEX idx_materials_code     ON materials (code)     WHERE deleted_at IS NULL;
CREATE INDEX idx_materials_status   ON materials (status)   WHERE deleted_at IS NULL;
CREATE INDEX idx_materials_category ON materials (category) WHERE deleted_at IS NULL;

COMMENT ON TABLE  materials            IS '物料主表';
COMMENT ON COLUMN materials.code       IS '物料编码，全局唯一';
COMMENT ON COLUMN materials.unit       IS '计量单位（pcs/kg/m 等）';
COMMENT ON COLUMN materials.status     IS '状态：active=启用, inactive=停用';
COMMENT ON COLUMN materials.deleted_at IS '软删除标记，NULL 表示未删除';
```

**down.sql**

```sql
DROP TABLE IF EXISTS materials;
```

## 规则

- 添加外键约束时，还应为该外键约束对应的列添加索引。
