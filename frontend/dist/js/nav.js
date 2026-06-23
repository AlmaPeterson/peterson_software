import { getCurrentUser, logout } from './auth.js'
import { initThemeToggle } from './theme.js'

export async function initNav() {
  const user = await getCurrentUser()
  const actions = document.getElementById('nav-actions')
  if (!actions) return user

  actions.innerHTML = ''
  initThemeToggle(actions)

  if (user?.role === 'admin') {
    const adminLink = document.createElement('a')
    adminLink.href = 'admin.html'
    adminLink.className = 'btn btn-ghost btn-sm'
    adminLink.textContent = 'Admin'
    actions.appendChild(adminLink)
  }

  if (user) {
    const name = document.createElement('span')
    name.className = 'desktop-only nav-username'
    name.textContent = user.username
    actions.appendChild(name)

    const signOut = document.createElement('button')
    signOut.className = 'btn btn-ghost btn-sm'
    signOut.textContent = 'Sign Out'
    signOut.addEventListener('click', () => {
      logout()
      window.location.href = 'index.html'
    })
    actions.appendChild(signOut)
  } else {
    const signIn = document.createElement('a')
    signIn.href = 'login.html'
    signIn.className = 'btn btn-ghost btn-sm'
    signIn.textContent = 'Sign In'
    actions.appendChild(signIn)

    const register = document.createElement('a')
    register.href = 'register.html'
    register.className = 'btn btn-primary btn-sm'
    register.textContent = 'Register'
    actions.appendChild(register)
  }

  return user
}
