package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type ReadinessGate struct {
	accepting atomic.Bool
	checker   ReadyChecker
}

func NewReadinessGate(checker ReadyChecker) *ReadinessGate {
	gate := &ReadinessGate{checker: checker}
	gate.accepting.Store(true)
	return gate
}

func (gate *ReadinessGate) Ready(ctx context.Context) bool {
	if gate == nil || !gate.accepting.Load() || gate.checker == nil {
		return false
	}
	ready := gate.checker.Ready(ctx)
	return ready && gate.accepting.Load()
}

func (gate *ReadinessGate) BeginDrain() {
	if gate != nil {
		gate.accepting.Store(false)
	}
}

type ServerConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

type Server struct {
	httpServer      *http.Server
	readiness       *ReadinessGate
	shutdownTimeout time.Duration
}

func NewServer(
	config ServerConfig,
	handler http.Handler,
	readiness *ReadinessGate,
) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              config.Address,
			Handler:           handler,
			ReadHeaderTimeout: config.ReadHeaderTimeout,
		},
		readiness:       readiness,
		shutdownTimeout: config.ShutdownTimeout,
	}
}

func (server *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.httpServer.Addr)
	if err != nil {
		return err
	}
	return server.Serve(ctx, listener)
}

func (server *Server) Serve(ctx context.Context, listener net.Listener) error {
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.httpServer.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		return normalizeServeError(err)
	case <-ctx.Done():
	}

	server.readiness.BeginDrain()
	server.httpServer.SetKeepAlivesEnabled(false)

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		server.shutdownTimeout,
	)
	defer cancel()

	shutdownErr := server.httpServer.Shutdown(shutdownContext)
	if shutdownErr != nil {
		closeErr := server.httpServer.Close()
		serveErr := <-serveResult
		return errors.Join(
			shutdownErr,
			closeErr,
			normalizeServeError(serveErr),
		)
	}

	return normalizeServeError(<-serveResult)
}

func normalizeServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
