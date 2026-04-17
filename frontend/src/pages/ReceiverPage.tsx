import { useEffect, useMemo, useState } from "react"
import FormCard from "../components/ui/FormCard"
import { apiListIncomingTransfers, apiDownloadFile } from "../api/client"
import type { TransferInfo } from "../api/client"

type PreparedTransferItem = TransferInfo & { isExpired: boolean }

export default function ReceiverPage() {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [items, setItems] = useState<TransferInfo[]>([])

  useEffect(() => {
    apiListIncomingTransfers()
      .then((list) => setItems(list))
      .catch(() => setItems([]))
  }, [])

  const preparedItems = useMemo<PreparedTransferItem[]>(() => {
    const currentTime = new Date().getTime()

    return items.map((item) => ({
      ...item,
      isExpired: new Date(item.expires_at).getTime() < currentTime,
    }))
  }, [items])

  const selectedItem = preparedItems.find((item) => item.id === selectedId)

  const handleDownload = (): void => {
    if (!selectedItem || selectedItem.isExpired) {
      return
    }

    apiDownloadFile(selectedItem.access_token).catch(() => {
      alert('Ошибка при скачивании файла')
    })
  }

  return (
    <section className="appPage appPage--wide">
      <FormCard title="Полученные файлы">
        <div className="mailLayout mailLayout--balancedFix">
          <aside className="mailSidebar">
            <div className="mailSidebar__title">Токены</div>

            {preparedItems.length === 0 && (
              <div className="emptyState">У вас пока нет входящих файлов.</div>
            )}

            {preparedItems.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`mailTokenCard ${selectedId === item.id ? "mailTokenCard--active" : ""}`}
                onClick={() => setSelectedId(item.id)}
              >
                <strong>{item.access_token}</strong>
                <span>{item.file_name}</span>
                <small>{item.isExpired ? "Срок истёк" : "Доступен"}</small>
              </button>
            ))}
          </aside>

          <div className="mailContent mailContent--balancedFix">

            {!selectedItem && <div className="emptyState">Выберите токен слева</div>}

            {selectedItem && (
              <>
                <div className="mailHeaderRow">
                  <h3>{selectedItem.file_name}</h3>
                  <div className="tokenBadge">{selectedItem.access_token}</div>
                </div>

                <div className="mailMetaGrid">
                  <div className="mailMetaCard">
                    <span>Статус</span>
                    <strong>{selectedItem.status}</strong>
                  </div>

                  <div className="mailMetaCard">
                    <span>Срок действия до</span>
                    <strong>{new Date(selectedItem.expires_at).toLocaleString()}</strong>
                  </div>
                </div>

                <div className="mailBodyCard">
                  <p>Файл готов к скачиванию и доступен по выбранному токену доступа.</p>
                </div>

                <button
                  className="primaryBtn"
                  disabled={selectedItem.isExpired}
                  onClick={handleDownload}
                  type="button"
                >
                  Скачать
                </button>
              </>
            )}
          </div>
        </div>
      </FormCard>
    </section>
  )
}
