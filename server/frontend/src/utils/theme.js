import { readonly, ref } from 'vue'

const STORAGE_KEY = 'logmaster-theme'
const THEMES = new Set(['dark', 'light'])

function readStoredTheme() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return THEMES.has(stored) ? stored : null
  } catch {
    return null
  }
}

function getPreferredTheme() {
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

const themeMode = ref(readStoredTheme() || getPreferredTheme())

function applyTheme(theme = themeMode.value) {
  document.documentElement.dataset.logTheme = theme
  document.documentElement.dataset.appTheme = theme
  document.documentElement.classList.toggle('theme-light', theme === 'light')
}

function setTheme(theme, { persist = true, notify = true } = {}) {
  const nextTheme = THEMES.has(theme) ? theme : getPreferredTheme()
  themeMode.value = nextTheme

  if (persist) {
    try { localStorage.setItem(STORAGE_KEY, nextTheme) } catch { /* Storage can be unavailable in private contexts. */ }
  }

  applyTheme(nextTheme)
  if (notify) window.dispatchEvent(new CustomEvent('logmaster-theme-change', { detail: nextTheme }))
}

function toggleTheme() {
  setTheme(themeMode.value === 'dark' ? 'light' : 'dark')
}

function initializeTheme() {
  applyTheme()
}

window.addEventListener('storage', (event) => {
  if (event.key !== STORAGE_KEY) return
  const nextTheme = THEMES.has(event.newValue) ? event.newValue : getPreferredTheme()
  setTheme(nextTheme, { persist: false })
})

window.matchMedia?.('(prefers-color-scheme: light)').addEventListener?.('change', (event) => {
  if (readStoredTheme()) return
  setTheme(event.matches ? 'light' : 'dark', { persist: false })
})

export function useTheme() {
  return {
    themeMode: readonly(themeMode),
    setTheme,
    toggleTheme
  }
}

export { initializeTheme }
