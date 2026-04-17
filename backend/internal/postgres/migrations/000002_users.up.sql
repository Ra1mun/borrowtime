-- Миграция 2: пользователи и refresh-токены (UC-01)

-- Таблица пользователей (FR-1, FR-2, FR-28)
CREATE TABLE IF NOT EXISTS users (
    id              UUID        PRIMARY KEY,
    email           TEXT        NOT NULL UNIQUE,
    password_hash   TEXT        NOT NULL,           -- Argon2id (FR-28)
    role            TEXT        NOT NULL DEFAULT 'user'
                        CHECK (role IN ('user', 'admin')),

    -- 2FA (FR-2)
    totp_secret     TEXT        NOT NULL DEFAULT '', -- пусто пока не настроена
    totp_enabled    BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Защита от перебора (FR-5, 5а потока UC-01)
    failed_attempts INT         NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,                    -- NULL = не заблокирован

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- Таблица refresh-токенов (для ротации JWT)
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL UNIQUE,        -- SHA-256 от токена
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id    ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
