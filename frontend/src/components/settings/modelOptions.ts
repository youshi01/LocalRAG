import type { ModelOption } from '../../services/api'

const secretFingerprint = (value: string) => {
  let hash = 2166136261
  for (const character of value) {
    hash = Math.imul(hash ^ character.charCodeAt(0), 16777619)
  }
  return (hash >>> 0).toString(16)
}

export const resetModelOptionsKey = (
  type: string,
  provider: string,
  baseUrl: string,
  apiKey = '',
  credentialState = '',
) => type + ':' + provider.trim().toLowerCase() + ':' + baseUrl.trim().replace(/\/+$/, '') + ':' + secretFingerprint(`${apiKey}\u0000${credentialState}`)

export const modelProbeKey = (
  type: string,
  provider: string,
  baseUrl: string,
  modelName: string,
  apiKey: string,
  temperature?: number,
  credentialState = '',
) => resetModelOptionsKey(type, provider, baseUrl, apiKey, credentialState) + ':' + modelName.trim() + ':' + (temperature ?? '')

export const modelOptionLabel = (option: ModelOption) => {
  const ownerLabel = option.owned_by ? ' · ' + option.owned_by : ''
  const capabilityLabel = option.capability_status === 'supported'
    ? ' · 能力匹配'
    : option.capability_status === 'unsupported'
      ? ' · 不支持当前类型'
      : option.capability_status === 'unknown'
        ? ' · 能力待确认'
        : ''
  return option.name + ownerLabel + capabilityLabel
}

export const isCurrentModelDiscoveryRequest = (
  requestKey: string,
  requestGeneration: number,
  currentKey: string,
  currentGeneration: number,
) => requestKey === currentKey && requestGeneration === currentGeneration
