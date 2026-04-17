import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import FormCard from '../components/ui/FormCard'
import { useAuth } from '../context/AuthContext'
import type { SyntheticEvent } from 'react'

export default function RegisterPage() {
  const navigate = useNavigate()
  const { register } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')

  const handleSubmit = async (event: SyntheticEvent) => {
    event.preventDefault()
    setError('')

    if (!email || !password || !confirmPassword) {
      setError('Заполните все поля.')
      return
    }

    if (password.length < 8) {
      setError('Пароль должен содержать минимум 8 символов.')
      return
    }

    if (password !== confirmPassword) {
      setError('Пароли не совпадают.')
      return
    }

    try {
      await register(email, password)
      navigate('/transfer')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось зарегистрироваться.')
    }
  }

  return (
    <FormCard title="Регистрация">
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

        <label>Подтвердите пароль</label>
        <input
          className="input"
          type="password"
          value={confirmPassword}
          onChange={(event) => setConfirmPassword(event.target.value)}
        />

        {error && <div className="errorText">{error}</div>}

        <button className="primaryBtn" type="submit">
          Создать аккаунт
        </button>
      </form>
    </FormCard>
  )
}
