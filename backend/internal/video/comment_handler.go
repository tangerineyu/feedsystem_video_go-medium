package video

import (
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/middleware/jwt"
	"feedsystem_video_go/internal/swagger"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	service        *CommentService
	accountService *account.AccountService
}

var _ = swagger.ErrorResponse{}

func NewCommentHandler(service *CommentService, accountService *account.AccountService) *CommentHandler {
	return &CommentHandler{service: service, accountService: accountService}
}

// PublishComment godoc
// @Summary Publish a comment
// @Tags comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body PublishCommentRequest true "publish comment payload"
// @Success 200 {object} swagger.MessageResponse
// @Failure 400 {object} swagger.ErrorResponse
// @Router /comment/publish [post]
func (h *CommentHandler) PublishComment(c *gin.Context) {
	var req PublishCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}
	if req.VideoID <= 0 {
		c.JSON(400, gin.H{"error": "video_id is required"})
		return
	}
	authorId, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	user, err := h.accountService.FindByID(c.Request.Context(), authorId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	comment := &Comment{
		Username:         user.Username,
		VideoID:          req.VideoID,
		AuthorID:         authorId,
		Content:          req.Content,
		ParentID:         req.ParentID,
		ReplyToCommentID: req.ReplyToCommentID,
	}
	if err := h.service.Publish(c.Request.Context(), comment); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "comment published successfully"})
}

// DeleteComment godoc
// @Summary Delete a comment
// @Tags comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteCommentRequest true "delete comment payload"
// @Success 200 {object} swagger.MessageResponse
// @Failure 400 {object} swagger.ErrorResponse
// @Router /comment/delete [post]
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	var req DeleteCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.CommentID <= 0 {
		c.JSON(400, gin.H{"error": "comment_id is required"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), req.CommentID, accountID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "comment deleted successfully"})
}

// GetAllComments godoc
// @Summary List root comments of a video
// @Tags comment
// @Accept json
// @Produce json
// @Param request body GetAllCommentsRequest true "list comments payload"
// @Success 200 {object} CommentsResponse
// @Failure 400 {object} swagger.ErrorResponse
// @Router /comment/listAll [post]
func (h *CommentHandler) GetAllComments(c *gin.Context) {
	var req GetAllCommentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.VideoID == 0 {
		c.JSON(400, gin.H{"error": "video_id is required"})
		return
	}
	comments, err := h.service.GetAll(c.Request.Context(), req.VideoID, req.Page, req.PageSize)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"comments": comments})
}

// ListReplies godoc
// @Summary List replies under a parent comment
// @Tags comment
// @Accept json
// @Produce json
// @Param request body ListRepliesRequest true "list replies payload"
// @Success 200 {object} RepliesResponse
// @Failure 400 {object} swagger.ErrorResponse
// @Router /comment/listReplies [post]
func (h *CommentHandler) ListReplies(c *gin.Context) {
	var req ListRepliesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ParentID == 0 {
		c.JSON(400, gin.H{"error": "parent_id is required"})
		return
	}
	replies, err := h.service.ListReplies(c.Request.Context(), req.ParentID, req.Page, req.PageSize)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"replies": replies})
}
