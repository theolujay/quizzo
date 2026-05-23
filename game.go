package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// GameState represents the current phase of the quiz.
type GameState string

const (
	StateLobby          GameState = "lobby"
	StateQuestionActive GameState = "question_active"
	StateTimeUp         GameState = "time_up"
	StateAnswerShown    GameState = "answer_shown"
	StateGameOver       GameState = "game_over"
)

// StateUpdate is the message broadcast to all connected clients.
type StateUpdate struct {
	Type          string     `json:"type"`
	GameState     string     `json:"gameState"`
	QuestionIndex int        `json:"questionIndex"`
	Total         int        `json:"total"`
	Question      string     `json:"question"`
	Answer        string     `json:"answer"`
	Questions     []Question `json:"questions"`
	TimeRemaining int        `json:"timeRemaining"`
	TotalTime     int        `json:"totalTime"`
}

// Game manages the quiz state machine, timer, and client broadcasting.
type Game struct {
	mu sync.Mutex

	questions   []Question
	totalTime   int // seconds per question
	state       GameState
	questionIdx int
	timeLeft    int
	timerStop   chan struct{}
	clients     map[chan []byte]struct{}
}

// NewGame creates a new game with the given questions and time per question.
func NewGame(questions []Question, totalTime int) *Game {
	return &Game{
		questions: questions,
		totalTime: totalTime,
		state:     StateLobby,
		clients:   make(map[chan []byte]struct{}),
	}
}

// AddClient registers a new WebSocket client and returns its message channel.
func (g *Game) AddClient() chan []byte {
	g.mu.Lock()
	defer g.mu.Unlock()

	ch := make(chan []byte, 16)
	g.clients[ch] = struct{}{}
	return ch
}

// RemoveClient unregisters a client.
func (g *Game) RemoveClient(ch chan []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.clients, ch)
	close(ch)
}

// broadcast sends the current state to all connected clients.
func (g *Game) broadcast() {
	update := g.buildUpdate()
	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal state update: %v", err)
		return
	}
	for ch := range g.clients {
		select {
		case ch <- data:
		default:
			// Client too slow, skip this message
		}
	}
}

// buildUpdate constructs a StateUpdate from the current game state.
func (g *Game) buildUpdate() StateUpdate {
	update := StateUpdate{
		Type:          "state_update",
		GameState:     string(g.state),
		QuestionIndex: g.questionIdx,
		Total:         len(g.questions),
		Questions:     g.questions,
		TotalTime:     g.totalTime,
		TimeRemaining: g.timeLeft,
	}

	if g.state != StateLobby && g.state != StateGameOver {
		if g.questionIdx >= 0 && g.questionIdx < len(g.questions) {
			update.Question = g.questions[g.questionIdx].Text
			update.Answer = g.questions[g.questionIdx].Answer
		}
	}

	return update
}

// SendCurrentState sends the current state to a single client.
func (g *Game) SendCurrentState(ch chan []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()

	update := g.buildUpdate()
	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal state update: %v", err)
		return
	}
	select {
	case ch <- data:
	default:
	}
}

// HandleAction processes a host action and transitions the game state.
func (g *Game) HandleAction(action string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch action {
	case "start":
		if g.state == StateLobby {
			g.questionIdx = 0
			g.startQuestion()
		}
	case "reveal":
		if g.state == StateQuestionActive || g.state == StateTimeUp {
			g.stopTimer()
			g.state = StateAnswerShown
			g.broadcast()
		}
	case "next":
		if g.state == StateAnswerShown {
			g.questionIdx++
			if g.questionIdx >= len(g.questions) {
				g.state = StateGameOver
				g.broadcast()
			} else {
				g.startQuestion()
			}
		}
	case "restart":
		if g.state == StateGameOver {
			g.questionIdx = 0
			g.state = StateLobby
			g.broadcast()
		}
	}
}

// startQuestion transitions to QUESTION_ACTIVE and starts the timer.
// Must be called with g.mu held.
func (g *Game) startQuestion() {
	g.state = StateQuestionActive
	g.timeLeft = g.totalTime
	g.broadcast()

	g.timerStop = make(chan struct{})
	go g.runTimer(g.timerStop)
}

// stopTimer stops the current timer goroutine if running.
// Must be called with g.mu held.
func (g *Game) stopTimer() {
	if g.timerStop != nil {
		close(g.timerStop)
		g.timerStop = nil
	}
}

// runTimer ticks every second and broadcasts the remaining time.
// When time reaches zero, it transitions to TIME_UP (not ANSWER_SHOWN).
func (g *Game) runTimer(stop chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			g.mu.Lock()
			if g.timeLeft > 0 {
				g.timeLeft--
				if g.timeLeft == 0 {
					g.state = StateTimeUp
					g.broadcast()
					g.mu.Unlock()
					return
				}
				g.broadcast()
			}
			g.mu.Unlock()
		}
	}
}
