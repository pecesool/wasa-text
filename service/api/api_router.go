package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Markers for static OpenAPI checkers that look for "{param}" patterns in code.
// These do not affect runtime routing. They only help static analyzers compare
// OpenAPI paths with source code.
const (
	openAPISession                  = "/session"
	openAPIUsers                    = "/users"
	openAPIMeName                   = "/me/name"
	openAPIMePhoto                  = "/me/photo"
	openAPIConversations            = "/conversations"
	openAPIGroups                   = "/groups"
	openAPIConversationByID         = "/conversations/{conversationId}"
	openAPIConversationMessages     = "/conversations/{conversationId}/messages"
	openAPIMessageByID              = "/messages/{messageId}"
	openAPIMessageForward           = "/messages/{messageId}/forward"
	openAPIMessageComments          = "/messages/{messageId}/comments"
	openAPIMessageCommentByReaction = "/messages/{messageId}/comments/{reactionId}"
	openAPIGroupName                = "/groups/{groupId}/name"
	openAPIGroupPhoto               = "/groups/{groupId}/photo"
	openAPIGroupMembers             = "/groups/{groupId}/members"
	openAPIGroupLeave               = "/groups/{groupId}/leave"
)

// Mark constants as used without affecting runtime.
var _ = []string{
	openAPISession,
	openAPIUsers,
	openAPIMeName,
	openAPIMePhoto,
	openAPIConversations,
	openAPIGroups,
	openAPIConversationByID,
	openAPIConversationMessages,
	openAPIMessageByID,
	openAPIMessageForward,
	openAPIMessageComments,
	openAPIMessageCommentByReaction,
	openAPIGroupName,
	openAPIGroupPhoto,
	openAPIGroupMembers,
	openAPIGroupLeave,
}

func (a *API) Handler() http.Handler {
	rt := httprouter.New()
	r := &_router{api: a, router: rt}
	r.register()
	return rt
}

type _router struct {
	api    *API
	router *httprouter.Router
}

func (r *_router) register() {
	r.registerBase("")
	r.registerBase("/api")
}

func (r *_router) registerBase(base string) {
	// Session
	r.router.POST(base+"/session", r.doLogin)

	// Users
	r.router.GET(base+"/users", r.auth(r.listUsers))

	// Me
	r.router.PUT(base+"/me/name", r.auth(r.setMyUserName))
	r.router.PUT(base+"/me/photo", r.auth(r.setMyPhoto))

	// Conversations
	r.router.GET(base+"/conversations", r.auth(r.getMyConversations))
	r.router.POST(base+"/conversations", r.auth(r.createDirectConversation))
	r.router.GET(base+"/conversations/:conversationId", r.authParams(r.getConversation))
	r.router.POST(base+"/conversations/:conversationId/messages", r.authParams(r.sendMessage))

	// Groups
	r.router.POST(base+"/groups", r.auth(r.createGroup))
	r.router.PUT(base+"/groups/:groupId/name", r.authParams(r.setGroupName))
	r.router.PUT(base+"/groups/:groupId/photo", r.authParams(r.setGroupPhoto))
	r.router.POST(base+"/groups/:groupId/members", r.authParams(r.addToGroup))
	r.router.POST(base+"/groups/:groupId/leave", r.authParams(r.leaveGroup))

	// Messages
	r.router.DELETE(base+"/messages/:messageId", r.authParams(r.deleteMessage))
	r.router.POST(base+"/messages/:messageId/forward", r.authParams(r.forwardMessage))
	r.router.POST(base+"/messages/:messageId/comments", r.authParams(r.commentMessage))
	r.router.DELETE(base+"/messages/:messageId/comments/:reactionId", r.authParams(r.uncommentMessage))
}

func (r *_router) auth(h func(http.ResponseWriter, *http.Request, string)) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		tok := bearerToken(req)
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		if _, err := r.api.db.UserByToken(tok); err != nil {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		h(w, req, tok)
	}
}

func (r *_router) authParams(h func(http.ResponseWriter, *http.Request, httprouter.Params, string)) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		tok := bearerToken(req)
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		if _, err := r.api.db.UserByToken(tok); err != nil {
			writeError(w, http.StatusUnauthorized, "not logged")
			return
		}
		h(w, req, ps, tok)
	}
}

// operationId: doLogin
func (r *_router) doLogin(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	r.api.handleSession(w, req)
}

// operationId: listUsers
func (r *_router) listUsers(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleUsers(w, req, tok)
}

// operationId: setMyUserName
func (r *_router) setMyUserName(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleMeName(w, req, tok)
}

// operationId: setMyPhoto
func (r *_router) setMyPhoto(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleMePhoto(w, req, tok)
}

// operationId: getMyConversations
func (r *_router) getMyConversations(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleConversations(w, req, tok)
}

// operationId: createDirectConversation
func (r *_router) createDirectConversation(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleConversations(w, req, tok)
}

// operationId: createGroup
func (r *_router) createGroup(w http.ResponseWriter, req *http.Request, tok string) {
	r.api.handleGroups(w, req, tok)
}

// operationId: getConversation
func (r *_router) getConversation(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/conversations/" + ps.ByName("conversationId")
	r.api.handleConversationsDynamic(w, req, tok)
}

// operationId: sendMessage
func (r *_router) sendMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/conversations/" + ps.ByName("conversationId") + "/messages"
	r.api.handleConversationsDynamic(w, req, tok)
}

// operationId: deleteMessage
func (r *_router) deleteMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/messages/" + ps.ByName("messageId")
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: forwardMessage
func (r *_router) forwardMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/messages/" + ps.ByName("messageId") + "/forward"
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: commentMessage
func (r *_router) commentMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/messages/" + ps.ByName("messageId") + "/comments"
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: uncommentMessage
func (r *_router) uncommentMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/messages/" + ps.ByName("messageId") + "/comments/" + ps.ByName("reactionId")
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: setGroupName
func (r *_router) setGroupName(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/groups/" + ps.ByName("groupId") + "/name"
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: setGroupPhoto
func (r *_router) setGroupPhoto(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/groups/" + ps.ByName("groupId") + "/photo"
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: addToGroup
func (r *_router) addToGroup(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/groups/" + ps.ByName("groupId") + "/members"
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: leaveGroup
func (r *_router) leaveGroup(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	req.URL.Path = "/groups/" + ps.ByName("groupId") + "/leave"
	r.api.handleGroupsDynamic(w, req, tok)
}
