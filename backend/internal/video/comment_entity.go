package video

import "time"

const (
	CommentStatusNormal  = 0
	CommentStatusDeleted = 1
)
type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"index" json:"username"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	AuthorID  uint      `gorm:"index" json:"author_id"`
	Content   string    `gorm:"type:text" json:"content"`
	ParentID  uint      `gorm:"index" json:"parent_id"`
	ReplyToCommentID uint      `gorm:"index" json:"reply_to_comment_id"`
	ReplyToUserID uint      `gorm:"index" json:"reply_to_user_id"`
	ReplyToUsername string    `gorm:"index" json:"reply_to_username"`
	ReplyCount uint	  `gorm:"default:0" json:"reply_count"`
	Status   int       `gorm:"default:0" json:"status"` // 0: normal, 1: deleted
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type PublishCommentRequest struct {
	VideoID uint   `json:"video_id"`
	Content string `json:"content"`
	ParentID uint `json:"parent_id"`
	ReplyToCommentID uint `json:"reply_to_comment_id"`
}

type DeleteCommentRequest struct {
	CommentID uint `json:"comment_id"`
}

type GetAllCommentsRequest struct {
	VideoID uint `json:"video_id"`
	Page int `json:"page"`
	PageSize int `json:"page_size"`
}

type ListRepliesRequest struct {
	ParentID uint `json:"parent_id"`
	Page int `json:"page"`
	PageSize int `json:"page_size"`
}


