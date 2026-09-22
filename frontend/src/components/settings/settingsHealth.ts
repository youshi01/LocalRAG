import type { HealthSummaryResponse } from '../../services/api'

const componentLabels: Array<[keyof HealthSummaryResponse, string]> = [
  ['chat_model', 'Chat'],
  ['embedding_model', 'Embedding'],
  ['qdrant', 'Qdrant'],
  ['storage', '存储'],
  ['auth', '认证'],
]

export const summarizeHealthWarning = (summary: HealthSummaryResponse): string => {
  const failed = componentLabels
    .map(([key, label]) => {
      const component = summary[key]
      if (component.status === 'ok' || component.status === 'not_configured') {
        return ''
      }
      return `${label}：${component.error_message || component.message || '健康检查未通过'}`
    })
    .filter(Boolean)

  return failed.length > 0
    ? `配置已保存，但健康检查发现问题：${failed.join('；')}`
    : ''
}
