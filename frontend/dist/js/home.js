import { api } from './api.js'
import { initNav } from './nav.js'
import { platformInfo } from './platforms.js'

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
      ${p === 'all' ? 'All' : escapeHtml(platformInfo(p).label)}
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
    const iconHtml = app.icon_url
      ? `<img src="${escapeHtml(app.icon_url)}" alt="" />`
      : '<span class="icon-fallback">⬡</span>'
    const platformTags = platforms.map(p => {
      const info = platformInfo(p)
      return `<span class="ptag ptag-${p}" title="${escapeHtml(info.label)}"><span class="ptag-dot"></span>${info.code}</span>`
    }).join('')
    return `
      <a class="app-card" href="template.html?slug=${encodeURIComponent(app.slug)}">
        <div class="app-card-icon">${iconHtml}</div>
        <div class="app-card-name">${escapeHtml(app.name)}</div>
        <div class="app-card-version">v${escapeHtml(app.version)}</div>
        <div class="app-card-platforms">${platformTags}</div>
      </a>
    `
  }).join('')
}

function renderSubtitle() {
  const subtitle = document.getElementById('page-subtitle')
  if (!subtitle) return
  if (apps.length === 0) {
    subtitle.textContent = 'Download the latest builds for all platforms.'
    return
  }
  const platformCount = new Set(apps.flatMap(appPlatforms)).size
  const buildWord = apps.length === 1 ? 'build' : 'builds'
  const platformWord = platformCount === 1 ? 'platform' : 'platforms'
  subtitle.textContent = `${apps.length} ${buildWord} across ${platformCount} ${platformWord}.`
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
  renderSubtitle()
  renderFilters()
  renderGrid()
}

main()
