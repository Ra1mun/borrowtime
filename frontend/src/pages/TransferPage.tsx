import { useEffect, useMemo, useState } from "react"
import type { ChangeEvent, SyntheticEvent } from "react"
import FormCard from "../components/ui/FormCard"
import { apiCreateTransfer, apiSearchUsers } from "../api/client"
import type { UserListItem } from "../api/client"

export default function TransferPage() {
  const [hours, setHours] = useState(24)
  const [recipientQuery, setRecipientQuery] = useState("")
  const [selectedReceiver, setSelectedReceiver] = useState<UserListItem | null>(null)
  const [receivers, setReceivers] = useState<UserListItem[]>([])
  const [token, setToken] = useState("")
  const [file, setFile] = useState<File | null>(null)
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    const q = recipientQuery.trim()
    if (!q || selectedReceiver) {
      setReceivers([])
      return
    }
    const timeout = setTimeout(() => {
      apiSearchUsers(q).then(setReceivers).catch(() => setReceivers([]))
    }, 300)
    return () => clearTimeout(timeout)
  }, [recipientQuery, selectedReceiver])

  const filteredReceivers = useMemo(() => receivers.slice(0, 5), [receivers])

  const handleSelectReceiver = (user: UserListItem): void => {
    setSelectedReceiver(user)
    setRecipientQuery(user.email)
  }

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setFile(event.target.files?.[0] ?? null)
  }

  const handleSubmit = async (event: SyntheticEvent): Promise<void> => {
    event.preventDefault()
    setError("")
    setMessage("")

    if (!selectedReceiver) {
      setError("Выберите получателя из списка подсказок.")
      return
    }

    if (!file) {
      setError("Добавьте файл для передачи.")
      return
    }

    if (!token.trim()) {
      setError("Введите токен для доступа.")
      return
    }

    try {
      await apiCreateTransfer(file, hours, [selectedReceiver.email])
      setMessage(`Файл ${file.name} отправлен пользователю ${selectedReceiver.email}.`)
      setToken("")
      setRecipientQuery("")
      setSelectedReceiver(null)
      setFile(null)

      const fileInput = document.getElementById("transfer-file-input") as HTMLInputElement | null
      if (fileInput) fileInput.value = ""
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка при создании передачи.")
    }
  }

  return (
    <section className="appPage appPage--wide">
      <FormCard title="Передача конфиденциальной информации">
        <div className="transferLayout">
          <form className="transferMainCard" onSubmit={handleSubmit}>
            <div className="fieldBlock">
              <div className="sliderCard sliderCard--flatFix">
                <label htmlFor="hours-range">Срок доступа к скачиванию</label>
                <input
                  id="hours-range"
                  className="rangeInput"
                  type="range"
                  min="1"
                  max="72"
                  value={hours}
                  onChange={(event) => setHours(Number(event.target.value))}
                />

                <div className="sliderMeta">
                  <span>1 час</span>
                  <strong>{hours} ч</strong>
                  <span>72 часа</span>
                </div>
              </div>
            </div>

            <div className="fieldBlock autocompleteBlock">
              <label htmlFor="receiver-input">Получатель</label>
              <input
                id="receiver-input"
                className="input"
                type="text"
                value={recipientQuery}
                onChange={(event) => {
                  setRecipientQuery(event.target.value)
                  setSelectedReceiver(null)
                }}
                placeholder="Начните вводить почту"
                autoComplete="off"
              />

              {filteredReceivers.length > 0 && recipientQuery.trim() && !selectedReceiver && (
                <div className="suggestionsBox">
                  {filteredReceivers.map((user) => (
                    <button
                      key={user.id}
                      type="button"
                      className="suggestionItem"
                      onClick={() => handleSelectReceiver(user)}
                    >
                      <strong>{user.email}</strong>
                      <small>{user.role}</small>
                    </button>
                  ))}
                </div>
              )}
            </div>

            <div className="fieldGridTwo fieldGridTwo--alignedFix">
              <div className="fieldBlock fieldBlock--tightTopFix">
                <label htmlFor="transfer-file-input">Добавьте файл</label>
                <input
                  id="transfer-file-input"
                  className="fileInput"
                  type="file"
                  onChange={handleFileChange}
                />
              </div>

              <div className="fieldBlock fieldBlock--tightTopFix">
                <label htmlFor="transfer-token">Токен доступа</label>
                <input
                  id="transfer-token"
                  className="input"
                  type="text"
                  value={token}
                  onChange={(event) => setToken(event.target.value)}
                  placeholder="Например: TOKEN-123"
                />
              </div>
            </div>

            {error && <div className="errorText">{error}</div>}
            {message && <div className="successText">{message}</div>}

            <button className="primaryBtn" type="submit">
              Отправить файл
            </button>
          </form>

          <aside className="transferSideCard">
            <h3>Как это работает</h3>
            <ul className="sideStepsList">
              <li>Страницы передачи и получения доступны только после входа или регистрации.</li>
              <li>Срок действия ссылки выбирается слайдером в часах.</li>
              <li>Получатель подбирается через подсказки по зарегистрированным пользователям.</li>
              <li>После отправки файл появится у получателя в разделе «Получение данных».</li>
            </ul>
          </aside>
        </div>
      </FormCard>
    </section>
  )
}
