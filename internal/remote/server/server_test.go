package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/stretchr/testify/assert"
)

type mockController struct{}

func (m *mockController) Status(ctx context.Context) (models.RemoteSessionState, error) {
	return models.Running, nil
}
func (m *mockController) Start(ctx context.Context) error { return nil }
func (m *mockController) Stop(ctx context.Context) error  { return nil }

func getFreePortOrDie() int32 {
	var a *net.TCPAddr
	var err error
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer func() {
				err := l.Close()
				if err != nil {
					panic(err)
				}
			}()
			return int32(l.Addr().(*net.TCPAddr).Port)
		}
		panic(err)
	}
	panic(err)
}

func TestReadyEndpoint(t *testing.T) {
	baseCfg := config.RemoteSessionControllerConfig{
		ServerPort:         65532,
		SessionPort:        0,
		SessionURLPath:     "/",
		ReadinessProbeType: "none",
	}

	tests := []struct {
		name         string
		makeCfg      func(port int32) config.RemoteSessionControllerConfig
		setupBackend func(port int32) (cleanup func())
		wantStatus   int
	}{
		{
			name: "none probe returns 200",
			makeCfg: func(int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.None)
				return c
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive tcp open port returns 200",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.TCP)
				c.SessionPort = port
				return c
			},
			setupBackend: func(port int32) func() {
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					panic(err)
				}
				return func() { _ = ln.Close() }
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive tcp closed port returns 503",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.TCP)
				c.SessionPort = port
				return c
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "interactive http serving 200 returns 200",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = port
				c.SessionURLPath = "/lab"
				return c
			},
			setupBackend: func(port int32) func() {
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					panic(err)
				}
				srv := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/lab" {
							w.WriteHeader(http.StatusOK)
							return
						}
						w.WriteHeader(http.StatusNotFound)
					}),
				}
				go func() { _ = srv.Serve(ln) }()
				return func() { _ = srv.Close() }
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive http redirect 302 returns 200",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = port
				c.SessionURLPath = "/"
				return c
			},
			setupBackend: func(port int32) func() {
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					panic(err)
				}
				srv := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						http.Redirect(w, r, "/other", http.StatusFound)
					}),
				}
				go func() { _ = srv.Serve(ln) }()
				return func() { _ = srv.Close() }
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive http serving 500 returns 503",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = port
				c.SessionURLPath = "/"
				return c
			},
			setupBackend: func(port int32) func() {
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					panic(err)
				}
				srv := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					}),
				}
				go func() { _ = srv.Serve(ln) }()
				return func() { _ = srv.Close() }
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "interactive http connection refused returns 503",
			makeCfg: func(port int32) config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = port
				c.SessionURLPath = "/"
				return c
			},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := getFreePortOrDie()
			cfg := tt.makeCfg(port)

			if tt.setupBackend != nil {
				cleanup := tt.setupBackend(port)
				defer cleanup()
			}

			e := newServer(&mockController{}, cfg)
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
