package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"wasa-text/service/database"
	"wasa-text/service/globaltime"
)

type Dependencies struct {
	DB    database.Database
	Clock globaltime.Clock
}

type API struct {
	db database.Database
	ck globaltime.Clock
}

func New(deps Dependencies) *API {
	return &API{
		db: deps.DB,
		ck: deps.Clock,
	}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	// Simplified login
	mux.HandleFunc("/api/session", a.handleSession)

	// Users list (for UI dropdown)
	mux.HandleFunc("/api/users", a.requireAuth(a.handleUsers))

	// Profile
	mux.HandleFunc("/api/me/name", a.requireAuth(a.handleMeName))
	mux.HandleFunc("/api/me/photo", a.requireAuth(a.handleMePhoto))

	// Conversations / groups
	mux.HandleFunc("/api/conversations", a.requireAuth(a.handleConversations))
	mux.HandleFunc("/api/groups", a.requireAuth(a.handleGroups))

	// Dynamic routes (manual parsing)
	mux.HandleFunc("/api/conversations/", a.requireAuth(a.handleConversationsDynamic))
	mux.HandleFunc("/api/messages/", a.requireAuth(a.handleMessagesDynamic))
	mux.HandleFunc("/api/groups/", a.requireAuth(a.handleGroupsDynamic))
}

/* -------------------- common helpers -------------------- */

type errResp struct {
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errResp{Message: msg})
}

func readJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// Extract Bearer token from Authorization header
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (a *API) requireAuth(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r)
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		if _, err := a.db.UserByToken(tok); err != nil {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		next(w, r, tok)
	}
}

// mapDBErr writes an HTTP error response for known database errors.
// Returns true if it wrote an error response.
func mapDBErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, database.ErrNotLogged):
		writeError(w, http.StatusUnauthorized, "not logged")

	case errors.Is(err, database.ErrNotFound), errors.Is(err, database.ErrConversationGone):
		writeError(w, http.StatusNotFound, "not found")

	case errors.Is(err, database.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")

	case errors.Is(err, database.ErrNameAlreadyUsed):
		writeError(w, http.StatusConflict, "name already used")

	case errors.Is(err, database.ErrInvalid):
		writeError(w, http.StatusBadRequest, "bad request")

	case errors.Is(err, database.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")

	case errors.Is(err, database.ErrNotAGroup):
		writeError(w, http.StatusBadRequest, "not a group")

	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}

	return true
}

/* -------------------- /session -------------------- */

type loginReq struct {
	Name string `json:"name"`
}

type loginResp struct {
	Identifier string `json:"identifier"`
}

func (a *API) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req loginReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	token, created, err := a.db.LoginCreateOrGet(req.Name)
	if err != nil {
		mapDBErr(w, err)
		return
	}

	// 201 either way (simple)
	_ = created
	writeJSON(w, http.StatusCreated, loginResp{Identifier: token})
}

/* -------------------- /users -------------------- */

func (a *API) handleUsers(w http.ResponseWriter, r *http.Request, tok string) {
	_ = tok

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	users := a.db.ListUsers()
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

/* -------------------- /me/name -------------------- */

type setNameReq struct {
	Name string `json:"name"`
}

func (a *API) handleMeName(w http.ResponseWriter, r *http.Request, tok string) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req setNameReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := a.db.SetMyName(tok, req.Name); err != nil {
		mapDBErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/* -------------------- /me/photo -------------------- */

type setPhotoReq struct {
	Photo string `json:"photo"`
}

func (a *API) handleMePhoto(w http.ResponseWriter, r *http.Request, tok string) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req setPhotoReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := a.db.SetMyPhoto(tok, req.Photo); err != nil {
		mapDBErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/* -------------------- /conversations -------------------- */

func (a *API) handleConversations(w http.ResponseWriter, r *http.Request, tok string) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.db.ListMyConversations(tok)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
		return

	case http.MethodPost:
		// create direct
		var req struct {
			Username string `json:"username"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		cid, err := a.db.CreateDirectConversation(tok, req.Username)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"conversationId": cid})
		return

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

/* -------------------- /groups -------------------- */

func (a *API) handleGroups(w http.ResponseWriter, r *http.Request, tok string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name    string   `json:"name"`
		Members []string `json:"members"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	cid, err := a.db.CreateGroupConversation(tok, req.Name, req.Members)
	if err != nil {
		mapDBErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"conversationId": cid})
}

/* -------------------- /conversations/{id} and /conversations/{id}/messages -------------------- */

func (a *API) handleConversationsDynamic(w http.ResponseWriter, r *http.Request, tok string) {
	// Path:
	// /api/conversations/{conversationId}
	// /api/conversations/{conversationId}/messages
	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	parts := strings.Split(path, "/")
	cid := parts[0]

	if len(parts) == 1 {
		// /api/conversations/{cid}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		view, err := a.db.GetConversation(tok, cid)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
		return
	}

	if len(parts) == 2 && parts[1] == "messages" {
		// /api/conversations/{cid}/messages
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Type    string  `json:"type"`
			Text    string  `json:"text"`
			Media   string  `json:"media"`
			ReplyTo *string `json:"replyTo"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		msg, err := a.db.SendMessage(tok, cid, req.Type, req.Text, req.Media, req.ReplyTo)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, msg)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

/* -------------------- /messages/{id} + forward + comments -------------------- */

func (a *API) handleMessagesDynamic(w http.ResponseWriter, r *http.Request, tok string) {
	// /api/messages/{messageId}
	// /api/messages/{messageId}/forward
	// /api/messages/{messageId}/comments
	// /api/messages/{messageId}/comments/{reactionId}
	path := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(path, "/")
	mid := parts[0]

	// /api/messages/{mid}
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := a.db.DeleteMessage(tok, mid); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// /api/messages/{mid}/forward
	if len(parts) == 2 && parts[1] == "forward" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ConversationId string `json:"conversationId"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		msg, err := a.db.ForwardMessage(tok, mid, req.ConversationId)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, msg)
		return
	}

	// /api/messages/{mid}/comments
	if len(parts) == 2 && parts[1] == "comments" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Emoji string `json:"emoji"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		rid, err := a.db.CommentMessage(tok, mid, req.Emoji)
		if err != nil {
			mapDBErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"reactionId": rid})
		return
	}

	// /api/messages/{mid}/comments/{reactionId}
	if len(parts) == 3 && parts[1] == "comments" {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rid := parts[2]
		if err := a.db.UncommentMessage(tok, mid, rid); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

/* -------------------- /groups/{id} subroutes -------------------- */

func (a *API) handleGroupsDynamic(w http.ResponseWriter, r *http.Request, tok string) {
	// /api/groups/{groupId}/name
	// /api/groups/{groupId}/photo
	// /api/groups/{groupId}/members
	// /api/groups/{groupId}/leave
	path := strings.TrimPrefix(r.URL.Path, "/api/groups/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(path, "/")
	gid := parts[0]

	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch parts[1] {
	case "name":
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.db.SetGroupName(tok, gid, req.Name); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "photo":
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Photo string `json:"photo"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.db.SetGroupPhoto(tok, gid, req.Photo); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "members":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Username string `json:"username"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.db.AddToGroup(tok, gid, req.Username); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "leave":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := a.db.LeaveGroup(tok, gid); err != nil {
			mapDBErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}
