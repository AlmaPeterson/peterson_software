import { login } from './auth.js'
import { initNav } from './nav.js'

initNav()

const form = document.getElementById('login-form')
const errorBox = document.getElementById('error')
const submitBtn = document.getElementById('submit-btn')

form.addEventListener('submit', async (e) => {
  e.preventDefault()
  errorBox.style.display = 'none'
  submitBtn.disabled = true
  submitBtn.textContent = 'Signing in…'
  try {
    await login(form.username.value, form.password.value)
    window.location.href = 'index.html'
  } catch (err) {
    errorBox.textContent = err.message
    errorBox.style.display = 'block'
    submitBtn.disabled = false
    submitBtn.textContent = 'Sign In'
  }
})
