package database

// Database is the interface used by the API layer.
// InMemory (memory.go) is one implementation.
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
