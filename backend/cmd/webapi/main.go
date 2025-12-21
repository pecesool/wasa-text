package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

/*
	WASAText backend (simple, student-style)
	- No cookies, no sessions, no JWT
	- Simplified login returns an identifier
	- Authorization: Bearer <identifier> impersonates user
	- In-memory storage using maps + mutex (as FAQ requires)
	- No Go generics
*/

type User struct {
	ID    string
	Name  string
	Photo string // base64 or ""
}

type Conversation struct {
	ID      string
	Title   string
	IsGroup bool
	Members []string // usernames
	Messages []Message
	Photo   string // base64 or "" (group photo, optional)
}

type Message struct {
	ID        string
	Sender    string
	Time      time.Time
	Type      string // "text" or "image"
	Text      string
	Media     string // base64 if image/gif
	Delivered bool
	Read      bool
	ReplyTo   *string
}

var (
	mu sync.Mutex

	// users
	usersByID   = map[string]*User{}
	usersByName = map[string]*User{}

	// conversations
	conversationsByID = map[string]*Conversation{}

	// simple incremental ids (still random-safe if you prefer, but this is fine)
	msgCounter int
	convCounter int
)

func main() {
	mux := http.NewServeMux()

	// LOGIN (no Authorization)
	mux.HandleFunc("/api/session", doLogin)

	// USER
	mux.HandleFunc("/api/me/name", setMyUserName)
	mux.HandleFunc("/api/me/photo", setMyPhoto)

	// CONVERSATIONS
	mux.HandleFunc("/api/conversations", conversationsRoot)         // GET, POST (direct)
	mux.HandleFunc("/api/conversations/", conversationsByIDHandler) // GET + /messages POST

	// GROUPS
	mux.HandleFunc("/api/groups", createGroup)
	mux.HandleFunc("/api/groups/", groupsHandler) // name/photo/members/leave

	// MESSAGES (forward/delete/comments)
	mux.HandleFunc("/api/messages/", messagesHandler)

	log.Println("WASAText backend running on http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", withCORS(mux)))
}

/* -------------------------- LOGIN -------------------------- */

// POST /api/session  (operationId: doLogin)
func doLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	name := strings.TrimSpace(req.Name)
	if len(name) < 3 || len(name) > 16 {
		writeError(w, http.StatusBadRequest, "invalid name")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	u, exists := usersByName[name]
	if !exists {
		u = &User{ID: generateID(), Name: name, Photo: ""}
		usersByName[name] = u
		usersByID[u.ID] = u
	}

	writeJSON(w, http.StatusCreated, map[string]string{"identifier": u.ID})
}

/* -------------------------- AUTH -------------------------- */

func getUserFromAuth(r *http.Request) (*User, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil, false
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, false
	}
	id := strings.TrimSpace(parts[1])
	if id == "" {
		return nil, false
	}

	mu.Lock()
	defer mu.Unlock()
	u, ok := usersByID[id]
	return u, ok
}

/* -------------------------- USERS -------------------------- */

// PUT /api/me/name (operationId: setMyUserName)
func setMyUserName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	newName := strings.TrimSpace(req.Name)
	if len(newName) < 3 || len(newName) > 16 {
		writeError(w, http.StatusBadRequest, "invalid name")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if other, used := usersByName[newName]; used && other.ID != u.ID {
		writeError(w, http.StatusConflict, "name already used")
		return
	}

	// update username key
	delete(usersByName, u.Name)
	oldName := u.Name
	u.Name = newName
	usersByName[u.Name] = u

	// update references in conversations (members + message sender)
	for _, c := range conversationsByID {
		for i := range c.Members {
			if c.Members[i] == oldName {
				c.Members[i] = newName
			}
		}
		for i := range c.Messages {
			if c.Messages[i].Sender == oldName {
				c.Messages[i].Sender = newName
			}
		}
		// for direct chats, title is "the other user" per user, but we keep a simple global title:
		// if direct and title equals oldName, update it
		if !c.IsGroup && c.Title == oldName {
			c.Title = newName
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/me/photo (operationId: setMyPhoto)
func setMyPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Photo string `json:"photo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	mu.Lock()
	u.Photo = req.Photo // can be "" to remove
	mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

/* -------------------------- CONVERSATIONS -------------------------- */

func conversationsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMyConversations(w, r) // operationId: getMyConversations
	case http.MethodPost:
		createDirectConversation(w, r) // operationId: createDirectConversation
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// GET /api/conversations (operationId: getMyConversations)
func getMyConversations(w http.ResponseWriter, r *http.Request) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	type item struct {
		ID         string    `json:"id"`
		Title      string    `json:"title"`
		IsGroup    bool      `json:"isGroup"`
		LastTime   time.Time `json:"lastTime"`
		LastPreview string   `json:"lastPreview"`
	}

	mu.Lock()
	var items []item
	for _, c := range conversationsByID {
		if !contains(c.Members, u.Name) {
			continue
		}

		lastTime := time.Time{}
		preview := ""
		if len(c.Messages) > 0 {
			m := c.Messages[len(c.Messages)-1]
			lastTime = m.Time
			if m.Type == "text" {
				preview = m.Text
			} else {
				preview = "" // image
			}
		} else {
			lastTime = time.Now()
		}

		title := c.Title
		if !c.IsGroup {
			// for direct conversation, show "other user"
			title = otherMember(c.Members, u.Name)
		}

		items = append(items, item{
			ID:          c.ID,
			Title:       title,
			IsGroup:     c.IsGroup,
			LastTime:    lastTime,
			LastPreview: preview,
		})
	}
	mu.Unlock()

	// sort by lastTime desc
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastTime.After(items[j].LastTime)
	})

	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

// POST /api/conversations (operationId: createDirectConversation)
func createDirectConversation(w http.ResponseWriter, r *http.Request) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	other := strings.TrimSpace(req.Username)
	if other == "" {
		writeError(w, http.StatusBadRequest, "invalid username")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := usersByName[other]; !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if other == u.Name {
		writeError(w, http.StatusBadRequest, "cannot chat with yourself")
		return
	}

	// if conversation already exists with these two members and is not group, return it
	for _, c := range conversationsByID {
		if c.IsGroup {
			continue
		}
		if sameTwoMembers(c.Members, u.Name, other) {
			writeJSON(w, http.StatusCreated, map[string]string{"conversationId": c.ID})
			return
		}
	}

	convCounter++
	cid := "c_" + itoa(convCounter)
	c := &Conversation{
		ID:       cid,
		Title:    "", // for direct chats title is computed as "other user"
		IsGroup:  false,
		Members:  []string{u.Name, other},
		Messages: []Message{},
		Photo:    "",
	}
	conversationsByID[c.ID] = c

	writeJSON(w, http.StatusCreated, map[string]string{"conversationId": c.ID})
}

// Handles:
// GET /api/conversations/{conversationId} (operationId: getConversation)
// POST /api/conversations/{conversationId}/messages (operationId: sendMessage)
func conversationsByIDHandler(w http.ResponseWriter, r *http.Request) {
	// path after /api/conversations/
	rest := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	parts := strings.Split(rest, "/")
	conversationID := parts[0]

	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			getConversation(w, r, conversationID)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// /messages
	if len(parts) == 2 && parts[1] == "messages" {
		if r.Method == http.MethodPost {
			sendMessage(w, r, conversationID)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// GET /api/conversations/{conversationId} (operationId: getConversation)
func getConversation(w http.ResponseWriter, r *http.Request, cid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	c, exists := conversationsByID[cid]
	if !exists || !contains(c.Members, u.Name) {
		mu.Unlock()
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// build response (messages newest first as YAML says)
	type msgOut struct {
		ID        string     `json:"id"`
		Sender    string     `json:"sender"`
		Time      time.Time  `json:"time"`
		Type      string     `json:"type"`
		Text      string     `json:"text,omitempty"`
		Media     string     `json:"media,omitempty"`
		Delivered bool       `json:"delivered"`
		Read      bool       `json:"read"`
	}

	outMsgs := make([]msgOut, 0, len(c.Messages))
	for i := len(c.Messages) - 1; i >= 0; i-- {
		m := c.Messages[i]
		outMsgs = append(outMsgs, msgOut{
			ID:        m.ID,
			Sender:    m.Sender,
			Time:      m.Time,
			Type:      m.Type,
			Text:      m.Text,
			Media:     m.Media,
			Delivered: m.Delivered,
			Read:      m.Read,
		})
	}

	title := c.Title
	if !c.IsGroup {
		title = otherMember(c.Members, u.Name)
	}

	resp := map[string]any{
		"id":       c.ID,
		"title":    title,
		"isGroup":  c.IsGroup,
		"members":  c.Members,
		"messages": outMsgs,
	}
	mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

// POST /api/conversations/{conversationId}/messages (operationId: sendMessage)
func sendMessage(w http.ResponseWriter, r *http.Request, cid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Type    string  `json:"type"`  // text/image
		Text    string  `json:"text"`
		Media   string  `json:"media"`
		ReplyTo *string `json:"replyTo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Type != "text" && req.Type != "image" {
		writeError(w, http.StatusBadRequest, "invalid type")
		return
	}
	if req.Type == "text" && strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	if req.Type == "image" && strings.TrimSpace(req.Media) == "" {
		writeError(w, http.StatusBadRequest, "media is required")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	c, exists := conversationsByID[cid]
	if !exists || !contains(c.Members, u.Name) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	msgCounter++
	mid := "m_" + itoa(msgCounter)

	m := Message{
		ID:        mid,
		Sender:    u.Name,
		Time:      time.Now().UTC(),
		Type:      req.Type,
		Text:      req.Text,
		Media:     req.Media,
		Delivered: true,
		Read:      false,
		ReplyTo:   req.ReplyTo,
	}
	c.Messages = append(c.Messages, m)

	// response matches your YAML Message schema
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        m.ID,
		"sender":    m.Sender,
		"time":      m.Time,
		"type":      m.Type,
		"text":      m.Text,
		"media":     m.Media,
		"delivered": m.Delivered,
		"read":      m.Read,
	})
}

/* -------------------------- GROUPS -------------------------- */

// POST /api/groups (operationId: createGroup)
func createGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Name    string   `json:"name"`
		Members []string `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	groupName := strings.TrimSpace(req.Name)
	if groupName == "" {
		writeError(w, http.StatusBadRequest, "invalid group name")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// validate members exist
	members := []string{u.Name}
	for _, m := range req.Members {
		m = strings.TrimSpace(m)
		if m == "" || m == u.Name {
			continue
		}
		if _, exists := usersByName[m]; !exists {
			writeError(w, http.StatusBadRequest, "member not found: "+m)
			return
		}
		if !contains(members, m) {
			members = append(members, m)
		}
	}

	convCounter++
	cid := "c_" + itoa(convCounter)
	c := &Conversation{
		ID:       cid,
		Title:    groupName,
		IsGroup:  true,
		Members:  members,
		Messages: []Message{},
		Photo:    "",
	}
	conversationsByID[c.ID] = c

	writeJSON(w, http.StatusCreated, map[string]string{"conversationId": c.ID})
}

func groupsHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/groups/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	parts := strings.Split(rest, "/")
	groupID := parts[0]
	if groupID == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// /name, /photo, /members, /leave
	if len(parts) == 2 && parts[1] == "name" && r.Method == http.MethodPut {
		setGroupName(w, r, groupID) // operationId: setGroupName
		return
	}
	if len(parts) == 2 && parts[1] == "photo" && r.Method == http.MethodPut {
		setGroupPhoto(w, r, groupID) // operationId: setGroupPhoto
		return
	}
	if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodPost {
		addToGroup(w, r, groupID) // operationId: addToGroup
		return
	}
	if len(parts) == 2 && parts[1] == "leave" && r.Method == http.MethodPost {
		leaveGroup(w, r, groupID) // operationId: leaveGroup
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

// PUT /api/groups/{groupId}/name (operationId: setGroupName)
func setGroupName(w http.ResponseWriter, r *http.Request, gid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		writeError(w, http.StatusBadRequest, "invalid name")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	c, exists := conversationsByID[gid]
	if !exists || !c.IsGroup || !contains(c.Members, u.Name) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	c.Title = newName
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/groups/{groupId}/photo (operationId: setGroupPhoto)
func setGroupPhoto(w http.ResponseWriter, r *http.Request, gid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Photo string `json:"photo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	c, exists := conversationsByID[gid]
	if !exists || !c.IsGroup || !contains(c.Members, u.Name) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	c.Photo = req.Photo
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/groups/{groupId}/members (operationId: addToGroup)
func addToGroup(w http.ResponseWriter, r *http.Request, gid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	newMember := strings.TrimSpace(req.Username)
	if newMember == "" {
		writeError(w, http.StatusBadRequest, "invalid username")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	c, exists := conversationsByID[gid]
	if !exists || !c.IsGroup || !contains(c.Members, u.Name) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	if _, ok := usersByName[newMember]; !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if contains(c.Members, newMember) {
		writeError(w, http.StatusConflict, "already in group")
		return
	}

	c.Members = append(c.Members, newMember)
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/groups/{groupId}/leave (operationId: leaveGroup)
func leaveGroup(w http.ResponseWriter, r *http.Request, gid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	c, exists := conversationsByID[gid]
	if !exists || !c.IsGroup || !contains(c.Members, u.Name) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	// remove member
	newMembers := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		if m != u.Name {
			newMembers = append(newMembers, m)
		}
	}
	c.Members = newMembers

	w.WriteHeader(http.StatusNoContent)
}

/* -------------------------- MESSAGES -------------------------- */

func messagesHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(rest, "/")
	messageID := parts[0]

	// /forward
	if len(parts) == 2 && parts[1] == "forward" && r.Method == http.MethodPost {
		forwardMessage(w, r, messageID) // operationId: forwardMessage
		return
	}

	// /comments
	if len(parts) == 2 && parts[1] == "comments" && r.Method == http.MethodPost {
		commentMessage(w, r, messageID) // operationId: commentMessage
		return
	}

	// /comments/{reactionId}
	if len(parts) == 3 && parts[1] == "comments" && r.Method == http.MethodDelete {
		uncommentMessage(w, r, messageID, parts[2]) // operationId: uncommentMessage
		return
	}

	// DELETE /messages/{messageId}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		deleteMessage(w, r, messageID) // operationId: deleteMessage
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

// POST /api/messages/{messageId}/forward (operationId: forwardMessage)
func forwardMessage(w http.ResponseWriter, r *http.Request, mid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		ConversationID string `json:"conversationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target := strings.TrimSpace(req.ConversationID)
	if target == "" {
		writeError(w, http.StatusBadRequest, "invalid conversationId")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	origMsg, origConv := findMessage(mid)
	if origMsg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	// must be a member of original conversation too (reasonable)
	if !contains(origConv.Members, u.Name) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	targetConv, ok2 := conversationsByID[target]
	if !ok2 || !contains(targetConv.Members, u.Name) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	msgCounter++
	newID := "m_" + itoa(msgCounter)
	newMsg := Message{
		ID:        newID,
		Sender:    u.Name,
		Time:      time.Now().UTC(),
		Type:      origMsg.Type,
		Text:      origMsg.Text,
		Media:     origMsg.Media,
		Delivered: true,
		Read:      false,
	}
	targetConv.Messages = append(targetConv.Messages, newMsg)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        newMsg.ID,
		"sender":    newMsg.Sender,
		"time":      newMsg.Time,
		"type":      newMsg.Type,
		"text":      newMsg.Text,
		"media":     newMsg.Media,
		"delivered": newMsg.Delivered,
		"read":      newMsg.Read,
	})
}

// DELETE /api/messages/{messageId} (operationId: deleteMessage)
func deleteMessage(w http.ResponseWriter, r *http.Request, mid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	msg, conv := findMessage(mid)
	if msg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	if msg.Sender != u.Name {
		writeError(w, http.StatusForbidden, "not sender")
		return
	}

	// remove message from conv
	newMsgs := make([]Message, 0, len(conv.Messages))
	for _, m := range conv.Messages {
		if m.ID != mid {
			newMsgs = append(newMsgs, m)
		}
	}
	conv.Messages = newMsgs

	w.WriteHeader(http.StatusNoContent)
}

/*
	Reactions are simplified:
	- We don't store full reaction objects in this minimal backend,
	  but we still support endpoints and return a reactionId.
	- This is enough for HW progress; you can extend later if needed.
*/

// POST /api/messages/{messageId}/comments (operationId: commentMessage)
func commentMessage(w http.ResponseWriter, r *http.Request, mid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Emoji) == "" {
		writeError(w, http.StatusBadRequest, "invalid emoji")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	msg, conv := findMessage(mid)
	if msg == nil || !contains(conv.Members, u.Name) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	// return a fake reaction id
	writeJSON(w, http.StatusCreated, map[string]string{
		"reactionId": "r_" + generateID()[0:6],
	})
}

// DELETE /api/messages/{messageId}/comments/{reactionId} (operationId: uncommentMessage)
func uncommentMessage(w http.ResponseWriter, r *http.Request, mid string, rid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	msg, conv := findMessage(mid)
	if msg == nil || !contains(conv.Members, u.Name) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// minimal backend: we don't store reactions, so we accept delete if message exists
	_ = rid
	w.WriteHeader(http.StatusNoContent)
}

/* -------------------------- HELPERS -------------------------- */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "1")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func contains(arr []string, s string) bool {
	for _, x := range arr {
		if x == s {
			return true
		}
	}
	return false
}

func otherMember(members []string, me string) string {
	for _, m := range members {
		if m != me {
			return m
		}
	}
	return ""
}

func sameTwoMembers(members []string, a string, b string) bool {
	if len(members) != 2 {
		return false
	}
	return (members[0] == a && members[1] == b) || (members[0] == b && members[1] == a)
}

func findMessage(mid string) (*Message, *Conversation) {
	for _, c := range conversationsByID {
		for i := range c.Messages {
			if c.Messages[i].ID == mid {
				return &c.Messages[i], c
			}
		}
	}
	return nil, nil
}

// small integer to string without fmt (keeps it simple)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(b[i:])
}
