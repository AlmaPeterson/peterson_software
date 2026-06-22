import { api } from './api.js'
import { initNav } from './nav.js'

const PLATFORM_ICONS = { android: '🤖', ios: '🍎', windows: '🪟', mac: '🍏', linux: '🐧', other: '📦' }
const EXTENSION_PLATFORMS = {
  apk: 'Android', aab: 'Android',
  ipa: 'iOS',
  exe: 'Windows', msi: 'Windows',
  dmg: 'Mac', pkg: 'Mac',
  deb: 'Linux', rpm: 'Linux', appimage: 'Linux',
}

function detectPlatformClient(filename) {
  const ext = filename.toLowerCase().split('.').pop()
  return EXTENSION_PLATFORMS[ext] || 'Other'
}

function escapeHtml(str) {
  const div = document.createElement('div')
  div.textContent = str
  return div.innerHTML
}

// 512KB — comfortably under nginx's stock 1MB client_max_body_size default,
// so uploads work even if whatever proxy sits in front was never explicitly
// configured for larger request bodies.
const CHUNK_SIZE = 512 * 1024

// Uploads a file as a sequence of small chunked requests instead of one long
// one, so no single request runs long enough to trip a reverse proxy's
// timeout regardless of file size or connection speed.
async function uploadFileInChunks(appId, file, onProgress) {
  const uploadId = crypto.randomUUID()
  const totalChunks = Math.max(1, Math.ceil(file.size / CHUNK_SIZE))
  let result = null
  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE
    const blob = file.slice(start, start + CHUNK_SIZE)
    const fd = new FormData()
    fd.append('uploadId', uploadId)
    fd.append('filename', file.name)
    fd.append('chunkIndex', String(i))
    fd.append('totalChunks', String(totalChunks))
    fd.append('chunk', blob, file.name)
    result = await api.uploadChunk(appId, fd)
    if (onProgress) onProgress((i + 1) / totalChunks)
  }
  return result
}

let apps = []
let users = []
let editingAppId = null

async function loadApps() {
  try {
    apps = await api.getApps()
  } catch (err) {
    console.error(err)
  }
  renderApps()
}

async function loadUsers() {
  try {
    users = await api.getUsers()
  } catch (err) {
    console.error(err)
  }
  renderUsers()
}

function renderApps() {
  document.getElementById('releases-title').textContent = `Software (${apps.length})`
  const list = document.getElementById('apps-list')

  if (apps.length === 0) {
    list.innerHTML = '<div class="list-row-empty">No software uploaded yet.</div>'
    return
  }

  list.innerHTML = apps.map(app => app.id === editingAppId ? renderEditForm(app) : renderAppRow(app)).join('')

  list.querySelectorAll('.delete-app-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Delete this software and all its files?')) return
      await api.deleteApp(btn.dataset.id)
      loadApps()
    })
  })

  list.querySelectorAll('.edit-app-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      editingAppId = Number(btn.dataset.id)
      renderApps()
    })
  })

  list.querySelectorAll('.release-delete-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Remove this file?')) return
      await api.deleteRelease(btn.dataset.releaseId)
      loadApps()
    })
  })

  list.querySelectorAll('.add-file-input').forEach(input => {
    input.addEventListener('change', async () => {
      const file = input.files[0]
      if (!file) return
      const label = input.closest('label')
      const originalText = label.firstChild.textContent
      try {
        await uploadFileInChunks(input.dataset.appId, file, (pct) => {
          label.firstChild.textContent = ` Uploading… ${Math.round(pct * 100)}% `
        })
        loadApps()
      } catch (err) {
        alert(err.message)
        label.firstChild.textContent = originalText
      }
    })
  })

  list.querySelectorAll('.cancel-edit-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      editingAppId = null
      renderApps()
    })
  })

  list.querySelectorAll('.edit-icon-input').forEach(input => {
    input.addEventListener('change', () => {
      const file = input.files[0]
      if (!file) return
      const preview = input.closest('form').querySelector('.icon-preview')
      preview.innerHTML = `<img src="${URL.createObjectURL(file)}" alt="" />`
    })
  })

  list.querySelectorAll('.edit-app-form').forEach(form => {
    form.addEventListener('submit', async (e) => {
      e.preventDefault()
      const appId = form.dataset.appId
      const saveBtn = form.querySelector('.save-edit-btn')
      saveBtn.disabled = true
      saveBtn.textContent = 'Saving…'
      try {
        await api.updateApp(appId, {
          name: form.querySelector('.edit-name').value,
          version: form.querySelector('.edit-version').value,
          description: form.querySelector('.edit-description').value,
          is_public: form.querySelector('.edit-visibility').value === 'true',
        })
        const iconFile = form.querySelector('.edit-icon-input').files[0]
        if (iconFile) {
          const fd = new FormData()
          fd.append('icon', iconFile)
          await api.uploadIcon(appId, fd)
        }
        editingAppId = null
        loadApps()
      } catch (err) {
        alert(err.message)
        saveBtn.disabled = false
        saveBtn.textContent = 'Save'
      }
    })
  })
}

function appIconHtml(app) {
  if (app.icon_url) return `<img src="${escapeHtml(app.icon_url)}" alt="" />`
  const platform = app.releases[0] ? app.releases[0].platform.toLowerCase() : 'other'
  return PLATFORM_ICONS[platform] || '📦'
}

function renderAppRow(app) {
  const badges = app.releases.map(rel => {
    const p = rel.platform.toLowerCase()
    const icon = PLATFORM_ICONS[p] || '📦'
    return `
      <span class="badge badge-${p}" style="display: inline-flex; align-items: center; gap: 4px;">
        ${icon} ${escapeHtml(rel.platform)}
        <button class="release-delete-btn" data-release-id="${rel.id}" title="Remove ${escapeHtml(rel.platform)} file"
          style="background: none; border: none; color: inherit; cursor: pointer; font-size: 0.95em; line-height: 1; padding: 0;">×</button>
      </span>
    `
  }).join('')

  return `
    <div class="list-row" style="align-items: flex-start; flex-wrap: wrap;">
      <div style="display: flex; gap: 12px; flex: 1; min-width: 220px;">
        <div class="icon-preview">${appIconHtml(app)}</div>
        <div>
          <div style="margin-bottom: 8px;">
            <span style="font-weight: 600; font-size: 0.94rem;">${escapeHtml(app.name)}</span>
            <span style="color: var(--text2); font-size: 0.82rem; margin-left: 8px;">v${escapeHtml(app.version)}</span>
            <span style="margin-left: 8px;" class="badge ${app.is_public ? 'badge-public' : 'badge-lock'}">${app.is_public ? 'Public' : 'Private'}</span>
          </div>
          <div style="display: flex; gap: 6px; flex-wrap: wrap; align-items: center;">
            ${badges}
            <label class="btn btn-secondary btn-sm" style="cursor: pointer;">
              + Add File
              <input type="file" class="add-file-input" data-app-id="${app.id}" style="display: none;" />
            </label>
          </div>
        </div>
      </div>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-secondary btn-sm edit-app-btn" data-id="${app.id}">Edit</button>
        <button class="btn btn-danger btn-sm delete-app-btn" data-id="${app.id}">Delete</button>
      </div>
    </div>
  `
}

function renderEditForm(app) {
  return `
    <div class="list-row" style="flex-direction: column; align-items: stretch; gap: 12px;">
      <form class="edit-app-form" data-app-id="${app.id}" style="display: grid; gap: 14px;">
        <div style="display: flex; gap: 12px; align-items: center;">
          <div class="icon-preview">${appIconHtml(app)}</div>
          <label class="btn btn-secondary btn-sm" style="cursor: pointer;">
            Change Icon
            <input type="file" class="edit-icon-input" accept="image/png,image/jpeg,image/webp" style="display: none;" />
          </label>
        </div>
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
          <div>
            <label class="field-label">Name *</label>
            <input class="edit-name" value="${escapeHtml(app.name)}" required />
          </div>
          <div>
            <label class="field-label">Version *</label>
            <input class="edit-version" value="${escapeHtml(app.version)}" required />
          </div>
        </div>
        <div>
          <label class="field-label">Visibility</label>
          <select class="edit-visibility">
            <option value="true" ${app.is_public ? 'selected' : ''}>Public</option>
            <option value="false" ${!app.is_public ? 'selected' : ''}>Private (login required)</option>
          </select>
        </div>
        <div>
          <label class="field-label">Description</label>
          <input class="edit-description" value="${escapeHtml(app.description || '')}" placeholder="Optional short description" />
        </div>
        <div style="display: flex; gap: 8px;">
          <button type="submit" class="btn btn-primary btn-sm save-edit-btn">Save</button>
          <button type="button" class="btn btn-ghost btn-sm cancel-edit-btn">Cancel</button>
        </div>
      </form>
    </div>
  `
}

function renderUsers() {
  document.getElementById('users-title').textContent = `Users (${users.length})`
  const list = document.getElementById('users-list')
  const pendingCount = users.filter(u => u.role === 'pending').length
  const badge = document.getElementById('pending-badge')
  if (pendingCount > 0) {
    badge.textContent = String(pendingCount)
    badge.style.display = 'inline'
  } else {
    badge.style.display = 'none'
  }

  if (users.length === 0) {
    list.innerHTML = '<div class="list-row-empty">No users yet.</div>'
    return
  }

  list.innerHTML = users.map(u => `
    <div class="list-row" style="flex-wrap: wrap;">
      <div style="flex: 1; min-width: 160px;">
        <span style="font-weight: 600; font-size: 0.94rem;">${escapeHtml(u.username)}</span>
        <span style="color: var(--text2); font-size: 0.82rem; margin-left: 8px;">${escapeHtml(u.email)}</span>
      </div>
      <div style="display: flex; align-items: center; gap: 8px;">
        <select class="role-select" data-id="${u.id}" style="width: auto; padding: 6px 10px; font-size: 0.82rem;">
          <option value="pending" ${u.role === 'pending' ? 'selected' : ''}>Pending</option>
          <option value="user" ${u.role === 'user' ? 'selected' : ''}>User</option>
          <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>Admin</option>
        </select>
        ${u.role !== 'admin' ? `<button class="btn btn-danger btn-sm remove-user-btn" data-id="${u.id}">Remove</button>` : ''}
      </div>
    </div>
  `).join('')

  list.querySelectorAll('.role-select').forEach(sel => {
    sel.addEventListener('change', async () => {
      await api.updateRole(sel.dataset.id, sel.value)
      loadUsers()
    })
  })

  list.querySelectorAll('.remove-user-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Remove this user?')) return
      await api.deleteUser(btn.dataset.id)
      loadUsers()
    })
  })
}

function setTab(tab) {
  document.getElementById('tab-apps').classList.toggle('is-active', tab === 'apps')
  document.getElementById('tab-apps').setAttribute('aria-selected', tab === 'apps')
  document.getElementById('tab-users').classList.toggle('is-active', tab === 'users')
  document.getElementById('tab-users').setAttribute('aria-selected', tab === 'users')
  document.getElementById('panel-apps').style.display = tab === 'apps' ? '' : 'none'
  document.getElementById('panel-users').style.display = tab === 'users' ? '' : 'none'
}

document.getElementById('tab-apps').addEventListener('click', () => setTab('apps'))
document.getElementById('tab-users').addEventListener('click', () => setTab('users'))

document.getElementById('up-files').addEventListener('change', (e) => {
  const preview = document.getElementById('file-preview')
  const files = Array.from(e.target.files)
  preview.innerHTML = files.map(f => {
    const platform = detectPlatformClient(f.name)
    const icon = PLATFORM_ICONS[platform.toLowerCase()] || '📦'
    return `<span class="badge badge-${platform.toLowerCase()}">${icon} ${platform} · ${escapeHtml(f.name)}</span>`
  }).join('')
})

const uploadForm = document.getElementById('upload-form')
const uploadError = document.getElementById('upload-error')
const uploadSubmit = document.getElementById('upload-submit')

uploadForm.addEventListener('submit', async (e) => {
  e.preventDefault()
  const filesInput = document.getElementById('up-files')
  const files = Array.from(filesInput.files)
  if (files.length === 0) {
    uploadError.textContent = 'Please select at least one file'
    uploadError.style.display = 'block'
    return
  }
  uploadError.style.display = 'none'
  uploadSubmit.disabled = true

  try {
    const app = await api.createApp({
      name: document.getElementById('up-name').value,
      version: document.getElementById('up-version').value,
      description: document.getElementById('up-description').value,
      is_public: document.getElementById('up-visibility').value === 'true',
    })

    const iconFile = document.getElementById('up-icon').files[0]
    if (iconFile) {
      try {
        const fd = new FormData()
        fd.append('icon', iconFile)
        await api.uploadIcon(app.id, fd)
      } catch (err) {
        console.error('icon upload failed:', err)
      }
    }

    const failed = []
    for (const file of files) {
      try {
        await uploadFileInChunks(app.id, file, (pct) => {
          uploadSubmit.textContent = `Uploading ${file.name}… ${Math.round(pct * 100)}%`
        })
      } catch (err) {
        console.error(`upload failed for ${file.name}:`, err)
        failed.push(file.name)
      }
    }

    uploadForm.reset()
    document.getElementById('file-preview').innerHTML = ''
    loadApps()
    if (failed.length > 0) {
      uploadError.textContent = `Uploaded, but these files failed and were skipped: ${failed.join(', ')}`
      uploadError.style.display = 'block'
    }
  } catch (err) {
    uploadError.textContent = err.message
    uploadError.style.display = 'block'
  } finally {
    uploadSubmit.disabled = false
    uploadSubmit.textContent = 'Upload Software'
  }
})

document.getElementById('redeploy-btn').addEventListener('click', async () => {
  if (!confirm('Pull the latest changes from git and restart the server now? The site will be briefly unavailable.')) return

  const status = document.getElementById('redeploy-status')
  const btn = document.getElementById('redeploy-btn')
  btn.disabled = true
  status.className = 'banner banner-success'
  status.textContent = 'Pulling latest changes and restarting…'
  status.style.display = 'block'

  try {
    const result = await api.redeploy()
    status.textContent = `Restarting with latest changes. Reload in a few seconds.\n${result.output || ''}`
  } catch (err) {
    // The server responds with JSON only when git pull actually failed
    // (and it did NOT restart in that case). Anything else here — a raw
    // network error — is expected if the connection dropped because the
    // restart already happened mid-response.
    let parsed = null
    try { parsed = JSON.parse(err.message) } catch {}
    if (parsed && parsed.error) {
      status.className = 'banner banner-danger'
      status.textContent = `git pull failed — server was NOT restarted:\n${parsed.output || parsed.error}`
    } else {
      status.textContent = 'Connection dropped, which is expected if the restart already happened. Reload in a few seconds to check.'
    }
  } finally {
    btn.disabled = false
  }
})

async function main() {
  const user = await initNav()
  if (!user || user.role !== 'admin') {
    window.location.href = 'login.html'
    return
  }
  loadApps()
  loadUsers()
}

main()
