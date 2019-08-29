package action

type FetchLogsWithSignedURLRequest struct {
	SignedURL string `json:"signed_url"`

	LogType string   `json:"log_type"`
	Filters []string `json:"filters"`
}

type FetchLogsWithSignedURLResponse struct {
	SHA1Digest string `json:"sha1_digest"`
}

type FetchLogsWithSignedURL struct{}

func (a FetchLogsWithSignedURL) Run(request FetchLogsWithSignedURLRequest) (FetchLogsWithSignedURLResponse, error) {
	return FetchLogsWithSignedURLResponse{}, nil
}
