package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Handler holds HTTP and WebSocket handlers, referencing the Game and AuthManager.
type Handler struct {
	game     *Game
	auth     *AuthManager
	tmpl     *template.Template
	upgrader websocket.Upgrader
}

// NewHandler creates a new Handler with authentication support.
func NewHandler(game *Game, auth *AuthManager, tmpl *template.Template) *Handler {
	return &Handler{
		game: game,
		auth: auth,
		tmpl: tmpl,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// PublicView renders the public projector view.
func (h *Handler) PublicView(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "public.html", nil); err != nil {
		log.Printf("Template error (public): %v", err)
		http.Error(w, "Internal server error", 500)
	}
}

// HostView renders the host control view.
func (h *Handler) HostView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "host.html", nil); err != nil {
		log.Printf("Template error (host): %v", err)
		http.Error(w, "Internal server error", 500)
	}
}

// HostLogin handles host login GET and POST requests.
func (h *Handler) HostLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Method == http.MethodGet {
		_ = h.tmpl.ExecuteTemplate(w, "login.html", nil)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if !h.auth.Authenticate(username, password) {
			_ = h.tmpl.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Invalid credentials"})
			return
		}

		token, err := h.auth.CreateSession()
		if err != nil {
			log.Printf("Session creation failed: %v", err)
			_ = h.tmpl.ExecuteTemplate(w, "login.html", map[string]string{"Error": "System error creating session"})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(sessionDuration),
		})

		http.Redirect(w, r, "/host", http.StatusSeeOther)
	}
}

// HostLogout invalidates the host session cookie.
func (h *Handler) HostLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		h.auth.DeleteSession(cookie.Value)
	}

	// Delete cookie in the browser
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	http.Redirect(w, r, "/host/login", http.StatusSeeOther)
}

// clientAction is the message shape sent from the host client.
type clientAction struct {
	Action string `json:"action"`
}

// WebSocket upgrades the connection and handles bidirectional communication.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate the upgrade request
	cookie, err := r.Cookie(sessionCookie)
	isHost := err == nil && h.auth.IsValidSession(cookie.Value)

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	ch := h.game.AddClient()
	defer func() {
		h.game.RemoveClient(ch)
		conn.Close()
	}()

	// Send the current state immediately on connect.
	h.game.SendCurrentState(ch)

	// Writer goroutine: sends state updates to the client.
	go func() {
		for msg := range ch {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Reader loop: reads actions only if client is authorized (isHost = true).
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var act clientAction
		if err := json.Unmarshal(data, &act); err != nil {
			continue
		}

		if isHost {
			h.game.HandleAction(act.Action)
		} else {
			log.Printf("[SECURITY] Unauthorized WS action attempt: %q from %s", act.Action, r.RemoteAddr)
		}
	}
}
