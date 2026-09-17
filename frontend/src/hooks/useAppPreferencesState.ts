import { useEffect, useMemo, useState } from 'react'
import type { AppConfig, ChatMode, ChatModeSettings } from '../App'

const THINK_MODEL_STORAGE_KEY = 'localrag-think-model'
const SIDEBAR_OPEN_STORAGE_KEY = 'localrag-conversation-sidebar-open'
const LEGACY_THINK_MODEL_STORAGE_KEY = 'ai-localbase-think-model'
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

export const useAppPreferencesState = (config: AppConfig) => {
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    if (typeof window === 'undefined') return true
    const storedValue = readMigratedPreference(SIDEBAR_OPEN_STORAGE_KEY, LEGACY_SIDEBAR_OPEN_STORAGE_KEY)
    if (storedValue === 'true') return true
    if (storedValue === 'false') return false
    return window.innerWidth > 768
  })
  const [chatMode, setChatMode] = useState<ChatMode>('fast')
  const [thinkModel, setThinkModel] = useState(() => {
    if (typeof window === 'undefined') {
      return 'deepseek-r1:8b'
    }
    return readMigratedPreference(THINK_MODEL_STORAGE_KEY, LEGACY_THINK_MODEL_STORAGE_KEY)?.trim() || 'deepseek-r1:8b'
  })

  useEffect(() => {
    window.localStorage.setItem(SIDEBAR_OPEN_STORAGE_KEY, String(sidebarOpen))
  }, [sidebarOpen])

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    const normalizedModel = thinkModel.trim()
    if (normalizedModel) {
      window.localStorage.setItem(THINK_MODEL_STORAGE_KEY, normalizedModel)
    } else {
      window.localStorage.removeItem(THINK_MODEL_STORAGE_KEY)
    }
  }, [thinkModel])

  const chatModeSettings = useMemo<ChatModeSettings>(
    () => ({
      fastModel: config.chat.model,
      thinkModel,
    }),
    [config.chat.model, thinkModel],
  )

  return {
    sidebarOpen,
    setSidebarOpen,
    chatMode,
    setChatMode,
    thinkModel,
    setThinkModel,
    chatModeSettings,
  }
}
