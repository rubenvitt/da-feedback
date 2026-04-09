package ui

import (
	"net/http"
	"strconv"

	"github.com/rubeen/da-feedback/internal/auth"
)

func (h *AdminHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler, adminMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return authMw(adminMw(fn)) }

	mux.Handle("GET /admin/users", wrap(h.listUsers))
	mux.Handle("POST /admin/users/{id}/groups", wrap(h.assignGroups))
	mux.Handle("POST /admin/users/{id}/delete", wrap(h.deleteUser))
}

func (h *AdminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.sessions.DB().QueryContext(ctx,
		"SELECT id, name, email, role FROM users ORDER BY name")
	if err != nil {
		http.Error(w, "Fehler", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type userWithGroups struct {
		auth.User
		GroupIDs map[int]bool
	}

	var users []userWithGroups
	for rows.Next() {
		var u userWithGroups
		rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role)
		u.GroupIDs = make(map[int]bool)
		users = append(users, u)
	}

	for i, u := range users {
		gRows, err := h.sessions.DB().QueryContext(ctx,
			"SELECT group_id FROM user_groups WHERE user_id = ?", u.ID)
		if err == nil {
			for gRows.Next() {
				var gid int
				gRows.Scan(&gid)
				users[i].GroupIDs[gid] = true
			}
			gRows.Close()
		}
	}

	groups, _ := h.groups.List(ctx)

	h.render.Render(w, "admin/users.html", http.StatusOK, map[string]any{
		"User":   auth.GetSession(r).User,
		"Users":  users,
		"Groups": groups,
	})
}

func (h *AdminHandler) assignGroups(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	r.ParseForm()

	ctx := r.Context()
	db := h.sessions.DB()

	db.ExecContext(ctx, "DELETE FROM user_groups WHERE user_id = ?", userID)

	for _, gid := range r.Form["group_ids"] {
		groupID, _ := strconv.Atoi(gid)
		db.ExecContext(ctx, "INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", userID, groupID)
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AdminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	ctx := r.Context()
	db := h.sessions.DB()

	db.ExecContext(ctx, "DELETE FROM user_groups WHERE user_id = ?", userID)
	db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID)

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
