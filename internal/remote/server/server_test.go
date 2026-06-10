package server

import (
	"context"
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

func TestReadyEndpoint(t *testing.T) {
	baseCfg := config.RemoteSessionControllerConfig{
		ServerPort:         65532,
		SessionPort:        0,
		SessionURLPath:     "/",
		ReadinessProbeType: "",
	}

	tests := []struct {
		name         string
		cfg          config.RemoteSessionControllerConfig
		setupBackend func() (cleanup func())
		wantStatus   int
	}{
		{
			name: "none probe returns 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.None)
				return c
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive with empty probe type defaults to 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = ""
				return c
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive tcp open port returns 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.TCP)
				c.SessionPort = 18000
				return c
			}(),
			setupBackend: func() func() {
				ln, err := net.Listen("tcp", "127.0.0.1:18000")
				if err != nil {
					panic(err)
				}
				return func() { _ = ln.Close() }
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive tcp closed port returns 503",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.TCP)
				c.SessionPort = 18001
				return c
			}(),
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "interactive tcp port 0 defaults to 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.TCP)
				c.SessionPort = 0
				return c
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name: "interactive http serving 200 returns 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = 18002
				c.SessionURLPath = "/lab"
				return c
			}(),
			setupBackend: func() func() {
				ln, err := net.Listen("tcp", "127.0.0.1:18002")
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
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = 18003
				c.SessionURLPath = "/"
				return c
			}(),
			setupBackend: func() func() {
				ln, err := net.Listen("tcp", "127.0.0.1:18003")
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
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = 18004
				c.SessionURLPath = "/"
				return c
			}(),
			setupBackend: func() func() {
				ln, err := net.Listen("tcp", "127.0.0.1:18004")
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
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = 18005
				c.SessionURLPath = "/"
				return c
			}(),
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "interactive http port 0 defaults to 200",
			cfg: func() config.RemoteSessionControllerConfig {
				c := baseCfg
				c.ReadinessProbeType = string(amaltheadevv1alpha1.HTTP)
				c.SessionPort = 0
				return c
			}(),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupBackend != nil {
				cleanup := tt.setupBackend()
				defer cleanup()
			}

			e := newServer(&mockController{}, tt.cfg)
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
