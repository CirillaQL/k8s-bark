package bark

type Response struct {
	Code      int    `json:"code"`
	message   string `json:"message"`
	timestamp int64  `json:"timestamp"`
}
