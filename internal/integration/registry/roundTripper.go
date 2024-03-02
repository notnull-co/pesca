package registry

import (
	"net/http"
	"sync"
)

type roundTripper struct {
	headers map[string]string
	mu      sync.Mutex
	http.RoundTripper
}

func newRoundTripper() *roundTripper {
	return &roundTripper{
		RoundTripper: http.DefaultTransport,
	}
}

func (r *roundTripper) addHeader(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	if _, ok := r.headers[key]; !ok {
		r.headers[key] = value
	}
}

func (rt *roundTripper) roundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range rt.headers {
		req.Header.Add(key, value)
	}

	return rt.RoundTripper.RoundTrip(req)
}
