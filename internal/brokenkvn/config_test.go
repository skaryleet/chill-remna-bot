package brokenkvn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinks(t *testing.T) {
	raw := "# comment\n\ntg://proxy?server=ru-ae.cormon.ru&port=2053&secret=" + strings.Repeat("a", 32)
	got, err := ParseLinks(raw)
	if err != nil || len(got) != 1 || got[0] != strings.Split(raw, "\n")[2] {
		t.Fatalf("valid links: %v", err)
	}
	for _, bad := range []string{"", raw + "&secret=bad", raw + "&x=1", strings.ReplaceAll(raw, "2053", "443"), strings.ReplaceAll(raw, "ru-ae", "evil"), raw + "\ninvalid"} {
		if _, err := ParseLinks(bad); err == nil {
			t.Fatal("accepted invalid links")
		}
	}
	if _, err := ParseLinks(strings.ReplaceAll(raw, "tg://proxy?", "https://t.me/proxy?")); err != nil {
		t.Fatal(err)
	}
}
func TestConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "managed.json")
	raw := `{"panel_url":"https://console.cormon.ru","panel_token":"test-secret","proxy_file":"/config/proxy-links.txt","nodes":{"RU-1":"a","RU-2":"b","DE-1":"c","NL-1":"d","SW-1":"e"},"features":true}`
	for _, bad := range []string{`{`, strings.ReplaceAll(raw, "https://", "http://"), strings.ReplaceAll(raw, "test-secret", ""), strings.ReplaceAll(raw, `"e"`, `"a"`)} {
		os.WriteFile(p, []byte(bad), 0600)
		if _, err := Load(p); err == nil || strings.Contains(err.Error(), "test-secret") {
			t.Fatalf("invalid config: %v", err)
		}
	}
	os.WriteFile(p, []byte(raw), 0600)
	if _, err := Load(p); err != nil {
		t.Fatal(err)
	}
}
func TestStatus(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	s := Snapshot{}
	if !strings.Contains(s.Text(now, map[string]string{"RU-1": "a"}), "нет данных") {
		t.Fatal("empty")
	}
	s = Snapshot{At: now.Add(-181 * time.Second), Failed: true, Nodes: []Node{{UUID: "a", Connected: true}, {UUID: "b", Disabled: true}}}
	got := s.Text(now, map[string]string{"RU-1": "a", "RU-2": "b", "DE-1": "missing"})
	for _, want := range []string{"устарели", "Ошибка обновления", "отключён", "нет данных", "по данным панели", "МСК"} {
		if !strings.Contains(got, want) {
			t.Fatal(want, got)
		}
	}
}
