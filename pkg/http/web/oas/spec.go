package oas

type Spec struct {
	Openapi    string                    `json:"openapi"`
	Info       DocInfo                   `json:"info"`
	Servers    []Server                  `json:"servers"`
	Tags       []string                  `json:"tags"`
	Paths      map[string]map[string]any `json:"paths"`
	Components struct {
		Schema map[string]Schema `json:"schema"`
	} `json:"components"`
}

type Server struct {
	Url string `json:"url"`
}
type OASRequest struct {
	Tags        []string       `json:"tags"`
	Summary     string         `json:"summary"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	RequestBody map[string]any `json:"requestBody,omitempty"`
	Responses   map[string]any `json:"responses"`
}

type DocInfo struct {
	Name        string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

func NewSpec() *Spec {
	return &Spec{
		Openapi: "3.0.3",
		Tags:    make([]string, 0),
	}
}
