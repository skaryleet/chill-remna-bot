package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-telegram/bot/models"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"remnabot/internal/brokenkvn"
	"remnabot/internal/model"
	"remnabot/internal/remnawave"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestManagedConfigurationAndGuards(t *testing.T) {
	a, fm, fs := newTestApp(t)
	a.cfg.Managed = &brokenkvn.Config{PanelURL: "https://console.cormon.ru", PanelToken: "managed", SupportURL: "https://t.me/drknsll"}
	a.cfg.DBKind = "sqlite"
	if err := a.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !a.botCfg.Installed || a.botCfg.Panel.APIToken != "managed" || a.botCfg.Contact.SupportURL != "https://t.me/drknsll" || a.botCfg.Trial.Enabled {
		t.Fatal("bootstrap")
	}
	a.showMenu(context.Background(), 42, false, "Test")
	if !slices.Contains(fm.btnText, "🛟 Поддержка") || !slices.Contains(fm.urls, "https://t.me/drknsll") {
		t.Fatalf("support is not enabled: labels=%v urls=%v", fm.btnText, fm.urls)
	}
	a.botCfg.Panel = model.PanelConfig{BaseURL: "https://evil.invalid", APIToken: "old"}
	pc := a.panelWithEnv(a.botCfg.Panel)
	if pc.APIToken != "managed" || pc.BaseURL != "https://console.cormon.ru" {
		t.Fatal("override")
	}
	for _, data := range []string{"menu:reconfig", "upd:run", "db:postgres", "pa:token", "menu:update"} {
		if !a.managedBlockedCallback(data) {
			t.Fatalf("not blocked %s", data)
		}
	}
	if a.managedBlockedCallback("menu:broadcast") {
		t.Fatal("broadcast removed")
	}
	if !a.managedBlockedCallback("ctc:support") || !a.managedBlockedCallback("ctc:clear") {
		t.Fatal("managed support can be changed")
	}
	_ = fm
	_ = fs
}

func TestManagedFeatureAuthorization(t *testing.T) {
	for _, mode := range []string{"active", "expired", "blocked", "missing", "ambiguous", "failure", "store-blocked"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "failure" {
					w.WriteHeader(503)
					return
				}
				if mode == "missing" {
					fmt.Fprint(w, `{"response":{"users":[],"nextCursor":null}}`)
					return
				}
				status, expiry := "ACTIVE", time.Now().Add(time.Hour).Format(time.RFC3339)
				if mode == "expired" {
					expiry = time.Now().Add(-time.Hour).Format(time.RFC3339)
				}
				if mode == "blocked" {
					status = "DISABLED"
				}
				users := []map[string]any{{"id": 1, "telegramId": int64(42), "status": status, "expireAt": expiry}}
				if mode == "ambiguous" {
					users = append(users, map[string]any{"id": 2, "telegramId": int64(42)})
				}
				json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"users": users, "nextCursor": nil}})
			}))
			defer srv.Close()
			a, fm, fs := newTestApp(t)
			a.store = fs
			fs.UpsertUser(context.Background(), 42)
			if mode == "store-blocked" {
				fs.SetBlocked(context.Background(), 42, true)
			}
			path := filepath.Join(t.TempDir(), "links")
			os.WriteFile(path, []byte("tg://proxy?server=ru-ae.cormon.ru&port=2053&secret="+strings.Repeat("a", 32)), 0600)
			a.cfg.Managed = &brokenkvn.Config{Features: true, ProxyFile: path}
			a.botCfg = &model.BotConfig{Installed: true, Language: "ru"}
			a.panel = remnawave.New(model.PanelConfig{BaseURL: srv.URL})
			a.panel.Managed = true
			q := cb(42, "bkv:proxy")
			q.Message = models.MaybeInaccessibleMessage{Message: &models.Message{Chat: models.Chat{ID: 42, Type: models.ChatTypePrivate}}}
			a.handleCallback(context.Background(), q)
			got := strings.Join(fm.texts, "\n")
			if mode != "active" && (len(fm.urls) != 0 || strings.Contains(got, "secret=")) {
				t.Fatalf("unexpected disclosure: text=%q urls=%v", got, fm.urls)
			}
			if mode == "active" {
				if !slices.Contains(fm.btnText, "⚡️ Подключить Proxy 1") || !slices.Contains(fm.btnText, "⬅️ Назад") || !slices.Contains(fm.btnText, "🏠 Главная") {
					t.Fatalf("proxy UX buttons: %v", fm.btnText)
				}
				if len(fm.urls) != 1 || !strings.HasPrefix(fm.urls[0], "tg://proxy?") || strings.Contains(got, "secret=") {
					t.Fatalf("proxy presentation: text=%q urls=%v", got, fm.urls)
				}
			}
			// A valid user cannot redirect the response into a group or someone else's chat.
			fm.texts = nil
			q.Message.Message.Chat.ID = -100
			q.Message.Message.Chat.Type = models.ChatTypeGroup
			a.handleCallback(context.Background(), q)
			if len(fm.texts) != 0 {
				t.Fatal("group callback handled")
			}
		})
	}
}

func TestManagedMenuLabelsAndStatusNavigation(t *testing.T) {
	a, fm, _ := newTestApp(t)
	a.cfg.Managed = &brokenkvn.Config{Features: true, Nodes: map[string]string{"RU-1": "a"}}
	a.botCfg = &model.BotConfig{Installed: true, Language: "ru"}
	a.showMenu(context.Background(), 42, false, "Test")
	if !slices.Contains(fm.btnText, "🛡️ Telegram Proxy") || !slices.Contains(fm.btnText, "📡 Статус сети") {
		t.Fatalf("managed menu labels: %v", fm.btnText)
	}
	fm.btnText, fm.cbData = nil, nil
	a.networkStatus = brokenkvn.Snapshot{At: time.Now(), Nodes: []brokenkvn.Node{{UUID: "a", Connected: true}}}
	a.showManagedStatus(context.Background(), 42)
	got := strings.Join(fm.texts, "\n")
	if !strings.Contains(got, "OK ✅ 1/1") || !slices.Contains(fm.btnText, "⬅️ Назад") || !slices.Contains(fm.btnText, "🏠 Главная") {
		t.Fatalf("status UX: text=%q buttons=%v", got, fm.btnText)
	}
}

func TestManagedStatusFailureKeepsTimestamp(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"response":[{"uuid":"a","isConnected":true,"isDisabled":false}]}`)
	}))
	defer srv.Close()
	a, _, _ := newTestApp(t)
	a.panel = remnawave.New(model.PanelConfig{BaseURL: srv.URL})
	a.pollManagedStatus(context.Background())
	at := a.networkStatus.At
	if at.IsZero() || a.networkStatus.Failed {
		t.Fatal("initial poll")
	}
	fail = true
	a.pollManagedStatus(context.Background())
	if !a.networkStatus.At.Equal(at) || !a.networkStatus.Failed {
		t.Fatal("error masked")
	}
}

func TestManagedRestartKeepsStoreConfiguration(t *testing.T) {
	a, _, _ := newTestApp(t)
	a.cfg.Managed = &brokenkvn.Config{PanelURL: "https://console.cormon.ru", PanelToken: "new"}
	original := &model.BotConfig{Installed: true, Language: "en", P2P: model.P2PConfig{Enabled: true}}
	updated, err := a.managedConfiguration(context.Background(), original, true)
	if err != nil || updated.Language != "en" || !updated.P2P.Enabled || updated.Panel.APIToken != "new" {
		t.Fatal("config reset")
	}
}
