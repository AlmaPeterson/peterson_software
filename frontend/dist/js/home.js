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
    const fallbackIcon = PLATFORM_ICONS[platforms[0]] || '📦'
    const iconHtml = app.icon_url
      ? `<img src="${escapeHtml(app.icon_url)}" alt="" />`
      : fallbackIcon
    const platformIcons = platforms.map(p => `<span title="${escapeHtml(p)}">${PLATFORM_ICONS[p] || '📦'}</span>`).join('')
    return `
      <a class="app-card" href="template.html?slug=${encodeURIComponent(app.slug)}">
        <div class="app-card-icon">${iconHtml}</div>
        <div class="app-card-name">${escapeHtml(app.name)}</div>
        <div class="app-card-version">v${escapeHtml(app.version)}</div>
        <div class="app-card-platforms">${platformIcons}</div>
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
