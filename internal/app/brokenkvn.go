package app

import (
	"context"
	"errors"
	"html"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"remnabot/internal/brokenkvn"
	"remnabot/internal/model"
	"remnabot/internal/remnawave"
)

func (a *App) managed() bool { return a.cfg != nil && a.cfg.Managed != nil }

func (a *App) managedConfiguration(ctx context.Context, cfg *model.BotConfig, exists bool) (*model.BotConfig, error) {
	if !a.managed() {
		return cfg, nil
	}
	fresh := !exists || cfg == nil || !cfg.Installed
	if fresh {
		cfg = &model.BotConfig{Installed: true, Language: "ru", DBKind: model.DBSQLite}
	}
	cfg.DBKind = model.DBSQLite
	cfg.Panel = a.panelWithEnv(cfg.Panel)
	cfg.UpdateCheck = model.UpdateCheckConfig{Init: true, Enabled: false, ChannelChosen: true}
	if fresh {
		if err := a.store.SaveConfig(ctx, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func (a *App) managedNotice(ctx context.Context, id int64) {
	a.send(ctx, id, "Эта настройка управляется через Ansible.")
}
func (a *App) managedBlockedCallback(data string) bool {
	key, val, _ := strings.Cut(data, ":")
	switch key {
	case cbLang, cbDB, cbLoc, cbInst, cbAPIProt, cbReconf, cbUpd, cbPanelAuth, "pa":
		return true
	case cbInput:
		return strings.HasPrefix(val, "panel_")
	case cbMenu:
		switch val {
		case "reconf", "reconfig", "update", "panelauth":
			return true
		}
	}
	return false
}
func (a *App) managedPrivateMessage(m *models.Message) bool {
	return m != nil && m.From != nil && m.From.ID > 0 && m.Chat.Type == models.ChatTypePrivate && m.Chat.ID == m.From.ID
}
func (a *App) managedPrivateCallback(q *models.CallbackQuery) bool {
	m := q.Message.Message
	return m != nil && q.From.ID > 0 && m.Chat.Type == models.ChatTypePrivate && m.Chat.ID == q.From.ID
}

// Admission also protects old personal-data callbacks backed by a stored panel ref.
func (a *App) managedAdmission(ctx context.Context, id int64, active bool) bool {
	if !active && id == a.cfg.AdminID {
		return true
	}
	a.mu.Lock()
	panel := a.panel
	a.mu.Unlock()
	if panel == nil {
		a.send(ctx, id, "Панель временно недоступна.")
		return false
	}
	lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u, err := panel.FindByTelegramID(lookup, id)
	if errors.Is(err, remnawave.ErrAmbiguous) {
		a.send(ctx, id, "Найдено несколько подписок. Обратитесь к администратору.")
		return false
	}
	if err != nil {
		a.send(ctx, id, "Не удалось проверить подписку. Повторите позже.")
		return false
	}
	if !active {
		return true
	}
	if u == nil {
		a.send(ctx, id, "Для доступа нужна активная подписка.")
		return false
	}
	expires, err := time.Parse(time.RFC3339, u.ExpireAt)
	if err != nil || u.Status != "ACTIVE" || !expires.After(time.Now()) {
		a.send(ctx, id, "Для доступа нужна активная подписка.")
		return false
	}
	return true
}
func (a *App) managedFeature(ctx context.Context, id int64, feature string) {
	if !a.cfg.Managed.Features {
		a.send(ctx, id, "Раздел пока недоступен.")
		return
	}
	if !a.managedAdmission(ctx, id, true) {
		return
	}
	switch feature {
	case "proxy":
		raw, err := os.ReadFile(a.cfg.Managed.ProxyFile)
		if err != nil {
			a.send(ctx, id, "Proxy-ссылки временно недоступны.")
			return
		}
		links, err := brokenkvn.ParseLinks(string(raw))
		if err != nil {
			a.send(ctx, id, "Proxy-ссылки временно недоступны.")
			return
		}
		a.send(ctx, id, "Telegram Proxy\n"+html.EscapeString(strings.Join(links, "\n")))
	case "status":
		a.mu.Lock()
		snapshot := a.networkStatus
		a.mu.Unlock()
		a.send(ctx, id, snapshot.Text(time.Now(), a.cfg.Managed.Nodes))
	}
}
func (a *App) pollManagedStatus(ctx context.Context) {
	a.mu.Lock()
	panel := a.panel
	a.mu.Unlock()
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	nodes, err := panel.ManagedNodes(check)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.networkStatus.Failed = true
		return
	}
	a.networkStatus = brokenkvn.Snapshot{At: time.Now().UTC(), Nodes: nodes}
}
func (a *App) RunManagedStatus(ctx context.Context) {
	if !a.managed() || !a.cfg.Managed.Features {
		return
	}
	timer := time.NewTicker(60 * time.Second)
	defer timer.Stop()
	for {
		a.pollManagedStatus(ctx)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
