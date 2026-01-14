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

type User struct {
	ID    string
	Name  string
	Photo string
}

type Reaction struct {
	ID    string `json:"id"`
	User  string `json:"user"`
	Emoji string `json:"emoji"`
	Time  string `json:"time"`
}

type Message struct {
	ID            string     `json:"id"`
	Sender        string     `json:"sender"`
	Time          time.Time  `json:"time"`
	Type          string     `json:"type"`
	Text          string     `json:"text,omitempty"`
	Media         string     `json:"media,omitempty"`

	Delivered bool `json:"delivered"`
	Read      bool `json:"read"`

	ReplyTo        *string    `json:"replyTo,omitempty"`
	ForwardedFrom  *string    `json:"forwardedFrom,omitempty"` 
	ForwardedBy    *string    `json:"forwardedBy,omitempty"`   
	Reactions      []Reaction `json:"reactions,omitempty"`

	
	DeliveredBy map[string]bool `json:"-"`
	ReadBy      map[string]bool `json:"-"`
}


type Conversation struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	IsGroup  bool      `json:"isGroup"`
	Members  []string  `json:"members"`
	Messages []Message `json:"messages"`
	Photo    string    `json:"photo,omitempty"`
}

var (
	mu sync.Mutex

	usersByID   = map[string]*User{}
	usersByName = map[string]*User{}

	conversationsByID = map[string]*Conversation{}

	msgCounter  int
	convCounter int
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/session", doLogin)

	mux.HandleFunc("/api/me/name", setMyUserName)
	mux.HandleFunc("/api/me/photo", setMyPhoto)
	mux.HandleFunc("/api/users", listUsers)

	mux.HandleFunc("/api/conversations", conversationsRoot)
	mux.HandleFunc("/api/conversations/", conversationsByIDHandler)

	mux.HandleFunc("/api/groups", createGroup)
	mux.HandleFunc("/api/groups/", groupsHandler)

	mux.HandleFunc("/api/messages/", messagesHandler)

	log.Println("WASAText backend running on http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", withCORS(mux)))
}

//login

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

	u, ok := usersByName[name]
	if !ok {
		u = &User{ID: generateID(), Name: name, Photo: ""}
		usersByName[name] = u
		usersByID[u.ID] = u
	}

	writeJSON(w, http.StatusCreated, map[string]string{"identifier": u.ID})
}

//auth

func getUserFromAuth(r *http.Request) (*User, bool) {
	auth := r.Header.Get("Authorization")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return nil, false
	}

	mu.Lock()
	defer mu.Unlock()
	u, ok := usersByID[token]
	return u, ok
}

//users

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

	oldName := u.Name
	delete(usersByName, u.Name)
	u.Name = newName
	usersByName[u.Name] = u

	// update name 
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
			if c.Messages[i].ForwardedBy != nil && *c.Messages[i].ForwardedBy == oldName {
				tmp := newName
				c.Messages[i].ForwardedBy = &tmp
			}
			for j := range c.Messages[i].Reactions {
				if c.Messages[i].Reactions[j].User == oldName {
					c.Messages[i].Reactions[j].User = newName
				}
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func setMyPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, ok := getUserFromAuth(r)
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
	// photo stored 
	
	mu.Unlock()

	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	u.Photo = req.Photo
	mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	mu.Lock()
	names := make([]string, 0, len(usersByName))
	for name := range usersByName {
		names = append(names, name)
	}
	mu.Unlock()

	sort.Strings(names)
	writeJSON(w, http.StatusOK, map[string]any{"users": names})
}

//CONVERSATIONS 

func conversationsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMyConversations(w, r)
	case http.MethodPost:
		createDirectConversation(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func getMyConversations(w http.ResponseWriter, r *http.Request) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	type item struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		IsGroup     bool      `json:"isGroup"`
		LastTime    time.Time `json:"lastTime"`
		LastPreview string    `json:"lastPreview"`
		Photo       string    `json:"photo,omitempty"`
	}

	mu.Lock()
	var out []item
	for _, c := range conversationsByID {
		if !contains(c.Members, u.Name) {
			continue
		}
		for i := range c.Messages {
			if c.Messages[i].Sender != u.Name {
				ensureMsgMaps(&c.Messages[i])
				c.Messages[i].DeliveredBy[u.Name] = true
			}
		}
		title := c.Title
		photo := c.Photo
		if !c.IsGroup {
			other := otherMember(c.Members, u.Name)
			title = other
			if ou, ok := usersByName[other]; ok {
				photo = ou.Photo
			}
		}

		lastTime := time.Now().UTC()
		preview := ""
		if len(c.Messages) > 0 {
			m := c.Messages[len(c.Messages)-1]
			lastTime = m.Time
			if m.Type == "text" {
				preview = m.Text
			} else {
				preview = "📷 Photo"
			}
		}

		out = append(out, item{
			ID:          c.ID,
			Title:       title,
			IsGroup:     c.IsGroup,
			LastTime:    lastTime,
			LastPreview: preview,
			Photo:       photo,
		})
	}
	mu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].LastTime.After(out[j].LastTime) })
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

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
	if other == u.Name {
		writeError(w, http.StatusBadRequest, "cannot chat with yourself")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := usersByName[other]; !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

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
		Title:    "",
		IsGroup:  false,
		Members:  []string{u.Name, other},
		Messages: []Message{},
		Photo:    "",
	}
	conversationsByID[cid] = c
	writeJSON(w, http.StatusCreated, map[string]string{"conversationId": cid})
}

func conversationsByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	parts := strings.Split(rest, "/")
	cid := parts[0]
	if cid == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		getConversation(w, r, cid)
		return
	}
	if len(parts) == 2 && parts[1] == "messages" && r.Method == http.MethodPost {
		sendMessage(w, r, cid)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

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

	// When user opens the conversation 
	for i := range c.Messages {
		if c.Messages[i].Sender != u.Name {
			ensureMsgMaps(&c.Messages[i])
			c.Messages[i].DeliveredBy[u.Name] = true
			c.Messages[i].ReadBy[u.Name] = true
		}
	}

	// return newest first
	outMsgs := make([]Message, 0, len(c.Messages))
	for i := len(c.Messages) - 1; i >= 0; i-- {
		outMsgs = append(outMsgs, publicMessage(c, c.Messages[i]))
	}

	title := c.Title
	photo := c.Photo
	if !c.IsGroup {
		other := otherMember(c.Members, u.Name)
		title = other
		if ou, ok := usersByName[other]; ok {
			photo = ou.Photo
		}
	}

	resp := map[string]any{
		"id":       c.ID,
		"title":    title,
		"isGroup":  c.IsGroup,
		"members":  c.Members,
		"photo":    photo,
		"messages": outMsgs,
	}
	mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}


func sendMessage(w http.ResponseWriter, r *http.Request, cid string) {
	u, ok := getUserFromAuth(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not logged")
		return
	}

	var req struct {
		Type    string  `json:"type"`
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

	// validate 
	if req.ReplyTo != nil {
		if !messageExistsInConversation(c, *req.ReplyTo) {
			writeError(w, http.StatusBadRequest, "replyTo not found")
			return
		}
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
	
		ReplyTo:   req.ReplyTo,
		Reactions: []Reaction{},
	
		DeliveredBy: map[string]bool{},
		ReadBy:      map[string]bool{},
	}
	
	c.Messages = append(c.Messages, m)
	writeJSON(w, http.StatusCreated, publicMessage(c, m))
}

//groups

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
	conversationsByID[cid] = &Conversation{
		ID:       cid,
		Title:    groupName,
		IsGroup:  true,
		Members:  members,
		Messages: []Message{},
		Photo:    "",
	}
	writeJSON(w, http.StatusCreated, map[string]string{"conversationId": cid})
}

func groupsHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/groups/")
	parts := strings.Split(rest, "/")
	gid := parts[0]
	if gid == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if len(parts) == 2 && parts[1] == "name" && r.Method == http.MethodPut {
		setGroupName(w, r, gid)
		return
	}
	if len(parts) == 2 && parts[1] == "photo" && r.Method == http.MethodPut {
		setGroupPhoto(w, r, gid)
		return
	}
	if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodPost {
		addToGroup(w, r, gid)
		return
	}
	if len(parts) == 2 && parts[1] == "leave" && r.Method == http.MethodPost {
		leaveGroup(w, r, gid)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

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
	newMembers := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		if m != u.Name {
			newMembers = append(newMembers, m)
		}
	}
	c.Members = newMembers
	w.WriteHeader(http.StatusNoContent)
}

//MESSAGES

func messagesHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	parts := strings.Split(rest, "/")
	mid := parts[0]
	if mid == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		deleteMessage(w, r, mid)
		return
	}
	if len(parts) == 2 && parts[1] == "forward" && r.Method == http.MethodPost {
		forwardMessage(w, r, mid)
		return
	}
	if len(parts) == 2 && parts[1] == "comments" && r.Method == http.MethodPost {
		commentMessage(w, r, mid)
		return
	}
	if len(parts) == 3 && parts[1] == "comments" && r.Method == http.MethodDelete {
		uncommentMessage(w, r, mid, parts[2])
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

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

	newMsgs := make([]Message, 0, len(conv.Messages))
	for _, m := range conv.Messages {
		if m.ID != mid {
			newMsgs = append(newMsgs, m)
		}
	}
	conv.Messages = newMsgs
	w.WriteHeader(http.StatusNoContent)
}

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

	fwdBy := u.Name
	fwdByPtr := &fwdBy
	fwdFrom := origMsg.ID
	fwdFromPtr := &fwdFrom

	newMsg := Message{
		ID:            newID,
		Sender:        u.Name,
		Time:          time.Now().UTC(),
		Type:          origMsg.Type,
		Text:          origMsg.Text,
		Media:         origMsg.Media,
		ReplyTo:       nil,
		ForwardedFrom: fwdFromPtr,
		ForwardedBy:   fwdByPtr,
		Reactions:     []Reaction{},
		DeliveredBy:   map[string]bool{},
		ReadBy:        map[string]bool{},
	}
	
	targetConv.Messages = append(targetConv.Messages, newMsg)
	writeJSON(w, http.StatusCreated, publicMessage(targetConv, newMsg))
	
}

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
	emoji := strings.TrimSpace(req.Emoji)
	if emoji == "" {
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

	rid := "r_" + generateID()[0:8]
	reac := Reaction{
		ID:    rid,
		User:  u.Name,
		Emoji: emoji,
		Time:  time.Now().UTC().Format(time.RFC3339),
	}

	// append on the real message 
	for i := range conv.Messages {
		if conv.Messages[i].ID == mid {
			conv.Messages[i].Reactions = append(conv.Messages[i].Reactions, reac)
			break
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{"reactionId": rid})
}

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

	// remove only if authored by this user
	for i := range conv.Messages {
		if conv.Messages[i].ID != mid {
			continue
		}
		reacs := conv.Messages[i].Reactions
		newReacs := make([]Reaction, 0, len(reacs))
		removed := false
		for _, rc := range reacs {
			if rc.ID == rid {
				if rc.User != u.Name {
					writeError(w, http.StatusForbidden, "not author")
					return
				}
				removed = true
				continue
			}
			newReacs = append(newReacs, rc)
		}
		if !removed {
			writeError(w, http.StatusNotFound, "reaction not found")
			return
		}
		conv.Messages[i].Reactions = newReacs
		break
	}

	w.WriteHeader(http.StatusNoContent)
}

//helpers

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
func recipientsOf(c *Conversation, sender string) []string {
	rec := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		if m != sender {
			rec = append(rec, m)
		}
	}
	return rec
}

func ensureMsgMaps(m *Message) {
	if m.DeliveredBy == nil {
		m.DeliveredBy = map[string]bool{}
	}
	if m.ReadBy == nil {
		m.ReadBy = map[string]bool{}
	}
}

func computeDeliveredRead(c *Conversation, m *Message) (bool, bool) {
	recipients := recipientsOf(c, m.Sender)
	if len(recipients) == 0 {
		
		return true, true
	}
	ensureMsgMaps(m)

	del := true
	rd := true
	for _, r := range recipients {
		if !m.DeliveredBy[r] {
			del = false
		}
		if !m.ReadBy[r] {
			rd = false
		}
	}
	return del, rd
}


func publicMessage(c *Conversation, m Message) Message {
	del, rd := computeDeliveredRead(c, &m)
	m.Delivered = del
	m.Read = rd
	
	m.DeliveredBy = nil
	m.ReadBy = nil
	return m
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

func messageExistsInConversation(c *Conversation, mid string) bool {
	for i := range c.Messages {
		if c.Messages[i].ID == mid {
			return true
		}
	}
	return false
}

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
