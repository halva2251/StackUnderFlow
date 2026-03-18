package model

type CreateQuestionRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type ArgueRequest struct {
	Argument string `json:"argument"`
}
