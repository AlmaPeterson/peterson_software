const BASE = '/api'

function getToken() {
  return localStorage.getItem('ps_token')
}

async function req(path, options = {}) {
  const token = getToken()
  const headers = { ...options.headers }
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }
  const res = await fetch(BASE + path, { ...options, headers })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  register: (data) => req('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
  login: (data) => req('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  me: () => req('/auth/me'),

  getApps: () => req('/apps'),
  getApp: (slug) => req(`/apps/${slug}`),
  uploadApp: (form) => req('/admin/apps/upload', { method: 'POST', body: form }),
  addAppFile: (appId, form) => req(`/admin/apps/${appId}/files`, { method: 'POST', body: form }),
  deleteApp: (id) => req(`/admin/apps/delete/${id}`, { method: 'DELETE' }),
  deleteRelease: (id) => req(`/admin/releases/${id}`, { method: 'DELETE' }),

  getUsers: () => req('/admin/users'),
  updateRole: (id, role) => req(`/admin/users/${id}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
  deleteUser: (id) => req(`/admin/users/${id}`, { method: 'DELETE' }),

  downloadUrl: (slug, platform) => `${BASE}/apps/${slug}/download/${platform}`,
}
