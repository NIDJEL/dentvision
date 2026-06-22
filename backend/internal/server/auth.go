package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string  `json:"token"`
	User  userDTO `json:"user"`
}

type userDTO struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type jwtClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var user userDTO
	var passwordHash string
	var isActive bool

	err := a.db.QueryRow(
		r.Context(),
		`
		SELECT u.id, u.email, u.password_hash, u.full_name, u.is_active, r.name
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1
		LIMIT 1
		`,
		req.Email,
	).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&user.FullName,
		&isActive,
		&user.Role,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		log.Println("login query:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if !isActive {
		writeError(w, http.StatusUnauthorized, "user is inactive")
		return
	}

	passwordHash = strings.Replace(passwordHash, "$2y$", "$2a$", 1)

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	claims := jwtClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.jwtSecret))
	if err != nil {
		log.Println("sign token:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token: token,
		User:  user,
	})
}
