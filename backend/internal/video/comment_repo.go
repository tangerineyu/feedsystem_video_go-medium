package video

import (
	"context"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) CreateComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *CommentRepository) DeleteComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Delete(comment).Error
}

func (r *CommentRepository) GetAllComments(ctx context.Context, videoID uint) ([]Comment, error) {
	var comments []Comment
	err := r.db.WithContext(ctx).Where("video_id = ?", videoID).Find(&comments).Error
	return comments, err
}

func (r *CommentRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*Comment, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) ListRootComments(ctx context.Context, videoID uint, offset, limit int) ([]Comment, error) {
	var comments []Comment
	error := r.db.WithContext(ctx).
		Where("video_id = ? AND parent_id = 0", videoID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&comments).Error
	return comments, error
}

func (r *CommentRepository) ListReplies(ctx context.Context, parentID uint, offset, limit int) ([]Comment, error) {
	var comments []Comment
	err := r.db.WithContext(ctx).
		Where("parent_id = ? AND status = ?", parentID, CommentStatusNormal).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

func (r *CommentRepository) HasReplies(ctx context.Context, parentID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Comment{}).
		Where("parent_id = ?", parentID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CommentRepository) IncreaseReplyCount(ctx context.Context, delta int64, parentID uint) error {
	// 网络波动等原因可能导致重复请求，允许增加回复数，但不允许减少回复数导致回复数变为负数
	if delta >= 0 {
		return r.db.WithContext(ctx).
			Model(&Comment{}).
			Where("id = ?", parentID).
			UpdateColumn("reply_count", gorm.Expr("reply_count + ?", delta)).Error
	}
	return r.db.WithContext(ctx).
		Model(&Comment{}).
		Where("id = ? AND reply_count > 0", parentID).
		UpdateColumn("reply_count", gorm.Expr("reply_count + ?", delta)).Error
}

func (r *CommentRepository) SoftDeleteRoot(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&Comment{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status": CommentStatusDeleted,
			"content": "",
		}).Error
}