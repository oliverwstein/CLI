// mock_server.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

func specHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("GET /console/spec - Handshake request from %s", r.RemoteAddr)
	
	spec := interfaces.SpecResponse{
		AppName:         "Mock Pokémon Server",
		AppVersion:      "v0.1.0",
		ProtocolVersion: "2.0",
		Features:        map[string]bool{"richContent": true, "actions": true},
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		log.Printf("Failed to encode spec response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	duration := time.Since(start)
	log.Printf("GET /console/spec - Completed in %v", duration)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("POST /console/command - Request from %s", r.RemoteAddr)
	
	var req interfaces.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode command request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Processing command: '%s'", req.Command)

	var resp interfaces.CommandResponse

	// Special command to test rich content and actions
	if req.Command == "look" {
		resp.Response.Type = "structured"
		resp.Response.Content = []interfaces.ContentBlock{
			{Type: "text", Content: "A wild Pidgey appears!", Status: "info"},
			{
				Type:      "collapsible",
				Title:     "Pidgey's Details",
				Collapsed: new(bool), // Defaults to false (expanded)
				Content: []interfaces.ContentBlock{
					{Type: "text", Content: "A tiny bird Pokémon. It is docile and prefers to avoid conflict."},
				},
			},
		}
		resp.Actions = []interfaces.Action{
			{Name: "Throw Poké Ball", Command: "throw_ball", Type: "primary", Icon: "🔴"},
			{Name: "Run Away", Command: "run", Type: "cancel", Icon: "🏃"},
		}
	} else if req.Command == "error" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		errorResp := interfaces.ErrorResponse{}
		errorResp.Error.Message = "The operation failed spectacularly."
		errorResp.Error.Code = "EPIC_FAIL"
		errorResp.Error.RecoveryActions = []interfaces.Action{
			{Name: "Try again", Command: "retry_op", Type: "primary"},
			{Name: "Give up", Command: "abandon_op", Type: "cancel"},
		}
		json.NewEncoder(w).Encode(errorResp)
		return
	} else {
		// Simple echo response for any other command
		resp.Response.Type = "text"
		resp.Response.Content = fmt.Sprintf("You executed command: '%s'", req.Command)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode command response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	duration := time.Since(start)
	log.Printf("POST /console/command - Completed in %v", duration)
}

func main() {
	http.HandleFunc("/console/spec", specHandler)
	http.HandleFunc("/console/command", commandHandler)
	// Add other handlers (action, suggest, etc.) as needed for testing

	fmt.Println("Mock Compliant Application server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
