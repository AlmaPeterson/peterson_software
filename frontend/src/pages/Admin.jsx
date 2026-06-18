import { useEffect, useState } from 'react'
import { api } from '../lib/api'

const PLATFORMS = ['Android', 'iOS', 'Windows', 'Mac', 'Linux', 'Other']

function UploadForm({ onSuccess }) {
  const [form, setForm] = useState({ name: '', platform: 'Android', version: '', description: '', is_public: true })
  const [file, setFile] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!file) { setError('Please select a file'); return }
    setError(''); setLoading(true)
    const fd = new FormData()
    Object.entries(form).forEach(([k, v]) => fd.append(k, v))
    fd.append('file', file)
    try {
      await api.uploadApp(fd)
      setForm({ name: '', platform: 'Android', version: '', description: '', is_public: true })
      setFile(null)
      e.target.reset()
      onSuccess()
    } catch (err) {
      setError(err.message)
    } finally { setLoading(false) }
  }

  return (
    <div className="card" style={{ padding: 20, marginBottom: 24 }}>
      <h2 style={{ fontWeight: 600, marginBottom: 16 }}>Upload New Release</h2>
      {error && <div style={{ background: '#fee2e2', color: '#991b1b', padding: '8px 12px', borderRadius: 8, marginBottom: 12, fontSize: '0.88rem' }}>{error}</div>}
      <form onSubmit={handleSubmit} style={{ display: 'grid', gap: 12 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div>
            <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>App Name *</label>
            <input value={form.name} onChange={e => setForm(f => ({...f, name: e.target.value}))} required placeholder="My App" />
          </div>
          <div>
            <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>Version *</label>
            <input value={form.version} onChange={e => setForm(f => ({...f, version: e.target.value}))} required placeholder="1.0.0" />
          </div>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div>
            <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>Platform *</label>
            <select value={form.platform} onChange={e => setForm(f => ({...f, platform: e.target.value}))}>
              {PLATFORMS.map(p => <option key={p}>{p}</option>)}
            </select>
          </div>
          <div>
            <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>Visibility</label>
            <select value={form.is_public} onChange={e => setForm(f => ({...f, is_public: e.target.value === 'true'}))}>
              <option value="true">Public</option>
              <option value="false">Private (login required)</option>
            </select>
          </div>
        </div>
        <div>
          <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>Description</label>
          <input value={form.description} onChange={e => setForm(f => ({...f, description: e.target.value}))} placeholder="Optional short description" />
        </div>
        <div>
          <label style={{ fontSize: '0.82rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>File *</label>
          <input type="file" onChange={e => setFile(e.target.files[0])} required style={{ padding: '8px 0', border: 'none', background: 'transparent' }} />
        </div>
        <button className="btn btn-primary" type="submit" disabled={loading}>
          {loading ? 'Uploading…' : '⬆ Upload Release'}
        </button>
      </form>
    </div>
  )
}

function AppsList({ apps, onDelete }) {
  return (
    <div className="card" style={{ padding: 20, marginBottom: 24 }}>
      <h2 style={{ fontWeight: 600, marginBottom: 16 }}>Releases ({apps.length})</h2>
      {apps.length === 0 && <p style={{ color: 'var(--text2)', fontSize: '0.9rem' }}>No releases yet.</p>}
      <div style={{ display: 'grid', gap: 10 }}>
        {apps.map(app => (
          <div key={app.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, padding: '10px 0', borderBottom: '1px solid var(--border)' }}>
            <div>
              <span style={{ fontWeight: 500 }}>{app.name}</span>
              <span style={{ color: 'var(--text2)', fontSize: '0.82rem', marginLeft: 8 }}>v{app.version} · {app.platform}</span>
              <span style={{ marginLeft: 8 }} className={`badge ${app.is_public ? 'badge-public' : 'badge-lock'}`}>{app.is_public ? 'Public' : 'Private'}</span>
            </div>
            <button className="btn btn-danger btn-sm" onClick={() => onDelete(app.id)}>Delete</button>
          </div>
        ))}
      </div>
    </div>
  )
}

function UsersList({ users, onRoleChange, onDelete }) {
  return (
    <div className="card" style={{ padding: 20 }}>
      <h2 style={{ fontWeight: 600, marginBottom: 16 }}>Users ({users.length})</h2>
      <div style={{ display: 'grid', gap: 10 }}>
        {users.map(u => (
          <div key={u.id} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 0', borderBottom: '1px solid var(--border)', flexWrap: 'wrap' }}>
            <div style={{ flex: 1 }}>
              <span style={{ fontWeight: 500 }}>{u.username}</span>
              <span style={{ color: 'var(--text2)', fontSize: '0.82rem', marginLeft: 8 }}>{u.email}</span>
            </div>
            <select
              value={u.role}
              onChange={e => onRoleChange(u.id, e.target.value)}
              style={{ width: 'auto', padding: '4px 8px', fontSize: '0.82rem' }}>
              <option value="pending">Pending</option>
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
            {u.role !== 'admin' && (
              <button className="btn btn-danger btn-sm" onClick={() => onDelete(u.id)}>Remove</button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export default function Admin() {
  const [apps, setApps] = useState([])
  const [users, setUsers] = useState([])
  const [tab, setTab] = useState('apps')

  const loadApps = () => api.getApps().then(setApps).catch(console.error)
  const loadUsers = () => api.getUsers().then(setUsers).catch(console.error)

  useEffect(() => { loadApps(); loadUsers() }, [])

  const handleDelete = async (id) => {
    if (!confirm('Delete this release?')) return
    await api.deleteApp(id)
    loadApps()
  }

  const handleRoleChange = async (id, role) => {
    await api.updateRole(id, role)
    loadUsers()
  }

  const handleDeleteUser = async (id) => {
    if (!confirm('Remove this user?')) return
    await api.deleteUser(id)
    loadUsers()
  }

  const pendingCount = users.filter(u => u.role === 'pending').length

  return (
    <main className="container" style={{ padding: '24px 16px' }}>
      <h1 style={{ fontWeight: 700, fontSize: '1.4rem', marginBottom: 20 }}>Admin Panel</h1>

      <div style={{ display: 'flex', gap: 8, marginBottom: 20 }}>
        <button onClick={() => setTab('apps')} className={`btn btn-sm ${tab === 'apps' ? 'btn-primary' : 'btn-ghost'}`}>📦 Releases</button>
        <button onClick={() => setTab('users')} className={`btn btn-sm ${tab === 'users' ? 'btn-primary' : 'btn-ghost'}`}>
          👥 Users {pendingCount > 0 && <span style={{ background: '#dc2626', color: '#fff', borderRadius: 999, padding: '1px 6px', fontSize: '0.72rem', marginLeft: 4 }}>{pendingCount}</span>}
        </button>
      </div>

      {tab === 'apps' && <>
        <UploadForm onSuccess={loadApps} />
        <AppsList apps={apps} onDelete={handleDelete} />
      </>}
      {tab === 'users' && <UsersList users={users} onRoleChange={handleRoleChange} onDelete={handleDeleteUser} />}
    </main>
  )
}