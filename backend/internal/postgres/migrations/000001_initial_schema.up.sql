-- Миграция 1: начальная схема BorrowTime
-- UC-02, UC-03, UC-04, UC-06: таблица передач

CREATE TABLE IF NOT EXISTS transfers (
    id                      UUID        PRIMARY KEY,
    owner_id                UUID        NOT NULL,
    file_name               TEXT        NOT NULL,
    file_size_bytes         BIGINT      NOT NULL,
    storage_path            TEXT        NOT NULL,
    access_token            TEXT        NOT NULL UNIQUE,

    -- Политика доступа (FR-9)
    policy_expires_at       TIMESTAMPTZ,
    policy_max_downloads    INT         NOT NULL DEFAULT 0,
    policy_require_auth     BOOLEAN     NOT NULL DEFAULT FALSE,
    policy_allowed_emails   TEXT[]      NOT NULL DEFAULT '{}',

    -- Метаданные шифрования (FR-8) — ключ не хранится (NFR-5)
    encryption_alg          TEXT        NOT NULL DEFAULT '',
    encryption_iv           TEXT        NOT NULL DEFAULT '',
    encryption_tag          TEXT        NOT NULL DEFAULT '',

    -- Жизненный цикл (NFR-7)
    status                  TEXT        NOT NULL DEFAULT 'ACTIVE'
                                CHECK (status IN ('ACTIVE', 'EXPIRED', 'REVOKED', 'DOWNLOADED')),
    download_count          INT         NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы для UC-04 (поиск истёкших/исчерпанных), UC-03 (по токену)
CREATE INDEX IF NOT EXISTS idx_transfers_owner_id    ON transfers (owner_id);
CREATE INDEX IF NOT EXISTS idx_transfers_status      ON transfers (status);
CREATE INDEX IF NOT EXISTS idx_transfers_expires_at  ON transfers (policy_expires_at)
    WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_transfers_access_token ON transfers (access_token);

-- UC-05, UC-07: таблица журнала аудита (FR-17)
CREATE TABLE IF NOT EXISTS audit_logs (
    id           UUID        PRIMARY KEY,
    transfer_id  UUID        REFERENCES transfers (id) ON DELETE SET NULL,
    owner_id     UUID,
    event_type   TEXT        NOT NULL,
    actor_id     TEXT        NOT NULL DEFAULT '',
    ip_address   TEXT        NOT NULL DEFAULT '',
    user_agent   TEXT        NOT NULL DEFAULT '',
    success      BOOLEAN     NOT NULL DEFAULT TRUE,
    details      TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы для фильтрации UC-05 (FR-18, FR-19)
CREATE INDEX IF NOT EXISTS idx_audit_transfer_id ON audit_logs (transfer_id);
CREATE INDEX IF NOT EXISTS idx_audit_owner_id    ON audit_logs (owner_id);
CREATE INDEX IF NOT EXISTS idx_audit_event_type  ON audit_logs (event_type);
CREATE INDEX IF NOT EXISTS idx_audit_created_at  ON audit_logs (created_at DESC);
-- FR-30: быстрый поиск несанкционированных попыток
CREATE INDEX IF NOT EXISTS idx_audit_unauthorized ON audit_logs (created_at DESC)
    WHERE event_type = 'UNAUTHORIZED_ACCESS';

-- UC-08: таблица глобальных настроек (FR-22) — singleton (id = 1)
CREATE TABLE IF NOT EXISTS global_settings (
    id                          INT         PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    max_file_size_bytes         BIGINT      NOT NULL DEFAULT 209715200, -- 200 MB
    max_retention_period_secs   BIGINT      NOT NULL DEFAULT 2592000,   -- 30 дней
    default_retention_secs      BIGINT      NOT NULL DEFAULT 604800,    -- 7 дней
    default_max_downloads       INT         NOT NULL DEFAULT 10,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by                  TEXT        NOT NULL DEFAULT 'system'
);

-- Предзаполнение настроек по умолчанию
INSERT INTO global_settings DEFAULT VALUES
ON CONFLICT (id) DO NOTHING;
