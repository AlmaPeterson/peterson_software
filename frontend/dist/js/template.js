import { api } from './api.js'
import { initNav } from './nav.js'

const PLATFORM_ICONS = { android: '🤖', ios: '🍎', windows: '🪟', mac: '🍏', linux: '🐧', other: '📦' }

function escapeHtml(str) {
  const div = document.createElement('div')
  div.textContent = str
  return div.innerHTML
}

function formatBytes(bytes) {
  if (!bytes) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function renderApp(app, currentUser) {
  const platforms = [...new Set(app.releases.map(r => r.platform.toLowerCase()))]
  const mainIcon = PLATFORM_ICONS[platforms[0]] || '📦'

  const downloadsHtml = app.releases.map(rel => {
    const p = rel.platform.toLowerCase()
    const icon = PLATFORM_ICONS[p] || '📦'
    return `
      <div class="list-row">
        <div style="display: flex; align-items: center; gap: 14px; min-width: 0;">
          <div class="list-row-icon">${icon}</div>
          <div>
            <div style="font-weight: 600; font-size: 0.95rem;">${escapeHtml(rel.platform)}</div>
            ${rel.file_size > 0 ? `<div style="color: var(--text3); font-size: 0.78rem;">${formatBytes(rel.file_size)}</div>` : ''}
          </div>
        </div>
        <button class="btn btn-primary btn-sm download-btn" data-platform="${escapeHtml(rel.platform)}">Download</button>
      </div>
    `
  }).join('')

  document.getElementById('app-detail').innerHTML = `
    <div class="detail-header">
      <div class="detail-icon">${mainIcon}</div>
      <div>
        <h1 class="detail-name">${escapeHtml(app.name)}</h1>
        <p style="color: var(--text2); font-size: 0.95rem;">
          v${escapeHtml(app.version)}
          ${!app.is_public ? '<span class="badge badge-lock" style="margin-left: 8px;">🔒 Private</span>' : ''}
        </p>
      </div>
    </div>

    ${app.description ? `<p style="color: var(--text2); font-size: 0.98rem; margin-bottom: 28px;">${escapeHtml(app.description)}</p>` : ''}

    <div class="section-title">Available Downloads</div>
    <div class="list-group">${downloadsHtml}</div>
  `

  document.querySelectorAll('.download-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      if (!app.is_public && !currentUser) {
        alert('Please sign in to download this file.')
        return
      }
      window.location.href = api.downloadUrl(app.slug, btn.dataset.platform)
    })
  })
}

async function main() {
  const currentUser = await initNav()
  const slug = new URLSearchParams(window.location.search).get('slug')

  let app = null
  if (slug) {
    try {
      app = await api.getApp(slug)
    } catch (err) {
      console.error(err)
    }
  }

  document.getElementById('loading').style.display = 'none'

  if (!app) {
    document.getElementById('not-found').style.display = 'block'
    return
  }

  document.getElementById('app-detail').style.display = 'block'
  document.title = `${app.name} · Peterson Software`
  renderApp(app, currentUser)
}

main()
