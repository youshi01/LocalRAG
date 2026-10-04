import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import MessageCitations from './MessageCitations'

const sources = ['a.pdf', 'b.xlsx', 'c.md'].map((name, index) => ({
 knowledgeBaseId:'kb', documentId:String(index), documentName:name,
 chunkId:'inventory:'+index, sourceType:'document-inventory', chunkKind:'document_inventory',
 snippet:'目录记录：'+name,
}))
const support={status:'supported',basis:'document_inventory',summary:'目录记录逐项核对，不代表验证正文。',claimCount:3,supportedClaimCount:3,coverage:1}

describe('document directory citations',()=>{
 it('labels metadata as directory verification rather than semantic evidence',()=>{
  const html=renderToStaticMarkup(createElement(MessageCitations,{sources,citationSupport:support}))
  expect(html).toContain('文件目录已核对')
  expect(html).toContain('3/3 份文件已核对')
  expect(html).not.toContain('答案已被引用片段完整支撑')
  expect(html).not.toContain('分数')
 })
 it('shows an explicit empty-directory message instead of unexplained 0/0',()=>{
  const html=renderToStaticMarkup(createElement(MessageCitations,{sources:[],citationSupport:{...support,claimCount:0,supportedClaimCount:0}}))
  expect(html).toContain('目录为空')
  expect(html).not.toContain('0/0')
 })
})


it('reports the complete directory count when citation cards are capped', () => {
 const many=Array.from({length:12},(_,i)=>({...sources[0],documentId:String(i),chunkId:'inventory:'+i,documentName:'file-'+i+'.txt'}))
 const html=renderToStaticMarkup(createElement(MessageCitations,{sources:many,citationSupport:{...support,claimCount:12,supportedClaimCount:12}}))
 expect(html).toContain('12/12 份文件已核对')
 expect(html).toContain('展示前')
 expect(html).toContain('目录来源')
})
