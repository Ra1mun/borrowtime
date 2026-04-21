-- Откат миграции 3: удалить дефолтного администратора

DELETE FROM users
WHERE email = 'admin@borrowtime.local';
