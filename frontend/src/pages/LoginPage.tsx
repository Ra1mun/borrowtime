import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import FormCard from '../components/ui/FormCard'
import { useAuth } from '../context/AuthContext'
import type { SyntheticEvent } from 'react'

export default function LoginPage() {
  const navigate = useNavigate()
  const { login, verify2FA } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  // 2FA state
  const [partialJwt, setPartialJwt] = useState<string | null>(null)
  const [totpCode, setTotpCode] = useState('')

  const handleSubmit = async (event: SyntheticEvent) => {
    event.preventDefault()
    setError('')

    try {
      const result = await login(email, password)
      if (result.twoFA === true) {
        setPartialJwt(result.partial_jwt)
      } else {
        navigate(result.user.role === 'admin' ? '/audit' : '/transfer')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось войти.')
    }
  }

  const handleVerify2FA = async (event: SyntheticEvent) => {
    event.preventDefault()
    setError('')

    if (!partialJwt) return

    try {
      const user = await verify2FA(partialJwt, totpCode)
      navigate(user.role === 'admin' ? '/audit' : '/transfer')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Неверный код.')
      setTotpCode('')
    }
  }

  // 2FA code entry screen
  if (partialJwt) {
    return (
      <FormCard title="Двухфакторная аутентификация">
        <form onSubmit={handleVerify2FA} className="formStack">
          <p className="formHint">
            Введите 6-значный код из приложения-аутентификатора.
          </p>

          <label>Код подтверждения</label>
          <input
            className="input"
            type="text"
            inputMode="numeric"
            maxLength={6}
            autoFocus
            value={totpCode}
            onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
            placeholder="000000"
          />

          {error && <div className="errorText">{error}</div>}

          <button className="primaryBtn" type="submit" disabled={totpCode.length !== 6}>
            Подтвердить
          </button>

          <button
            className="secondaryBtn"
            type="button"
            onClick={() => { setPartialJwt(null); setTotpCode(''); setError('') }}
          >
            Назад
          </button>
        </form>
      </FormCard>
    )
  }

  return (
    <FormCard title="Вход в аккаунт">
      <form onSubmit={handleSubmit} className="formStack">
        <label>Почта</label>
        <input className="input" value={email} onChange={(event) => setEmail(event.target.value)} />

        <label>Пароль</label>
        <input
          className="input"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />

        {error && <div className="errorText">{error}</div>}

        <button className="primaryBtn" type="submit">
          Войти
        </button>
      </form>
    </FormCard>
  )
}
