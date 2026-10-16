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
	b.WriteString("Статус сети — по данным панели\n")
	if s.At.IsZero() {
		b.WriteString("Время проверки: нет данных\n")
	} else {
		fmt.Fprintf(&b, "Получено: %s МСК; возраст %d с\n", s.At.In(time.FixedZone("MSK", 3*3600)).Format("02.01.2006 15:04:05"), int(now.Sub(s.At).Seconds()))
		if now.Sub(s.At) > 180*time.Second {
			b.WriteString("Данные устарели\n")
		}
	}
	if s.Failed {
		b.WriteString("Ошибка обновления; показан последний доступный снимок\n")
	}
	byID := map[string]Node{}
	for _, n := range s.Nodes {
		byID[n.UUID] = n
	}
	for _, name := range Hosts {
		id, configured := ids[name]
		if !configured {
			continue
		}
		state := "нет данных"
		if n, ok := byID[id]; ok && !s.At.IsZero() {
			state = "не подключён"
			if n.Connected {
				state = "подключён"
			}
			if n.Disabled {
				state = "отключён"
			}
		}
		fmt.Fprintf(&b, "%s: %s\n", name, state)
	}
	b.WriteString("Это наблюдение панели, не проверка клиентского VPN.")
	return b.String()
}
