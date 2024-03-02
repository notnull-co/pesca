package registry

import (
	"fmt"
)

type token struct {
	Token     string  `json:"token"`
	ExpiresIn float64 `json:"expires_in"`
}

type manifestv1 struct {
	History []history `json:"History"`
}

type history struct {
	V1Compatibility string `json:"v1Compatibility"`
}

type v1Compatibility struct {
	Created string `json:"created"`
}

type httpError struct {
	Code    int
	Message string
	Details any
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP Error: %d - %s", e.Code, e.Message)
}

type tags struct {
	Tags []string `json:"tags"`
}
