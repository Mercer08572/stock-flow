-- Migration metadata
-- status: pending
-- description: create month-partitioned append-only request audit log

-- 分区表不能使用 BIGSERIAL 的隐式序列（不同 PostgreSQL 版本对分区表 identity 的支持不一致），
-- 这里显式建序列并作为默认值。写入方不提供 id，由默认值分配。
CREATE SEQUENCE audit_logs_id_seq;

CREATE TABLE audit_logs (
    id              BIGINT      NOT NULL DEFAULT nextval('audit_logs_id_seq'),
    occurred_at     TIMESTAMPTZ NOT NULL,
    trace_id        TEXT        NOT NULL,
    method          TEXT        NOT NULL,
    path            TEXT        NOT NULL,
    route           TEXT        NULL,
    status_code     INT         NOT NULL,
    latency_ms      BIGINT      NOT NULL,
    auth_method     TEXT        NOT NULL,
    actor_type      TEXT        NULL,
    actor_id        BIGINT      NULL,
    actor_label     TEXT        NULL,
    credential_id   TEXT        NULL,
    client_ip       INET        NULL,
    user_agent      TEXT        NULL,
    error_code      TEXT        NULL,
    request_bytes   BIGINT      NOT NULL DEFAULT 0,
    response_bytes  BIGINT      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ NULL,

    CONSTRAINT pk_audit_logs PRIMARY KEY (id, occurred_at),
    CONSTRAINT chk_audit_logs_trace_id_not_blank
        CHECK (btrim(trace_id) <> ''),
    CONSTRAINT chk_audit_logs_method_not_blank
        CHECK (btrim(method) <> ''),
    CONSTRAINT chk_audit_logs_path_not_blank
        CHECK (btrim(path) <> ''),
    CONSTRAINT chk_audit_logs_status_code
        CHECK (status_code BETWEEN 100 AND 599),
    CONSTRAINT chk_audit_logs_auth_method
        CHECK (auth_method IN ('admin_session', 'api_app', 'none', 'ambiguous')),
    CONSTRAINT chk_audit_logs_actor_type
        CHECK (actor_type IS NULL OR actor_type IN ('admin', 'api_app'))
) PARTITION BY RANGE (occurred_at);

ALTER SEQUENCE audit_logs_id_seq OWNED BY audit_logs.id;

-- 时间范围 + 游标分页的主索引。
CREATE INDEX idx_audit_logs_occurred_at
    ON audit_logs (occurred_at DESC, id DESC);

-- 按调用方过滤：管理员用 id，API 应用用 app 的 id。
CREATE INDEX idx_audit_logs_actor
    ON audit_logs (actor_type, actor_id, occurred_at DESC);

-- 管理页按 username / app_id 文本过滤。
CREATE INDEX idx_audit_logs_actor_label
    ON audit_logs (actor_label, occurred_at DESC);

CREATE INDEX idx_audit_logs_client_ip
    ON audit_logs (client_ip, occurred_at DESC);

CREATE INDEX idx_audit_logs_status
    ON audit_logs (status_code, occurred_at DESC);

CREATE INDEX idx_audit_logs_auth
    ON audit_logs (auth_method, occurred_at DESC);

-- 排障时用 trace_id 反查同一次请求。
CREATE INDEX idx_audit_logs_trace
    ON audit_logs (trace_id);

COMMENT ON TABLE audit_logs IS
    'Append-only request audit log. Partitions are created and dropped by internal/audit PartitionManager.';
COMMENT ON COLUMN audit_logs.occurred_at IS
    'Server-side request start time; also the partition key.';
COMMENT ON COLUMN audit_logs.trace_id IS
    'Trace id from the TraceID middleware.';
COMMENT ON COLUMN audit_logs.route IS
    'Gin route template, empty for unmatched routes.';
COMMENT ON COLUMN audit_logs.auth_method IS
    'admin_session, api_app, none, or ambiguous (both methods were supplied).';
COMMENT ON COLUMN audit_logs.actor_type IS
    'admin or api_app; NULL for unauthenticated / failed requests.';
COMMENT ON COLUMN audit_logs.actor_id IS
    'Admin user id or API application id.';
COMMENT ON COLUMN audit_logs.actor_label IS
    'Admin username or API application app_id.';
COMMENT ON COLUMN audit_logs.credential_id IS
    'API secret id used by an API application caller.';
COMMENT ON COLUMN audit_logs.client_ip IS
    'Caller IP after trusted-proxy handling; NULL when it cannot be parsed.';
COMMENT ON COLUMN audit_logs.error_code IS
    'Stable business error code such as AUTH_AMBIGUOUS_METHOD.';
COMMENT ON COLUMN audit_logs.created_at IS
    'Insert time. Audit rows are append-only; occurred_at is the request start time.';
COMMENT ON COLUMN audit_logs.updated_at IS
    'Reserved by the table convention; audit rows are never updated.';
COMMENT ON COLUMN audit_logs.deleted_at IS
    'Reserved by the table convention; retention drops whole partitions instead.';
