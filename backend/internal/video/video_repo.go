package video

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type VideoRepository struct {
	db *gorm.DB
}

type searchVideoIDRow struct {
	VideoID    uint
	CreateTime time.Time
}

func NewVideoRepository(db *gorm.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (vr *VideoRepository) CreateVideo(ctx context.Context, video *Video) error {
	if err := vr.db.WithContext(ctx).Create(video).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) CreateMsg(ctx context.Context, Msg *OutboxMsg) error {
	if err := vr.db.WithContext(ctx).Create(Msg).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) DeleteVideo(ctx context.Context, id uint) error {
	if err := vr.db.WithContext(ctx).Delete(&Video{}, id).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) ListByAuthorID(ctx context.Context, authorID int64) ([]Video, error) {
	var videos []Video
	if err := vr.db.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("create_time desc").
		Offset(0).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (vr *VideoRepository) Search(ctx context.Context, keyword string, limit int, latestBefore time.Time) ([]Video, error) {
	terms := buildVideoSearchTerms(keyword)
	if len(terms) == 0 {
		return []Video{}, nil
	}

	videoIDs, err := vr.searchVideoIDs(ctx, vr.db, terms, limit, latestBefore)
	if err != nil {
		return nil, err
	}
	if len(videoIDs) == 0 {
		return []Video{}, nil
	}

	var videos []Video
	if err := vr.db.WithContext(ctx).
		Where("id IN ?", videoIDs).
		Find(&videos).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]Video, len(videos))
	for _, video := range videos {
		byID[video.ID] = video
	}

	ordered := make([]Video, 0, len(videoIDs))
	for _, id := range videoIDs {
		if video, ok := byID[id]; ok {
			ordered = append(ordered, video)
		}
	}
	return ordered, nil
}

func (vr *VideoRepository) searchVideoIDs(ctx context.Context, db *gorm.DB, terms []string, limit int, latestBefore time.Time) ([]uint, error) {
	var rows []searchVideoIDRow
	query := vr.searchVideoIDQuery(ctx, db, terms, latestBefore)
	if err := query.Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}

	videoIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		videoIDs = append(videoIDs, row.VideoID)
	}
	return videoIDs, nil
}

func (vr *VideoRepository) searchVideoIDQuery(ctx context.Context, db *gorm.DB, terms []string, latestBefore time.Time) *gorm.DB {
	query := db.WithContext(ctx).
		Model(&VideoSearchTerm{})
	if len(terms) == 1 {
		query = query.Select("video_id, create_time").Where("term = ?", terms[0])
	} else {
		query = query.Select("video_id, MAX(create_time) AS create_time").
			Where("term IN ?", terms).
			Group("video_id")
	}
	query = query.Order("create_time desc")
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	return query
}

func (vr *VideoRepository) CreateSearchTerms(ctx context.Context, db *gorm.DB, video *Video) error {
	terms := buildVideoSearchTerms(video.Title, video.Description, video.Username)
	if len(terms) == 0 {
		return nil
	}

	searchTerms := make([]VideoSearchTerm, 0, len(terms))
	for _, term := range terms {
		searchTerms = append(searchTerms, VideoSearchTerm{
			Term:       term,
			VideoID:    video.ID,
			CreateTime: video.CreateTime,
		})
	}

	return db.WithContext(ctx).Create(&searchTerms).Error
}

func (vr *VideoRepository) GetByID(ctx context.Context, id uint) (*Video, error) {
	var video Video
	if err := vr.db.WithContext(ctx).First(&video, id).Error; err != nil {
		return (*Video)(nil), err
	}
	return &video, nil
}

func (vr *VideoRepository) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	if err := vr.db.WithContext(ctx).Model(&Video{}).
		Where("id = ?", id).
		Update("likes_count", likesCount).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var video Video
	if err := vr.db.WithContext(ctx).First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (vr *VideoRepository) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	if err := vr.db.WithContext(ctx).Model(&Video{}).
		Where("id = ?", id).
		Update("popularity", gorm.Expr("popularity + ?", change)).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) ChangeLikesCount(ctx context.Context, id uint, change int64) error {
	if err := vr.db.WithContext(ctx).Model(&Video{}).
		Where("id = ?", id).
		UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?, 0)", change)).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) ChangePopularity(ctx context.Context, id uint, change int64) error {
	if err := vr.db.WithContext(ctx).Model(&Video{}).
		Where("id = ?", id).
		UpdateColumn("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", change)).Error; err != nil {
		return err
	}
	return nil
}
