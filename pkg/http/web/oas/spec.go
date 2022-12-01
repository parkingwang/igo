package oas

type Spec struct {
	Openapi    string                    `json:"openapi"`
	Info       DocInfo                   `json:"info"`
	Servers    []Server                  `json:"servers"`
	Tags       []Tag                     `json:"tags"`
	Paths      map[string]map[string]any `json:"paths"`
	Components struct {
		Schema map[string]Schema `json:"schema"`
	} `json:"components"`
}

type Server struct {
	Url string `json:"url"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RequestComment struct {
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
}
type Request struct {
	Tags []string `json:"tags,omitempty"`
	RequestComment
	OperationID string          `json:"operationId"`
	Parameters  []any           `json:"parameters,omitempty"`
	RequestBody *Body           `json:"requestBody,omitempty"`
	Responses   map[string]Body `json:"responses,omitempty"`
}

type DocInfo struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Version        string `json:"version"`
	TermsOfService string `json:"termsOfService"`
}

func NewSpec() *Spec {
	return &Spec{
		Openapi: "3.0.3",
		Tags:    make([]Tag, 0),
	}
}

type Body struct {
	Description string                       `json:"description,omitempty"`
	Content     map[string]map[string]Schema `json:"content,omitempty"`
	Required    bool                         `json:"required,omitempty"`
}
