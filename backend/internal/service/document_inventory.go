package service

import (
	"fmt"
	"localrag/internal/model"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// DocumentInventoryAnswer describes a scope-checked metadata snapshot, never
// a set of retrieval hits or a claim about document body contents.
type DocumentInventoryAnswer struct {
	Content    string
	Sources    []map[string]string
	Count      int
	NeedsScope bool
}

var inventoryCountPattern = regexp.MustCompile("(?:几|多少)(?:个|份|篇)?(?:文件|文档|资料)|(?:文件|文档|资料)(?:数量|总数)|(?:文件|文档|资料)(?:有)?(?:几个|多少|几份)(?:呢|啊|呀)?$")
var inventoryEnglishTarget = regexp.MustCompile("(?i)\\b(?:files?|documents?)\\b")
var inventoryEnglishAction = regexp.MustCompile("(?i)\\b(?:list|show|which|what)\\b")
var inventoryEnglishCount = regexp.MustCompile("(?i)\\b(?:how many|number of|count)\\b")

func documentInventoryQuery(query string) (bool, bool) {
	raw := strings.ToLower(strings.TrimSpace(query))
	q := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, raw)
	if q == "" {
		return false, false
	}
	if containsAnyText(q, []string{"内容", "正文", "结论", "观点", "摘要", "总结", "接口", "怎么", "如何", "实现", "建议", "策略", "字段", "记录", "数据行", "文件里", "文件中", "文档里", "文档中", "content", "summari", "implement", "recommend"}) {
		return false, false
	}
	chineseTarget := containsAnyText(q, []string{"文件", "文档", "资料"})
	englishTarget := inventoryEnglishTarget.MatchString(raw)
	if !chineseTarget && !englishTarget {
		return false, false
	}
	count := inventoryCountPattern.MatchString(q) || (englishTarget && inventoryEnglishCount.MatchString(raw))
	list := containsAnyText(q, []string{"列出", "列举", "显示", "查看", "清单", "列表", "目录", "文件名", "文档名", "有哪些", "哪些文件", "哪些文档", "还有"}) || (englishTarget && inventoryEnglishAction.MatchString(raw))
	return count || list, count
}

func (s *AppService) BuildDocumentInventoryAnswer(req model.ChatCompletionRequest) (DocumentInventoryAnswer, bool, error) {
	matched, countOnly := documentInventoryQuery(latestUserMessage(req.Messages))
	if !matched {
		return DocumentInventoryAnswer{}, false, nil
	}
	if len(req.Messages) == 0 {
		return DocumentInventoryAnswer{}, true, fmt.Errorf("messages cannot be empty")
	}
	if err := s.ValidateChatRequestScope(req); err != nil {
		return DocumentInventoryAnswer{}, true, err
	}
	kbID, docID := strings.TrimSpace(req.KnowledgeBaseID), strings.TrimSpace(req.DocumentID)
	if kbID == "" && docID == "" {
		return DocumentInventoryAnswer{Content: "请先选择一个知识库，再查询其中的文件目录。", NeedsScope: true}, true, nil
	}
	s.state.Mu.RLock()
	defer s.state.Mu.RUnlock()
	var kb model.KnowledgeBase
	if kbID != "" {
		var exists bool
		kb, exists = s.state.KnowledgeBases[kbID]
		if !exists {
			return DocumentInventoryAnswer{}, true, fmt.Errorf("knowledge base not found")
		}
	} else {
		// ValidateChatRequestScope has already rejected missing or ambiguous IDs.
		for _, candidate := range s.state.KnowledgeBases {
			for _, doc := range candidate.Documents {
				if doc.ID == docID {
					kb = candidate
					break
				}
			}
			if kb.ID != "" {
				break
			}
		}
	}
	docs := make([]model.Document, 0, len(kb.Documents))
	for _, doc := range kb.Documents {
		if docID == "" || doc.ID == docID {
			docs = append(docs, doc)
		}
	}
	if docID != "" && len(docs) == 0 {
		return DocumentInventoryAnswer{}, true, fmt.Errorf("document not found")
	}
	count := len(docs)
	scope := fmt.Sprintf("知识库《%s》", kb.Name)
	if docID != "" {
		scope = "当前检索范围为单独文档（不扩展到整个知识库）"
	}
	content := fmt.Sprintf("%s中共有 **%d** 份文件。", scope, count)
	if count == 0 {
		content += "目录为空。"
	} else if !countOnly {
		content += "\n\n"
		for index, doc := range docs {
			content += fmt.Sprintf("%d. %s\n", index+1, inventoryFileNameMarkdown(doc.Name))
		}
	}
	content += "\n\n以上来自已保存的文件目录，不受检索 TopK、相似度或索引状态影响。"
	sources := make([]map[string]string, 0, count)
	for index, doc := range docs {
		chunkID := "document-inventory:" + doc.ID
		snippet := fmt.Sprintf("文件目录记录：%s；状态：%s。", doc.Name, doc.Status)
		sources = append(sources, map[string]string{"knowledgeBaseId": kb.ID, "documentId": doc.ID, "documentName": doc.Name, "chunkId": chunkID, "evidenceId": "inventory:" + kb.ID + ":" + doc.ID, "chunkIndex": strconv.Itoa(index + 1), "chunkKind": "document_inventory", "sourceType": "document-inventory", "toolName": "knowledge_base.list_documents", "snippet": snippet})
	}
	return DocumentInventoryAnswer{Content: strings.TrimSpace(content), Sources: sources, Count: count}, true, nil
}

func inventoryFileNameMarkdown(name string) string {
	// Names are data, not Markdown instructions. Use a code span with a delimiter
	// longer than every backtick run in the original name.
	longest, run := 0, 0
	for _, r := range name {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	clean := strings.NewReplacer("\r", " ", "\n", " ").Replace(name)
	return fence + " " + clean + " " + fence
}
