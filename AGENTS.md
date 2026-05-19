# AGENTS.md

## 适用范围

本文件适用于整个仓库。更深层目录如有自己的 `AGENTS.md`，以更深层规则为准。

## 项目背景

这是一个短视频 Feed 系统，包含 Go 后端、Vue 前端、MySQL、Redis、RabbitMQ 和 Docker Compose。

后端位于 `backend/`，前端位于 `frontend/`，主要说明文档是 `README.md` 和 `feedsystem_video_go项目设计.md`。

## 通用规则

- 修改前先阅读相关代码和文档，不要只凭文件名猜测行为。
- 变更必须聚焦用户请求，不做无关重构或风格整理。
- 不要回滚、覆盖或删除用户已有改动，除非用户明确要求。
- 搜索文件和文本优先使用 `rg` 或 `rg --files`。
- 手工编辑文件时使用 `apply_patch`。
- 不要主动提交 Git commit，除非用户明确要求。
- 不要写入真实密钥、私有令牌、机器专属路径或个人环境配置。
- 不要随意改动依赖版本、锁文件或 Docker 配置，除非任务需要。

## 后端规则

- 后端代码在 `backend/` 下，遵循现有 Gin、GORM、MySQL、Redis、RabbitMQ 结构。
- Handler 负责请求解析和响应，Service 负责业务逻辑，Repo 负责数据访问。
- 修改 API 行为时，同步检查路由、请求响应结构、测试和接口文档。
- Go 代码必须保持 `gofmt` 格式。
- 后端命令从 `backend/` 目录执行。

## 前端规则

- 前端代码在 `frontend/` 下，遵循现有 Vue 3 和 Vite 结构。
- 后端接口调用遵循现有 `/api` 代理模式。
- 不要引入新的 UI 框架或状态管理库，除非用户明确要求或已有代码需要。
- UI 改动要保持当前调试工具风格，优先清晰、稳定、可操作。

## 验证规则

- 后端改动优先运行 `cd backend && go test ./...`。
- 前端改动优先运行 `cd frontend && npm run build`。
- Docker 或启动流程改动需验证相关 `docker compose` 命令。
- 如果无法运行验证命令，最终回复中说明原因和建议用户运行的命令。

## 生成物与本地数据

- 不要主动编辑 `frontend/dist/`、缓存目录、运行时数据或构建产物。
- 不要删除 `data/`、数据库卷、缓存目录或上传文件，除非用户明确要求。
- Swagger、文档或生成代码只有在相关接口行为变化时才更新。
