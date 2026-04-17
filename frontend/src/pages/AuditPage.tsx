import { useEffect, useState } from "react"
import { apiListAudit } from "../api/client"
import type { AuditEvent } from "../api/client"

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditEvent[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [error, setError] = useState("")

  useEffect(() => {
    apiListAudit()
      .then((res) => {
        setLogs(res.events || [])
        if (res.events?.length > 0) setSelectedId(res.events[0].ID)
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Ошибка загрузки"))
  }, [])

  const selectedLog = logs.find((item) => item.ID === selectedId)

  const getStatusClass = (success: boolean): string => {
    return success ? "statusDot--success" : "statusDot--danger"
  }

  return (
    <section className="appPage appPage--wide">
      <h1 className="auditTitle">Журнал аудита</h1>

      {error && <div className="errorText">{error}</div>}

      <div className="auditLayout auditLayout--compactFix">
        <aside className="auditTableCard auditTableCard--compactFix">
          <div className="auditTableHead auditTableHead--compactFix">
            <span>Дата</span>
            <span>Тип события</span>
            <span>Статус</span>
          </div>

          <div className="auditRows">
            {logs.map((item) => (
              <button
                key={item.ID}
                type="button"
                className={`auditRow auditRow--compactFix ${selectedId === item.ID ? "auditRow--active" : ""}`}
                onClick={() => setSelectedId(item.ID)}
              >
                <span>{new Date(item.CreatedAt).toLocaleString("ru-RU")}</span>
                <span>{item.EventType}</span>
                <span className="auditStatusCell">
                  <span className={`statusDot ${getStatusClass(item.Success)}`} />
                </span>
              </button>
            ))}
          </div>
        </aside>

        <div className="auditInfoCard auditInfoCard--compactFix">
          {!selectedLog && <div className="emptyState emptyState--large">Выберите запись слева</div>}

          {selectedLog && (
            <>
              <h3>{selectedLog.EventType}</h3>

              <div className="auditInfoMeta auditInfoMeta--compactFix">
                <div>
                  <span>Актор</span>
                  <strong>{selectedLog.ActorID}</strong>
                </div>

                <div>
                  <span>Дата</span>
                  <strong>{new Date(selectedLog.CreatedAt).toLocaleString("ru-RU")}</strong>
                </div>

                <div>
                  <span>IP</span>
                  <strong>{selectedLog.IPAddress}</strong>
                </div>
              </div>

              <p>{selectedLog.Details}</p>
            </>
          )}
        </div>
      </div>
    </section>
  )
}
