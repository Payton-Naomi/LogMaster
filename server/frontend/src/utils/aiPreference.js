const AI_ENABLED_KEY = 'logmaster_ai_enabled'

export function getAIEnabled() {
  return window.localStorage.getItem(AI_ENABLED_KEY) !== 'false'
}

export function setAIEnabled(enabled) {
  window.localStorage.setItem(AI_ENABLED_KEY, enabled ? 'true' : 'false')
  window.dispatchEvent(new CustomEvent('logmaster-ai-preference', { detail: Boolean(enabled) }))
}
