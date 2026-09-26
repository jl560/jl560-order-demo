package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

func TestUserAPIDuplicateAndNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if err := ensureUsernameUnique(db); err != nil {
		t.Fatal(err)
	}

	const username = "f8errtest"
	_, _ = db.Exec(`DELETE FROM users WHERE username = $1`, username)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users WHERE username = $1`, username)
	})

	router := SetupRouter(db)
	body := `{"username":"f8errtest","password":"secret1","age":21}`

	created := serveJSON(router, http.MethodPost, "/users", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}

	listed := serveJSON(router, http.MethodGet, "/users", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}
	var payload struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, user := range payload.Users {
		if user["username"] != username {
			continue
		}
		found = true
		if _, ok := user["password"]; ok {
			t.Fatal("list response included password")
		}
		if _, ok := user["password_hash"]; ok {
			t.Fatal("list response included password_hash")
		}
	}
	if !found {
		t.Fatal("created user was not in the list")
	}

	duplicate := serveJSON(router, http.MethodPost, "/users", body)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Code, duplicate.Body.String())
	}

	missing := serveJSON(router, http.MethodGet, "/users/999999999", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, body = %s", missing.Code, missing.Body.String())
	}
}

func ensureUsernameUnique(db *sql.DB) error {
	_, err := db.Exec(`ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username)`)
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && (pqErr.Code == "42710" || pqErr.Code == "42P07") {
		return nil
	}
	return err
}

func serveJSON(router http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
