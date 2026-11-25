// A tiny in-memory To-Do API. It is only a test target for the PoC.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
)

type todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type store struct {
	mu     sync.Mutex
	nextID int
	items  map[int]todo
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *store) create(w http.ResponseWriter, r *http.Request) {
	var in todo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	in.ID = s.nextID
	s.items[in.ID] = in
	writeJSON(w, http.StatusCreated, in)
}

func (s *store) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]todo, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *store) get(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *store) update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var in todo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	in.ID = id
	s.items[id] = in
	writeJSON(w, http.StatusOK, in)
}

func (s *store) remove(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	delete(s.items, id)
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	s := &store{items: map[int]todo{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /todos", s.create)
	mux.HandleFunc("GET /todos", s.list)
	mux.HandleFunc("GET /todos/{id}", s.get)
	mux.HandleFunc("PUT /todos/{id}", s.update)
	mux.HandleFunc("DELETE /todos/{id}", s.remove)

	// ":8081" listens on all interfaces, so a Docker container can reach it.
	log.Println("To-Do server listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
