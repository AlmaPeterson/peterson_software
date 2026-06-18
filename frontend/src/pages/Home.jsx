import { useEffect, useState } from 'react'
import { useAuth } from '../hooks/useAuth.jsx'
import { api } from '../lib/api'

const PLATFORM_ICONS = {
  android: '🤖', ios: '🍎', windows: '🪟', mac: '🍏', linux: '🐧', other: '📦'
}

function platformBadge(p) {
  return <span className={`badge badge-${p.toLowerCase()}`}>{PLATFORM_ICONS[p.toLowerCase()] || '📦'} {p}</span>
}

function formatBytes(bytes) {
  if (!bytes) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export default function Home() {
  const [apps, setApps] = useState([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('all')
  const { user } = useAuth()

  useEffect(() => {
    api.getApps().then(setApps).catch(console.error).finally(() => setLoading(false))
  }, [user])

  const platforms = ['all', ...new Set(apps.map(a => a.platform.toLowerCase()))]
  const filtered = filter === 'all' ? apps : apps.filter(a => a.platform.toLowerCase() === filter)

  const handleDownload = (app) => {
    if (!app.is_public && !user) {
      alert('Please sign in to download this file.')
      return
    }
    window.location.href = api.downloadUrl(app.slug)
  }

  return (
    <main className="container" style={{ padding: '24px 16px' }}>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.6rem', fontWeight: 700, marginBottom: 4 }}>Software Releases</h1>
        <p style={{ color: 'var(--text2)', fontSize: '0.95rem' }}>Download the latest builds for all platforms.</p>
      </div>

      {/* Platform filter */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 20 }}>
        {platforms.map(p => (
          <button key={p} onClick={() => setFilter(p)}
            className="btn btn-sm"
            style={{
              background: filter === p ? 'var(--accent)' : 'var(--bg2)',
              color: filter === p ? '#fff' : 'var(--text2)',
              border: '1px solid var(--border)',
              textTransform: 'capitalize',
            }}>
            {p === 'all' ? '🗂 All' : `${PLATFORM_ICONS[p] || '📦'} ${p}`}
          </button>
        ))}
      </div>

      {loading && <p style={{ color: 'var(--text2)' }}>Loading releases…</p>}
      {!loading && filtered.length === 0 && <p style={{ color: 'var(--text2)' }}>No releases yet.</p>}

      <div style={{ display: 'grid', gap: 12 }}>
        {filtered.map(app => (
          <div key={app.id} className="card" style={{ padding: '16px 18px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginBottom: 4 }}>
                <span style={{ fontWeight: 600, fontSize: '1rem' }}>{app.name}</span>
                <span style={{ color: 'var(--text2)', fontSize: '0.82rem' }}>v{app.version}</span>
                {platformBadge(app.platform)}
                {!app.is_public && <span className="badge badge-lock">🔒 Private</span>}
              </div>
              {app.description && <p style={{ color: 'var(--text2)', fontSize: '0.87rem', marginBottom: 2 }}>{app.description}</p>}
              {app.file_size > 0 && <p style={{ color: 'var(--text2)', fontSize: '0.78rem' }}>{formatBytes(app.file_size)}</p>}
            </div>
            <button
              className="btn btn-primary btn-sm"
              style={{ whiteSpace: 'nowrap', flexShrink: 0 }}
              onClick={() => handleDownload(app)}>
              ⬇ Download
            </button>
          </div>
        ))}
      </div>
    </main>
  )
}