package oas

type OAS struct {
	Openapi string `json:"openapi"`
	Info    struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Version     string `json:"version"`
	} `json:"info"`
	Tags       []string                  `json:"tags"`
	Paths      map[string]map[string]any `json:"paths"`
	Components struct {
		Schema map[string]Schema `json:"schema"`
	} `json:"components"`
}

type OASRequest struct {
	Tags        []string       `json:"tags"`
	Summary     string         `json:"summary"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	RequestBody map[string]any `json:"requestBody,omitempty"`
	Responses   map[string]any `json:"responses"`
}

func NewOAS() *OAS {
	return &OAS{
		Openapi: "3.0.3",
		Tags:    make([]string, 0),
	}
}
