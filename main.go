package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/justinas/alice"
)

//go:embed templates/*
var templateFS embed.FS

var (
	port            int
	timePerQuestion int
)

func init() {
	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.IntVar(&timePerQuestion, "time", 30, "Seconds per question")
}

// Question holds a single quiz question and its answer.
type Question struct {
	Text   string `json:"text"`
	Answer string `json:"answer"`
}

var AllQuestions = []Question{
	{Text: "If it is 11:00 AM now, what time will it be in exactly 32 hours?", Answer: "7:00 PM"},
	{Text: "5 people meet in a room and shake hands with everyone else exactly once. How many handshakes take place in total?", Answer: "10 handshakes"},
	{Text: "How can you write ⅗ using three identical fractions?", Answer: "⅕ + ⅕ + ⅕"},
	{Text: "A snail climbs up a 12-meter wall. It climbs 3 meters during the day but slips back 2 meters at night. How many days will it take for the snail to reach the top?", Answer: "10 days"},
	{Text: "What mathematical symbol, when placed between 6 and 7, makes the result greater than 6 but less than 7?", Answer: "A decimal point → 6.7"},
	{Text: "A group of people collects ₦529. If each person contributes an amount in naira equal to the total number of people in the group, how many people are in the group?", Answer: "23 people"},
	{Text: "How can you add three more matches to the Roman numeral V to get 8?", Answer: "Make it VIII"},
	{Text: "When I was 12 years old, my brother was half my age. If I am 40 years old today, how old is my brother?", Answer: "34"},
	{Text: "What do you get when you divide 30 by ½ and add 10?", Answer: "70"},
	{Text: "Which three numbers give the same result whether they are added or multiplied together?", Answer: "1, 2, and 3"},
	{Text: "Solve this quickly: 9 − 3 ÷ ⅓ + 1 = ?", Answer: "1"},
	{Text: "What is the perimeter of a circle called?", Answer: "Circumference"},
	{Text: "A bat and a ball cost ₦1.10 in total. The bat costs ₦1.00 more than the ball. How much does the ball cost?", Answer: "₦0.05"},
	{Text: "How many birthdays does the average person have in their lifetime?", Answer: "Only 1"},
	{Text: "If an elephant and a mouse step onto a scale together, they weigh 1,002 kg. If the elephant weighs 1,000 kg more than the mouse, how much does the mouse weigh?", Answer: "1 kg"},
	{Text: "I have a rectangle that is 4 meters long and 3 meters wide. What is its area?", Answer: "12 square meters"},
	{Text: "A water lily doubles its size every day. If it takes 28 days to cover the entire lake, how many days did it take to cover half the lake?", Answer: "27 days"},
	{Text: "There are 14 bicycles and tricycles in a park. If there are 38 wheels in total, how many tricycles are there?", Answer: "10 tricycles"},
	{Text: "There are 12 months in a year. How many months have 28 days?", Answer: "All 12 months"},
	{Text: "What is the sum of the interior angles of a triangle?", Answer: "180°"},
	{Text: "I have ₦100. I increase it by 10%, and then decrease the new total by 10%. How much money do I have now?", Answer: "₦99"},
	{Text: "What is any non-zero number raised to the power of 0?", Answer: "1"},
	{Text: "A father and son have a combined age of 60. The father is 40 years older than the son. How old is the son?", Answer: "10 years old"},
	{Text: "There are 10 fish in a tank. 2 drown, 4 swim away, and 3 are removed. How many fish are left in the tank?", Answer: "7 fish — fish don't drown, and they can't swim away from a tank, but 3 were still removed"},
}

func main() {
	flag.Parse()

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	game := NewGame(AllQuestions, timePerQuestion)
	auth := NewAuthManager()
	handler := NewHandler(game, auth, tmpl)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.PublicView)
	mux.HandleFunc("/host/login", handler.HostLogin)
	mux.HandleFunc("/host/logout", handler.HostLogout)

	// Serve favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := templateFS.ReadFile("templates/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(data)
	})
	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		data, err := templateFS.ReadFile("templates/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	})
	mux.Handle("/host", auth.RequireAuthentication(http.HandlerFunc(handler.HostView)))

	mux.HandleFunc("/ws", handler.WebSocket)

	standard := alice.New(recoverPanic, logRequest, secureHeaders)
	wrappedHandler := standard.Then(mux)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("	Quizzo is live at http://localhost%s", addr)
	log.Printf("	Public view: http://localhost%s/", addr)
	log.Printf("   	Host view:   http://localhost%s/host", addr)
	log.Fatal(http.ListenAndServe(addr, wrappedHandler))
}
