// Package brokenkvn contains the managed deployment contract, without host access.
package brokenkvn

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	PanelURL   string            `json:"panel_url"`
	PanelToken string            `json:"panel_token"`
	SupportURL string            `json:"support_url"`
	ProxyFile  string            `json:"proxy_file"`
	Nodes      map[string]string `json:"nodes"`
	Features   bool              `json:"features"`
}

var Hosts = []string{"RU-1", "RU-2", "DE-1", "NL-1", "SW-1"}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("управляемая конфигурация недоступна")
	}
	var c Config
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return nil, errors.New("некорректная управляемая конфигурация")
	}
	if c.PanelURL != "https://console.cormon.ru" || strings.TrimSpace(c.PanelToken) == "" || !validSupportURL(c.SupportURL) || c.ProxyFile != "/config/proxy-links.txt" || len(c.Nodes) != len(Hosts) {
		return nil, errors.New("неполная управляемая конфигурация")
	}
	seen := map[string]bool{}
	for _, name := range Hosts {
		id := c.Nodes[name]
		if id == "" || seen[id] {
			return nil, errors.New("некорректные идентификаторы узлов")
		}
		seen[id] = true
	}
	return &c, nil
}

func validSupportURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "t.me" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	name := strings.TrimPrefix(u.Path, "/")
	if len(name) < 5 || len(name) > 32 || strings.Contains(name, "/") {
		return false
	}
	for _, r := range name {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func ParseLinks(raw string) ([]string, error) {
	var links []string
	invalid := errors.New("некорректный файл общих Proxy-ссылок")
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			return nil, invalid
		}
		if !(u.Scheme == "tg" && u.Host == "proxy" && u.Path == "" || u.Scheme == "https" && u.Host == "t.me" && u.Path == "/proxy") || u.Fragment != "" || u.User != nil {
			return nil, invalid
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil || len(q) != 3 || len(q["server"]) != 1 || len(q["port"]) != 1 || len(q["secret"]) != 1 || q.Get("server") != "ru-ae.cormon.ru" || q.Get("port") != "2053" {
			return nil, invalid
		}
		secret := q.Get("secret")
		b, err := hex.DecodeString(secret)
		if err != nil || !(len(b) == 16 || len(b) == 17 && b[0] == 0xdd || len(b) > 17 && b[0] == 0xee) {
			return nil, invalid
		}
		links = append(links, line)
	}
	if len(links) == 0 {
		return nil, invalid
	}
	return links, nil
}
