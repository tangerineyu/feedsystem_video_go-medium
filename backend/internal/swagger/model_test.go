package swagger_test

import (
	"testing"

	"feedsystem_video_go/internal/swagger"
)

func TestSwaggerModelsCompileWithoutBusinessImports(t *testing.T) {
	_ = swagger.ErrorResponse{Error: "bad request"}
	_ = swagger.MessageResponse{Message: "ok"}
	_ = swagger.IsLikedResponse{IsLiked: true}
	_ = swagger.UploadVideoResponse{URL: "u", PlayURL: "p"}
	_ = swagger.UploadCoverResponse{URL: "u", CoverURL: "c"}
}
