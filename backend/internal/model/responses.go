package model

type QuestionResponse struct {
	Question Question `json:"question"`
	Answers  []Answer `json:"answers"`
}

type QuestionListResponse struct {
	Questions []Question `json:"questions"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
	Sort      string     `json:"sort"`
}

type LeaderboardEntry struct {
	Answer   Answer   `json:"answer"`
	Question Question `json:"question"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
