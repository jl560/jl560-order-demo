package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

func TestRespondUserError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})

	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "not found", err: ErrUserNotFound, status: http.StatusNotFound, body: "用户不存在"},
		{name: "conflict", err: ErrUsernameTaken, status: http.StatusConflict, body: "用户名已存在"},
		{name: "other", err: errors.New("db down"), status: http.StatusInternalServerError, body: "查询用户失败"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			respondUserError(ctx, test.err, "查询用户失败")
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if !strings.Contains(recorder.Body.String(), test.body) {
				t.Fatalf("body = %s, want it to contain %s", recorder.Body.String(), test.body)
			}
		})
	}
}

func TestUniqueViolationBecomesUsernameTaken(t *testing.T) {
	err := asUserWriteError(&pq.Error{Code: "23505"}, "创建用户失败")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("23505 = %v, want ErrUsernameTaken", err)
	}

	other := asUserWriteError(&pq.Error{Code: "23502"}, "创建用户失败")
	if errors.Is(other, ErrUsernameTaken) {
		t.Fatal("non-unique database error was treated as a username conflict")
	}
	if !strings.Contains(other.Error(), "创建用户失败") {
		t.Fatalf("other error = %v, want the wrap text", other)
	}
}

func TestCreateUserRejectsBadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(nil)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
