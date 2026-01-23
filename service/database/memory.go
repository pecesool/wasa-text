package database

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"wasa-text/service/globaltime"
)

var (
	ErrNotLogged        = errors.New("not logged")
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrNameAlreadyUsed  = errors.New("name already used")
	ErrInvalid          = errors.New("invalid")
	ErrIDMismatch       = errors.New("id mismatch")
	ErrNotAGroup        = errors.New("not a group")
	ErrAlreadyMember    = errors.New("already member")
	ErrUserNotFound     = errors.New("user not found")
	ErrConversationGone = errors.New("conversation not found")
)

type User struct {
	Name  string
	Token string
	Photo string // base64 data URL or ""
}

type Reaction struct {
	ID    string    `json:"id"`
	User  string    `json:"user"`
	Emoji string    `json:"emoji"`
	Time  time.Time `json:"time"`
}


type Message struct {
	ID       string
	Sender   string
	Time     time.Time
	Type     string // "text" | "image"
	Text     string
	Media    string
	ReplyTo  *string

	ForwardedFrom *string
	ForwardedBy   *string

	Reactions []Reaction

	DeliveredBy map[string]bool
	ReadBy      map[string]bool
}

type Conversation struct {
	ID      string
	IsGroup bool
	Title   string // group name (for direct computed per viewer)
	Photo   string // group photo (for direct computed per viewer)
	Members []string

	Messages []Message
}

type ConversationItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	IsGroup     bool      `json:"isGroup"`
	LastTime    time.Time `json:"lastTime"`
	LastPreview string    `json:"lastPreview"`
	Photo       string    `json:"photo,omitempty"`
}

type PublicMessage struct {
	ID     string    `json:"id"`
	Sender string    `json:"sender"`
	Time   time.Time `json:"time"`
	Type   string    `json:"type"`
	Text   string    `json:"text,omitempty"`
	Media  string    `json:"media,omitempty"`

	Delivered bool `json:"delivered"`
	Read      bool `json:"read"`

	ReplyTo       *string    `json:"replyTo,omitempty"`
	ForwardedFrom *string    `json:"forwardedFrom,omitempty"`
	ForwardedBy   *string    `json:"forwardedBy,omitempty"`
	Reactions     []Reaction `json:"reactions,omitempty"`
}

type ConversationView struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	IsGroup  bool            `json:"isGroup"`
	Members  []string        `json:"members"`
	Photo    string          `json:"photo,omitempty"`
	Messages []PublicMessage `json:"messages"`
}

// Database is the interface the API layer will use.
type Database interface {
	LoginCreateOrGet(name string) (token string, created bool, err error)
	UserByToken(token string) (*User, error)

	ListUsers() []string

	SetMyName(token, newName string) error
	SetMyPhoto(token, photo string) error

	CreateDirectConversation(token, other string) (string, error)
	CreateGroupConversation(token, name string, members []string) (string, error)

	ListMyConversations(token string) ([]ConversationItem, error)
	GetConversation(token, cid string) (ConversationView, error)

	SendMessage(token, cid string, typ, text, media string, replyTo *string) (PublicMessage, error)
	DeleteMessage(token, messageID string) error
	ForwardMessage(token, messageID string, targetCID string) (PublicMessage, error)

	CommentMessage(token, messageID string, emoji string) (reactionID string, err error)
	UncommentMessage(token, messageID string, reactionID string) error

	SetGroupName(token, groupID, name string) error
	SetGroupPhoto(token, groupID, photo string) error
	AddToGroup(token, groupID, username string) error
	LeaveGroup(token, groupID string) error
}

// InMemory implements Database with maps+slices and a mutex.
type InMemory struct {
	mu sync.Mutex
	ck globaltime.Clock

	usersByName  map[string]*User
	usersByToken map[string]*User

	conversations map[string]*Conversation

	// message index -> (conversationID, messageIndex)
	msgIndex map[string]msgLoc

	convCounter int
	msgCounter  int
}

type msgLoc struct {
	cid string
	idx int
}

func NewInMemory() Database {
	return NewInMemoryWithClock(globaltime.NewSystemClock())
}

func NewInMemoryWithClock(ck globaltime.Clock) Database {
	return &InMemory{
		ck:            ck,
		usersByName:   map[string]*User{},
		usersByToken:  map[string]*User{},
		conversations: map[string]*Conversation{},
		msgIndex:      map[string]msgLoc{},
	}
}

// ---------- helpers ----------

func (db *InMemory) now() time.Time { return db.ck.Now() }

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (db *InMemory) nextConvID() string {
	db.convCounter++
	return "c_" + itoa(db.convCounter)
}

func (db *InMemory) nextMsgID() string {
	db.msgCounter++
	return "m_" + itoa(db.msgCounter)
}

func itoa(n int) string {
	// tiny integer to string without fmt (keeps file small)
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	return sign + string(buf[i:])
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
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

func (db *InMemory) ensureMsgMaps(m *Message) {
	if m.DeliveredBy == nil {
		m.DeliveredBy = map[string]bool{}
	}
	if m.ReadBy == nil {
		m.ReadBy = map[string]bool{}
	}
}

func recipientsOf(c *Conversation, sender string) []string {
	out := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		if m != sender {
			out = append(out, m)
		}
	}
	return out
}

func (db *InMemory) deliveredRead(c *Conversation, m *Message) (bool, bool) {
	rec := recipientsOf(c, m.Sender)
	if len(rec) == 0 {
		return true, true
	}
	db.ensureMsgMaps(m)

	del := true
	rd := true
	for _, r := range rec {
		if !m.DeliveredBy[r] {
			del = false
		}
		if !m.ReadBy[r] {
			rd = false
		}
	}
	return del, rd
}

func (db *InMemory) publicMessage(c *Conversation, m *Message) PublicMessage {
	del, rd := db.deliveredRead(c, m)

	pm := PublicMessage{
		ID:       m.ID,
		Sender:   m.Sender,
		Time:     m.Time,
		Type:     m.Type,
		Text:     m.Text,
		Media:    m.Media,
		Delivered: del,
		Read:      rd,
		ReplyTo:   m.ReplyTo,
		ForwardedFrom: m.ForwardedFrom,
		ForwardedBy:   m.ForwardedBy,
		Reactions:     append([]Reaction{}, m.Reactions...),
	}
	return pm
}

// ---------- auth/users ----------

func (db *InMemory) LoginCreateOrGet(name string) (string, bool, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 16 {
		return "", false, ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if u, ok := db.usersByName[name]; ok {
		return u.Token, false, nil
	}

	token := randToken()
	u := &User{Name: name, Token: token, Photo: ""}
	db.usersByName[name] = u
	db.usersByToken[token] = u
	return token, true, nil
}

func (db *InMemory) UserByToken(token string) (*User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	u, ok := db.usersByToken[token]
	if !ok {
		return nil, ErrNotLogged
	}
	// return copy pointer-safe (but keep simple)
	c := *u
	return &c, nil
}

func (db *InMemory) ListUsers() []string {
	db.mu.Lock()
	defer db.mu.Unlock()

	out := make([]string, 0, len(db.usersByName))
	for n := range db.usersByName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (db *InMemory) SetMyName(token, newName string) error {
	newName = strings.TrimSpace(newName)
	if len(newName) < 3 || len(newName) > 16 {
		return ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	u, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}

	oldName := u.Name
	if oldName == newName {
		return nil
	}

	// name already used by someone else
	if other, ok := db.usersByName[newName]; ok && other != u {
		return ErrNameAlreadyUsed
	}

	// 1) update users map key
	delete(db.usersByName, oldName)
	u.Name = newName
	db.usersByName[newName] = u

	// 2) update all conversations/members/messages/reactions and read/delivered maps
	for _, c := range db.conversations {

		// members rename
		for i := range c.Members {
			if c.Members[i] == oldName {
				c.Members[i] = newName
			}
		}

		// messages rename (sender + forwardedBy + maps + reactions)
		for mi := range c.Messages {
			m := &c.Messages[mi]

			if m.Sender == oldName {
				m.Sender = newName
			}

			if m.ForwardedBy != nil && *m.ForwardedBy == oldName {
				nb := newName
				m.ForwardedBy = &nb
			}

			// deliveredBy / readBy keys rename
			if m.DeliveredBy != nil {
				if v, ok := m.DeliveredBy[oldName]; ok {
					delete(m.DeliveredBy, oldName)
					m.DeliveredBy[newName] = v
				}
			}
			if m.ReadBy != nil {
				if v, ok := m.ReadBy[oldName]; ok {
					delete(m.ReadBy, oldName)
					m.ReadBy[newName] = v
				}
			}

			// reactions rename
			for ri := range m.Reactions {
				if m.Reactions[ri].User == oldName {
					m.Reactions[ri].User = newName
				}
			}
		}
	}

	return nil
}



// tokenUserNameUnsafe is unused; left to keep file short; name updates handled below properly in a separate function.

func (db *InMemory) SetMyPhoto(token, photo string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	u, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}
	u.Photo = photo
	return nil
}

// ---------- conversations ----------

func (db *InMemory) CreateDirectConversation(token, other string) (string, error) {
	other = strings.TrimSpace(other)

	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return "", ErrNotLogged
	}
	if other == "" || other == me.Name {
		return "", ErrInvalid
	}
	if _, ok := db.usersByName[other]; !ok {
		return "", ErrUserNotFound
	}

	// if exists already between two users, return it
	for _, c := range db.conversations {
		if !c.IsGroup && len(c.Members) == 2 {
			if contains(c.Members, me.Name) && contains(c.Members, other) {
				return c.ID, nil
			}
		}
	}

	id := db.nextConvID()
	c := &Conversation{
		ID:       id,
		IsGroup:  false,
		Title:    "",
		Photo:    "",
		Members:  []string{me.Name, other},
		Messages: []Message{},
	}
	db.conversations[id] = c
	return id, nil
}

func (db *InMemory) CreateGroupConversation(token, name string, members []string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return "", ErrNotLogged
	}

	uniq := map[string]bool{}
	finalMembers := []string{me.Name}
	uniq[me.Name] = true

	for _, m := range members {
		m = strings.TrimSpace(m)
		if m == "" || uniq[m] {
			continue
		}
		if _, ok := db.usersByName[m]; !ok {
			return "", ErrUserNotFound
		}
		uniq[m] = true
		finalMembers = append(finalMembers, m)
	}

	id := db.nextConvID()
	c := &Conversation{
		ID:       id,
		IsGroup:  true,
		Title:    name,
		Photo:    "",
		Members:  finalMembers,
		Messages: []Message{},
	}
	db.conversations[id] = c
	return id, nil
}

func (db *InMemory) ListMyConversations(token string) ([]ConversationItem, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return nil, ErrNotLogged
	}

	items := make([]ConversationItem, 0)
	for _, c := range db.conversations {
		if !contains(c.Members, me.Name) {
			continue
		}

		// mark delivered for messages not sent by me (conversation list = delivered)
		for i := range c.Messages {
			if c.Messages[i].Sender != me.Name {
				db.ensureMsgMaps(&c.Messages[i])
				c.Messages[i].DeliveredBy[me.Name] = true
			}
		}

		title := c.Title
		photo := c.Photo
		if !c.IsGroup {
			other := otherMember(c.Members, me.Name)
			title = other
			if ou, ok := db.usersByName[other]; ok {
				photo = ou.Photo
			}
		}

		lastTime := time.Time{}
		lastPreview := ""
		if len(c.Messages) > 0 {
			m := c.Messages[len(c.Messages)-1]
			lastTime = m.Time
			if m.Type == "image" {
				lastPreview = "📷 Photo"
			} else {
				lastPreview = m.Text
			}
		}

		items = append(items, ConversationItem{
			ID:          c.ID,
			Title:       title,
			IsGroup:     c.IsGroup,
			LastTime:    lastTime,
			LastPreview: lastPreview,
			Photo:       photo,
		})
	}

	// sort by lastTime desc (reverse chronological)
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastTime.After(items[j].LastTime)
	})

	return items, nil
}

func (db *InMemory) GetConversation(token, cid string) (ConversationView, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ConversationView{}, ErrNotLogged
	}

	c, ok := db.conversations[cid]
	if !ok || !contains(c.Members, me.Name) {
		return ConversationView{}, ErrConversationGone
	}

	// opening conversation => delivered+read for messages not sent by me
	for i := range c.Messages {
		if c.Messages[i].Sender != me.Name {
			db.ensureMsgMaps(&c.Messages[i])
			c.Messages[i].DeliveredBy[me.Name] = true
			c.Messages[i].ReadBy[me.Name] = true
		}
	}

	title := c.Title
	photo := c.Photo
	if !c.IsGroup {
		other := otherMember(c.Members, me.Name)
		title = other
		if ou, ok := db.usersByName[other]; ok {
			photo = ou.Photo
		}
	}

	// newest first in response
	outMsgs := make([]PublicMessage, 0, len(c.Messages))
	for i := len(c.Messages) - 1; i >= 0; i-- {
		outMsgs = append(outMsgs, db.publicMessage(c, &c.Messages[i]))
	}

	return ConversationView{
		ID:       c.ID,
		Title:    title,
		IsGroup:  c.IsGroup,
		Members:  append([]string{}, c.Members...),
		Photo:    photo,
		Messages: outMsgs,
	}, nil
}

// ---------- messages ----------

func (db *InMemory) SendMessage(token, cid, typ, text, media string, replyTo *string) (PublicMessage, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return PublicMessage{}, ErrNotLogged
	}
	c, ok := db.conversations[cid]
	if !ok || !contains(c.Members, me.Name) {
		return PublicMessage{}, ErrConversationGone
	}

	if typ != "text" && typ != "image" {
		return PublicMessage{}, ErrInvalid
	}
	if typ == "text" && strings.TrimSpace(text) == "" {
		return PublicMessage{}, ErrInvalid
	}
	if typ == "image" && strings.TrimSpace(media) == "" {
		return PublicMessage{}, ErrInvalid
	}

	id := db.nextMsgID()
	m := Message{
		ID:          id,
		Sender:      me.Name,
		Time:        db.now(),
		Type:        typ,
		Text:        text,
		Media:       media,
		ReplyTo:     replyTo,
		Reactions:   []Reaction{},
		DeliveredBy: map[string]bool{},
		ReadBy:      map[string]bool{},
	}
	c.Messages = append(c.Messages, m)
	db.msgIndex[id] = msgLoc{cid: cid, idx: len(c.Messages) - 1}

	return db.publicMessage(c, &c.Messages[len(c.Messages)-1]), nil
}

func (db *InMemory) DeleteMessage(token, messageID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}

	loc, ok := db.msgIndex[messageID]
	if !ok {
		return ErrNotFound
	}
	c, ok := db.conversations[loc.cid]
	if !ok {
		return ErrNotFound
	}
	if loc.idx < 0 || loc.idx >= len(c.Messages) || c.Messages[loc.idx].ID != messageID {
		// stale index -> linear search
		found := false
		for i := range c.Messages {
			if c.Messages[i].ID == messageID {
				loc.idx = i
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}
	}

	if c.Messages[loc.idx].Sender != me.Name {
		return ErrForbidden
	}

	// remove message
	c.Messages = append(c.Messages[:loc.idx], c.Messages[loc.idx+1:]...)
	delete(db.msgIndex, messageID)

	// rebuild indices for that conversation (simple and safe)
	for i := range c.Messages {
		db.msgIndex[c.Messages[i].ID] = msgLoc{cid: c.ID, idx: i}
	}
	return nil
}

func (db *InMemory) ForwardMessage(token, messageID string, targetCID string) (PublicMessage, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return PublicMessage{}, ErrNotLogged
	}

	loc, ok := db.msgIndex[messageID]
	if !ok {
		return PublicMessage{}, ErrNotFound
	}
	src, ok := db.conversations[loc.cid]
	if !ok {
		return PublicMessage{}, ErrNotFound
	}
	if loc.idx < 0 || loc.idx >= len(src.Messages) || src.Messages[loc.idx].ID != messageID {
		return PublicMessage{}, ErrNotFound
	}
	orig := src.Messages[loc.idx]

	dst, ok := db.conversations[targetCID]
	if !ok || !contains(dst.Members, me.Name) {
		return PublicMessage{}, ErrConversationGone
	}

	newID := db.nextMsgID()
	from := messageID
	by := me.Name

	m := Message{
		ID:            newID,
		Sender:        me.Name,
		Time:          db.now(),
		Type:          orig.Type,
		Text:          orig.Text,
		Media:         orig.Media,
		ReplyTo:       nil,
		ForwardedFrom: &from,
		ForwardedBy:   &by,
		Reactions:     []Reaction{},
		DeliveredBy:   map[string]bool{},
		ReadBy:        map[string]bool{},
	}

	dst.Messages = append(dst.Messages, m)
	db.msgIndex[newID] = msgLoc{cid: dst.ID, idx: len(dst.Messages) - 1}

	return db.publicMessage(dst, &dst.Messages[len(dst.Messages)-1]), nil
}

// ---------- reactions ----------

func (db *InMemory) CommentMessage(token, messageID string, emoji string) (string, error) {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return "", ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return "", ErrNotLogged
	}

	loc, ok := db.msgIndex[messageID]
	if !ok {
		return "", ErrNotFound
	}
	c, ok := db.conversations[loc.cid]
	if !ok {
		return "", ErrNotFound
	}
	if loc.idx < 0 || loc.idx >= len(c.Messages) || c.Messages[loc.idx].ID != messageID {
		return "", ErrNotFound
	}

	
	rs := c.Messages[loc.idx].Reactions
	for i := range rs {
		if rs[i].User == me.Name {
			rs[i].Emoji = emoji
			rs[i].Time = db.now()
			c.Messages[loc.idx].Reactions = rs
			return rs[i].ID, nil
		}
	}

	
	rid := "r_" + randToken()[:10]
	r := Reaction{ID: rid, User: me.Name, Emoji: emoji, Time: db.now()}
	c.Messages[loc.idx].Reactions = append(c.Messages[loc.idx].Reactions, r)
	return rid, nil
}


func (db *InMemory) UncommentMessage(token, messageID string, reactionID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}

	loc, ok := db.msgIndex[messageID]
	if !ok {
		return ErrNotFound
	}
	c, ok := db.conversations[loc.cid]
	if !ok {
		return ErrNotFound
	}
	if loc.idx < 0 || loc.idx >= len(c.Messages) || c.Messages[loc.idx].ID != messageID {
		return ErrNotFound
	}

	rs := c.Messages[loc.idx].Reactions
	for i := range rs {
		if rs[i].ID == reactionID {
			if rs[i].User != me.Name {
				return ErrForbidden
			}
			// delete reaction
			c.Messages[loc.idx].Reactions = append(rs[:i], rs[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// ---------- group ops ----------

func (db *InMemory) SetGroupName(token, groupID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}
	c, ok := db.conversations[groupID]
	if !ok || !contains(c.Members, me.Name) {
		return ErrNotFound
	}
	if !c.IsGroup {
		return ErrNotAGroup
	}
	c.Title = name
	return nil
}

func (db *InMemory) SetGroupPhoto(token, groupID, photo string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}
	c, ok := db.conversations[groupID]
	if !ok || !contains(c.Members, me.Name) {
		return ErrNotFound
	}
	if !c.IsGroup {
		return ErrNotAGroup
	}
	c.Photo = photo
	return nil
}

func (db *InMemory) AddToGroup(token, groupID, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrInvalid
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}
	c, ok := db.conversations[groupID]
	if !ok || !contains(c.Members, me.Name) {
		return ErrNotFound
	}
	if !c.IsGroup {
		return ErrNotAGroup
	}
	if _, ok := db.usersByName[username]; !ok {
		return ErrUserNotFound
	}
	if contains(c.Members, username) {
		return ErrAlreadyMember
	}
	c.Members = append(c.Members, username)
	return nil
}

func (db *InMemory) LeaveGroup(token, groupID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	me, ok := db.usersByToken[token]
	if !ok {
		return ErrNotLogged
	}
	c, ok := db.conversations[groupID]
	if !ok || !contains(c.Members, me.Name) {
		return ErrNotFound
	}
	if !c.IsGroup {
		return ErrNotAGroup
	}

	// remove member
	newMembers := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		if m != me.Name {
			newMembers = append(newMembers, m)
		}
	}
	c.Members = newMembers

	// if group becomes empty, delete it
	if len(c.Members) == 0 {
		// delete messages indices
		for _, msg := range c.Messages {
			delete(db.msgIndex, msg.ID)
		}
		delete(db.conversations, c.ID)
	}
	return nil
}
