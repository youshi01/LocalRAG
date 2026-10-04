import { useEffect, useState } from 'react'
import type { ChatMode } from '../App'

const SIDEBAR_OPEN_STORAGE_KEY = 'localrag-conversation-sidebar-open'
const LEGACY_SIDEBAR_OPEN_STORAGE_KEY = 'ai-localbase-conversation-sidebar-open'

const readMigratedPreference = (key: string, legacyKey: string) => {
  const currentValue = window.localStorage.getItem(key)
  if (currentValue !== null) return currentValue

  const legacyValue = window.localStorage.getItem(legacyKey)
  if (legacyValue !== null) {
    window.localStorage.setItem(key, legacyValue)
  }
  return legacyValue
}

export const useAppPreferencesState = () => {
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    if (typeof window === 'undefined') return true
    const storedValue = readMigratedPreference(SIDEBAR_OPEN_STORAGE_KEY, LEGACY_SIDEBAR_OPEN_STORAGE_KEY)
    if (storedValue === 'true') return true
    if (storedValue === 'false') return false
    return window.innerWidth > 768
  })
  const [chatMode, setChatMode] = useState<ChatMode>('fast')
  useEffect(() => {
    window.localStorage.setItem(SIDEBAR_OPEN_STORAGE_KEY, String(sidebarOpen))
  }, [sidebarOpen])

  return {
    sidebarOpen,
    setSidebarOpen,
    chatMode,
    setChatMode,
  }
}
