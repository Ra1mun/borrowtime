import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import Header from './components/layout/Header'
import HomePage from './pages/HomePage'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import TransferPage from './pages/TransferPage'
import ReceiverPage from './pages/ReceiverPage'
import HistoryPage from './pages/HistoryPage'
import UsersPage from './pages/UsersPage'
import AuditPage from './pages/AuditPage'
import AdminSettingsPage from './pages/AdminSettingsPage'
import TwoFAPage from './pages/TwoFAPage'
import { AuthProvider, useAuth } from './context/AuthContext'
import type { ReactNode } from 'react'

type ProtectedRouteProps = {
  children: ReactNode
  adminOnly?: boolean
}

function ProtectedRoute({ children, adminOnly = false }: ProtectedRouteProps) {
  const { user, loading } = useAuth()

  if (loading) return null

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (adminOnly && user.role !== 'admin') {
    return <Navigate to="/transfer" replace />
  }

  return children
}

function AppRoutes() {
  return (
    <>
      <Header />

      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        <Route
          path="/transfer"
          element={
            <ProtectedRoute>
              <TransferPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/receiver"
          element={
            <ProtectedRoute>
              <ReceiverPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/history"
          element={
            <ProtectedRoute>
              <HistoryPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/control"
          element={
            <ProtectedRoute adminOnly>
              <UsersPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/audit"
          element={
            <ProtectedRoute adminOnly>
              <AuditPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/settings"
          element={
            <ProtectedRoute adminOnly>
              <AdminSettingsPage />
            </ProtectedRoute>
          }
        />

        <Route
          path="/2fa"
          element={
            <ProtectedRoute>
              <TwoFAPage />
            </ProtectedRoute>
          }
        />
      </Routes>
    </>
  )
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
