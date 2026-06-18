import { api } from './api.js'
import { initNav } from './nav.js'

const PLATFORM_ICONS = { android: '🤖', ios: '🍎', windows: '🪟', mac: '🍏', linux: '🐧', other: '📦' }

let apps = []
let filter = 'all'

function escapeHtml(str) {
  const div = document.createElement('div')
  div.textContent = str
  return div.innerHTML
}

function appPlatforms(app) {
  return [...new Set(app.releases.map(r => r.platform.toLowerCase()))]
}

function renderFilters() {
  const platformSet = new Set()
  apps.forEach(a => appPlatforms(a).forEach(p => platformSet.add(p)))
  const platforms = ['all', ...platformSet]

  const el = document.getElementById('filters')
  el.innerHTML = platforms.map(p => `
    <button class="segmented-item ${filter === p ? 'is-active' : ''}" data-platform="${p}" role="tab" aria-selected="${filter === p}">
      ${p === 'all' ? 'All' : `${PLATFORM_ICONS[p] || '📦'} ${escapeHtml(p)}`}
    </button>
  `).join('')

  el.querySelectorAll('button').forEach(btn => {
    btn.addEventListener('click', () => {
      filter = btn.dataset.platform
      renderFilters()
      renderGrid()
    })
  })
}

function renderGrid() {
  const grid = document.getElementById('app-grid')
  const filtered = filter === 'all' ? apps : apps.filter(a => appPlatforms(a).includes(filter))

  if (filtered.length === 0) {
    grid.innerHTML = '<p class="skeleton" style="grid-column: 1 / -1;">No releases yet.</p>'
    return
  }

  grid.innerHTML = filtered.map(app => {
    const platforms = appPlatforms(app)
    const mainIcon = PLATFORM_ICONS[platforms[0]] || '📦'
    const badges = platforms.map(p => `<span class="badge badge-${p}" title="${escapeHtml(p)}">${PLATFORM_ICONS[p] || '📦'}</span>`).join('')
    return `
      <a class="app-card" href="template.html?slug=${encodeURIComponent(app.slug)}">
        <div class="app-card-icon">${mainIcon}</div>
        <div class="app-card-name">${escapeHtml(app.name)}</div>
        <div class="app-card-version">v${escapeHtml(app.version)}</div>
        <div class="app-card-platforms">${badges}</div>
      </a>
    `
  }).join('')
}

async function main() {
  await initNav()
  try {
    apps = await api.getApps()
  } catch (err) {
    console.error(err)
  }
  document.getElementById('loading').style.display = 'none'
  document.getElementById('app-grid').style.display = ''
  renderFilters()
  renderGrid()
}

main()
