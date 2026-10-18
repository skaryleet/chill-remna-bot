package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"remnabot/internal/assets"
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
	cfg.Contact.SupportURL = a.cfg.Managed.SupportURL
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
	case cbContacts:
		return val == "support" || val == "clear"
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
		a.showManagedProxy(ctx, id)
	case "status":
		a.showManagedStatus(ctx, id)
	}
}

func (a *App) managedNavigation(id int64) []models.InlineKeyboardButton {
	return navBack(a.lang(id), "menu:home")
}

func (a *App) showManagedProxy(ctx context.Context, id int64) {
	raw, err := os.ReadFile(a.cfg.Managed.ProxyFile)
	if err != nil {
		a.sendKBSection(ctx, id, assets.SectionMainMenu, "🛡️ <b>Telegram Proxy</b>\n\nСсылки временно недоступны.", [][]models.InlineKeyboardButton{a.managedNavigation(id)})
		return
	}
	links, err := brokenkvn.ParseLinks(string(raw))
	if err != nil {
		a.sendKBSection(ctx, id, assets.SectionMainMenu, "🛡️ <b>Telegram Proxy</b>\n\nСсылки временно недоступны.", [][]models.InlineKeyboardButton{a.managedNavigation(id)})
		return
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(links)+1)
	for i, link := range links {
		rows = append(rows, []models.InlineKeyboardButton{{Text: fmt.Sprintf("⚡️ Подключить Proxy %d", i+1), URL: link}})
	}
	rows = append(rows, a.managedNavigation(id))
	a.sendKBSection(ctx, id, assets.SectionMainMenu, "🛡️ <b>Telegram Proxy</b>\n\nВыберите способ подключения:", rows)
}

func (a *App) showManagedStatus(ctx context.Context, id int64) {
	a.mu.Lock()
	snapshot := a.networkStatus
	a.mu.Unlock()
	a.sendKBSection(ctx, id, assets.SectionMainMenu, snapshot.Text(time.Now(), a.cfg.Managed.Nodes), [][]models.InlineKeyboardButton{a.managedNavigation(id)})
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
