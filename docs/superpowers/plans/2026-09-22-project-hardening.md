# LocalRAG Project Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复当前 LocalRAG 的 Docker 发布链路和 Windows 测试问题，并补齐模型能力识别、Embedding 模型变更保护与文档一致性。

**Architecture:** 保持现有 Ollama Native 与 OpenAI Compatible 两套协议，不引入厂商绑定。模型列表增加能力元数据，配置保存成功后由前端触发一次后台健康检查；Embedding 指纹持久化到应用配置并参与知识库索引版本判断，模型变化时阻止静默混用旧向量。Docker 镜像统一发布到当前 GitHub 仓库 owner，Compose 默认使用 `latest`。

**Tech Stack:** Go 1.25+、Gin、SQLite、Qdrant、React、TypeScript、Vitest、Docker Compose、GitHub Actions。

**Spec:** 用户确认的五项项目完善清单：Docker owner/发布、Windows Go 测试、模型能力识别与保存后健康检查、Embedding 模型指纹与自动重建提示、文档同步。

## Global Constraints

- Embedding 仍按协议能力判断：Ollama 使用 `/api/embed`，OpenAI Compatible 使用 `/v1/embeddings`。
- 不恢复聊天或 Embedding 页面上的“探测模型”按钮；保存后的健康检查是后台/状态反馈。
- 不把未知能力的 OpenAI Compatible 模型误判为不支持；未知能力必须明确标记并允许用户保存。
- Chat 与 Embedding 的 provider、Base URL、模型名和 API Key 必须保持独立。
- 向量维度必须继续匹配 `QDRANT_VECTOR_SIZE`；模型身份变化即使维度相同也必须触发重新索引提示。
- Docker 镜像统一使用 `ghcr.io/youshi01/localrag-*`，Compose 默认 tag 为 `latest`，仍允许 `LOCALRAG_IMAGE_TAG` 覆盖。
- 不修改用户已有提交材料内容，只更新其随代码一起提交的材料副本时必须保持可读取。

---

### Task 1: 修复 Docker 镜像 owner 和 latest 发布链路

**Files:**
- Modify: `.github/workflows/docker-build.yml`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.app.yml`
- Modify: `docker-compose.prod.yml`
- Modify: `README.md`
- Modify: `DOCKER_DEPLOY.md`
- Modify: `docs/getting-started.md`
- Test: GitHub workflow YAML parse and Compose config checks

**Interfaces:**
- Produces images `ghcr.io/youshi01/localrag-backend:{branch|sha|latest}` and `ghcr.io/youshi01/localrag-frontend:{branch|sha|latest}`.
- Compose resolves image tag as `${LOCALRAG_IMAGE_TAG:-${AI_LOCALBASE_IMAGE_TAG:-latest}}`.

- [ ] **Step 1: Write the failing repository consistency check**

Add a shell/PowerShell validation command in the task notes or CI check that asserts every Docker publish reference uses `youshi01`, and asserts the production/default compose fallback is `latest` rather than `v1.4.6`.

- [ ] **Step 2: Run the check and observe the current failure**

Run from the repository root:

```powershell
rg -n "GITHUB_OWNER:|ghcr.io/|v1\.4\.6" .github docker-compose*.yml DOCKER_DEPLOY.md README.md docs/getting-started.md
```
Expected: the command shows `GITHUB_OWNER: veyliss` and the stale `v1.4.6` fallback.

- [ ] **Step 3: Update owner and tag references**

Set the workflow owner to `youshi01`, update all Compose image expressions to use `latest` as the fallback, and update deployment examples/documentation to explain that `LOCALRAG_IMAGE_TAG` can pin a release or commit-derived tag.

- [ ] **Step 4: Validate configuration**

Run:

```powershell
rg -n "GITHUB_OWNER:|ghcr.io/|v1\.4\.6|LOCALRAG_IMAGE_TAG" .github docker-compose*.yml DOCKER_DEPLOY.md README.md docs/getting-started.md
docker compose -f docker-compose.yml config
docker compose -f docker-compose.app.yml config
docker compose -f docker-compose.prod.yml config
```

Expected: all image references use `ghcr.io/youshi01`, no default `v1.4.6` remains, and all Compose files parse.

- [ ] **Step 5: Commit**

```powershell
git add .github/workflows/docker-build.yml docker-compose*.yml README.md DOCKER_DEPLOY.md docs/getting-started.md
git commit -m "fix: align docker publishing with github repository"
```

### Task 2: Make Windows Go tests pass without weakening Unix protection

**Files:**
- Modify: `backend/eval/cmd/eval_main.go`
- Modify: `backend/eval/cmd/eval_main_test.go`
- Modify: `backend/internal/service/mcp_job_store_test.go`
- Possibly create: `backend/internal/service/private_file_mode_windows_test.go`

**Interfaces:**
- `rewriteEvalPath` returns container-style `/` paths regardless of host OS.
- Unix tests continue asserting `0600`; Windows tests assert the protection operation succeeds and do not compare Unix permission bits that Windows does not expose through `os.FileMode`.

- [ ] **Step 1: Add a cross-platform path regression test**

Extend `TestRewriteEvalPath` with a rule whose target uses POSIX separators and assert the result is `/workspace/backend/data/uploads/demo.csv` on every OS.

- [ ] **Step 2: Run the focused test and observe the Windows failure**

```powershell
$go='E:\codex\ai-localbase-main\.tools\go1.27.1\go\bin\go.exe'
& $go test ./eval/cmd -run TestRewriteEvalPath -count=1
```

Expected before the implementation change: Windows output uses backslashes.

- [ ] **Step 3: Use logical POSIX joining**

Change the evaluator’s virtual path composition to use `path.Join` and normalize the input prefix comparison without changing real filesystem paths used elsewhere.

- [ ] **Step 4: Make permission assertions platform-aware**

Keep Unix `0600` assertions. On Windows, assert `protectFiles` returns no error and that database/sidecar files remain accessible only through the store path; avoid asserting Unix mode bits. Ensure every test closes its `MCPJobStore` before temp-directory cleanup.

- [ ] **Step 5: Run focused and package tests**

```powershell
& $go test ./eval/cmd -run TestRewriteEvalPath -count=1
& $go test ./internal/service -run 'TestMCPJobStorePersistsBatchInputMetadataAndProtectsFile|TestMCPJobStoreProtectsDatabaseAndSQLiteSidecarsAfterWrite' -count=1
```

- [ ] **Step 6: Commit**

```powershell
git add backend/eval/cmd backend/internal/service/mcp_job_store_test.go
git commit -m "fix: make evaluation and file protection tests cross platform"
```

### Task 3: Add model capabilities and save-time health feedback

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/service/model_service.go`
- Modify: `backend/internal/service/model_service_test.go`
- Modify: `backend/internal/handler/app_handler.go`
- Modify: `backend/internal/handler/config_handler.go`
- Modify: `frontend/src/services/api.ts`
- Modify: `frontend/src/components/settings/ModelConfigProbe.tsx`
- Modify: `frontend/src/components/settings/tabs/AISettings.tsx`
- Modify: `frontend/src/components/settings/SettingsPanel.tsx`
- Test: backend model discovery tests and frontend API/settings tests

**Interfaces:**
- `model.ModelOption` adds `Capabilities []string` and `CapabilityStatus string` with values `known`, `unknown`, or `unsupported`.
- Ollama `/api/tags` capabilities are preserved; OpenAI Compatible `/v1/models` remains `unknown` unless the provider supplies capability metadata.
- `GET /api/config/health-summary` remains the authoritative real-call health result.
- After a successful config save, the frontend calls the existing health-summary API and displays Chat/Embedding status without restoring a probe button.

- [ ] **Step 1: Add failing backend capability tests**

Cover Ollama model entries with `capabilities: ["completion"]` and `["embedding"]`, and verify OpenAI model entries use `capability_status: "unknown"` when metadata is absent.

- [ ] **Step 2: Run focused tests and observe failure**

```powershell
& $go test ./internal/service -run 'TestListModels.*Capability' -count=1
```

Expected: current response has no capability fields.

- [ ] **Step 3: Implement capability parsing and safe unknown behavior**

Parse provider metadata without filtering unknown models. For Embedding candidates, mark known non-embedding models as unsupported and show them disabled/labelled in the UI; keep unknown OpenAI-compatible models selectable.

- [ ] **Step 4: Add failing frontend save-health test**

Mock `PUT /api/config` success followed by `/api/config/health-summary` and assert the settings state renders the returned Chat and Embedding status.

- [ ] **Step 5: Implement post-save health feedback**

Call health summary after saving, keep saving successful even when a provider is temporarily unavailable, and show an actionable warning rather than blocking configuration persistence.

- [ ] **Step 6: Run frontend regression**

```powershell
cd frontend
npm test -- --run
npm run typecheck
npm run lint
```

- [ ] **Step 7: Commit**

```powershell
git add backend frontend/src
git commit -m "feat: surface model capabilities and save health status"
```

### Task 4: Track Embedding model identity and force safe reindexing

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/service/app_state_store.go`
- Modify: `backend/internal/service/app_service.go`
- Modify: `backend/internal/service/index_governance.go`
- Modify: `backend/internal/service/app_service_test.go`
- Modify: `backend/internal/handler/app_handler.go`
- Modify: frontend health/config types and knowledge-base health UI as needed

**Interfaces:**
- Persist an `EmbeddingFingerprint` containing normalized provider, endpoint, model, and vector size.
- A knowledge base stores the fingerprint used for its current index.
- A changed fingerprint marks the knowledge base/document index as stale; a same-dimension model change is not silently treated as healthy.
- Existing state without a fingerprint migrates as `unknown` and requires a one-time reindex confirmation rather than deleting data.

- [ ] **Step 1: Add failing state migration and fingerprint tests**

Test normalization, persistence/reload, missing legacy fingerprint migration, and same-dimension model changes producing `needs_reindex`.

- [ ] **Step 2: Run focused tests and observe failure**

```powershell
& $go test ./internal/service -run 'Test.*EmbeddingFingerprint|Test.*NeedsReindex' -count=1
```

- [ ] **Step 3: Implement fingerprint calculation and persistence**

Use the same endpoint normalization already used by model runtime calls. Record the fingerprint when a new index generation commits, not when a document upload merely starts.

- [ ] **Step 4: Gate retrieval/index health on fingerprint compatibility**

Return a clear health recommendation and prevent retrieval from reporting a fully healthy index when the configured Embedding fingerprint differs from the committed index fingerprint.

- [ ] **Step 5: Add UI warning and reindex action wiring**

Display “Embedding 模型已变化，需要重新索引” in knowledge-base health and keep the existing batch/reindex action as the remediation.

- [ ] **Step 6: Run backend and frontend focused tests**

```powershell
& $go test ./internal/service ./internal/router -count=1
cd frontend; npm test -- --run
```

- [ ] **Step 7: Commit**

```powershell
git add backend frontend/src
git commit -m "feat: protect indexes from embedding model changes"
```

### Task 5: Synchronize documentation and add final verification

**Files:**
- Modify: `README.md`
- Modify: `TROUBLESHOOTING.md`
- Modify: `docs/getting-started.md`
- Modify: `DOCKER_DEPLOY.md`
- Modify: `docs/superpowers/specs/2026-09-17-model-interface-design.md` (mark UI probe behavior as historical or update the current contract)

- [ ] **Step 1: Remove stale UI instructions**

Replace “点击探测模型” with “获取模型并查看保存后的健康状态”; retain backend probe API language only where it describes internal health checks.

- [ ] **Step 2: Document capability and fingerprint behavior**

Explain known/unknown model capabilities, separate Chat and Embedding, same-dimension model changes, and reindex requirements.

- [ ] **Step 3: Validate docs/config**

```powershell
rg -n -i "探测模型|v1\.4\.6|veyliss|ghcr\.io" README.md TROUBLESHOOTING.md docs DOCKER_DEPLOY.md .github docker-compose*.yml
docker compose -f docker-compose.yml config
docker compose -f docker-compose.app.yml config
docker compose -f docker-compose.prod.yml config
```

- [ ] **Step 4: Run the full verification matrix**

```powershell
cd frontend
npm test -- --run
npm run typecheck
npm run lint
npm run build
cd ..\backend
& $go test ./...
```

Record any remaining platform-only failures separately; do not call the suite green if any remain.

- [ ] **Step 5: Commit and prepare integration**

```powershell
git add README.md TROUBLESHOOTING.md docs DOCKER_DEPLOY.md .github docker-compose*.yml
git commit -m "docs: align deployment and model capability guidance"
git status --short --branch
```
