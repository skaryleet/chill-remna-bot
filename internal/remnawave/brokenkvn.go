package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"remnabot/internal/brokenkvn"
)

var ErrAmbiguous = errors.New("несколько подписок: требуется разбор администратором")

// managedUser never falls back to username or a pre-v3 endpoint.
func (c *Client) managedUser(ctx context.Context, telegramID int64) (*panelUser, error) {
	if telegramID <= 0 {
		return nil, errors.New("некорректный Telegram ID")
	}
	c.gen.Store(int32(genV3))
	cursor := ""
	seen := map[string]bool{}
	var found *panelUser
	for {
		users, next, err := c.managedPage(ctx, telegramID, cursor)
		if err != nil {
			return nil, err
		}
		for i := range users {
			u := &users[i]
			if u.TelegramID != telegramID || u.ID <= 0 {
				return nil, errors.New("панель вернула несоответствующую запись")
			}
			if found != nil && found.ID != u.ID {
				return nil, ErrAmbiguous
			}
			found = u
		}
		if next == "" {
			return found, nil
		}
		if seen[next] {
			return nil, errors.New("панель вернула повторяющийся курсор")
		}
		seen[next] = true
		cursor = next
	}
}
func (c *Client) managedPage(ctx context.Context, id int64, cursor string) ([]panelUser, string, error) {
	q := url.Values{"telegramId": {strconv.FormatInt(id, 10)}, "size": {"100"}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	resp, err := c.do(ctx, http.MethodGet, "/api/users/stream?"+q.Encode(), nil)
	if err != nil {
		return nil, "", errors.New("нет связи с панелью")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New("панель отклонила поиск пользователя")
	}
	var body struct {
		Response *struct {
			Users   []panelUser `json:"users"`
			Next    *string     `json:"nextCursor"`
			HasMore bool        `json:"hasMore"`
		} `json:"response"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil || body.Response == nil || body.Response.Users == nil {
		return nil, "", errors.New("некорректный ответ панели")
	}
	next := ""
	if body.Response.Next != nil {
		next = *body.Response.Next
	}
	if body.Response.HasMore && next == "" {
		return nil, "", errors.New("панель не вернула следующий курсор")
	}
	return body.Response.Users, next, nil
}
func (c *Client) ManagedNodes(ctx context.Context) ([]brokenkvn.Node, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/nodes", nil)
	if err != nil {
		return nil, errors.New("нет связи с панелью")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("статус узлов недоступен")
	}
	var body struct {
		Response []struct {
			UUID      string `json:"uuid"`
			Connected *bool  `json:"isConnected"`
			Disabled  *bool  `json:"isDisabled"`
		} `json:"response"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil || body.Response == nil {
		return nil, errors.New("некорректный статус узлов")
	}
	var nodes []brokenkvn.Node
	seen := map[string]bool{}
	for _, n := range body.Response {
		if n.UUID == "" || n.Connected == nil || n.Disabled == nil || seen[n.UUID] {
			return nil, errors.New("неполный статус узлов")
		}
		seen[n.UUID] = true
		nodes = append(nodes, brokenkvn.Node{UUID: n.UUID, Connected: *n.Connected, Disabled: *n.Disabled})
	}
	return nodes, nil
}
