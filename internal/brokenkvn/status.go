package brokenkvn

import (
	"fmt"
	"strings"
	"time"
)

type Node struct {
	UUID      string
	Connected bool
	Disabled  bool
}
type Snapshot struct {
	At     time.Time
	Failed bool
	Nodes  []Node
}

func (s Snapshot) Text(now time.Time, ids map[string]string) string {
	var b strings.Builder
	b.WriteString("📡 <b>Статус сети</b>\n\n")
	if s.At.IsZero() {
		fmt.Fprintf(&b, "Нет данных ⏳ 0/%d", len(ids))
		return b.String()
	} else {
		fmt.Fprintf(&b, "Обновлено: %s МСК\n", s.At.In(time.FixedZone("MSK", 3*3600)).Format("02.01.2006 15:04:05"))
		if now.Sub(s.At) > 180*time.Second {
			b.WriteString("Данные устарели ⚠️\n")
		}
	}
	if s.Failed {
		b.WriteString("Не удалось обновить ⚠️\n")
	}
	byID := map[string]Node{}
	for _, n := range s.Nodes {
		byID[n.UUID] = n
	}
	okCount, problemCount, deniedCount := 0, 0, 0
	for _, name := range Hosts {
		id, configured := ids[name]
		if !configured {
			continue
		}
		n, found := byID[id]
		switch {
		case !found || n.Disabled:
			deniedCount++
		case n.Connected:
			okCount++
		default:
			problemCount++
		}
	}
	total := len(ids)
	fmt.Fprintf(&b, "\nOK ✅ %d/%d", okCount, total)
	if problemCount > 0 {
		fmt.Fprintf(&b, "\nЕсть проблемы ⚠️ %d/%d", problemCount, total)
	}
	if deniedCount > 0 {
		fmt.Fprintf(&b, "\nНет доступа ❗ %d/%d", deniedCount, total)
	}
	return b.String()
}
