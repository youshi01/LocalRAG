# LocalRAG 项目中期检查表

## 一 项目基本信息

| 项目 | 内容 |
| --- | --- |
| 项目名称 | LocalRAG 本地知识库问答系统 |
| 项目定位 | 本地优先的文档检索增强生成系统 |
| GitHub 仓库 | https://github.com/youshi01/LocalRAG |
| 后端技术 | Go、Gin、SQLite |
| 前端技术 | React、Vite、TypeScript |
| 向量数据库 | Qdrant |
| 模型接入 | Ollama、OpenAI Compatible API |

## 二 中期检查目标

本阶段以“核心功能完成并能够演示”为验收目标，重点展示知识库建立、文档导入、文档索引、检索测试和基于检索结果的问答流程。MCP、评估和高级检索策略属于扩展能力，不影响核心 RAG 主链路验收。

## 三 已完成功能

| 序号 | 功能模块 | 已完成内容 | 代码或界面依据 | 中期状态 |
| --- | --- | --- | --- | --- |
| 1 | 知识库管理 | 创建、删除知识库，查看知识库文档列表 | `backend/internal/router/router.go`、知识库页面 | 已完成 |
| 2 | 文档上传 | 支持 TXT、Markdown、PDF、XLSX、CSV 文件上传 | `backend/internal/handler/app_handler.go`、上传组件 | 已完成 |
| 3 | 文档解析 | 提取文本和结构化表格内容，并生成文档摘要 | `backend/internal/util/document_text.go`、`structured_table.go` | 已完成 |
| 4 | 文本分块 | 对文档进行语义切分，形成可检索 Chunk | `backend/internal/util/document_text.go` | 已完成 |
| 5 | 向量索引 | 调用 Embedding 模型生成向量并写入 Qdrant | `backend/internal/service/rag_service.go`、`qdrant_service.go` | 已完成 |
| 6 | 检索测试 | 输入问题，召回相关文档片段并显示命中结果 | `backend/internal/router/router.go`、前端“检索测试”页面 | 已完成 |
| 7 | RAG 问答 | 将检索结果注入上下文，调用 Chat 模型生成回答 | `backend/internal/service/llm_service.go`、`app_handler.go` | 已完成 |
| 8 | 引用来源 | 回答中展示文档来源，并支持定位到原文或 Chunk | `frontend/src/components/chat/MessageCitations.tsx` | 已完成 |
| 9 | 索引健康检查 | 展示文档数量、索引状态、Chunk 数量和失败信息 | `frontend/src/components/knowledge/KnowledgeHealthPanel.tsx` | 已完成 |
| 10 | 数据持久化 | 持久化配置、知识库状态和聊天记录 | `backend/internal/service/app_state_store.go`、`chat_history_store.go` | 已完成 |
| 11 | 模型配置 | 配置 Chat 模型和 Embedding 模型，支持连通性检查 | 设置页面及 `config_handler.go` | 已完成 |
| 12 | 部署方式 | 提供 Docker Compose 和 Windows 单 exe 构建方式 | `docker-compose.yml`、`build.bat`、`docs/windows-build.md` | 已完成 |

## 四 核心演示流程

1. 启动 Qdrant、后端和前端服务。
2. 进入 LocalRAG，创建一个新的知识库。
3. 上传一份 TXT、Markdown、PDF 或表格文件。
4. 等待文档状态变为“已索引”。
5. 打开“检索测试”，输入文档中的问题，确认出现命中 Chunk。
6. 返回聊天页面，选择该知识库并提问。
7. 检查回答内容和引用来源，打开引用定位到对应文档片段。
8. 打开知识库健康检查，确认文档、索引和 Chunk 状态正常。

## 五 运行方式

### Docker Compose

- 前端：`http://localhost:4173`
- 后端：`http://localhost:8080`
- Qdrant：`http://localhost:6333`

在项目根目录执行：

```bash
docker compose up --build
```

### Windows 本地开发

- Qdrant：`http://localhost:6333`
- 后端：`http://localhost:8080`
- Vite 前端：`http://localhost:3000`

后端和前端的详细启动命令见 `docs/windows-build.md`。如果需要生成单个 Windows 可执行文件，可执行项目根目录的 `build.bat`。

## 六 测试与验证依据

- 后端包含单元测试、路由测试和 Qdrant/Ollama 模拟端到端测试。
- 前端包含组件测试、服务测试和 Playwright 浏览器工作流测试。
- `frontend/e2e/knowledge-workflow.spec.ts` 覆盖“创建知识库、上传文件、确认索引、运行检索、打开文档详情”的核心流程，并提供带引用的聊天工作流测试。
- 提交前应使用当前版本重新构建并完成一次实际演示，截图只使用当前 LocalRAG 界面产生的结果。

本次中期检查使用当前代码执行了以下验证：

| 验证项 | 结果 |
| --- | --- |
| 后端 Go 测试 | `go test ./...` 通过 |
| 前端单元测试 | 12 个测试文件、44 个测试通过 |
| 前端类型检查与 lint | `npm run typecheck`、`npm run lint` 通过 |
| 前端生产构建 | `npm run build` 通过，构建预算和历史基线检查通过 |
| 浏览器核心工作流 | 公开演示 Fixture 三个用例通过；合成文档工作流的上传、索引、检索和引用定位通过 |

## 七 当前限制与后续计划

- 系统当前按本地单机或轻量自托管场景设计。
- PDF 解析效果受原始文档排版影响。
- Embedding 模型输出维度必须与 Qdrant 配置一致。
- 多用户隔离、知识库导入导出和更复杂的检索增强属于后续迭代内容。

## 八 中期提交材料清单

- [ ] 本中期检查表文档
- [ ] LocalRAG 当前版本核心功能演示截图
- [ ] 排除 `.env`、依赖目录和运行数据后的项目代码压缩包

## 九 本次实际演示记录

本次演示使用本地 Ollama，未将 API Key 或登录密码写入项目材料：

- Chat 模型：`qwen3.5:9b`
- Embedding 模型：`nomic-embed-text`
- Embedding 向量维度：`768`
- 向量数据库：Qdrant `qdrant/qdrant:v1.13.4`
- 演示知识库：`LocalRAG中期演示知识库`
- 演示文档：`LocalRAG_中期验收核心事实.md`
- 索引结果：文档 `1` 份、已索引 `1` 份、Chunk `1` 个、向量 `1` 个、健康评分 `100`
- 检索结果：命中演示文档，置信状态正常，证据覆盖 `100%`
- 问答结果：HTTP `200`，回答包含 `nomic-embed-text` 和 `768`，引用来源可定位到文档正文 Chunk `#1`

Docker Compose 的标准访问地址仍以 README 和启动文档为准：前端 `4173`、后端 `8080`、Qdrant `6333`。本次本机验证为避免占用已有端口使用了临时映射端口，不影响提交代码。
