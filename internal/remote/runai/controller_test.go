package runai

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
)

func TestStartDoesNotLogEnvironment(t *testing.T) {
	const secret = "super-secret-wstunnel-value"
	t.Setenv("RSC_WSTUNNEL_SECRET", secret)
	t.Setenv("RENKU_MOUNT_DIR", t.TempDir())

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	c := &RunaiRemoteSessionController{
		project:       "test",
		currentStatus: models.NotReady,
		statusTicker:  time.NewTicker(time.Minute),
		fakeStart:     true,
	}
	t.Cleanup(c.statusTicker.Stop)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("log output leaked an environment secret")
	}
	if strings.Contains(out, "env=") {
		t.Fatalf("log output contains an environment dump")
	}
}
