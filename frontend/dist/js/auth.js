import { api } from './api.js'

export function getToken() {
  return localStorage.getItem('ps_token')
}

export function clearToken() {
  localStorage.removeItem('ps_token')
}

export async function getCurrentUser() {
  if (!getToken()) return null
  try {
    return await api.me()
  } catch {
    clearToken()
    return null
  }
}

export async function login(username, password) {
  const data = await api.login({ username, password })
  localStorage.setItem('ps_token', data.token)
  return { username: data.username, role: data.role }
}

export function logout() {
  clearToken()
}
