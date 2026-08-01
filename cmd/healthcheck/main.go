package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	defaultHealthcheckURL     = "http://127.0.0.1:8080/readyz"
	defaultHealthcheckTimeout = 3 * time.Second
)

var errUnhealthy = errors.New("healthcheck failed")

func main() {
	target := os.Getenv("HEALTHCHECK_URL")
	if target == "" {
		target = defaultHealthcheckURL
	}
	if check(context.Background(), target, defaultHealthcheckTimeout) != nil {
		os.Exit(1)
	}
}

func check(ctx context.Context, target string, timeout time.Duration) error {
	parsed, err := url.Parse(target)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Path != "/readyz" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		timeout <= 0 {
		return errUnhealthy
	}
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodGet,
		parsed.String(),
		nil,
	)
	if err != nil {
		return errUnhealthy
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errUnhealthy
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return errUnhealthy
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return errUnhealthy
	}
	return nil
}
