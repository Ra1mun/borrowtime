import { useEffect, useState } from "react"
import FormCard from "../components/ui/FormCard"
import { apiListTransfers, apiRevokeTransfer } from "../api/client"
import type { TransferInfo } from "../api/client"

export default function HistoryPage() {
  const [transfers, setTransfers] = useState<TransferInfo[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")

  const loadTransfers = async () => {
    try {
      const list = await apiListTransfers()
      setTransfers(list || [])
      if (list?.length > 0 && !selectedId) setSelectedId(list[0].id)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка загрузки")
    }
  }

  useEffect(() => {
    loadTransfers()
  }, [])

  const selectedTransfer = transfers.find((t) => t.id === selectedId)

  const isExpired = (t: TransferInfo) => new Date(t.expires_at).getTime() < Date.now()

  const getStatusLabel = (t: TransferInfo) => {
    if (t.status === "REVOKED") return "Отозван"
    if (t.status === "EXPIRED" || isExpired(t)) return "Истёк"
    if (t.status === "DOWNLOADED") return "Скачан"
    return "Активен"
  }

  const getStatusClass = (t: TransferInfo) => {
    const label = getStatusLabel(t)
    if (label === "Активен") return "statusDot--success"
    if (label === "Отозван") return "statusDot--danger"
    return "statusDot--muted"
  }

  const handleRevoke = async () => {
    if (!selectedTransfer) return
    try {
      await apiRevokeTransfer(selectedTransfer.id)
      setMessage("Передача отозвана.")
      await loadTransfers()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка отзыва")
    }
  }

  return (
    <section className="appPage appPage--wide">
      <FormCard title="История передач">
        <div className="auditLayout auditLayout--compactFix">
          <aside className="auditTableCard auditTableCard--compactFix">
            <div className="auditTableHead auditTableHead--compactFix">
              <span>Дата</span>
              <span>Файл</span>
              <span>Статус</span>
            </div>

            <div className="auditRows">
              {transfers.length === 0 && (
                <div className="emptyState">У вас пока нет передач.</div>
              )}
              {transfers.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className={`auditRow auditRow--compactFix ${selectedId === t.id ? "auditRow--active" : ""}`}
                  onClick={() => setSelectedId(t.id)}
                >
                  <span>{new Date(t.created_at).toLocaleString("ru-RU")}</span>
                  <span>{t.file_name}</span>
                  <span className="auditStatusCell">
                    <span className={`statusDot ${getStatusClass(t)}`} />
                  </span>
                </button>
              ))}
            </div>
          </aside>

          <div className="auditInfoCard auditInfoCard--compactFix">
            {!selectedTransfer && <div className="emptyState emptyState--large">Выберите передачу слева</div>}

            {selectedTransfer && (
              <>
                <h3>{selectedTransfer.file_name}</h3>

                <div className="auditInfoMeta auditInfoMeta--compactFix">
                  <div>
                    <span>Статус</span>
                    <strong>{getStatusLabel(selectedTransfer)}</strong>
                  </div>
                  <div>
                    <span>Скачиваний</span>
                    <strong>{selectedTransfer.download_count}</strong>
                  </div>
                  <div>
                    <span>Размер</span>
                    <strong>{(selectedTransfer.file_size / 1024).toFixed(1)} КБ</strong>
                  </div>
                  <div>
                    <span>Истекает</span>
                    <strong>{new Date(selectedTransfer.expires_at).toLocaleString("ru-RU")}</strong>
                  </div>
                </div>

                {selectedTransfer.status === "ACTIVE" && !isExpired(selectedTransfer) && (
                  <button className="secondaryBtn" type="button" onClick={handleRevoke}>
                    Отозвать доступ
                  </button>
                )}

                {error && <div className="errorText">{error}</div>}
                {message && <div className="successText">{message}</div>}
              </>
            )}
          </div>
        </div>
      </FormCard>
    </section>
  )
}
