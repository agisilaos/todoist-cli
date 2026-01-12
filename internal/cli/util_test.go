package cli

import (
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestTableWidthConfig(t *testing.T) {
	ctx := &Context{Config: config.Config{TableWidth: 99}}
	if w := tableWidth(ctx); w != 99 {
		t.Fatalf("expected 99, got %d", w)
	}
}
