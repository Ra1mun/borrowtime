import { useEffect, useState } from "react"
import FormCard from "../components/ui/FormCard"
import { apiGetSettings, apiUpdateSettings, apiGetStats } from "../api/client"
import type { GlobalSettings } from "../api/client"

export default function AdminSettingsPage() {
  const [settings, setSettings] = useState<GlobalSettings | null>(null)
  const [stats, setStats] = useState<{ active_transfers: number; total_storage_bytes: number; security_incidents_today: number } | null>(null)
  const [form, setForm] = useState({
    max_file_size_mb: 0,
    max_retention_days: 0,
    default_retention_h: 0,
    default_max_downloads: 0,
  })
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    apiGetSettings()
      .then((s) => {
        setSettings(s)
        setForm({
          max_file_size_mb: s.max_file_size_mb,
          max_retention_days: s.max_retention_days,
          default_retention_h: s.default_retention_h,
          default_max_downloads: s.default_max_downloads,
        })
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Ошибка загрузки настроек"))

    apiGetStats()
      .then(setStats)
      .catch(() => {})
  }, [])

  const handleSave = async () => {
    setError("")
    setMessage("")
    try {
      const updated = await apiUpdateSettings(form)
      setSettings(updated)
      setMessage("Настройки сохранены.")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка сохранения")
    }
  }

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} Б`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} КБ`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} ГБ`
  }

  return (
    <section className="appPage appPage--wide">
      <div className="usersTopTitle">Глобальные настройки</div>

      <div className="usersPageShell">
        <aside className="usersSidebar">
          <div className="settingsStatBlock">
            <h3>Статистика системы</h3>
            {stats ? (
              <div className="userStatsGrid">
                <div className="userStatCard">
                  <span>Активные передачи</span>
                  <strong>{stats.active_transfers}</strong>
                </div>
                <div className="userStatCard">
                  <span>Объём хранилища</span>
                  <strong>{formatBytes(stats.total_storage_bytes)}</strong>
                </div>
                <div className="userStatCard">
                  <span>Инциденты за сегодня</span>
                  <strong>{stats.security_incidents_today}</strong>
                </div>
              </div>
            ) : (
              <div className="emptyState">Загрузка...</div>
            )}
          </div>

          {settings && (
            <div className="settingsStatBlock settingsStatBlock--spaced">
              <small>Последнее обновление: {new Date(settings.updated_at).toLocaleString("ru-RU")}</small>
            </div>
          )}
        </aside>

        <main className="usersDetailsPanel">
          <FormCard title="Параметры">
            <div className="fieldBlock">
              <label htmlFor="settings-max-file">Макс. размер файла (МБ)</label>
              <input
                id="settings-max-file"
                className="input"
                type="number"
                min="1"
                value={form.max_file_size_mb}
                onChange={(e) => setForm({ ...form, max_file_size_mb: Number(e.target.value) })}
              />
            </div>

            <div className="fieldBlock">
              <label htmlFor="settings-max-ret">Макс. срок хранения (дни)</label>
              <input
                id="settings-max-ret"
                className="input"
                type="number"
                min="1"
                value={form.max_retention_days}
                onChange={(e) => setForm({ ...form, max_retention_days: Number(e.target.value) })}
              />
            </div>

            <div className="fieldBlock">
              <label htmlFor="settings-def-ret">Срок по умолчанию (часы)</label>
              <input
                id="settings-def-ret"
                className="input"
                type="number"
                min="1"
                value={form.default_retention_h}
                onChange={(e) => setForm({ ...form, default_retention_h: Number(e.target.value) })}
              />
            </div>

            <div className="fieldBlock">
              <label htmlFor="settings-def-downloads">Макс. скачиваний по умолчанию</label>
              <input
                id="settings-def-downloads"
                className="input"
                type="number"
                min="0"
                value={form.default_max_downloads}
                onChange={(e) => setForm({ ...form, default_max_downloads: Number(e.target.value) })}
              />
            </div>

            {error && <div className="errorText">{error}</div>}
            {message && <div className="successText">{message}</div>}

            <button className="primaryBtn" type="button" onClick={handleSave}>
              Сохранить настройки
            </button>
          </FormCard>
        </main>
      </div>
    </section>
  )
}
