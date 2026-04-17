import { useEffect, useState } from 'react'
import FormCard from '../components/ui/FormCard'
import { apiListUsers, apiDeleteUser, apiUpdateUserRole } from '../api/client'
import type { UserListItem } from '../api/client'
import { useAuth } from '../context/AuthContext'

export default function UsersPage() {
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<UserListItem[]>([])
  const [selectedUser, setSelectedUser] = useState<UserListItem | null>(null)
  const [error, setError] = useState('')

  const loadUsers = async () => {
    try {
      const list = await apiListUsers()
      const filtered = list.filter((u) => u.id !== currentUser?.id)
      setUsers(filtered)
      if (filtered.length > 0 && !selectedUser) {
        setSelectedUser(filtered[0])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки пользователей')
    }
  }

  useEffect(() => {
    loadUsers()
  }, [])

  const handleRoleChange = async () => {
    if (!selectedUser) return
    const nextRole = selectedUser.role === 'admin' ? 'user' : 'admin'
    try {
      await apiUpdateUserRole(selectedUser.id, nextRole)
      await loadUsers()
      setSelectedUser((prev) => prev ? { ...prev, role: nextRole } : null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка смены роли')
    }
  }

  const handleDelete = async () => {
    if (!selectedUser) return
    try {
      await apiDeleteUser(selectedUser.id)
      setSelectedUser(null)
      await loadUsers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления')
    }
  }

  return (
    <section className="appPage appPage--wide">
      <div className="usersTopTitle">Управление пользователями</div>

      {error && <div className="errorText">{error}</div>}

      <div className="usersPageShell">
        <aside className="usersSidebar">
          {users.map((user) => (
            <button
              key={user.id}
              type="button"
              onClick={() => setSelectedUser(user)}
              className={`usersSidebar__item ${selectedUser?.id === user.id ? 'usersSidebar__item--active' : ''}`}
            >
              <div>{user.email}</div>
              <small>{user.role}</small>
            </button>
          ))}
        </aside>

        <main className="usersDetailsPanel">
          {selectedUser ? (
            <>
              <div className="usersDetailsHeader">
                <div>
                  <h2>{selectedUser.email}</h2>
                </div>
                <span className={`rolePill rolePill--${selectedUser.role}`}>
                  {selectedUser.role === 'admin' ? 'Администратор' : 'Пользователь'}
                </span>
              </div>

              <div className="userStatsGrid">
                <div className="userStatCard">
                  <span>Роль</span>
                  <strong>{selectedUser.role}</strong>
                </div>
                <div className="userStatCard">
                  <span>Дата создания</span>
                  <strong>{new Date(selectedUser.created_at).toLocaleDateString('ru-RU')}</strong>
                </div>
              </div>

              <div className="usersActions">
                <button className="secondaryBtn" type="button" onClick={handleDelete}>
                  Удалить
                </button>
                <button className="primaryBtn" type="button" onClick={handleRoleChange}>
                  Изменить роль
                </button>
              </div>
            </>
          ) : (
            <FormCard title="Пользователи">
              <div className="emptyState">Нет доступных пользователей.</div>
            </FormCard>
          )}
        </main>
      </div>
    </section>
  )
}
