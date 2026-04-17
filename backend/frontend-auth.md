# BorrowTime — Документация API для фронтенда

> **Base URL:** `http://localhost:8080/api/v1`  
> **Content-Type:** `application/json`  
> **Интерактивная документация:** `http://localhost:8080/swagger/index.html`

---

## Содержание

1. [Концепция токенов](#1-концепция-токенов)
2. [Флоу авторизации — без 2FA](#2-флоу-авторизации--без-2fa)
3. [Флоу авторизации — с 2FA](#3-флоу-авторизации--с-2fa)
4. [Настройка 2FA](#4-настройка-2fa)
5. [Обновление токенов](#5-обновление-токенов)
6. [Справочник эндпоинтов — Авторизация](#6-справочник-эндпоинтов--авторизация)
7. [Справочник эндпоинтов — Передачи файлов](#7-справочник-эндпоинтов--передачи-файлов)
8. [Справочник эндпоинтов — Аудит](#8-справочник-эндпоинтов--аудит)
9. [Справочник эндпоинтов — Администрирование](#9-справочник-эндпоинтов--администрирование)
10. [Обработка ошибок](#10-обработка-ошибок)

---

## 1. Концепция токенов

Система использует **два типа JWT-токенов**:

| Токен | Где хранить | TTL | Назначение |
|---|---|---|---|
| `access_token` | Память (не localStorage!) | 15 мин | Отправляется в заголовке `Authorization: Bearer …` при каждом запросе |
| `refresh_token` | `httpOnly` cookie или secure storage | 7 дней | Используется **только** для получения новой пары токенов |

Также существует временный `partial_jwt` — используется **только** внутри флоу 2FA (TTL 5 мин).

### Как отправлять access_token

```http
GET /api/v1/transfers
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Структура access_token (payload)

```json
{
  "uid":  "550e8400-e29b-41d4-a716-446655440000",
  "role": "user",
  "iat":  1711360000,
  "exp":  1711360900
}
```

- `uid` — UUID пользователя, используется как `owner_id` при создании передач
- `role` — `"user"` или `"admin"`

---

## 2. Флоу авторизации — без 2FA

```
Фронтенд                          Сервер
    │                                │
    │  POST /auth/login              │
    │  { email, password }           │
    │ ──────────────────────────────>│
    │                                │  Проверка пароля (Argon2id)
    │  200 OK                        │
    │  { access_token, refresh_token}│
    │ <──────────────────────────────│
    │                                │
    │  Сохраняем токены              │
    │  Используем access_token       │
    │  в заголовке Authorization     │
```

**Шаги:**

1. `POST /auth/login` с `email` + `password`
2. Получаем `access_token` и `refresh_token`
3. `access_token` кладём в память (переменная/state)
4. `refresh_token` кладём в безопасное хранилище
5. При каждом API-запросе: `Authorization: Bearer {access_token}`
6. Когда `access_token` истёк (ошибка `401`) → см. [раздел 5](#5-обновление-токенов)

---

## 3. Флоу авторизации — с 2FA

```
Фронтенд                          Сервер
    │                                │
    │  POST /auth/login              │
    │  { email, password }           │
    │ ──────────────────────────────>│
    │                                │  Пароль верен, 2FA включена
    │  200 OK                        │
    │  { status: "2fa_required",     │
    │    partial_jwt: "eyJ..." }      │
    │ <──────────────────────────────│
    │                                │
    │  Показываем форму ввода кода   │
    │  из приложения-аутентификатора │
    │  (Google Authenticator и т.д.) │
    │                                │
    │  POST /auth/2fa/verify         │
    │  { partial_jwt, code: "123456"}│
    │ ──────────────────────────────>│
    │                                │  Проверка TOTP-кода
    │  200 OK                        │
    │  { access_token, refresh_token}│
    │ <──────────────────────────────│
    │                                │
    │  2FA пройдена, работаем        │
    │  с токенами как обычно         │
```

**Как определить, нужна ли 2FA:**

```js
const response = await fetch('/api/v1/auth/login', { ... });
const data = await response.json();

if (data.status === '2fa_required') {
  // Сохраняем partial_jwt, показываем форму с полем "код"
  store.partialJwt = data.partial_jwt;
  router.push('/2fa');
} else {
  // Сохраняем токены, редирект в ЛК
  store.accessToken  = data.access_token;
  store.refreshToken = data.refresh_token;
  router.push('/dashboard');
}
```

**Важно о `partial_jwt`:**
- Живёт **5 минут**
- Если пользователь ввёл неверный код — можно попробовать ещё раз с тем же `partial_jwt` (пока не истёк)
- После истечения — нужно заново пройти `POST /auth/login`

---

## 4. Настройка 2FA

Настройка 2FA — **двухэтапный процесс**: сначала генерация секрета, затем подтверждение кодом. Оба запроса требуют `access_token`.

```
Фронтенд                          Сервер
    │                                │
    │  POST /auth/2fa/setup          │
    │  Authorization: Bearer ...     │
    │ ──────────────────────────────>│
    │                                │  Генерируется TOTP-секрет
    │  200 OK                        │
    │  { secret, provision_url }     │
    │ <──────────────────────────────│
    │                                │
    │  Рендерим QR-код               │
    │  из provision_url              │
    │  Пользователь сканирует        │
    │  в Google Authenticator        │
    │                                │
    │  POST /auth/2fa/confirm        │
    │  Authorization: Bearer ...     │
    │  { code: "123456" }            │
    │ ──────────────────────────────>│
    │                                │  Проверяем код, включаем 2FA
    │  200 OK                        │
    │  { status: "enabled" }         │
    │ <──────────────────────────────│
```

### Как сгенерировать QR-код на фронтенде

```js
// Пример с библиотекой qrcode (npm install qrcode)
import QRCode from 'qrcode';

const { provision_url } = await setupTwoFARequest();

// Рендер в canvas или img
QRCode.toDataURL(provision_url, (err, url) => {
  imgElement.src = url;
});
```

`provision_url` имеет вид:
```
otpauth://totp/BorrowTime:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=BorrowTime
```

### Отключение 2FA

```js
// Требует валидный код из аутентификатора
POST /auth/2fa/disable
Authorization: Bearer {access_token}
{ "code": "123456" }
```

---

## 5. Обновление токенов

`access_token` живёт **15 минут**. Когда он истекает, сервер вернёт `401`. Нужно автоматически обновить токены.

```
Фронтенд                          Сервер
    │                                │
    │  GET /transfers                │
    │  Authorization: Bearer (истёк) │
    │ ──────────────────────────────>│
    │  401 Unauthorized              │
    │ <──────────────────────────────│
    │                                │
    │  POST /auth/refresh            │
    │  { refresh_token: "..." }      │
    │ ──────────────────────────────>│
    │                                │  Старый refresh удаляется
    │  200 OK                        │  Новая пара выдаётся
    │  { access_token, refresh_token}│
    │ <──────────────────────────────│
    │                                │
    │  Повторяем исходный запрос     │
    │  с новым access_token          │
```

### Пример перехватчика (axios interceptor)

```js
axios.interceptors.response.use(
  response => response,
  async error => {
    if (error.response?.status === 401 && !error.config._retry) {
      error.config._retry = true;
      
      const { data } = await axios.post('/api/v1/auth/refresh', {
        refresh_token: store.refreshToken,
      });
      
      store.accessToken  = data.access_token;
      store.refreshToken = data.refresh_token;
      
      error.config.headers['Authorization'] = `Bearer ${data.access_token}`;
      return axios(error.config);
    }
    
    // refresh тоже истёк → редирект на логин
    if (error.response?.status === 401) {
      store.clear();
      router.push('/login');
    }
    
    return Promise.reject(error);
  }
);
```

**Важно:** при каждом `/auth/refresh` старый `refresh_token` **удаляется**. Всегда сохраняйте новый.

---

## 6. Справочник эндпоинтов — Авторизация

Все эндпоинты находятся по пути `/api/v1/...`

---

### `POST /auth/register`

Регистрация нового пользователя.

**Тело запроса:**
```json
{
  "email":    "user@example.com",
  "password": "minlength8chars"
}
```

**Ответ `201 Created`:**
```json
{
  "id":    "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com"
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Неверный формат email или пароль короче 8 символов |
| `409` | Пользователь с таким email уже существует |

---

### `POST /auth/login`

Вход в систему. Возможны два варианта ответа в зависимости от того, включена ли 2FA.

**Тело запроса:**
```json
{
  "email":    "user@example.com",
  "password": "yourpassword"
}
```

**Ответ `200 OK` — 2FA выключена:**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "a3f1c8d2e9b047..."
}
```

**Ответ `200 OK` — 2FA включена:**
```json
{
  "status":      "2fa_required",
  "partial_jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Неверный email или пароль |
| `423` | Аккаунт заблокирован на 15 минут (≥5 неудачных попыток) |

> При блокировке (`423`) покажите пользователю сообщение: «Слишком много неверных попыток. Попробуйте через 15 минут.»

---

### `POST /auth/2fa/verify`

Второй шаг входа при включённой 2FA. Принимает `partial_jwt` из `/auth/login` и 6-значный код из приложения-аутентификатора.

**Тело запроса:**
```json
{
  "partial_jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "code":        "123456"
}
```

**Ответ `200 OK`:**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "a3f1c8d2e9b047..."
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Неверный или устаревший код; истёкший `partial_jwt` |

---

### `POST /auth/refresh`

Обновление пары токенов. Старый `refresh_token` сразу становится невалидным.

**Тело запроса:**
```json
{
  "refresh_token": "a3f1c8d2e9b047..."
}
```

**Ответ `200 OK`:**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "f8c3de1d4872b9..."
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | `refresh_token` не найден или истёк → нужна повторная авторизация |

---

### `POST /auth/logout`

Выход из системы. Инвалидирует `refresh_token` на сервере.

**Тело запроса:**
```json
{
  "refresh_token": "a3f1c8d2e9b047..."
}
```

**Ответ `204 No Content`** — тело пустое.

> Не забудьте после этого очистить `access_token` и `refresh_token` на клиенте.

---

### `POST /auth/2fa/setup` 🔒

> Требует `Authorization: Bearer {access_token}`

Начало настройки 2FA. Генерирует секрет и возвращает URI для QR-кода. **2FA ещё не включена** — нужно подтвердить через `/auth/2fa/confirm`.

**Тело запроса:** пустое

**Ответ `200 OK`:**
```json
{
  "secret":       "JBSWY3DPEHPK3PXP",
  "provision_url": "otpauth://totp/BorrowTime:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=BorrowTime"
}
```

- `secret` — показывайте как текст (для ручного ввода в приложение)
- `provision_url` — генерируйте из него QR-код библиотекой

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Отсутствует или невалидный `access_token` |

---

### `POST /auth/2fa/confirm` 🔒

> Требует `Authorization: Bearer {access_token}`

Подтверждает, что пользователь успешно добавил секрет в приложение, и **включает 2FA**. Без этого шага 2FA остаётся выключенной.

**Тело запроса:**
```json
{
  "code": "123456"
}
```

**Ответ `200 OK`:**
```json
{
  "status": "enabled"
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Неверный TOTP-код |
| `401` | Не аутентифицирован |
| `409` | 2FA уже включена |

---

### `POST /auth/2fa/disable` 🔒

> Требует `Authorization: Bearer {access_token}`

Отключает 2FA. Для защиты требует валидный код из аутентификатора.

**Тело запроса:**
```json
{
  "code": "123456"
}
```

**Ответ `200 OK`:**
```json
{
  "status": "disabled"
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Неверный TOTP-код |
| `401` | Не аутентифицирован |
| `409` | 2FA и так не включена |

---

## 10. Обработка ошибок

Все ошибки возвращаются в едином формате:

```json
{
  "error": "описание ошибки"
}
```

### Таблица HTTP-кодов

| Код | Значение | Что делать на фронте |
|---|---|---|
| `400` | Неверные данные запроса | Показать сообщение из `error` пользователю |
| `401` | Не аутентифицирован / неверные данные | Редирект на логин (или повтор с refresh) |
| `403` | Нет прав | Показать «Доступ запрещён» |
| `404` | Ресурс не найден | Показать страницу 404 |
| `409` | Конфликт (дубль email, 2FA уже включена) | Показать сообщение из `error` |
| `410` | Передача истекла | Показать «Срок действия ссылки истёк» |
| `423` | Аккаунт заблокирован | Показать «Попробуйте через 15 минут» |
| `500` | Ошибка сервера | Показать общее «Что-то пошло не так» |

### Пример универсального обработчика

```js
async function apiRequest(url, options = {}) {
  const response = await fetch(`/api/v1${url}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(store.accessToken
        ? { 'Authorization': `Bearer ${store.accessToken}` }
        : {}),
    },
    ...options,
  });

  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw { status: response.status, message: data.error ?? 'Unknown error' };
  }

  // 204 — нет тела
  if (response.status === 204) return null;
  return response.json();
}
```

---

## 7. Справочник эндпоинтов — Передачи файлов

> 🔒 Эндпоинты помеченные этим значком требуют `Authorization: Bearer {access_token}`

---

### `POST /transfers` 🔒

Создаёт новую безопасную передачу файла. Файл должен быть **уже зашифрован на клиенте** перед отправкой (E2EE). Сервер хранит только зашифрованные байты — ключ шифрования **никогда не покидает браузер**.

**Тип запроса:** `multipart/form-data`

**Поля формы:**

| Поле | Тип | Обязательное | Описание |
|---|---|---|---|
| `file` | File | ✅ | Зашифрованный файл (бинарный) |
| `policy_expires_at` | string (RFC3339) | ❌ | Срок действия ссылки. Пример: `2026-04-01T18:00:00Z` |
| `policy_max_downloads` | number | ❌ | Лимит скачиваний. `0` = без ограничений |
| `policy_require_auth` | boolean | ❌ | `true` — получатель должен быть авторизован |
| `policy_allowed_emails` | string[] | ❌ | Белый список email. Передаётся несколько раз |
| `encryption_alg` | string | ❌ | Алгоритм шифрования, например `AES-GCM` |
| `encryption_iv` | string (base64) | ❌ | Initialization Vector |
| `encryption_tag` | string (base64) | ❌ | Auth tag (GCM) |

**Пример запроса (JS):**

```js
const formData = new FormData();
formData.append('file', encryptedBlob, 'document.pdf.enc');
formData.append('policy_expires_at', '2026-04-01T18:00:00Z');
formData.append('policy_max_downloads', '3');
formData.append('policy_require_auth', 'false');
formData.append('encryption_alg', 'AES-GCM');
formData.append('encryption_iv', btoa(ivBytes));
formData.append('encryption_tag', btoa(tagBytes));

const response = await fetch('/api/v1/transfers', {
  method: 'POST',
  headers: { 'Authorization': `Bearer ${store.accessToken}` },
  body: formData,
});
```

**Ответ `201 Created`:**
```json
{
  "transfer_id":  "550e8400-e29b-41d4-a716-446655440000",
  "share_url":    "http://localhost:8080/api/v1/s/tok_a3f1c8d2e9b047...",
  "access_token": "tok_a3f1c8d2e9b047..."
}
```

- `share_url` — готовая ссылка для отправки получателю
- `access_token` — токен доступа к файлу (входит в `share_url`)

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Файл не передан, неверный формат `policy_expires_at`, отрицательный лимит |
| `401` | Не аутентифицирован |
| `413` | Файл превышает максимально допустимый размер (по умолчанию 200 МБ) |

---

### `GET /s/{token}`

Скачивает зашифрованный файл по токену из ссылки. **Публичный эндпоинт** — авторизация не обязательна, если политика её не требует.

**Параметры пути:**

| Параметр | Описание |
|---|---|
| `token` | Токен из `share_url` |

**Query-параметры:**

| Параметр | Описание |
|---|---|
| `email` | Email получателя (если политика ограничивает список разрешённых) |

**Ответ `200 OK`:**
- Тело — бинарный поток зашифрованного файла
- Заголовки с метаданными шифрования для расшифровки на клиенте:

```
Content-Disposition: attachment; filename="document.pdf.enc"
Content-Type:        application/octet-stream
X-Encryption-Alg:   AES-GCM
X-Encryption-IV:    <base64>
X-Encryption-Tag:   <base64>
```

**Пример получения и расшифровки (JS):**

```js
const response = await fetch(`/api/v1/s/${token}`);

// Читаем метаданные шифрования из заголовков
const alg = response.headers.get('X-Encryption-Alg');
const iv  = base64ToBytes(response.headers.get('X-Encryption-IV'));
const tag = base64ToBytes(response.headers.get('X-Encryption-Tag'));

// Скачиваем зашифрованный файл
const encryptedBuffer = await response.arrayBuffer();

// Расшифровываем на клиенте ключом, который отправитель передал отдельно
const decrypted = await crypto.subtle.decrypt(
  { name: 'AES-GCM', iv },
  cryptoKey,       // ключ отправитель передал получателю вне сервиса
  encryptedBuffer,
);
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Политика требует авторизацию (`policy_require_auth: true`) |
| `403` | Email получателя не в белом списке или лимит скачиваний исчерпан |
| `404` | Токен не существует |
| `410` | Срок действия передачи истёк |

---

### `GET /transfers/{id}` 🔒

Возвращает полные метаданные передачи. Доступно **только владельцу**. Используется для отображения деталей перед отзывом доступа.

**Параметры пути:**

| Параметр | Описание |
|---|---|
| `id` | UUID передачи из `transfer_id` |

**Ответ `200 OK`:**
```json
{
  "id":             "550e8400-e29b-41d4-a716-446655440000",
  "file_name":      "document.pdf.enc",
  "file_size":      1048576,
  "status":         "ACTIVE",
  "download_count": 1,
  "policy": {
    "expires_at":     "2026-04-01T18:00:00Z",
    "max_downloads":  3,
    "require_auth":   false,
    "allowed_emails": []
  },
  "created_at": "2026-03-25T12:00:00Z"
}
```

**Значения `status`:**

| Значение | Описание |
|---|---|
| `ACTIVE` | Ссылка активна, файл доступен |
| `EXPIRED` | Срок действия истёк (автоматически) |
| `REVOKED` | Доступ отозван владельцем вручную |
| `DOWNLOADED` | Лимит скачиваний исчерпан |

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Не аутентифицирован |
| `403` | Запрашивающий не является владельцем |
| `404` | Передача не найдена |

---

### `DELETE /transfers/{id}` 🔒

Досрочно отзывает доступ к передаче. После этого любые попытки скачать файл по ссылке вернут `403`. Доступно **только владельцу**.

**Параметры пути:**

| Параметр | Описание |
|---|---|
| `id` | UUID передачи |

**Ответ `200 OK`:**
```json
{
  "status": "revoked"
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Не аутентифицирован |
| `403` | Запрашивающий не является владельцем |
| `404` | Передача не найдена |
| `409` | Передача уже не активна (уже истекла, отозвана или скачана) |

---

## 8. Справочник эндпоинтов — Аудит

---

### `GET /audit` 🔒

Возвращает журнал событий с фильтрацией. **Обычный пользователь** видит только события по своим передачам. **Администратор** видит все события в системе.

**Query-параметры (все необязательные):**

| Параметр | Тип | Описание |
|---|---|---|
| `transfer_id` | UUID | Фильтр по конкретной передаче |
| `event_type` | string | Тип события (см. ниже) |
| `from` | RFC3339 | Начало периода. Пример: `2026-03-01T00:00:00Z` |
| `to` | RFC3339 | Конец периода. Пример: `2026-03-31T23:59:59Z` |

**Типы событий (`event_type`):**

| Значение | Описание |
|---|---|
| `CREATED` | Передача создана |
| `VIEWED` | Файл открыт (до скачивания) |
| `DOWNLOADED` | Файл скачан |
| `MANUALLY_REVOKED` | Доступ отозван владельцем |
| `EXPIRED` | Передача истекла автоматически |
| `AUTO_DELETED` | Файл удалён планировщиком |
| `UNAUTHORIZED_ACCESS` | Попытка несанкционированного доступа |
| `USER_LOGIN` | Успешный вход в систему |

**Ответ `200 OK` (события есть):**
```json
{
  "events": [
    {
      "id":          "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "transfer_id": "550e8400-e29b-41d4-a716-446655440000",
      "owner_id":    "a87ff679-a2f3-71d8-20ad-4a5b48b4f56c",
      "event_type":  "DOWNLOADED",
      "actor_id":    "guest",
      "ip_address":  "195.12.34.56",
      "user_agent":  "Mozilla/5.0 ...",
      "success":     true,
      "details":     "",
      "created_at":  "2026-03-25T14:32:01Z"
    }
  ],
  "total": 1
}
```

**Ответ `200 OK` (события не найдены):**
```json
{
  "events":  [],
  "total":   0,
  "message": "Нет записей аудита за выбранный период"
}
```

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Неверный формат `from`/`to` |
| `401` | Не аутентифицирован |

---

### `GET /audit/export` 🔒 (только admin)

Экспортирует журнал аудита в CSV-файл. Доступно **только администраторам**. Поддерживает те же фильтры, что и `GET /audit`.

**Query-параметры:** те же, что у `GET /audit`

**Ответ `200 OK`:**
- Тип: `text/csv; charset=utf-8`
- Заголовок: `Content-Disposition: attachment; filename="audit_20260325_143201.csv"`
- Тело — CSV-поток (стриминг, без буферизации)

**Пример скачивания:**
```js
const response = await fetch('/api/v1/audit/export?from=2026-03-01T00:00:00Z', {
  headers: { 'Authorization': `Bearer ${store.accessToken}` },
});

const blob = await response.blob();
const url  = URL.createObjectURL(blob);

const a    = document.createElement('a');
a.href     = url;
a.download = 'audit.csv';
a.click();
URL.revokeObjectURL(url);
```

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Не аутентифицирован |
| `403` | Роль `user` — требуется `admin` |

---

## 9. Справочник эндпоинтов — Администрирование

> Все эндпоинты этой группы доступны **только пользователям с ролью `admin`**.

---

### `GET /admin/settings` 🔒 (только admin)

Возвращает текущие глобальные настройки системы.

**Ответ `200 OK`:**
```json
{
  "max_file_size_mb":      200,
  "max_retention_days":    30,
  "default_retention_h":   168,
  "default_max_downloads": 10,
  "updated_at":            "2026-03-25T12:00:00Z",
  "updated_by":            "admin-uuid"
}
```

| Поле | Описание |
|---|---|
| `max_file_size_mb` | Максимальный размер файла в МБ |
| `max_retention_days` | Максимальный срок хранения передачи в днях |
| `default_retention_h` | Срок по умолчанию при создании передачи (часы) |
| `default_max_downloads` | Лимит скачиваний по умолчанию |
| `updated_by` | UUID администратора, который последним изменил настройки |

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Не аутентифицирован |
| `403` | Не администратор |

---

### `PUT /admin/settings` 🔒 (только admin)

Обновляет глобальные ограничения системы. Изменения применяются ко **всем новым передачам** (существующие не затрагиваются).

**Тело запроса:**
```json
{
  "max_file_size_mb":      500,
  "max_retention_days":    60,
  "default_retention_hours": 72,
  "default_max_downloads": 5
}
```

| Поле | Тип | Описание |
|---|---|---|
| `max_file_size_mb` | number | Максимальный размер файла в МБ (> 0) |
| `max_retention_days` | number | Максимальный срок хранения в днях (> 0) |
| `default_retention_hours` | number | Срок хранения по умолчанию в часах |
| `default_max_downloads` | number | Лимит скачиваний по умолчанию |

**Ответ `200 OK`:** — те же поля, что и у `GET /admin/settings`

**Ошибки:**

| Код | Причина |
|---|---|
| `400` | Отрицательный размер, нулевой срок или другое невалидное значение |
| `401` | Не аутентифицирован |
| `403` | Не администратор |

---

### `GET /admin/stats` 🔒 (только admin)

Возвращает агрегированную статистику системы в реальном времени.

**Ответ `200 OK`:**
```json
{
  "active_transfers":          142,
  "total_storage_bytes":       2147483648,
  "security_incidents_today":  3
}
```

| Поле | Описание |
|---|---|
| `active_transfers` | Количество передач со статусом `ACTIVE` прямо сейчас |
| `total_storage_bytes` | Суммарный объём всех хранимых зашифрованных файлов |
| `security_incidents_today` | Количество событий `UNAUTHORIZED_ACCESS` за сегодня |

**Ошибки:**

| Код | Причина |
|---|---|
| `401` | Не аутентифицирован |
| `403` | Не администратор |

---

