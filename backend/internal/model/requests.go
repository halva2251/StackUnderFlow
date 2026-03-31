package model

type CreateQuestionRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type ArgueRequest struct {
	Argument string `json:"argument"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type VoteRequest struct {
	Value int `json:"value"`
}
