import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function Register() {
  const [form, setForm] = useState({ username: '', email: '', password: '' })
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      await api.register(form)
      setSuccess(true)
    } catch (err) {
      setError(err.message)
    } finally { setLoading(false) }
  }

  if (success) return (
    <main className="container" style={{ maxWidth: 420, padding: '48px 16px' }}>
      <div className="card" style={{ padding: 28, textAlign: 'center' }}>
        <div style={{ fontSize: '2.5rem', marginBottom: 12 }}>✅</div>
        <h2 style={{ fontWeight: 700, marginBottom: 8 }}>Registration Sent</h2>
        <p style={{ color: 'var(--text2)', fontSize: '0.92rem' }}>Your account is pending admin approval. You'll be able to log in once approved.</p>
        <Link to="/login" className="btn btn-primary" style={{ marginTop: 20 }}>Back to Sign In</Link>
      </div>
    </main>
  )

  return (
    <main className="container" style={{ maxWidth: 420, padding: '48px 16px' }}>
      <div className="card" style={{ padding: 28 }}>
        <h1 style={{ fontWeight: 700, fontSize: '1.3rem', marginBottom: 20 }}>Create Account</h1>
        {error && <div style={{ background: '#fee2e2', color: '#991b1b', padding: '10px 14px', borderRadius: 8, marginBottom: 16, fontSize: '0.9rem' }}>{error}</div>}
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <label style={{ fontSize: '0.85rem', fontWeight: 500, display: 'block', marginBottom: 6 }}>Username</label>
            <input value={form.username} onChange={e => setForm(f => ({...f, username: e.target.value}))} required />
          </div>
          <div>
            <label style={{ fontSize: '0.85rem', fontWeight: 500, display: 'block', marginBottom: 6 }}>Email</label>
            <input type="email" value={form.email} onChange={e => setForm(f => ({...f, email: e.target.value}))} required />
          </div>
          <div>
            <label style={{ fontSize: '0.85rem', fontWeight: 500, display: 'block', marginBottom: 6 }}>Password</label>
            <input type="password" value={form.password} onChange={e => setForm(f => ({...f, password: e.target.value}))} required minLength={6} />
          </div>
          <button className="btn btn-primary" type="submit" disabled={loading} style={{ marginTop: 4 }}>
            {loading ? 'Registering…' : 'Create Account'}
          </button>
        </form>
        <p style={{ marginTop: 18, fontSize: '0.88rem', color: 'var(--text2)', textAlign: 'center' }}>
          Already have an account? <Link to="/login">Sign In</Link>
        </p>
      </div>
    </main>
  )
}