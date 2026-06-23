// Manual light/dark toggle, layered on top of the no-flash inline script in
// each page's <head> (which sets the same data-theme attribute from
// localStorage/system preference before first paint).
const STORAGE_KEY = 'ps_theme'
const META_COLORS = { light: '#f5f5fb', dark: '#0a0826' }

const SUN_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="4.5"/><path d="M12 2.5v3M12 18.5v3M4.2 4.2l2.1 2.1M17.7 17.7l2.1 2.1M2.5 12h3M18.5 12h3M4.2 19.8l2.1-2.1M17.7 6.3l2.1-2.1"/></svg>`
const MOON_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5z"/></svg>`

export function getTheme() {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

export function setTheme(theme) {
  document.documentElement.dataset.theme = theme
  localStorage.setItem(STORAGE_KEY, theme)
  syncMeta(theme)
}

function syncMeta(theme) {
  const meta = document.getElementById('theme-color-meta')
  if (meta) meta.setAttribute('content', META_COLORS[theme])
}

export function initThemeToggle(container) {
  syncMeta(getTheme())

  const btn = document.createElement('button')
  btn.type = 'button'
  btn.className = 'btn btn-ghost btn-sm theme-toggle'
  btn.setAttribute('aria-label', 'Switch to the other color theme')

  const render = () => {
    const isDark = getTheme() === 'dark'
    btn.innerHTML = isDark ? SUN_ICON : MOON_ICON
    btn.setAttribute('aria-pressed', String(isDark))
  }
  render()

  btn.addEventListener('click', () => {
    setTheme(getTheme() === 'dark' ? 'light' : 'dark')
    render()
  })

  container.appendChild(btn)
}
