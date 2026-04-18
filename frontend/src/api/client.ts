const API_BASE = '/api/v1'

type TokenStore = {
  access_token: string
  refresh_token: string
}

const TOKENS_KEY = 'borrowtime_tokens'

export function getTokens(): TokenStore | null {
  const raw = localStorage.getItem(TOKENS_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as TokenStore
  } catch {
    return null
  }
}

export function saveTokens(tokens: TokenStore) {
  localStorage.setItem(TOKENS_KEY, JSON.stringify(tokens))
}

export function clearTokens() {
  localStorage.removeItem(TOKENS_KEY)
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const tokens = getTokens()
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string> || {}),
  }

  if (tokens?.access_token) {
    headers['Authorization'] = `Bearer ${tokens.access_token}`
  }

  // Don't set Content-Type for FormData (browser sets multipart boundary)
  if (!(options.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  // Try to refresh token on 401
  if (res.status === 401 && tokens?.refresh_token) {
    const refreshRes = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: tokens.refresh_token }),
    })

    if (refreshRes.ok) {
      const newTokens = await refreshRes.json() as TokenStore
      saveTokens(newTokens)

      // Retry original request
      headers['Authorization'] = `Bearer ${newTokens.access_token}`
      const retryRes = await fetch(`${API_BASE}${path}`, { ...options, headers })

      if (!retryRes.ok) {
        const err = await retryRes.json().catch(() => ({ error: retryRes.statusText }))
        throw new ApiError(retryRes.status, (err as { error: string }).error || retryRes.statusText)
      }
      return retryRes.json() as Promise<T>
    } else {
      clearTokens()
      window.location.href = '/login'
      throw new ApiError(401, 'Session expired')
    }
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, (err as { error: string }).error || res.statusText)
  }

  if (res.status === 204) return undefined as T

  return res.json() as Promise<T>
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

// ─── Auth ───────────────────────────────────────────────────────────────────

export type UserInfo = {
  id: string
  email: string
  role: 'user' | 'admin'
  totp_enabled: boolean
}

export type LoginResult =
  | { twoFA: false; user: UserInfo }
  | { twoFA: true; partial_jwt: string }

export async function apiLogin(email: string, password: string): Promise<LoginResult> {
  const res = await request<TokenStore & { status?: string; partial_jwt?: string }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })

  if (res.status === '2fa_required' && res.partial_jwt) {
    return { twoFA: true, partial_jwt: res.partial_jwt }
  }

  saveTokens({ access_token: res.access_token, refresh_token: res.refresh_token })
  const user = await apiGetMe()
  return { twoFA: false, user }
}

export async function apiVerify2FA(partialJwt: string, code: string): Promise<UserInfo> {
  const res = await request<TokenStore>('/auth/2fa/verify', {
    method: 'POST',
    body: JSON.stringify({ partial_jwt: partialJwt, code }),
  })

  saveTokens({ access_token: res.access_token, refresh_token: res.refresh_token })
  return apiGetMe()
}

export async function apiSetup2FA(): Promise<{ secret: string; provision_url: string }> {
  return request('/auth/2fa/setup', { method: 'POST' })
}

export async function apiConfirm2FA(code: string): Promise<void> {
  await request('/auth/2fa/confirm', {
    method: 'POST',
    body: JSON.stringify({ code }),
  })
}

export async function apiDisable2FA(code: string): Promise<void> {
  await request('/auth/2fa/disable', {
    method: 'POST',
    body: JSON.stringify({ code }),
  })
}

export async function apiRegister(email: string, password: string): Promise<{ id: string; email: string }> {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export async function apiLogout(): Promise<void> {
  const tokens = getTokens()
  if (tokens?.refresh_token) {
    try {
      await request('/auth/logout', {
        method: 'POST',
        body: JSON.stringify({ refresh_token: tokens.refresh_token }),
      })
    } catch {
      // ignore
    }
  }
  clearTokens()
}

export async function apiGetMe(): Promise<UserInfo> {
  return request<UserInfo>('/auth/me')
}

// ─── Transfers ──────────────────────────────────────────────────────────────

export type TransferInfo = {
  id: string
  file_name: string
  file_size: number
  status: string
  access_token: string
  download_count: number
  expires_at: string
  created_at: string
}

export async function apiCreateTransfer(
  file: File,
  expiresInHours: number,
  allowedEmails?: string[],
  maxDownloads?: number,
): Promise<{ transfer_id: string; share_url: string; access_token: string }> {
  const formData = new FormData()
  formData.append('file', file)

  const expiresAt = new Date(Date.now() + expiresInHours * 3600000).toISOString()
  formData.append('policy_expires_at', expiresAt)
  formData.append('policy_max_downloads', String(maxDownloads ?? 0))

  if (allowedEmails) {
    for (const email of allowedEmails) {
      formData.append('policy_allowed_emails', email)
    }
  }

  return request('/transfers', {
    method: 'POST',
    body: formData,
  })
}

export async function apiListTransfers(): Promise<TransferInfo[]> {
  return request<TransferInfo[]>('/transfers')
}

export async function apiListIncomingTransfers(): Promise<TransferInfo[]> {
  return request<TransferInfo[]>('/transfers/incoming')
}

export async function apiRevokeTransfer(id: string): Promise<void> {
  await request(`/transfers/${id}`, { method: 'DELETE' })
}

export function getDownloadUrl(token: string): string {
  return `${API_BASE}/s/${token}`
}

export async function apiDownloadFile(accessToken: string): Promise<void> {
  const tokens = getTokens()
  const headers: Record<string, string> = {}
  if (tokens?.access_token) {
    headers['Authorization'] = `Bearer ${tokens.access_token}`
  }

  const res = await fetch(`${API_BASE}/s/${accessToken}`, { headers })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, (err as { error: string }).error || res.statusText)
  }

  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = disposition.match(/filename="?([^"]+)"?/)
  const fileName = match ? match[1] : 'download'

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

// ─── Audit ──────────────────────────────────────────────────────────────────

export type AuditEvent = {
  ID: string
  TransferID: string
  OwnerID: string
  EventType: string
  ActorID: string
  IPAddress: string
  UserAgent: string
  Success: boolean
  Details: string
  CreatedAt: string
}

export async function apiListAudit(): Promise<{ events: AuditEvent[]; total: number }> {
  return request('/audit')
}

export function getAuditExportUrl(): string {
  const tokens = getTokens()
  return `${API_BASE}/audit/export?token=${tokens?.access_token || ''}`
}

// ─── Users (admin) ──────────────────────────────────────────────────────────

export type UserListItem = {
  id: string
  email: string
  role: string
  created_at: string
}

export async function apiListUsers(): Promise<UserListItem[]> {
  return request<UserListItem[]>('/users')
}

export async function apiSearchUsers(q: string): Promise<UserListItem[]> {
  return request<UserListItem[]>(`/users/search?q=${encodeURIComponent(q)}`)
}

export async function apiDeleteUser(id: string): Promise<void> {
  await request(`/users/${id}`, { method: 'DELETE' })
}

export async function apiUpdateUserRole(id: string, role: string): Promise<void> {
  await request(`/users/${id}/role`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  })
}

// ─── Admin Settings ─────────────────────────────────────────────────────────

export type GlobalSettings = {
  max_file_size_mb: number
  max_retention_days: number
  default_retention_h: number
  default_max_downloads: number
  updated_at: string
  updated_by: string
}

export async function apiGetSettings(): Promise<GlobalSettings> {
  return request<GlobalSettings>('/admin/settings')
}

export async function apiUpdateSettings(settings: Omit<GlobalSettings, 'updated_at' | 'updated_by'>): Promise<GlobalSettings> {
  return request<GlobalSettings>('/admin/settings', {
    method: 'PUT',
    body: JSON.stringify({
      max_file_size_mb: settings.max_file_size_mb,
      max_retention_days: settings.max_retention_days,
      default_retention_hours: settings.default_retention_h,
      default_max_downloads: settings.default_max_downloads,
    }),
  })
}

export async function apiGetStats(): Promise<{
  active_transfers: number
  total_storage_bytes: number
  security_incidents_today: number
}> {
  return request('/admin/stats')
}

export async function apiExportAuditCsv(): Promise<void> {
  const tokens = getTokens()
  const headers: Record<string, string> = {}
  if (tokens?.access_token) {
    headers['Authorization'] = `Bearer ${tokens.access_token}`
  }

  const res = await fetch(`${API_BASE}/audit/export`, { headers })
  if (!res.ok) {
    throw new ApiError(res.status, 'Export failed')
  }

  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `audit_${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
