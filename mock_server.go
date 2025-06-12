// Mock server that sends the exact JSON structure the Go content renderer expects
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

func specHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("=== SPEC REQUEST ===")
	log.Printf("Method: %s, URL: %s, Remote: %s", r.Method, r.URL.Path, r.RemoteAddr)

	spec := interfaces.SpecResponse{
		AppName:         "Mock Pokémon Server",
		AppVersion:      "v0.1.0",
		ProtocolVersion: "2.0",
		Features: map[string]bool{
			"richContent": true,
			"actions":     true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)

	duration := time.Since(start)
	log.Printf("=== SPEC COMPLETED in %v ===\n", duration)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("=== COMMAND REQUEST ===")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	log.Printf("Request body: %s", string(body))

	var req interfaces.CommandRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("ERROR: Failed to decode command request: %v", err)
		http.Error(w, fmt.Sprintf("JSON decode error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Parsed command: '%s'", req.Command)

	var resp interfaces.CommandResponse

	switch req.Command {
	case "look":
		log.Printf("Processing 'look' command - creating simple structured response")

		// Start with a simple response that should work
		resp.Response.Type = "structured"
		resp.Response.Content = []interfaces.ContentBlock{
			{
				Type:    "text",
				Content: "A wild Pidgey appears!",
				Status:  "info",
			},
			{
				Type:    "text", // Use simple text instead of collapsible for now
				Content: "Pidgey Details: A tiny bird Pokémon. Level 5, HP: 40/40, Type: Normal/Flying",
				Status:  "info",
			},
		}
		resp.Actions = []interfaces.Action{
			{
				Name:    "Throw Poké Ball",
				Command: "throw_ball",
				Type:    "primary",
				Icon:    "🔴",
			},
			{
				Name:    "Run Away",
				Command: "run",
				Type:    "cancel",
				Icon:    "🏃",
			},
		}

	case "look_advanced":
		log.Printf("Processing 'look_advanced' command - with collapsible content")

		// Try the collapsible content with the structure the Go code expects
		collapsed := false

		// Create the collapsible content as a structured object
		collapsibleContent := map[string]interface{}{
			"title":     "Pidgey's Details",
			"collapsed": collapsed,
			"content": []map[string]interface{}{
				{
					"type":    "text",
					"content": "A tiny bird Pokémon. It is docile and prefers to avoid conflict.",
				},
			},
			"expanded": true,
			"level":    0,
		}

		resp.Response.Type = "structured"
		resp.Response.Content = []interfaces.ContentBlock{
			{
				Type:    "text",
				Content: "A wild Pidgey appears!",
				Status:  "info",
			},
			{
				Type:    "collapsible",
				Content: collapsibleContent, // Send as structured object
			},
		}
		resp.Actions = []interfaces.Action{
			{
				Name:    "Throw Poké Ball",
				Command: "throw_ball",
				Type:    "primary",
				Icon:    "🔴",
			},
		}

	case "throw_ball":
		log.Printf("Processing 'throw_ball' action")
		resp.Response.Type = "text"
		resp.Response.Content = "You threw a Poké Ball! Gotcha! Pidgey was caught!"
		resp.Actions = []interfaces.Action{
			{
				Name:    "Continue adventure",
				Command: "continue",
				Type:    "primary",
				Icon:    "➡️",
			},
		}

	default:
		log.Printf("Processing default command: '%s'", req.Command)
		resp.Response.Type = "text"
		resp.Response.Content = fmt.Sprintf("You executed command: '%s'.\n\nTry these commands:\n  look - Simple structured response\n  look_advanced - With collapsible content", req.Command)
	}

	w.Header().Set("Content-Type", "application/json")
	responseJSON, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal command response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Sending command response: %s", string(responseJSON))
	w.Write(responseJSON)

	duration := time.Since(start)
	log.Printf("=== COMMAND COMPLETED in %v ===\n", duration)
}

func actionHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("=== ACTION REQUEST ===")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read action request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	log.Printf("Action request body: %s", string(body))

	var req interfaces.ActionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("ERROR: Failed to decode action request: %v", err)
		http.Error(w, fmt.Sprintf("JSON decode error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Processing action: '%s'", req.Command)

	// Simple action responses
	var resp interfaces.CommandResponse

	switch req.Command {
	case "throw_ball":
		log.Printf("Action: Throwing Poke Ball")
		resp.Response.Type = "text"
		resp.Response.Content = "You threw a Poké Ball! Gotcha! Pidgey was caught!"
		resp.Actions = []interfaces.Action{
			{
				Name:    "Continue",
				Command: "continue",
				Type:    "primary",
				Icon:    "➡️",
			},
		}

	case "run":
		log.Printf("Action: Running away")
		resp.Response.Type = "text"
		resp.Response.Content = "You ran away safely!"

	default:
		log.Printf("Generic action response for: %s", req.Command)
		resp.Response.Type = "text"
		resp.Response.Content = fmt.Sprintf("Executed action: %s", req.Command)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	duration := time.Since(start)
	log.Printf("=== ACTION COMPLETED in %v ===\n", duration)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	http.HandleFunc("/console/spec", specHandler)
	http.HandleFunc("/console/command", commandHandler)
	http.HandleFunc("/console/action", actionHandler)

	fmt.Println("Mock Server starting on :8080...")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  look           - Simple structured response (should work)")
	fmt.Println("  look_advanced  - With collapsible content (test)")
	fmt.Println("  anything_else  - Simple text response")
	fmt.Println()
	fmt.Println("This version starts with simple content to avoid JSON parsing issues.")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
