# QUESTION.md 面试题回答交接进度

## 当前任务目标

用户正在让我们把 `QUESTION.md` 里的面试问题逐部分补成“回答稿”，要求答案直接写入 `QUESTION.md` 文档中。整体风格是中文、面试可复述、结合本项目真实代码，不要只写泛泛概念。

## 已完成情况

- 第一部分“整体架构”：已在聊天中回答过，但没有写入 `QUESTION.md`。
- 第二部分“视频发布”：已在聊天中回答过，但没有写入 `QUESTION.md`。
- 第三部分“双 Token 鉴权”：已在聊天中回答过，但没有写入 `QUESTION.md`。
- 第四部分“接口限流”：已经写入 `QUESTION.md`，位置在 `**四、接口限流**` 下方，新增了 `回答稿：` 和 8 个逐题回答。
- 第五部分“Feed 流”：已经写入 `QUESTION.md`，位置在 `**五、Feed 流**` 下方，新增了 `回答稿：` 和 12 个逐题回答。

如果用户说“继续回答剩下的问题”，优先从第六部分“RabbitMQ 异步化”开始写。如果用户明确要求把第一到第三部分也写入文档，再回头补 I-III。

## 写作格式约定

在每个章节的“重点准备”后面追加：

```markdown
回答稿：

1. **问题原文？**

   回答内容...
```

保持和第四、第五部分一致：

- 中文回答，适合面试口述。
- 每个问题单独编号。
- 先说结论，再结合项目实现。
- 不要写太虚的“高并发架构”套话，要落到本项目的表、Redis key、MQ exchange/queue、worker、事务或降级逻辑。
- 代码细节以当前仓库为准，尤其是同步/异步边界，不要过度美化。

## 关键代码地图

总体入口：

- `backend/internal/http/router.go`：路由、中间件、限流、模块装配、API 内启动 timeline outbox poller/consumer。
- `backend/cmd/main.go`：API 进程启动，连接 MySQL、Redis、RabbitMQ。
- `backend/cmd/worker/main.go`：worker 进程启动，消费 like/comment/social/popularity 队列。
- `backend/internal/db/db.go`：AutoMigrate 的实体列表。

账号/鉴权：

- `backend/internal/auth/jwt.go`：access token 15 小时，refresh token 7 天，claims 含 `account_id`、`username`、`token_type`。
- `backend/internal/middleware/jwt/jwt.go`：JWTAuth/SoftJWTAuth，先查 Redis `account:<id>`，失败回退 MySQL。
- `backend/internal/account/service.go`：登录、刷新、改名、登出、改密，写 DB 和 Redis。
- `backend/internal/account/session_keys.go`：`account:<id>`、`account:<id>:refresh`，refresh token 只存 SHA-256 hash。

视频发布：

- `backend/internal/video/video_handler.go`：`/video/publish`、上传视频/封面校验。
- `backend/internal/video/video_service.go`：发布时同一个事务写 `videos` 和 `outbox_msgs`。
- `backend/internal/video/video_entity.go`：`Video` 和 `OutboxMsg` 结构。
- `backend/internal/worker/outboxworker.go`：扫描 pending outbox，投递 timeline MQ，消费后写 Redis ZSET `feed:global_timeline`。
- `backend/internal/middleware/rabbitmq/timelineMQ.go`：`video.timeline.events`、`video.timeline.update.queue`。

接口限流：

- `backend/internal/middleware/ratelimit/ratelimit.go`：Redis `INCR + EXPIRE` 固定窗口，Redis 异常 fail-open。
- `backend/internal/http/router.go`：登录 10/min/IP，注册 5/hour/IP，点赞 30/min/account，评论 10/min/account，关注 20/min/account。

Feed：

- `backend/internal/feed/handler.go`：Feed 请求参数、游标校验、软鉴权。
- `backend/internal/feed/service.go`：最新流冷热分离、三级缓存、关注流缓存、热榜快照。
- `backend/internal/feed/repo.go`：SQL 排序和游标条件。
- `backend/internal/feed/entity.go`：请求/响应字段。

RabbitMQ/worker：

- `backend/internal/middleware/rabbitmq/*.go`：LikeMQ、CommentMQ、SocialMQ、PopularityMQ、TimelineMQ。
- `backend/internal/worker/likeworker.go`：点赞/取消点赞消费，幂等处理重复点赞，更新 `likes_count` 和 `popularity`。
- `backend/internal/worker/commentworker.go`：评论发布/删除消费，维护回复数和热度。
- `backend/internal/worker/socialworker.go`：关注/取关消费，重复关注按 MySQL 1062 当幂等成功。
- `backend/internal/worker/popularityworker.go`：热度事件写 Redis 热榜分钟桶。

点赞：

- `backend/internal/video/like_entity.go`：`video_id + account_id` 唯一索引。
- `backend/internal/video/like_service.go`：点赞/取消点赞校验，优先 MQ，失败 fallback 直写 MySQL 或 Redis。
- `backend/internal/video/like_repo.go`：`IsLiked`、`BatchGetLiked`、幂等写/删。

评论：

- `backend/internal/video/comment_entity.go`：`parent_id`、`reply_to_comment_id`、`reply_to_user_id`、`reply_count`、`status`。
- `backend/internal/video/comment_service.go`：发布校验、二级回复元信息、删除逻辑。
- `backend/internal/video/comment_repo.go`：根评论/回复分页、保留式删除。
- `backend/internal/worker/commentworker.go`：异步发布/删除的落库逻辑。

关注：

- `backend/internal/social/entity.go`：`follower_id + vlogger_id` 唯一索引。
- `backend/internal/social/service.go`：禁止关注自己，校验账号存在。注意：当前代码发 MQ 后仍然同步写 DB，不是纯异步。
- `backend/internal/social/repo.go`：关注/取关、粉丝列表、关注列表。

缓存：

- `backend/internal/feed/service.go`：视频实体 L1 本地缓存、L2 Redis、L3 MySQL；`singleflight` 合并回源。
- `backend/internal/video/video_service.go`：视频详情缓存 `video:detail:id=<id>`，Redis 锁防击穿。
- `backend/internal/middleware/redis/redis.go`：Lock/Unlock、IncrementWithExpire。
- `backend/internal/middleware/redis/zset.go`、`cache.go`：Redis 封装。

## 剩余章节建议完成顺序

1. 第六部分“RabbitMQ 异步化”
   - 重点回答：为什么异步、哪些事件走 MQ、API/worker 分工、失败和重复消费怎么处理、最终一致性。
   - 需要谨慎：`social` 当前不是纯异步，service 里发 MQ 后还同步 `repo.Follow/Unfollow`；可以说“发事件用于异步扩展/补偿，但当前接口同步生效”。
   - 重要事实：消息发布用 persistent delivery；消费者手动 ack，失败 nack requeue；点赞重复靠唯一索引/幂等 repo；outbox 用于视频发布时间线。

2. 第七部分“点赞系统”
   - 重点回答：唯一索引防重复、`likes_count` 维护、MQ + fallback、取消点赞防负数、Feed 中批量查 `is_liked`。
   - 需要核对文件：`like_service.go`、`like_repo.go`、`likeworker.go`。

3. 第八部分“评论系统”
   - 重点回答：只做两级评论；`parent_id=0` 是一级，`parent_id>0` 是回复；`reply_to_comment_id/user_id` 表达回复目标；一级评论有回复时软删除；回复数原子增减；删除后不能再回复。
   - 需要核对文件：`comment_service.go`、`comment_repo.go`、`commentworker.go`。

4. 第九部分“关注系统”
   - 重点回答：`follower_id -> vlogger_id`，唯一索引防重复，禁止关注自己，关注列表/粉丝列表，关注 Feed 查询。
   - 注意说清楚当前代码“关注/取关接口同步写 DB，同时发布 MQ 事件”，不要说成只有 worker 落库。

5. 第十部分“三级缓存”
   - 重点回答：本地缓存、Redis、MySQL分别缓存什么；本地缓存短 TTL；Redis 长一些；singleflight 解决同进程重复回源；Redis 锁解决跨实例击穿；Redis 挂了回源 MySQL。
   - 注意当前 `GetVideoByIDs` 里未判空 `f.rediscache`，回答时可以描述设计，不要主动引出 bug，除非用户要求 review。

6. 第十一部分“数据一致性”
   - 重点回答：账号 token 和视频主记录偏强一致；点赞数、热度、Feed 时间线最终一致；outbox 保证视频发布事件不丢；MQ 重复消费靠唯一索引/幂等；可补充定时校准是后续改进。

7. 第十二部分“性能与扩展”
   - 重点回答：瓶颈可能在 MySQL 热点写、Feed 查询、Redis 热 key、MQ 堆积；优化方向包括索引、缓存、队列扩容、worker 横向扩展、热榜预聚合、评论分页、分库分表/对象存储。

8. 第十三部分“项目缺陷与改进”
   - 重点回答要诚实：社交 MQ 当前同步+异步重复语义不够干净；缺少死信队列/最大重试；缺少定时校准；Feed/缓存失效可更完善；上传仍是本地磁盘，生产可换对象存储；监控告警不足。

## 可复用的事实点

- API 进程和 worker 进程可拆分部署，docker-compose 里有 `backend` 和 `worker` 两个服务。
- MySQL 是最终事实源；Redis 是缓存、时间线、热榜、限流、token 加速；RabbitMQ 是异步事件总线。
- API 层使用 Gin，数据层使用 GORM，Redis 使用 go-redis，MQ 使用 RabbitMQ topic exchange。
- Feed 返回会组装 `FeedVideoItem`，包含作者、播放地址、封面、点赞数、`is_liked`。
- 软鉴权用于公开 Feed：不带 token 可以看，带 token 但非法会 401。
- 限流和缓存多数采用“Redis 故障优先保证业务可用”的降级思路。

## 建议下一个 agent 的工作方法

1. 先打开 `QUESTION.md`，从 `**六、RabbitMQ 异步化**` 开始补。
2. 每写一个章节前，快速读对应 service/repo/worker 文件，避免写出和代码不一致的回答。
3. 只改 `QUESTION.md`，除非用户明确要求其他文件。
4. 使用 `apply_patch` 追加回答稿，不要重排已有内容。
5. 写完后用 `sed` 读回新增段落，确认 Markdown 编号、代码块和后续标题没有粘连。
