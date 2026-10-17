package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func (cfg *APIConfig) HandlerGetPostsForUser(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID          uuid.UUID  `json:"id"`
		Title       string     `json:"title"`
		Url         string     `json:"url"`
		Description *string    `json:"description"`
		FeedID      uuid.UUID  `json:"feed_id"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
		PublishedAt *time.Time `json:"published_at"`
	}

	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to get posts for user")
		return
	}

	limit := parseQueryParamLimit(r.URL.Query().Get("limit"))
	postsDB, err := cfg.DB.GetPostsForUser(r.Context(), database.GetPostsForUserParams{UserID: userID, Limit: limit})
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to get posts")
		log.Printf("failed to get posts for user from database: %v", err)
		return
	}

	responses := make([]response, 0, len(postsDB))
	for _, post := range postsDB {
		var description *string
		var publishedAt *time.Time
		if post.Description.Valid {
			description = &post.Description.String
		}
		if post.PublishedAt.Valid {
			publishedAt = &post.PublishedAt.Time
		}
		responses = append(responses, response{
			ID:          post.ID,
			Title:       post.Title,
			Url:         post.Url,
			Description: description,
			FeedID:      post.FeedID,
			CreatedAt:   post.CreatedAt,
			UpdatedAt:   post.UpdatedAt,
			PublishedAt: publishedAt,
		})
	}
	respWithJson(w, http.StatusOK, responses)
}

func (cfg *APIConfig) HandlerMarkPostRead(w http.ResponseWriter, r *http.Request) {
	postIDStr := chi.URLParam(r, "postID")
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		respWithError(w, http.StatusBadRequest, "invalid post id")
		log.Println("failed to parse postID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to mark post read")
		return
	}
	if err = cfg.DB.MarkPostRead(r.Context(), database.MarkPostReadParams{
		UserID: userID,
		PostID: postID,
	}); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" { // unique violation
				respWithJson(w, http.StatusOK, map[string]string{"status": "post already marked as read"})
				return
			}
			if pqErr.Code == "23503" { // foreign key violation
				respWithError(w, http.StatusNotFound, "post not found")
				return
			}
		}
		respWithError(w, http.StatusInternalServerError, "failed to mark post read")
		log.Printf("failed to mark post read: %v", err)
		return
	}
	respWithJson(w, http.StatusOK, map[string]string{"status": "post marked as read"})

}

func (cfg *APIConfig) HandlerGetReadPostsForUser(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID          uuid.UUID  `json:"id"`
		Title       string     `json:"title"`
		Url         string     `json:"url"`
		Description *string    `json:"description"`
		FeedID      uuid.UUID  `json:"feed_id"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
		PublishedAt *time.Time `json:"published_at"`
		ReadAt      time.Time  `json:"read_at"`
	}
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to get post reads for user")
		return
	}
	posts, err := cfg.DB.GetPostReadsPerUser(r.Context(), userID)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to get read posts for user")
		log.Printf("failed to get read posts for user: %v", err)
		return
	}
	responses := make([]response, 0, len(posts))
	for _, post := range posts {
		var description *string
		var publishedAt *time.Time
		if post.Description.Valid {
			description = &post.Description.String
		}
		if post.PublishedAt.Valid {
			publishedAt = &post.PublishedAt.Time
		}
		responses = append(responses, response{
			ID:          post.ID,
			Title:       post.Title,
			Url:         post.Url,
			Description: description,
			FeedID:      post.FeedID,
			CreatedAt:   post.CreatedAt,
			UpdatedAt:   post.UpdatedAt,
			PublishedAt: publishedAt,
			ReadAt:      post.ReadAt,
		})
	}
	respWithJson(w, http.StatusOK, responses)

}
