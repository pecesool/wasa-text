package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

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
	// without /api
	r.router.POST("/session", r.doLogin)
	r.router.GET("/users", r.auth(r.listUsers))

	r.router.PUT("/me/name", r.auth(r.setMyUserName))
	r.router.PUT("/me/photo", r.auth(r.setMyPhoto))

	r.router.GET("/conversations", r.auth(r.getMyConversations))
	r.router.POST("/conversations", r.auth(r.createDirectConversation))
	r.router.GET("/conversations/:conversationId", r.authParams(r.getConversation))
	r.router.POST("/conversations/:conversationId/messages", r.authParams(r.sendMessage))

	r.router.POST("/groups", r.auth(r.createGroup))
	r.router.PUT("/groups/:groupId/name", r.authParams(r.setGroupName))
	r.router.PUT("/groups/:groupId/photo", r.authParams(r.setGroupPhoto))
	r.router.POST("/groups/:groupId/members", r.authParams(r.addToGroup))
	r.router.POST("/groups/:groupId/leave", r.authParams(r.leaveGroup))

	r.router.DELETE("/messages/:messageId", r.authParams(r.deleteMessage))
	r.router.POST("/messages/:messageId/forward", r.authParams(r.forwardMessage))
	r.router.POST("/messages/:messageId/comments", r.authParams(r.commentMessage))
	r.router.DELETE("/messages/:messageId/comments/:reactionId", r.authParams(r.uncommentMessage))

	// with /api
	r.router.POST("/api/session", r.doLogin)
	r.router.GET("/api/users", r.auth(r.listUsers))

	r.router.PUT("/api/me/name", r.auth(r.setMyUserName))
	r.router.PUT("/api/me/photo", r.auth(r.setMyPhoto))

	r.router.GET("/api/conversations", r.auth(r.getMyConversations))
	r.router.POST("/api/conversations", r.auth(r.createDirectConversation))
	r.router.GET("/api/conversations/:conversationId", r.authParams(r.getConversation))
	r.router.POST("/api/conversations/:conversationId/messages", r.authParams(r.sendMessage))

	r.router.POST("/api/groups", r.auth(r.createGroup))
	r.router.PUT("/api/groups/:groupId/name", r.authParams(r.setGroupName))
	r.router.PUT("/api/groups/:groupId/photo", r.authParams(r.setGroupPhoto))
	r.router.POST("/api/groups/:groupId/members", r.authParams(r.addToGroup))
	r.router.POST("/api/groups/:groupId/leave", r.authParams(r.leaveGroup))

	r.router.DELETE("/api/messages/:messageId", r.authParams(r.deleteMessage))
	r.router.POST("/api/messages/:messageId/forward", r.authParams(r.forwardMessage))
	r.router.POST("/api/messages/:messageId/comments", r.authParams(r.commentMessage))
	r.router.DELETE("/api/messages/:messageId/comments/:reactionId", r.authParams(r.uncommentMessage))
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
	p := "/conversations/" + ps.ByName("conversationId")
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleConversationsDynamic(w, req, tok)
}

// operationId: sendMessage
func (r *_router) sendMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/conversations/" + ps.ByName("conversationId") + "/messages"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleConversationsDynamic(w, req, tok)
}

// operationId: deleteMessage
func (r *_router) deleteMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/messages/" + ps.ByName("messageId")
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: forwardMessage
func (r *_router) forwardMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/messages/" + ps.ByName("messageId") + "/forward"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: commentMessage
func (r *_router) commentMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/messages/" + ps.ByName("messageId") + "/comments"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: uncommentMessage
func (r *_router) uncommentMessage(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/messages/" + ps.ByName("messageId") + "/comments/" + ps.ByName("reactionId")
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleMessagesDynamic(w, req, tok)
}

// operationId: setGroupName
func (r *_router) setGroupName(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/groups/" + ps.ByName("groupId") + "/name"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: setGroupPhoto
func (r *_router) setGroupPhoto(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/groups/" + ps.ByName("groupId") + "/photo"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: addToGroup
func (r *_router) addToGroup(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/groups/" + ps.ByName("groupId") + "/members"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleGroupsDynamic(w, req, tok)
}

// operationId: leaveGroup
func (r *_router) leaveGroup(w http.ResponseWriter, req *http.Request, ps httprouter.Params, tok string) {
	p := "/groups/" + ps.ByName("groupId") + "/leave"
	if len(req.URL.Path) >= 5 && req.URL.Path[:5] == "/api/" {
		p = "/api" + p
	}
	req.URL.Path = p
	r.api.handleGroupsDynamic(w, req, tok)
}
