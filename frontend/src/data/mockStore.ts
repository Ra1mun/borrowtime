export type Role = 'user' | 'admin'

export type StoredUser = {
  id: string
  nickname: string
  email: string
  password: string
  role: Role
  createdAt: string
}

export type TransferItem = {
  id: string
  token: string
  senderNickname: string
  senderEmail: string
  senderId: string
  receiverNickname: string
  receiverEmail: string
  receiverId: string
  fileName: string
  fileUrl: string
  expiresInHours: number
  createdAt: string
  expiresAt: string
  isDownloaded: boolean
}

export type AuditLog = {
  id: string
  date: string
  user: string
  action: string
  status: 'success' | 'warning' | 'danger'
  details: string
}

const USERS_KEY = 'secure_share_users'
const CURRENT_USER_KEY = 'secure_share_current_user'
const RECEIVED_ITEMS_KEY = 'secure_share_received_items'
const AUDIT_LOGS_KEY = 'secure_share_audit_logs'
const TRANSFERS_KEY = "borrowed_time_transfers"

const demoUsers: StoredUser[] = [
  {
    id: 'u-1',
    nickname: 'admin',
    email: 'admin@demo.com',
    password: 'admin123',
    role: 'admin',
    createdAt: new Date().toISOString(),
  },
  {
    id: 'u-2',
    nickname: 'user',
    email: 'user@demo.com',
    password: '123456',
    role: 'user',
    createdAt: new Date().toISOString(),
  },
  {
    id: 'u-3',
    nickname: 'sergey',
    email: 'sergey@demo.com',
    password: '123456',
    role: 'user',
    createdAt: new Date().toISOString(),
  },
]

const demoAuditLogs: AuditLog[] = [
  {
    id: 'log-1',
    date: formatDateTime(new Date().toISOString()),
    user: 'admin',
    action: 'Создал админский аккаунт',
    status: 'success',
    details: 'Система инициализирована. Демо-админ доступен по почте admin@demo.com и паролю admin123.',
  },
  {
    id: 'log-2',
    date: formatDateTime(new Date().toISOString()),
    user: 'user',
    action: 'Регистрация аккаунта',
    status: 'success',
    details: 'Пользователь создан в демонстрационном хранилище браузера.',
  },
]

function readStorage<T>(key: string, defaultValue: T): T {
  const raw = localStorage.getItem(key)

  if (!raw) {
    return defaultValue
  }

  try {
    return JSON.parse(raw) as T
  } catch {
    return defaultValue
  }
}

export function seedMockData() {
  if (!localStorage.getItem(USERS_KEY)) {
    localStorage.setItem(USERS_KEY, JSON.stringify(demoUsers))
  }

  if (!localStorage.getItem(RECEIVED_ITEMS_KEY)) {
    localStorage.setItem(RECEIVED_ITEMS_KEY, JSON.stringify([]))
  }

  if (!localStorage.getItem(AUDIT_LOGS_KEY)) {
    localStorage.setItem(AUDIT_LOGS_KEY, JSON.stringify(demoAuditLogs))
  }
}

export function formatDateTime(date: string) {
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(date))
}

export function getUsers(): StoredUser[] {
  seedMockData()
  return JSON.parse(localStorage.getItem(USERS_KEY) || '[]')
}

export function saveUsers(users: StoredUser[]) {
  localStorage.setItem(USERS_KEY, JSON.stringify(users))
}

export function getCurrentUser(): StoredUser | null {
  seedMockData()
  return JSON.parse(localStorage.getItem(CURRENT_USER_KEY) || 'null')
}

export function setCurrentUser(user: StoredUser | null) {
  if (!user) {
    localStorage.removeItem(CURRENT_USER_KEY)
    return
  }

  localStorage.setItem(CURRENT_USER_KEY, JSON.stringify(user))
}

export function registerUser(payload: {
  nickname: string
  email: string
  password: string
}) {
  const users = getUsers()
  const alreadyExists = users.some(
    (user) =>
      user.email.toLowerCase() === payload.email.toLowerCase() ||
      user.nickname.toLowerCase() === payload.nickname.toLowerCase(),
  )

  if (alreadyExists) {
    throw new Error('Пользователь с таким ником или почтой уже существует.')
  }

  const newUser: StoredUser = {
    id: crypto.randomUUID(),
    nickname: payload.nickname,
    email: payload.email,
    password: payload.password,
    role: 'user',
    createdAt: new Date().toISOString(),
  }

  const updatedUsers = [...users, newUser]
  saveUsers(updatedUsers)
  addAuditLog({
    user: newUser.nickname,
    action: 'Регистрация аккаунта',
    status: 'success',
    details: `Зарегистрирован пользователь ${newUser.nickname}.`,
  })

  return newUser
}

export function loginUser(email: string, password: string) {
  const users = getUsers()
  const foundUser = users.find(
    (user) => user.email.toLowerCase() === email.toLowerCase() && user.password === password,
  )

  if (!foundUser) {
    addAuditLog({
      user: email,
      action: 'Неудачная попытка входа',
      status: 'danger',
      details: 'Пользователь ввёл неверную почту или пароль.',
    })
    throw new Error('Неверная почта или пароль.')
  }

  setCurrentUser(foundUser)
  addAuditLog({
    user: foundUser.nickname,
    action: 'Вход в систему',
    status: 'success',
    details: `${foundUser.nickname} успешно вошёл в систему.`,
  })

  return foundUser
}

export function logoutUser() {
  const currentUser = getCurrentUser()

  if (currentUser) {
    addAuditLog({
      user: currentUser.nickname,
      action: 'Выход из системы',
      status: 'warning',
      details: `${currentUser.nickname} завершил текущую сессию.`,
    })
  }

  setCurrentUser(null)
}

export function getReceivedItems(): TransferItem[] {
  seedMockData()
  return JSON.parse(localStorage.getItem(RECEIVED_ITEMS_KEY) || '[]')
}

export function saveReceivedItems(items: TransferItem[]) {
  localStorage.setItem(RECEIVED_ITEMS_KEY, JSON.stringify(items))
}

export function createTransfer(payload: {
  sender: StoredUser
  receiver: StoredUser
  file: File
  token: string
  expiresInHours: number
}) {
  const items = getReceivedItems()

  const fileUrl = URL.createObjectURL(payload.file)
  const createdAt = new Date().toISOString()
  const expiresAt = new Date(Date.now() + payload.expiresInHours * 60 * 60 * 1000).toISOString()

  const newTransfer: TransferItem = {
    id: crypto.randomUUID(),
    token: payload.token,

    senderId: payload.sender.id,
    senderNickname: payload.sender.nickname,
    senderEmail: payload.sender.email,

    receiverNickname: payload.receiver.nickname,
    receiverEmail: payload.receiver.email,
    receiverId: payload.receiver.id,

    fileName: payload.file.name,
    fileUrl,
    expiresInHours: payload.expiresInHours,
    createdAt,
    expiresAt,
    isDownloaded: false,
  }

  saveReceivedItems([newTransfer, ...items])

  addAuditLog({
    user: payload.sender.nickname,
    action: 'Отправка файла',
    status: 'success',
    details: `Файл ${payload.file.name} отправлен пользователю ${payload.receiver.nickname}. Срок действия ссылки: ${payload.expiresInHours} ч.`,
  })

  return newTransfer
}

export function markTransferDownloaded(id: string) {
  const items = getReceivedItems()
  const updatedItems = items.map((item) =>
    item.id === id
      ? {
          ...item,
          isDownloaded: true,
        }
      : item,
  )

  const downloadedItem = updatedItems.find((item) => item.id === id)
  if (downloadedItem) {
    addAuditLog({
      user: downloadedItem.receiverNickname,
      action: 'Скачивание файла',
      status: 'success',
      details: `Файл ${downloadedItem.fileName} скачан по токену ${downloadedItem.token}.`,
    })
  }

  saveReceivedItems(updatedItems)
}

export function getAuditLogs(): AuditLog[] {
  seedMockData()
  return JSON.parse(localStorage.getItem(AUDIT_LOGS_KEY) || '[]')
}

export function addAuditLog(payload: Omit<AuditLog, 'id' | 'date'>) {
  const logs = getAuditLogs()

  const newLog: AuditLog = {
    id: crypto.randomUUID(),
    date: formatDateTime(new Date().toISOString()),
    ...payload,
  }

  localStorage.setItem(AUDIT_LOGS_KEY, JSON.stringify([newLog, ...logs]))
}

export function updateUserRole(userId: string, role: Role) {
  const users = getUsers()
  const updatedUsers = users.map((user) => (user.id === userId ? { ...user, role } : user))
  saveUsers(updatedUsers)

  const changedUser = updatedUsers.find((user) => user.id === userId)
  if (changedUser) {
    addAuditLog({
      user: changedUser.nickname,
      action: 'Изменение роли',
      status: 'warning',
      details: `Пользователю ${changedUser.nickname} назначена роль ${role}.`,
    })
  }

  const currentUser = getCurrentUser()
  if (currentUser?.id === userId && changedUser) {
    setCurrentUser(changedUser)
  }
}

export function deleteUser(userId: string) {
  const users = getUsers()
  const userToDelete = users.find((user) => user.id === userId)
  const filteredUsers = users.filter((user) => user.id !== userId)
  saveUsers(filteredUsers)

  if (userToDelete) {
    addAuditLog({
      user: userToDelete.nickname,
      action: 'Удаление пользователя',
      status: 'danger',
      details: `Пользователь ${userToDelete.nickname} был удалён из демонстрационной системы.`,
    })
  }

  const currentUser = getCurrentUser()
  if (currentUser?.id === userId) {
    setCurrentUser(null)
  }
}

export function getAvailableReceivers(currentUserId?: string) {
  return getUsers().filter((user) => user.id !== currentUserId)
}

export function getTransfersForCurrentUser(): TransferItem[] {
  const currentUser = getCurrentUser()

  if (!currentUser) {
    return []
  }

  const transfers: TransferItem[] =
      readStorage<TransferItem[]>(TRANSFERS_KEY, [])

  return transfers.filter(
      (item: TransferItem) => item.receiverId === currentUser.id
  )
}