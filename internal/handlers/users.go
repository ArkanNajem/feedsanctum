package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/auth"
	"github.com/ArkanNajem/feedsanctum/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type contextIDKey string

const UserIDKey contextIDKey = "userID"

type UserLoginData struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func rotateRefreshToken(api *APIConfig, userID uuid.UUID, refreshtoken string) (database.RefreshToken, error) {
	tx, err := api.DBConn.BeginTx(context.Background(), &sql.TxOptions{})

	if err != nil {
		return database.RefreshToken{}, err
	}
	defer tx.Rollback()
	dbTx := api.DB.WithTx(tx)

	err = dbTx.RevokeRefreshToken(context.Background(), refreshtoken)
	if err != nil {
		return database.RefreshToken{}, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}
	token, err := auth.MakeRefreshToken()
	if err != nil {
		return database.RefreshToken{}, err
	}
	newRefreshToken, err := dbTx.CreateRefreshToken(context.Background(), database.CreateRefreshTokenParams{
		RefreshToken: token,
		ExpiresAt:    time.Now().Add(24 * time.Hour * 7),
		UserID:       userID,
	})

	if err != nil {
		return database.RefreshToken{}, err
	}
	err = tx.Commit()
	if err != nil {
		return database.RefreshToken{}, err
	}

	return newRefreshToken, nil
}

func (cfg *APIConfig) HandlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type requestParam struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type responseErr struct {
		Errors []inputError `json:"errors"`
	}
	var reqData requestParam
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if errs := validateInput(reqData.Name, reqData.Email, reqData.Password); len(errs) > 0 {
		respWithJson(w, http.StatusBadRequest, responseErr{Errors: errs})
		return
	}
	hashedPassword, err := auth.HashPassword(reqData.Password)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	createdUser, err := cfg.DB.CreateUser(r.Context(), database.CreateUserParams{
		Name:           reqData.Name,
		Email:          reqData.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			respWithError(w, http.StatusConflict, "email already registered")
			return
		}

		respWithError(w, http.StatusBadRequest, "something went wrong")
		return
	}
	respWithJson(w, http.StatusCreated, User{
		ID:        createdUser.ID,
		Name:      createdUser.Name,
		Email:     createdUser.Email,
		CreatedAt: createdUser.CreatedAt,
		UpdatedAt: createdUser.UpdatedAt,
	})
}

func (cfg *APIConfig) HandlerLogin(w http.ResponseWriter, r *http.Request) {
	type requestParam struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		User
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	const dummyHash = "$argon2id$v=19$m=65536,t=1,p=12$LpTO8GOk8ajNAFczIs12uQ$e48btjY28JEWiIfEDYcJBjb1GLBZGqXoOyrClQ9EIr0"
	var reqData requestParam
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if cfg.LoginLimiter.RespondOnLimit(w, r, strings.ToLower(reqData.Email)) {
		return
	}

	DBUser, err := cfg.DB.GetUserByEmail(r.Context(), reqData.Email)
	userExists := err == nil
	hashToValidate := dummyHash

	if userExists {
		hashToValidate = DBUser.HashedPassword
	}

	match, err := auth.ValidatePasswordHash(reqData.Password, hashToValidate)
	if !userExists || !match || err != nil {
		respWithError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}
	accessToken, err := auth.MakeJWT(DBUser.ID, cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	_, err = cfg.DB.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 7),
		UserID:       DBUser.ID,
	})
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to save refresh token")
		return
	}

	respWithJson(w, http.StatusOK, response{
		User: User{
			ID:        DBUser.ID,
			Name:      DBUser.Name,
			Email:     DBUser.Email,
			CreatedAt: DBUser.CreatedAt,
			UpdatedAt: DBUser.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})

}

func (cfg *APIConfig) HandlerRefresh(w http.ResponseWriter, r *http.Request) {
	type response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respWithError(w, http.StatusBadRequest, "failed to get bearer token")
		return
	}
	DBUser, err := cfg.DB.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respWithError(w, http.StatusUnauthorized, "failed to find token")
		return
	}

	newAccessToken, err := auth.MakeJWT(DBUser.ID, cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	newRefreshTokenDB, err := rotateRefreshToken(cfg, DBUser.ID, refreshToken)
	if err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	respWithJson(w, http.StatusOK, response{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenDB.RefreshToken,
	})
}

func (cfg *APIConfig) MiddlewareAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		accessToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			respWithError(w, http.StatusUnauthorized, "invalid access token")
			return
		}
		id, err := auth.ValidateJWT(accessToken, cfg.JWTSecret)
		if err != nil {
			respWithError(w, http.StatusUnauthorized, "invalid access token")
			return

		}
		ctx := context.WithValue(r.Context(), UserIDKey, id)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
func (cfg *APIConfig) HandlerGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		return
	}
	user, err := cfg.DB.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respWithError(w, http.StatusNotFound, "user not found")
			return
		}
		respWithError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	respWithJson(w, http.StatusOK, User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
}

func (cfg *APIConfig) HandlerLogOut(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respWithError(w, http.StatusBadRequest, "failed to get bearer token")
		return
	}

	if err := cfg.DB.RevokeRefreshToken(r.Context(), refreshToken); err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *APIConfig) HandlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	type requestParam struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
	}
	var reqData requestParam
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		respWithError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if reqData.Name == nil && reqData.Email == nil && reqData.Password == nil {
		respWithError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	var nameParam sql.NullString
	if reqData.Name != nil {
		name := *reqData.Name
		if err := parseName(name); err != nil {
			respWithError(w, http.StatusBadRequest, "invalid name")
			return
		}
		nameParam = sql.NullString{String: name, Valid: true}
	}
	var emailParam sql.NullString
	if reqData.Email != nil {
		email := *reqData.Email
		if err := parseEmail(email); err != nil {
			respWithError(w, http.StatusBadRequest, "invalid email")
			return
		}
		emailParam = sql.NullString{String: email, Valid: true}
	}
	var hashedPasswordParam sql.NullString
	if reqData.Password != nil {
		password := *reqData.Password
		if err := parsePassword(password); err != nil {
			respWithError(w, http.StatusBadRequest, "invalid password")
			return
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			respWithError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		hashedPasswordParam = sql.NullString{String: hash, Valid: true}
	}

	id, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		return
	}

	DBUser, err := cfg.DB.UpdateUser(r.Context(), database.UpdateUserParams{
		ID:             id,
		Name:           nameParam,
		Email:          emailParam,
		HashedPassword: hashedPasswordParam,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			respWithError(w, http.StatusConflict, "email already registered")
			return
		}
		respWithError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	respWithJson(w, http.StatusOK, User{
		ID:        DBUser.ID,
		Name:      DBUser.Name,
		Email:     DBUser.Email,
		UpdatedAt: DBUser.UpdatedAt,
	})
}

func (cfg *APIConfig) HandlerDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respWithError(w, http.StatusInternalServerError, "missing user id in context")
		return
	}
	if err := cfg.DB.DeleteUser(r.Context(), id); err != nil {
		respWithError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
