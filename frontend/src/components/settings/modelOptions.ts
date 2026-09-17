import type { ModelOption } from '../../services/api'

export const resetModelOptionsKey = (type: string, provider: string, baseUrl: string) =>
  type + ':' + provider.trim().toLowerCase() + ':' + baseUrl.trim().replace(/\/+$/, '')

export const modelOptionLabel = (option: ModelOption) =>
  option.owned_by ? option.name + ' · ' + option.owned_by : option.name