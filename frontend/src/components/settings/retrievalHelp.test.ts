import { describe, expect, it } from 'vitest'
import { citationScoreHelpText, retrievalHelpSections } from './retrievalHelp'

describe('retrieval help content', () => {
  it('explains every retrieval settings section in user-facing Chinese', () => {
    expect(retrievalHelpSections.preset.title).toBe('检索预设说明')
    expect(retrievalHelpSections.strategy.title).toBe('核心策略说明')
    expect(retrievalHelpSections.enhancement.title).toBe('查询增强说明')
    expect(retrievalHelpSections.scale.title).toBe('召回规模说明')

    const content = JSON.stringify(retrievalHelpSections)
    expect(content).toContain('向量检索')
    expect(content).toContain('关键词')
    expect(content).toContain('TopK')
    expect(content).toContain('上下文')
    expect(content).toContain('问题改写')
    expect(content).toContain('模型辅助检索')
  })

  it('clarifies that citation score is relevance, not answer correctness', () => {
    expect(citationScoreHelpText).toContain('相关性')
    expect(citationScoreHelpText).toContain('不是答案正确率')
    expect(citationScoreHelpText).toContain('分数越高')
  })
})
