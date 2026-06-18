import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.jsx'

export default function Nav() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => { logout(); navigate('/') }

  return (
    <header style={{
      background: 'var(--card)',
      borderBottom: '1px solid var(--border)',
      position: 'sticky', top: 0, zIndex: 100,
    }}>
      <div className="container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px' }}>
        <Link to="/" style={{ fontWeight: 700, fontSize: '1.1rem', color: 'var(--text)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ color: 'var(--accent)' }}>⬡</span> Peterson Software
        </Link>
        <nav style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {user?.role === 'admin' && <Link to="/admin" className="btn btn-ghost btn-sm">Admin</Link>}
          {user ? (
            <>
              <span style={{ fontSize: '0.85rem', color: 'var(--text2)', display: 'none' }} className="desktop-only">{user.username}</span>
              <button className="btn btn-ghost btn-sm" onClick={handleLogout}>Sign Out</button>
            </>
          ) : (
            <>
              <Link to="/login" className="btn btn-ghost btn-sm">Sign In</Link>
              <Link to="/register" className="btn btn-primary btn-sm">Register</Link>
            </>
          )}
        </nav>
      </div>
    </header>
  )
}