import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

export default function Header() {
  const navigate = useNavigate()
  const { user, logout } = useAuth()

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <header className="header">
      <nav className="header__nav">
        <div className="header__left">
            <div className="header__links">
            <NavLink to="/">Главная</NavLink>
            <NavLink to="/transfer">Передача данных</NavLink>
            <NavLink to="/receiver">Получение данных</NavLink>
            {user?.role === 'admin' && <NavLink to="/control">Пользователи</NavLink>}
            {user?.role === 'admin' && <NavLink to="/audit">Журнал аудита</NavLink>}
            {user && <NavLink to="/2fa">2FA</NavLink>}
          </div>
        </div>

        <div className="header__right">
          {user ? (
            <>
              <div className="header__userBox">
                <span className="header__nickname">{user.email}</span>
                <span className={`header__role header__role--${user.role}`}>
                  {user.role === 'admin' ? 'admin' : 'user'}
                </span>
              </div>
              <button className="header__button header__button--ghost" onClick={handleLogout}>
                Выйти
              </button>
            </>
          ) : (
            <>
              <NavLink to="/login" className="header__button header__button--ghost">
                Вход
              </NavLink>
              <NavLink to="/register" className="header__button header__button--primary">
                Регистрация
              </NavLink>
            </>
          )}
        </div>
      </nav>
    </header>
  )
}
