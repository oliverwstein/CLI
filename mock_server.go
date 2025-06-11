// mock_server.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/universal-console/console/internal/interfaces"
)

func specHandler(w http.ResponseWriter, r *http.Request) {
	spec := interfaces.SpecResponse{
		AppName:         "Mock Pokémon Server",
		AppVersion:      "v0.1.0",
		ProtocolVersion: "2.0",
		Features:        map[string]bool{"richContent": true, "actions": true},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	var req interfaces.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received command: %s", req.Command)

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
	json.NewEncoder(w).Encode(resp)
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
