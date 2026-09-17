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

export const modelOptionLabel = (option: ModelOption) =>
  option.owned_by ? option.name + ' · ' + option.owned_by : option.name

export const isCurrentModelDiscoveryRequest = (
  requestKey: string,
  requestGeneration: number,
  currentKey: string,
  currentGeneration: number,
) => requestKey === currentKey && requestGeneration === currentGeneration
