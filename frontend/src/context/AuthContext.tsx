import { createContext, useContext, useEffect, useState, useCallback } from 'react'
import type { ReactNode } from 'react'
import { apiGetMe, apiLogin, apiLogout, apiRegister, apiVerify2FA, clearTokens, getTokens } from '../api/client'
import type { LoginResult, UserInfo } from '../api/client'

type AuthContextValue = {
  user: UserInfo | null
  loading: boolean
  login: (email: string, password: string) => Promise<LoginResult>
  verify2FA: (partialJwt: string, code: string) => Promise<UserInfo>
  register: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null)
  const [loading, setLoading] = useState(true)

  // On mount, try to restore session
  useEffect(() => {
    const tokens = getTokens()
    if (!tokens?.access_token) {
      setLoading(false)
      return
    }
    apiGetMe()
      .then(setUser)
      .catch(() => clearTokens())
      .finally(() => setLoading(false))
  }, [])

  const login = useCallback(async (email: string, password: string): Promise<LoginResult> => {
    const result = await apiLogin(email, password)
    if (result.twoFA === false) {
      setUser(result.user)
    }
    return result
  }, [])

  const verify2FA = useCallback(async (partialJwt: string, code: string): Promise<UserInfo> => {
    const u = await apiVerify2FA(partialJwt, code)
    setUser(u)
    return u
  }, [])

  const register = useCallback(async (email: string, password: string) => {
    await apiRegister(email, password)
    const result = await apiLogin(email, password)
    if (result.twoFA === false) {
      setUser(result.user)
    }
  }, [])

  const logout = useCallback(async () => {
    await apiLogout()
    setUser(null)
  }, [])

  const refreshUser = useCallback(async () => {
    const u = await apiGetMe()
    setUser(u)
  }, [])

  return (
    <AuthContext.Provider value={{ user, loading, login, verify2FA, register, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
