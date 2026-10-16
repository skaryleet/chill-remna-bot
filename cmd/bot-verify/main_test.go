package main

import (
	"context"
	"os"
	"path/filepath"
	"remnabot/internal/crypto"
	"remnabot/internal/model"
	"remnabot/internal/storage"
	"testing"
)

func TestVerifyData(t *testing.T) {
	dir := t.TempDir()
	if verify(dir) == nil {
		t.Fatal("missing files accepted")
	}
	key, err := crypto.LoadOrCreate("", dir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(model.DBSQLite, filepath.Join(dir, "bot.db"), key)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveConfig(context.Background(), &model.BotConfig{Installed: true, Language: "ru"}); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = verify(dir); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "secret.key"), []byte("wrong key"), 0600)
	if verify(dir) == nil {
		t.Fatal("wrong key accepted")
	}
}
