package server

import "net/http"

func (a *App) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    claims.UserID,
		"email": claims.Email,
		"role":  claims.Role,
	})
}
