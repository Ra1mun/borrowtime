import { useState } from 'react'
import FormCard from '../components/ui/FormCard'
import { apiSetup2FA, apiConfirm2FA, apiDisable2FA } from '../api/client'
import { useAuth } from '../context/AuthContext'

export default function TwoFAPage() {
  const { user, refreshUser } = useAuth()
  const [step, setStep] = useState<'idle' | 'setup' | 'disable'>('idle')
  const [secret, setSecret] = useState('')
  const [provisionUrl, setProvisionUrl] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const handleSetup = async () => {
    setError('')
    setSuccess('')
    try {
      const res = await apiSetup2FA()
      setSecret(res.secret)
      setProvisionUrl(res.provision_url)
      setStep('setup')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка при настройке 2FA.')
    }
  }

  const handleConfirm = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await apiConfirm2FA(code)
      await refreshUser()
      setSuccess('Двухфакторная аутентификация включена!')
      setStep('idle')
      setCode('')
      setSecret('')
      setProvisionUrl('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Неверный код.')
      setCode('')
    }
  }

  const handleDisable = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await apiDisable2FA(code)
      await refreshUser()
      setSuccess('Двухфакторная аутентификация отключена.')
      setStep('idle')
      setCode('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Неверный код.')
      setCode('')
    }
  }

  return (
    <section className="appPage">
      <FormCard title="Двухфакторная аутентификация (2FA)">
        {success && <div className="successText">{success}</div>}
        {error && <div className="errorText">{error}</div>}

        {step === 'idle' && (
          <div className="formStack">
            <p className="formHint">
              Двухфакторная аутентификация добавляет дополнительный уровень защиты вашего аккаунта.
              При входе, помимо пароля, потребуется ввести одноразовый код из приложения-аутентификатора
              (Google Authenticator, Authy и др.).
            </p>

            <div className="twofa-actions">
              <button className="primaryBtn" type="button" onClick={handleSetup} disabled={user?.totp_enabled}>
                Включить 2FA
              </button>
              <button className="secondaryBtn" type="button" onClick={() => { setStep('disable'); setError(''); setSuccess('') }} disabled={!user?.totp_enabled}>
                Отключить 2FA
              </button>
            </div>
          </div>
        )}

        {step === 'setup' && (
          <form onSubmit={handleConfirm} className="formStack">
            <p className="formHint">
              Отсканируйте QR-код или введите секретный ключ вручную в приложении-аутентификаторе.
            </p>

            <div className="twofa-qr">
              <img
                src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(provisionUrl)}`}
                alt="QR-код для 2FA"
                width={200}
                height={200}
              />
            </div>

            <div className="twofa-secret">
              <label>Секретный ключ</label>
              <code className="twofa-secret-code">{secret}</code>
            </div>

            <label>Код подтверждения</label>
            <input
              className="input"
              type="text"
              inputMode="numeric"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
              placeholder="000000"
            />

            <button className="primaryBtn" type="submit" disabled={code.length !== 6}>
              Подтвердить и включить
            </button>

            <button className="secondaryBtn" type="button" onClick={() => { setStep('idle'); setCode('') }}>
              Отмена
            </button>
          </form>
        )}

        {step === 'disable' && (
          <form onSubmit={handleDisable} className="formStack">
            <p className="formHint">
              Для отключения 2FA введите текущий код из приложения-аутентификатора.
            </p>

            <label>Код подтверждения</label>
            <input
              className="input"
              type="text"
              inputMode="numeric"
              maxLength={6}
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
              placeholder="000000"
            />

            <button className="primaryBtn" type="submit" disabled={code.length !== 6}>
              Отключить 2FA
            </button>

            <button className="secondaryBtn" type="button" onClick={() => { setStep('idle'); setCode('') }}>
              Отмена
            </button>
          </form>
        )}
      </FormCard>
    </section>
  )
}
