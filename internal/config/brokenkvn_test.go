package config

import (
	"errors"
	"log/slog"
	"remnabot/internal/brokenkvn"
	"strings"
	"testing"
)

func TestManagedLogRedactsCredentials(t *testing.T) {
	c := &Config{BotToken: "telegram-secret", Managed: &brokenkvn.Config{PanelToken: "panel-secret"}}
	for _, attr := range []slog.Attr{slog.Any("err", errors.New("request telegram-secret panel-secret")), slog.String("msg", "telegram-secret")} {
		got := c.RedactLog(nil, attr).Value.String()
		if strings.Contains(got, "telegram-secret") || strings.Contains(got, "panel-secret") {
			t.Fatal("credential in log")
		}
	}
}
