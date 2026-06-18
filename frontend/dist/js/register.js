import { api } from './api.js'
import { initNav } from './nav.js'

initNav()

const form = document.getElementById('register-form')
const errorBox = document.getElementById('error')
const submitBtn = document.getElementById('submit-btn')

form.addEventListener('submit', async (e) => {
  e.preventDefault()
  errorBox.style.display = 'none'
  submitBtn.disabled = true
  submitBtn.textContent = 'Registering…'
  try {
    await api.register({
      username: form.username.value,
      email: form.email.value,
      password: form.password.value,
    })
    document.getElementById('form-card').style.display = 'none'
    document.getElementById('success-card').style.display = 'block'
  } catch (err) {
    errorBox.textContent = err.message
    errorBox.style.display = 'block'
    submitBtn.disabled = false
    submitBtn.textContent = 'Create Account'
  }
})
