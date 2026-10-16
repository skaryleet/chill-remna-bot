package remnawave

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"remnabot/internal/model"
	"testing"
)

func TestManagedLookup(t *testing.T) {
	for _, mode := range []string{"unique", "multiple", "wrong", "failure", "cycle", "empty"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/users/stream" {
					t.Error("legacy lookup")
					w.WriteHeader(500)
					return
				}
				if r.URL.Query().Get("telegramId") != "42" {
					t.Error("wrong id")
				}
				if mode == "failure" {
					w.WriteHeader(503)
					return
				}
				if mode == "empty" {
					fmt.Fprint(w, `{"response":{"users":[],"nextCursor":null}}`)
					return
				}
				id, tg := 1, 42
				if mode == "wrong" {
					tg = 43
				}
				if r.URL.Query().Get("cursor") == "next" && mode == "multiple" {
					id = 2
				}
				next := `"next"`
				if r.URL.Query().Get("cursor") == "next" && mode != "cycle" {
					next = `null`
				}
				fmt.Fprintf(w, `{"response":{"users":[{"id":%d,"telegramId":%d}],"nextCursor":%s}}`, id, tg, next)
			}))
			defer srv.Close()
			c := New(model.PanelConfig{BaseURL: srv.URL})
			c.Managed = true
			u, err := c.FindByTelegramID(context.Background(), 42)
			if mode == "unique" {
				if err != nil || u == nil || calls != 2 {
					t.Fatalf("unique: %v %v %d", u, err, calls)
				}
				return
			}
			if mode == "empty" {
				if err != nil || u != nil {
					t.Fatal("empty")
				}
				return
			}
			if err == nil {
				t.Fatal("unsafe lookup accepted")
			}
		})
	}
}
