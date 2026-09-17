# LocalRAG 模型接口增强设计

- 日期：2026-09-17
- 状态：已获用户确认，已完成文档自检，待用户审阅
- 范围：后端模型列表与模型探测、健康检查复用、前端设置页模型选择与延迟展示

## 1. 目标

为 LocalRAG 增加统一的模型接口，使用户可以在设置页：

1. 从 Ollama 或 OpenAI Compatible Provider 获取候选模型列表。
2. 对指定 Chat 或 Embedding 模型执行一次真实调用探测。
3. 查看请求延迟、实际响应模型和 Embedding 向量维度。
4. 在健康检查中复用同一套探测逻辑，避免“HTTP 可访问”被误认为“模型可用”。
5. 在 Docker、本机和 OpenAI Compatible 地址之间保持一致的 Base URL 处理。
6. 保留现有聊天、Embedding 和旧测试接口的兼容性。

模型列表只是候选发现结果；**只有真实探测成功才认为模型可用**。

## 2. 当前实现与问题

当前后端已有：

- `POST /api/config/test-chat-model`
- `POST /api/config/test-embedding-model`
- Ollama Chat、Ollama Embedding、OpenAI Compatible Chat、OpenAI Compatible Embedding 调用。
- 健康汇总中的 Chat 和 Embedding 检查。

当前缺少：

- Provider 模型列表获取。
- Chat/Embedding 统一的模型能力服务。
- 列表获取、真实调用、健康检查之间的统一错误和超时策略。
- 设置页的模型候选选择。

## 3. 方案

新增后端模型能力服务，统一负责：

- Base URL 规范化。
- Ollama `/api/tags` 模型列表读取。
- OpenAI Compatible `/models` 模型列表读取。
- Chat 最小真实调用探测。
- Embedding 最小真实调用探测。
- 单次请求超时、延迟测量和响应校验。
- 不记录、不返回 API Key 或 Authorization。

现有的聊天和 Embedding 运行路径不改变协议；现有两个测试接口保留，并复用统一探测服务，避免旧前端或脚本失效。

不采用前端直连 Provider：这会造成 Docker 地址不一致，并可能暴露 API Key。

## 4. 后端 HTTP 接口

### 4.1 获取候选模型

路由：

`POST /api/config/models`

请求：

```json
{
  "type": "chat",
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "apiKey": ""
}
```

字段约束：

- `type` 只能是 `chat` 或 `embedding`。
- `provider` 支持 `ollama` 和 `openai-compatible`。
- `baseUrl` 必须是 Provider 根地址，不能直接填写 `/chat/completions`、`/embeddings` 或 `/models`。
- `apiKey` 仅用于本次请求；响应和日志不得包含它。

Ollama 请求：

- 对规范化后的 Base URL 调用 `GET /api/tags`。
- 读取 `models[].name`，兼容 `models[].model`。
- Ollama 的 tags 接口通常无法可靠区分 Chat 与 Embedding，返回的模型标记为当前请求的候选类型；最终类型兼容性由探测确认。

OpenAI Compatible 请求：

- 对规范化后的 Base URL 调用 `GET /models`。
- 读取 `data[].id`，保留 `owned_by`（如果存在）作为展示信息。
- 使用 Bearer API Key（如果提供）。

成功响应：

```json
{
  "success": true,
  "provider": "ollama",
  "type": "chat",
  "models": [
    {
      "id": "qwen3.5:9b",
      "name": "qwen3.5:9b",
      "type": "chat",
      "owned_by": ""
    }
  ],
  "latency_ms": 32
}
```

失败响应使用 HTTP 200 表示一次请求已完成，但 `success=false`；请求字段不合法使用 HTTP 400。失败响应只返回脱敏后的 `error_code` 和 `error_message`。

### 4.2 探测指定模型

路由：

`POST /api/config/models/probe`

请求：

```json
{
  "type": "chat",
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "model": "qwen3.5:9b",
  "apiKey": "",
  "temperature": 0
}
```

Chat 探测：

- Ollama 调用 `POST /api/chat`，`stream=false`，使用最小测试消息，并验证返回的 assistant 内容非空。
- OpenAI Compatible 调用 `POST /chat/completions`，使用最小测试消息、低输出上限，并验证 `choices[0].message.content` 非空。
- 探测请求不写入会话、不触发知识库检索、不保存任何业务数据。

Embedding 探测：

- Ollama 调用 `POST /api/embed`，输入一条固定短文本。
- OpenAI Compatible 调用 `POST /embeddings`，输入一条固定短文本。
- 验证至少返回一个非空向量。
- 返回实际向量维度，并与 Qdrant 配置维度比较。
- 维度不匹配时 `success=false`，同时返回实际维度和期望维度。

成功响应示例：

```json
{
  "success": true,
  "type": "embedding",
  "provider": "ollama",
  "model": "nomic-embed-text",
  "latency_ms": 214,
  "vector_size": 768,
  "expected_vector_size": 768,
  "dimension_match": true,
  "model_info": "Embedding probe succeeded"
}
```

失败响应：

- 连接失败、超时、鉴权失败、模型不存在、响应格式错误和空响应分别映射到稳定错误类别。
- 不返回上游原始响应全文。
- 不返回 API Key、Authorization、带凭据的 URL 或请求头。
- 探测失败仍返回实际耗时，便于健康审查。

### 4.3 现有接口兼容

保留：

- `POST /api/config/test-chat-model`
- `POST /api/config/test-embedding-model`

两个旧接口内部转换为统一探测请求，保持原有 `TestModelResponse` 字段兼容。

### 4.4 健康汇总复用

`GET /api/config/health-summary` 中的 Chat 和 Embedding 检查改为调用统一探测服务：

- 继续返回现有 `status`、`message`、`latency_ms`、`error_message` 字段。
- Chat/Embedding 成功必须来自一次真实模型调用。
- 健康汇总使用有界超时，不因为 Provider 卡住而无限等待。
- `/readyz` 继续只做配置检查，不执行模型推理，避免 Docker 健康轮询反复消耗模型资源。

## 5. Base URL 规范化

统一规则：

- 去除首尾空白和末尾斜杠。
- Ollama 根地址移除末尾的 `/v1`，再拼接 native API 路径。
- OpenAI Compatible 根地址补齐 `/v1`（如果尚未存在）。
- 拒绝已经包含 `/chat/completions`、`/embeddings`、`/models` 的调用地址作为 Base URL。
- Provider 名称大小写不敏感，内部归一化为 `ollama` 或 `openai-compatible`。
- 空地址、空模型和不支持的 Provider 在 handler 层返回 HTTP 400。

## 6. 前端交互

修改模型设置页：

- Chat 和 Embedding 各自显示“获取模型”按钮。
- 获取成功后显示候选模型下拉选择。
- 选择候选模型会更新当前草稿的 `model` 字段，但不自动保存配置。
- 保留现有文本输入，允许手动填写列表中没有的模型。
- 显示“探测模型”按钮，探测当前草稿中的 Provider、Base URL、模型和 API Key。
- 探测结果显示：
  - 成功或失败；
  - 实际请求延迟；
  - 模型信息；
  - Embedding 实际维度和 Qdrant 期望维度。
- Provider 或 Base URL 变化后清空旧的候选列表，避免误选其他服务的模型。
- 请求进行中禁用对应按钮；请求结束后恢复。
- 页面不显示或缓存后端保存的 API Key 明文。

前端 API 层新增：

- `fetchAvailableModels(config, type)`
- `probeModel(config, type)`

已有 `testChatModelConfig`、`testEmbeddingModelConfig` 保留，内部可继续用于兼容调用。

## 7. 超时与延迟

- 获取模型列表使用独立的短超时，默认不超过 8 秒。
- 模型探测使用独立的短超时，默认不超过 15 秒。
- 列表和探测均只进行一次请求，不自动重试，确保延迟反映本次健康检查的真实耗时。
- 延迟从服务端发起上游请求前开始，直到完成响应校验为止。
- 超时返回失败结果和实际耗时，不阻塞其他请求。

## 8. 测试策略

后端：

- Provider/Base URL 规范化测试。
- Ollama tags 响应解析测试。
- OpenAI models 响应解析测试。
- Chat 探测成功、空响应、模型不存在、鉴权失败和超时测试。
- Embedding 探测成功、空向量、维度匹配、维度不匹配和超时测试。
- API Key 不出现在错误响应测试。
- 新路由和旧兼容路由测试。
- 健康汇总使用真实探测且返回延迟的测试。

前端：

- 模型列表 API 请求字段和响应解析测试。
- 模型探测 API 请求字段和响应解析测试。
- 获取模型后候选项显示和选择回写测试。
- 加载中禁用按钮、失败提示和延迟展示测试。
- 保留手动模型输入测试。

回归：

- `go test ./...`
- 前端测试、类型检查、Lint 和生产构建。
- 若本机 Ollama 可用，验证实际模型列表、Chat 延迟、Embedding 延迟和向量维度。
- 不要求 Docker 在本次代码修改阶段运行；后续编译时再验证容器地址。

## 9. 不在本次范围

- 自动下载、删除或更新 Ollama 模型。
- 对每个模型逐一执行昂贵的能力分类。
- 自动保存模型配置。
- 远程 Provider 的长期监控、历史延迟曲线和告警。
- 修改 Qdrant collection 或自动重建知识库索引。
