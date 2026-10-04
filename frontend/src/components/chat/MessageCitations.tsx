import React, { useState } from 'react'
import type { ChatSourceMetadata, CitationSupportMetadata } from '../../App'
import { chunkKindLabel } from '../knowledge/knowledgeLabels'
import { filterDocumentCitationSources } from './citationSources'
import { citationScoreHelpText } from '../settings/retrievalHelp'

interface MessageCitationsProps {
  sources: ChatSourceMetadata[]
  citationSupport?: CitationSupportMetadata
  onOpenCitationSource?: (source: ChatSourceMetadata) => void
}

const sourceIdentity = (source: ChatSourceMetadata, index: number) =>
  [
    source.knowledgeBaseId,
    source.documentId,
    source.chunkId,
    source.chunkIndex,
  ].filter(Boolean).join(':') || `source-${index}`

const normalizeSources = (sources?: ChatSourceMetadata[]) => {
  if (!sources || sources.length === 0) return []
  const seen = new Set<string>()
  return filterDocumentCitationSources(sources).filter((source, index) => {
    const key = sourceIdentity(source, index)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

const sourceTypeLabel = (source: ChatSourceMetadata) => {
  if (source.sourceType === 'document-inventory') return '文件目录'
  if (source.sourceType === 'structured-data') return '结构化数据'
  if (source.chunkKind) return chunkKindLabel(source.chunkKind)
  return '来源'
}

const sourceRankLabel = (source: ChatSourceMetadata, index: number) => {
  if (source.chunkIndex) return `#${source.chunkIndex}`
  return `#${index + 1}`
}

const scoreLabel = (score?: string) => {
  if (!score) return ''
  const value = Number(score)
  if (!Number.isFinite(value)) return ''
  return `分数 ${value.toFixed(4)}`
}

const supportLabel = (status?: CitationSupportMetadata['status']) => {
  switch (status) {
    case 'supported':
      return '答案已被引用片段完整支撑'
    case 'partial':
      return '仅部分答案被引用片段支撑'
    case 'abstained':
      return '回答未声明有确定证据'
    default:
      return '引用片段未能支撑答案'
  }
}

const MessageCitations: React.FC<MessageCitationsProps> = ({
  sources,
  citationSupport,
  onOpenCitationSource,
}) => {
  const directoryEvidence = citationSupport?.basis === 'document_inventory'
  const normalizedSources = normalizeSources(sources)
  const visibleSources = normalizedSources.slice(0, 6)
  const [expanded, setExpanded] = useState(true)
  if (visibleSources.length === 0 && !citationSupport) return null

  return (
    <details
      className="message-citations"
      open={expanded}
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary>
        <span>{directoryEvidence ? '目录来源' : '引用来源'}</span>
        <strong>{directoryEvidence ? normalizedSources.length : visibleSources.length}</strong>
      </summary>
      {citationSupport && (
        <div className={`message-citation-support message-citation-support-${citationSupport.status}`}>
          <strong>{directoryEvidence ? (citationSupport.status === 'supported' ? '文件目录已核对' : '尚未选择知识库目录') : supportLabel(citationSupport.status)}</strong>
          <span>
            {directoryEvidence
              ? citationSupport.claimCount === 0
                ? citationSupport.status === 'supported' ? '目录为空（0 份文件）' : '请选择知识库后查询目录'
                : `${citationSupport.supportedClaimCount}/${citationSupport.claimCount} 份文件已核对`
              : `${citationSupport.supportedClaimCount}/${citationSupport.claimCount} 条陈述已核对`}
          </span>
          {(directoryEvidence || citationSupport.status === 'partial') && <span>{citationSupport.summary}</span>}
        </div>
      )}
      {directoryEvidence && normalizedSources.length > visibleSources.length && (
        <p className="message-citation-excerpt">清单已核对全部 {normalizedSources.length} 份文件，下方展示前 {visibleSources.length} 条目录记录。</p>
      )}
      <div className="message-citation-list">
        {visibleSources.map((source, index) => (
          <article className="message-citation" key={sourceIdentity(source, index)}>
            <div className="message-citation-head">
              <strong>{source.documentName || '未知来源'}</strong>
              <span>{sourceTypeLabel(source)}</span>
              <span>{sourceRankLabel(source, index)}</span>
              {scoreLabel(source.score) && (
                <span
                  className="message-citation-score"
                  title={citationScoreHelpText}
                  aria-label={`${scoreLabel(source.score)}。${citationScoreHelpText}`}
                >
                  {scoreLabel(source.score)}
                </span>
              )}
              {source.documentId && (
                <button
                  type="button"
                  onClick={() => onOpenCitationSource?.(source)}
                  disabled={!onOpenCitationSource}
                >
                  定位
                </button>
              )}
            </div>
            {(source.citationSnippet || source.snippet) && (
              <p className={source.citationSnippet ? 'message-citation-excerpt' : undefined}>
                {source.citationSnippet || source.snippet}
              </p>
            )}
          </article>
        ))}
      </div>
    </details>
  )
}

export default MessageCitations
