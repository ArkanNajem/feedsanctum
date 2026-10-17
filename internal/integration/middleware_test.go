package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/auth"
	"github.com/ArkanNajem/feedsanctum/internal/handlers"
	"github.com/google/uuid"
)

func TestMiddlewareAuth(t *testing.T) {

	userID := uuid.New()
	token, err := auth.MakeJWT(userID, testCfg.JWTSecret, time.Hour)
	if err != nil {
		t.Fatalf("failed to make JWT: %v", err)
	}

	nextHandlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		id, ok := r.Context().Value(handlers.UserIDKey).(uuid.UUID)
		if !ok || id != userID {
			t.Errorf("expected user ID %s in context, got %s", userID, id)
		}
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := testCfg.MiddlewareAuth(testHandler)

	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !nextHandlerCalled {
		t.Error("expected next handler to be called")
	}
}
