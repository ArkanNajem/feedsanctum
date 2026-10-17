package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/database"
	"github.com/ArkanNajem/feedsanctum/internal/handlers"
	"github.com/google/uuid"
)

func TestHandlerCreateFeed(t *testing.T) {
	uniqueEmail := fmt.Sprintf("test_%s@example.com", uuid.New().String())
	user, err := testCfg.DB.CreateUser(context.Background(), database.CreateUserParams{
		Name:           "Test User",
		Email:          uniqueEmail,
		HashedPassword: "$argon2id$v=19$m=65536,t=1,p=12$gpkp+j/yfZEvLC3KUb29yA$9Z5sb9kL5veQCfSk7tteFGkUqmaMkgEFztGAc48YFNI",
		// hash for password : placeholdertest1234567890
	})
	if err != nil {
		t.Fatalf("failed to create dummy user for test: %v", err)
	}

	reqBody := map[string]string{
		"url":  "https://example.com/rss.xml",
		"name": "Example Feed",
	}
	reqDataBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/api/vi/feeds", bytes.NewBuffer(reqDataBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), handlers.UserIDKey, user.ID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	testCfg.HandlerCreateFeed(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var respBody struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		FeedID    uuid.UUID `json:"feed_id"`
		Name      string    `json:"name"`
		UserName  string    `json:"user_name"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if respBody.Name != "Example Feed" {
		t.Errorf("expected feed name 'Example Feed', got '%s'", respBody.Name)
	}
	if respBody.UserID != user.ID {
		t.Errorf("expected user ID '%s', got '%s'", user.ID, respBody.UserID)
	}
	if respBody.UserName != "Test User" {
		t.Errorf("expected user name '%s', got '%s'", user.Name, respBody.UserName)
	}

}

func TestHandlerCreateFeed_ErrorCases(t *testing.T) {
	uniqueEmail := fmt.Sprintf("test_%s@example.com", uuid.New().String())
	user, err := testCfg.DB.CreateUser(context.Background(), database.CreateUserParams{
		Name:           "Test User",
		Email:          uniqueEmail,
		HashedPassword: "placeholder_hash",
	})
	if err != nil {
		t.Fatalf("failed to create dummy user for test: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    any
		injectUserID   bool
		expectedStatus int
	}{
		{
			name:           "Invalid JSON Body",
			requestBody:    "not a json string",
			injectUserID:   true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid URL Format",
			requestBody: map[string]string{
				"url":  "invalid-url-schema",
				"name": "Valid Name",
			},
			injectUserID:   true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty Feed Name",
			requestBody: map[string]string{
				"url":  "https://example.com/rss.xml",
				"name": "",
			},
			injectUserID:   true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing User ID in Context",
			requestBody: map[string]string{
				"url":  "https://example.com/rss.xml",
				"name": "Valid Name",
			},
			injectUserID:   false,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request body: %v", err)
				}
			}

			req, err := http.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewBuffer(bodyBytes))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			if tt.injectUserID {
				ctx := context.WithValue(req.Context(), handlers.UserIDKey, user.ID)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()

			testCfg.HandlerCreateFeed(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
