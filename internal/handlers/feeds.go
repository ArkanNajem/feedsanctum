package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/database"
	"github.com/ArkanNajem/feedsanctum/internal/scraper"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func (cfg *APIConfig) HandlerCreateFeed(w http.ResponseWriter, r *http.Request) {
	type requestParam struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	type response struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		FeedID    uuid.UUID `json:"feed_id"`
		Name      string    `json:"name"`
		UserName  string    `json:"user_name"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	var reqData requestParam
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := parseURL(reqData.URL)
	if err != nil {
		respWithError(w, http.StatusBadRequest, "invalid URL")
		return
	}
	err = parseName(reqData.Name)
	if err != nil {
		respWithError(w, http.StatusBadRequest, "invalid feed name")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to create feed")
		return
	}
	tx, err := cfg.DBConn.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to begin tx: %v", err)
		return
	}
	defer tx.Rollback()
	dbTx := cfg.DB.WithTx(tx)
	feedDB, err := dbTx.CreateFeed(r.Context(), reqData.URL)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			respWithError(w, http.StatusConflict, "feed already registered")
			return
		}
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to create feed: %v", err)
		return
	}
	feedFollowDB, err := dbTx.CreateFeedFollow(r.Context(), database.CreateFeedFollowParams{
		UserID: userID,
		FeedID: feedDB.ID,
		Name:   reqData.Name,
	})
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to create feed follow: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to commit tx: %v", err)
		return
	}

	go func() {
		if err := scraper.ScrapeFeed(cfg.ServerCTX, cfg.DB, reqData.URL, feedDB.ID); err != nil {
			log.Printf("immediate scrape failed for feed ID:%s, URL:%s: %v", feedDB.ID, reqData.URL, err)
		}
	}()

	respWithJson(w, http.StatusCreated, response{
		ID:        feedFollowDB.ID,
		UserID:    feedFollowDB.UserID,
		FeedID:    feedFollowDB.FeedID,
		Name:      feedFollowDB.Name,
		UserName:  feedFollowDB.UserName,
		CreatedAt: feedFollowDB.CreatedAt,
		UpdatedAt: feedFollowDB.UpdatedAt,
	})
}

func (cfg *APIConfig) HandlerGetFeedFollows(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		FeedID    uuid.UUID `json:"feed_id"`
		Name      string    `json:"name"`
		UserName  string    `json:"user_name"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to get feed follows")
		return
	}
	userFeedFollows, err := cfg.DB.GetFeedFollowsForUser(r.Context(), userID)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to get feed follows from database: %v", err)
		return
	}
	responses := make([]response, 0, len(userFeedFollows))

	for _, userFollows := range userFeedFollows {
		responses = append(responses, response{
			ID:        userFollows.ID,
			UserID:    userFollows.UserID,
			FeedID:    userFollows.FeedID,
			Name:      userFollows.Name,
			UserName:  userFollows.UserName,
			CreatedAt: userFollows.CreatedAt,
			UpdatedAt: userFollows.UpdatedAt,
		})
	}

	respWithJson(w, http.StatusOK, responses)

}

func (cfg *APIConfig) HandlerDeleteFeedFollow(w http.ResponseWriter, r *http.Request) {
	feedID, err := uuid.Parse(chi.URLParam(r, "feedID"))
	if err != nil {
		respWithError(w, http.StatusBadRequest, "invalid feed id")
		log.Println("failed to get feed id from url to delete feed follow")
		return
	}
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to delete feed follow")
		return
	}

	_, err = cfg.DB.DeleteFeedFollow(r.Context(), database.DeleteFeedFollowParams{
		UserID: userID,
		FeedID: feedID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respWithError(w, http.StatusNotFound, "no such feed follow found")
			log.Println("feed follow not found")
			return
		}
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to delete feed follow: %v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *APIConfig) HandlerGetAllFeeds(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID            uuid.UUID  `json:"id"`
		Url           string     `json:"url"`
		Category      string     `json:"category"`
		LastFetchedAt *time.Time `json:"last_fetched_at"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
	}
	feeds, err := cfg.DB.GetAllFeeds(r.Context())
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to get feeds from database: %v", err)
		return
	}

	responses := make([]response, 0, len(feeds))

	for _, feed := range feeds {
		var lastFetchedAt *time.Time
		if feed.LastFetchedAt.Valid {
			lastFetchedAt = &feed.LastFetchedAt.Time
		}
		responses = append(responses, response{
			ID:            feed.ID,
			Url:           feed.Url,
			Category:      feed.Category,
			LastFetchedAt: lastFetchedAt,
			CreatedAt:     feed.CreatedAt,
			UpdatedAt:     feed.UpdatedAt,
		})
	}
	respWithJson(w, http.StatusOK, responses)
}

func (cfg *APIConfig) HandlerGetFeedByID(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID            uuid.UUID  `json:"id"`
		Url           string     `json:"url"`
		Category      string     `json:"category"`
		LastFetchedAt *time.Time `json:"last_fetched_at"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
	}
	feedID, err := uuid.Parse(chi.URLParam(r, "feedID"))
	if err != nil {
		respWithError(w, http.StatusBadRequest, "invalid feed id")
		log.Printf("failed to get feed id from url: %v", err)
		return
	}

	feedDB, err := cfg.DB.GetFeedByID(r.Context(), feedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respWithError(w, http.StatusNotFound, "no such feed found")
			log.Println("failed to find feed in database")
			return
		}
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to get feed from database by id:%v", err)
		return
	}

	var lastFetchedAt *time.Time
	if feedDB.LastFetchedAt.Valid {
		lastFetchedAt = &feedDB.LastFetchedAt.Time
	}
	respWithJson(w, http.StatusOK, response{
		ID:            feedDB.ID,
		Url:           feedDB.Url,
		Category:      feedDB.Category,
		LastFetchedAt: lastFetchedAt,
		CreatedAt:     feedDB.CreatedAt,
		UpdatedAt:     feedDB.UpdatedAt,
	})
}

func (cfg *APIConfig) HandlerCreateFeedFollow(w http.ResponseWriter, r *http.Request) {
	type requestParam struct {
		FeedID uuid.UUID `json:"feed_id"`
		Name   string    `json:"name"`
	}
	type response struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		FeedID    uuid.UUID `json:"feed_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Name      string    `json:"name"`
		UserName  string    `json:"user_name"`
	}

	var reqData requestParam
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := parseName(reqData.Name); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid feed name")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		log.Println("failed to get user id from context to follow feed")
		return
	}
	DBFeedFollow, err := cfg.DB.CreateFeedFollow(r.Context(), database.CreateFeedFollowParams{
		UserID: userID,
		FeedID: reqData.FeedID,
		Name:   reqData.Name,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" {
				respWithJson(w, http.StatusOK, map[string]string{"status": "feed already followed"})
				return
			}

			if pqErr.Code == "23503" {
				respWithError(w, http.StatusNotFound, "feed not found")
				return
			}
		}
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		log.Printf("failed to create feed follow: %v", err)
		return
	}
	respWithJson(w, http.StatusCreated, response{
		ID:        DBFeedFollow.ID,
		UserID:    DBFeedFollow.UserID,
		FeedID:    DBFeedFollow.FeedID,
		CreatedAt: DBFeedFollow.CreatedAt,
		UpdatedAt: DBFeedFollow.UpdatedAt,
		Name:      DBFeedFollow.Name,
		UserName:  DBFeedFollow.UserName,
	})
}
