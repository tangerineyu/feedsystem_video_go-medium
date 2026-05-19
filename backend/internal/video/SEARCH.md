# 搜索功能交接文档

## 当前背景

项目是短视频 Feed 系统，后端使用 Go + Gin + GORM，视频相关代码位于 `backend/internal/video`。当前本地常用启动方式是：

```bash
docker compose up -d mysql redis rabbitmq
cd backend
go run ./cmd
cd ../frontend
npm run dev
```

但在这种启动方式下，本地 Go 进程读取 `backend/configs/config.yaml`，数据库连接是 `localhost:3306`，实际连接到本机 MariaDB；Docker MySQL 映射在 `localhost:3307`，当前没有被本地 Go 使用。Redis 和 RabbitMQ 使用 Docker 容器，因为它们映射到 `localhost:6379` 和 `localhost:5672`。

## 已完成进度

已新增基础搜索功能：

- `POST /video/search` 路由已注册在 `backend/internal/http/router.go`。
- 请求结构 `SearchVideoRequest` 和响应结构 `SearchVideoResponse` 已加在 `video_entity.go`。
- `VideoRepository.Search` 当前使用 `LIKE` 模糊查询 `title`、`description`、`username`。
- `VideoService.Search` 当前负责：
  - `keyword` trim 后不能为空；
  - `limit <= 0` 或 `limit > 50` 时默认 10；
  - 查询 `limit + 1` 条来判断 `has_more`；
  - 使用 `latest_time` 做发布时间游标分页。
- `VideoHandler.Search` 已解析 JSON 请求并返回 `SearchVideoResponse`。
- 前端 `frontend/src/api/video.ts` 已新增 `searchVideos`。
- 前端首页 `frontend/src/views/HomeView.vue` 在 URL query `q` 存在时调用后端搜索接口，清空 `q` 后恢复原来的推荐/关注/点赞榜逻辑。

当前搜索请求体：

```json
{
  "keyword": "搜索词",
  "limit": 10,
  "latest_time": 0
}
```

当前搜索响应：

```json
{
  "video_list": [],
  "next_time": 0,
  "has_more": false
}
```

## 当前问题

现有 SQL 是：

```sql
WHERE title LIKE '%keyword%'
   OR description LIKE '%keyword%'
   OR username LIKE '%keyword%'
ORDER BY create_time DESC
LIMIT 11
```

在 MariaDB 上该查询不会使用普通 B-Tree 索引，已经观察到每次搜索会全表扫描约 20w 行，查询耗时大于 200ms，属于慢 SQL。

根因：

- `LIKE '%keyword%'` 前缀通配无法走普通索引；
- 三个字段之间使用 `OR`，进一步降低索引可用性；
- 结果还需要按 `create_time DESC` 排序；
- 当前数据库实际是本机 MariaDB，不是 Docker MySQL 8，不能直接套 MySQL 8 的 `WITH PARSER ngram` 方案。

## 代码边界

下一步只处理“优化搜索慢查询”，不要做无关重构。

允许修改：

- `backend/internal/video/video_entity.go`
- `backend/internal/video/video_repo.go`
- `backend/internal/video/video_service.go`
- `backend/internal/video/video_handler.go`
- 必要时新增 `backend/internal/video` 下与搜索索引相关的 entity/repo/service 小文件
- `backend/internal/db/db.go`，仅在需要自动迁移新增搜索索引表时修改
- 前端搜索 API 或首页搜索调用，仅在后端响应结构变化时小范围同步
- 必要的搜索相关测试文件

不要修改：

- 账号、点赞、评论、关注、Feed 排序、MQ worker、上传逻辑
- Docker Compose、依赖版本、锁文件，除非用户明确要求
- 现有数据迁移方式，除非用户明确确认
- 本地 MariaDB 和 Docker MySQL 的连接配置，除非用户明确要求切库

## 行为约束

- 不要回滚用户已有改动。当前工作区已有不少未提交变更，必须增量处理。
- 手工编辑必须使用 `apply_patch`。
- 搜索优化必须基于 MariaDB 现实环境考虑，不能假设 MySQL 8 ngram parser。
- 不要继续优化 `LIKE '%keyword%'`，这不是根因修复。
- 查询结果行为应尽量保持现有接口兼容：
  - 输入仍是 `keyword`、`limit`、`latest_time`；
  - 输出仍是 `video_list`、`next_time`、`has_more`；
  - 默认按发布时间倒序分页，除非明确引入相关性排序并同步说明。
- 优先保持小步提交式改动，先能解决慢 SQL，再考虑搜索质量。

## 下一步目标

目标：让 `/video/search` 避免扫描 `videos` 全表，搜索耗时从 200ms+ 降到可接受范围。

推荐方案：为 MariaDB 实现轻量倒排索引表。

建议设计：

1. 新增搜索词表，例如：

```sql
CREATE TABLE video_search_terms (
  term VARCHAR(64) NOT NULL,
  video_id BIGINT UNSIGNED NOT NULL,
  create_time DATETIME NOT NULL,
  PRIMARY KEY (term, create_time, video_id),
  KEY idx_video_id (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

2. 发布视频成功时，把 `title + description + username` 生成 term，写入 `video_search_terms`。

3. 搜索时先查倒排表：

```sql
SELECT video_id
FROM video_search_terms
WHERE term = ?
ORDER BY create_time DESC
LIMIT 11;
```

4. 再用 `video_id` 批量回表查 `videos`，并按倒排表返回顺序组装结果。

5. `latest_time` 分页可以落在倒排表：

```sql
WHERE term = ?
  AND create_time < ?
ORDER BY create_time DESC
LIMIT 11;
```

6. 如果要兼容多个搜索词，可以先保守做：
   - 单词/短中文 term 精确匹配；
   - 多词输入拆分后取交集或并集；
   - 第一版建议做并集后按 `create_time DESC` 去重，避免复杂相关性排序。

## 需要先确认或验证

接手后先做这些检查：

```sql
SELECT VERSION();
SHOW VARIABLES LIKE 'version_comment';
SHOW CREATE TABLE videos;
EXPLAIN ANALYZE
SELECT id, author_id, username, title, description, play_url, cover_url, create_time, likes_count
FROM videos
WHERE title LIKE '%测试%'
   OR description LIKE '%测试%'
   OR username LIKE '%测试%'
ORDER BY create_time DESC
LIMIT 11;
```

确认当前 MariaDB 版本、`videos` 表结构、慢 SQL 是否仍然全表扫描。

## 已知验证阻塞

后端 `go test ./internal/video` 当前会被上传模块既有编译错误阻塞，错误集中在：

- `backend/internal/video/upload_minio.go`
- `backend/internal/video/upload_handler.go`
- `backend/internal/video/upload_service.go`

这些不是搜索任务引入的问题。优化搜索时不要顺手修上传模块，除非用户明确扩大范围。

前端验证命令此前通过：

```bash
cd frontend
npm run build
```
