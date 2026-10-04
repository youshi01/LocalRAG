export interface RetrievalHelpItem {
  label: string
  description: string
}

export interface RetrievalHelpSection {
  title: string
  summary: string
  items: ReadonlyArray<RetrievalHelpItem>
}

export type RetrievalHelpSectionId = 'preset' | 'strategy' | 'enhancement' | 'scale'

export const retrievalHelpSections: Record<RetrievalHelpSectionId, RetrievalHelpSection> = {
  preset: {
    title: '检索预设说明',
    summary: '预设会一次性调整下方的检索参数，适合不熟悉参数含义时直接选择。选中后仍可以继续手动修改。',
    items: [
      { label: '快速', description: '候选片段较少，响应更快，适合短文档和问题目标明确的场景。' },
      { label: '均衡', description: '在速度、召回范围和上下文消耗之间折中，适合作为日常默认配置。' },
      { label: '高质量', description: '扩大候选、上下文和语义处理范围，适合跨文档或复杂问题，但响应可能更慢。' },
      { label: '自定义', description: '表示你已经手动改过参数，不再完全匹配某个预设。' },
    ],
  },
  strategy: {
    title: '核心策略说明',
    summary: '决定系统如何找到相关片段、如何排序，以及最多给模型多少原文证据。',
    items: [
      { label: '向量检索', description: '按问题和文档的语义相似度召回，适合自然语言提问和同义表达。' },
      { label: '混合检索', description: '同时结合向量相似度和关键词匹配，适合系统名称、编号、域名等精确内容。' },
      { label: '重排策略', description: '对初步召回结果再次排序；关键词融合偏向字面命中，语义重排偏向上下文含义。' },
      { label: '回答上下文字符', description: '允许放入回答提示词的原文字符上限。数值越大，证据可能更完整，但模型耗时和上下文消耗也会增加。' },
      { label: '启用混合召回', description: '打开后会同时使用向量和关键词信号；如果只需要语义检索，可以关闭。' },
    ],
  },
  enhancement: {
    title: '查询增强说明',
    summary: '把一个问题转换成多个检索表达，帮助系统覆盖不同说法。',
    items: [
      { label: '启用问题改写', description: '适合问题较长、含多个条件或表达比较模糊的情况；简单问题通常不必开启。' },
      { label: '改写数量', description: '每个问题最多生成多少个检索表达。数量越大，覆盖范围可能越广，但会增加检索耗时。' },
      { label: '模型辅助检索', description: '模型先判断问题是否需要知识库，并从“总结、具体内容、实施建议”等意图生成检索表达；模型调用失败时会自动回退到原有检索，不会绕过知识库范围。' },
      { label: '最大规划轮数', description: '首轮证据不足时允许再规划一次补充查询。默认 2 轮，最多只补检索 1 次，以控制延迟和模型调用成本。' },
    ],
  },
  scale: {
    title: '召回规模说明',
    summary: '控制系统从多少候选片段中筛选答案证据。数值越大，可能更完整，但也会增加耗时和上下文消耗。',
    items: [
      { label: '文档 TopK', description: '在指定文档范围内，最终保留的片段数量。' },
      { label: '文档候选', description: '在指定文档范围内，先进入排序的候选片段数量，通常应不小于文档 TopK。' },
      { label: '知识库 TopK', description: '在整个知识库范围内，最终保留的片段数量。' },
      { label: '知识库候选', description: '在整个知识库范围内，先进入排序的候选片段数量，通常应不小于知识库 TopK。' },
      { label: '每文档片段数', description: '同一文档最多贡献多少个片段，避免单个文档占满全部上下文。' },
      { label: '低置信自动补强', description: '当初次检索信号偏弱时，自动扩大候选并补充片段，可能增加响应时间。' },
    ],
  },
}

export const citationScoreHelpText = '这是检索相关性匹配分数：分数越高，表示该片段与当前问题越相关；它不是答案正确率，也不是事实可信度评分。'
