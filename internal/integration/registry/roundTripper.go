package registry

import "net/http"

type RoundTripper struct {
	headers map[string]string
	http.RoundTripper
}

func newRoundTripper(headers map[string]string) *RoundTripper {
	return &RoundTripper{
		RoundTripper: http.DefaultTransport,
		headers:      headers,
	}
}

func (r *RoundTripper) AddHeader(key, value string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	r.headers[key] = value
}

func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range rt.headers {
		req.Header.Add(key, value)
	}

	return rt.RoundTripper.RoundTrip(req)
}
