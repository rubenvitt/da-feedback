package auth

import (
	"context"
	"net/http"
)

type contextKey string

const sessionKey contextKey = "session"

const cookieName = "daf_session"

func GetSession(r *http.Request) *Session {
	sess, _ := r.Context().Value(sessionKey).(*Session)
	return sess
}

func RequireAuth(sessions *SessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		sess, err := sessions.Get(r.Context(), cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		sessions.Refresh(r.Context(), sess.ID)

		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := GetSession(r)
		if sess == nil || sess.User.Role != role {
			if sess != nil && sess.User.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireGroupAccess(sessions *SessionStore, next http.HandlerFunc, getGroupID func(r *http.Request) int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := GetSession(r)
		if sess == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if sess.User.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		groupID := getGroupID(r)
		groups, err := sessions.GetUserGroups(r.Context(), sess.UserID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		for _, id := range groups {
			if id == groupID {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}
