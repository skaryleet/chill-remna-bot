// bot-verify checks a restored SQLite volume without Telegram or network access.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"remnabot/internal/crypto"
	"remnabot/internal/model"
)

func verify(dir string) error {
	key, err := os.ReadFile(filepath.Join(dir, "secret.key"))
	if err != nil || len(key) == 0 {
		return errors.New("нет ключа")
	}
	c, err := crypto.NewFromKeyMaterial(key)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "bot.db")+"?mode=ro&immutable=1")
	if err != nil {
		return err
	}
	defer db.Close()
	var integrity, encrypted string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return errors.New("нарушена целостность SQLite")
	}
	if err = db.QueryRow("SELECT config FROM settings WHERE id=1").Scan(&encrypted); err != nil {
		return errors.New("нет конфигурации")
	}
	raw, err := c.Decrypt(encrypted)
	if err != nil {
		return errors.New("конфигурация не расшифрована")
	}
	var cfg model.BotConfig
	if err = json.Unmarshal(raw, &cfg); err != nil || !cfg.Installed {
		return errors.New("конфигурация не установлена")
	}
	return nil
}
func main() {
	if err := verify("/data"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("SQLite integrity + secret.key/config decrypt: PASS")
}
