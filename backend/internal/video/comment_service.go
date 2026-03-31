package video

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/middleware/rabbitmq"
	rediscache "feedsystem_video_go/internal/middleware/redis"
	"strings"

	"gorm.io/gorm"
)

type CommentService struct {
	repo            *CommentRepository
	VideoRepository *VideoRepository
	cache           *rediscache.Client
	commentMQ       *rabbitmq.CommentMQ
	popularityMQ    *rabbitmq.PopularityMQ
}

func NewCommentService(repo *CommentRepository, videoRepo *VideoRepository, cache *rediscache.Client, commentMQ *rabbitmq.CommentMQ, popularityMQ *rabbitmq.PopularityMQ) *CommentService {
	return &CommentService{repo: repo, VideoRepository: videoRepo, cache: cache, commentMQ: commentMQ, popularityMQ: popularityMQ}
}

func (s *CommentService) Publish(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}
	comment.Username = strings.TrimSpace(comment.Username)
	comment.Content = strings.TrimSpace(comment.Content)
	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return errors.New("content is required")
	}

	exists, err := s.VideoRepository.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("video not found")
	}
	comment.Status = CommentStatusNormal
	mysqlEnqueued := false
	redisEnqueued := false
	if s.commentMQ != nil {
		evt := rabbitmq.CommentEvent{
			Username: comment.Username,
			VideoID: comment.VideoID,
			AuthorID: comment.AuthorID,
			Content: comment.Content,
			ParentID: comment.ParentID,
			ReplyToCommentID: comment.ReplyToCommentID,
			ReplyToUserID: comment.ReplyToUserID,
			ReplyToUsername: comment.ReplyToUsername,
		}
		if err := s.commentMQ.Publish(ctx, evt); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when comment MQ publish fails.
	if !mysqlEnqueued {
		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Select("id").First(&Video{}, comment.VideoID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("video not found")
				}
				return err
			}
			if err := tx.Create(comment).Error; err != nil {
				return err
			}
			if comment.ParentID > 0 {
				if err := tx.Model(&Comment{}).Where("id = ?", comment.ParentID).UpdateColumn("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
					return err
				}
			}
			return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
				UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
		}); err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, comment.VideoID, 1)
	}
	return nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return errors.New("comment not found")
	}
	if comment.AuthorID != accountID {
		return errors.New("permission denied")
	}
	if s.commentMQ != nil {
		if err := s.commentMQ.Delete(ctx, commentID); err == nil {
			return nil
		}
	}
	return s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if comment.ParentID == 0 {
			var count int64
			if err := tx.Model(&Comment{}).
				Where("parent_id = ?", comment.ID).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return tx.Model(&Comment{}).
					Where("id = ?", comment.ID).
					Updates(map[string]interface{}{
						"content": "This comment has been deleted.",
						"status":  CommentStatusDeleted,
					}).Error
			}
			return tx.Delete(comment).Error
		}
		if err := tx.Delete(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Comment{}).
			Where("id = ?", comment.ParentID).
			UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error
	})
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint, page, pageSize int) ([]Comment, error) {
	exists, err := s.VideoRepository.IsExist(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("video not found")
	}
	page, pageSize = normalizePage(page, pageSize)
	offset := (page - 1) * pageSize
	return s.repo.ListRootComments(ctx, videoID, offset, pageSize)
}

func (s *CommentService) ListReplies(ctx context.Context, parentID uint, page, pageSize int) ([]Comment, error) {
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if parent == nil || parent.ParentID != 0 {
		return nil, errors.New("parent comment not found")
	}
	page, pageSize = normalizePage(page, pageSize)
	offset := (page - 1) * pageSize
	return s.repo.ListReplies(ctx, parentID, offset, pageSize)
}
func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func (s *CommentService) fillReplyMeta(ctx context.Context, comment *Comment) error {
	if comment.ParentID == 0 {
		comment.ReplyToCommentID = 0
		comment.ReplyToUserID = 0
		comment.ReplyToUsername = ""
		return nil
	}
	parent, err := s.repo.GetByID(ctx, comment.ParentID)
	if err != nil {
		return err
	}
	if parent == nil || parent.ParentID != 0 {
		return errors.New("parent comment not found")
	}
	if parent.VideoID != comment.VideoID {
		return errors.New("parent comment belongs to different video")
	}
	if parent.Status == CommentStatusDeleted {
		return errors.New("cannot reply to deleted comment")
	}
	replyTarget := parent
	if comment.ReplyToCommentID != 0 && comment.ReplyToCommentID != parent.ID {
		replyTarget, err = s.repo.GetByID(ctx, comment.ReplyToCommentID)
		if err != nil {
			return err
		}
		if replyTarget == nil {
			return errors.New("reply target comment not found")
		}
		sameThread := replyTarget.ID == parent.ID || replyTarget.ParentID == parent.ID
		if replyTarget.VideoID != comment.VideoID || !sameThread {
			return errors.New("reply target comment belongs to different thread")
		}
		if replyTarget.Status == CommentStatusDeleted {
			return errors.New("cannot reply to deleted comment")
		}
	}
	comment.ReplyToCommentID = replyTarget.ID
	comment.ReplyToUserID = replyTarget.AuthorID
	comment.ReplyToUsername = replyTarget.Username
	return nil
}