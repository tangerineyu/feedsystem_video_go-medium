package swagger

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type IsLikedResponse struct {
	IsLiked bool `json:"is_liked"`
}

type UploadVideoResponse struct {
	URL     string `json:"url"`
	PlayURL string `json:"play_url"`
}

type UploadCoverResponse struct {
	URL      string `json:"url"`
	CoverURL string `json:"cover_url"`
}
