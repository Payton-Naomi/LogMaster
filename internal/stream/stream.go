package stream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"logmaster-agent/internal/auth"
)

type ThreadMessage struct {
	Box  int    `json:"box"`
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type Authenticator interface {
	CurrentUser(r *http.Request) (auth.UserInfo, bool)
}

func RegisterRoutes(mux *http.ServeMux, authenticator Authenticator) {
	mux.HandleFunc("/api/stream", streamHandler(authenticator))
}

func streamHandler(authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticator.CurrentUser(r); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		messages := make(chan ThreadMessage)
		go runThreads(messages)

		for message := range messages {
			data, err := json.Marshal(message)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func runThreads(messages chan<- ThreadMessage) {
	var wg sync.WaitGroup

	wg.Add(3)

	go printSteps(messages, &wg, 1, "Alice", 1)
	go printSteps(messages, &wg, 2, "Wzy", 2)
	go printSteps(messages, &wg, 3, "zzh", 3)

	wg.Wait()
	close(messages)
}

func printSteps(messages chan<- ThreadMessage, wg *sync.WaitGroup, box int, name string, versionNum int) {
	defer wg.Done()

	for step := 1; step <= 8; step++ {
		messages <- ThreadMessage{
			Box: box,
			Text: fmt.Sprintf(
				"Version: %d\nName: %s\nStep: %d/8\nStatus: goroutine %d is working...\n\n",
				versionNum,
				name,
				step,
				box,
			),
		}

		time.Sleep(2 * time.Second)
	}

	messages <- ThreadMessage{
		Box:  box,
		Text: fmt.Sprintf("Thread %d finished.\n", box),
		Done: true,
	}
}
