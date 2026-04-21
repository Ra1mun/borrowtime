-- Миграция 3: сид администратора по умолчанию
-- ВАЖНО: смените пароль после первого входа.

INSERT INTO users (
    id,
    email,
    password_hash,
    role,
    totp_secret,
    totp_enabled,
    failed_attempts,
    locked_until,
    created_at,
    updated_at
)
VALUES (
    '8b4f9c87-0cb9-4f9e-a3f3-40f5f61f3ef1',
    'admin@borrowtime.local',
    '$argon2id$v=19$m=65536,t=1,p=12$h4tmKmCGhnuE2Up70QkN5A$Xz8tJ/2k+dUHOqDghlHjmBRAZXj3IlGGR/7y8Sw7K/w',
    'admin',
    '',
    FALSE,
    0,
    NULL,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE SET
    role = 'admin',
    password_hash = EXCLUDED.password_hash,
    updated_at = NOW();
