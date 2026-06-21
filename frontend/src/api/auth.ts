const STORAGE_KEY = 'stock_advisor_auth'

export interface AuthCredentials {
  user: string
  pass: string
}

export function getCredentials(): AuthCredentials | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw)
  } catch {}
  return null
}

export function saveCredentials(user: string, pass: string) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ user, pass }))
}

export function clearCredentials() {
  localStorage.removeItem(STORAGE_KEY)
}

export function isWebMode(): boolean {
  return typeof window !== 'undefined' && !(window as any).go?.main?.App
}
