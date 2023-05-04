package httpx

import (
	"log"
	"net/http"
	"time"
)

type LoggingRoundtripper struct {
	Transport http.RoundTripper
}

func NewLoggingRoundTripper() http.RoundTripper {
	return &LoggingRoundtripper{Transport: http.DefaultTransport}
}

func (t *LoggingRoundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t0 := time.Now()

	res, err := t.Transport.RoundTrip(req)
	if err != nil {
		return res, err
	}

	log.Printf("[info] %s %s -> %d (%d ms)", res.Request.Method, res.Request.URL, res.StatusCode, time.Since(t0).Milliseconds())

	return res, err
}
