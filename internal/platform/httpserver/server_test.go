package httpserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestA10(t *testing.T) {
	t.Parallel()

	readinessGate := NewReadinessGate(Readiness{
		Timeout: 50 * time.Millisecond,
		Probes: []Probe{
			{
				Name:     "database",
				Critical: true,
				Check: func(context.Context) error {
					return nil
				},
			},
		},
	})
	router := NewRouter(readinessGate)

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseRequest)
		})
	}
	t.Cleanup(release)

	router.GET("/slow", func(ctx *gin.Context) {
		close(requestStarted)
		<-releaseRequest
		ctx.String(http.StatusOK, "complete")
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})

	server := NewServer(
		ServerConfig{
			ReadHeaderTimeout: time.Second,
			ShutdownTimeout:   time.Second,
		},
		router,
		readinessGate,
	)
	serverContext, cancelServer := context.WithCancel(context.Background())
	t.Cleanup(cancelServer)

	serverResult := make(chan error, 1)
	go func() {
		serverResult <- server.Serve(serverContext, listener)
	}()

	type clientResult struct {
		status int
		body   string
		err    error
	}
	responseResult := make(chan clientResult, 1)
	go func() {
		response, requestErr := (&http.Client{Timeout: 2 * time.Second}).Get(
			"http://" + listener.Addr().String() + "/slow",
		)
		if requestErr != nil {
			responseResult <- clientResult{err: requestErr}
			return
		}
		defer response.Body.Close()

		body, readErr := io.ReadAll(response.Body)
		responseResult <- clientResult{
			status: response.StatusCode,
			body:   string(body),
			err:    readErr,
		}
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("slow request did not start")
	}

	cancelServer()
	require.Eventually(t, func() bool {
		return !readinessGate.Ready(context.Background())
	}, time.Second, 5*time.Millisecond)

	select {
	case result := <-responseResult:
		t.Fatalf("in-flight request ended before release: %+v", result)
	case err := <-serverResult:
		t.Fatalf("server exited before in-flight request drained: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	release()

	select {
	case result := <-responseResult:
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		require.Equal(t, "complete", result.body)
	case <-time.After(time.Second):
		t.Fatal("in-flight request was not completed")
	}

	select {
	case err := <-serverResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("server did not exit after request drain")
	}
}
