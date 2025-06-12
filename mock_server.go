// mock_server.go
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
	log.Printf("Headers: %+v", r.Header)

	spec := interfaces.SpecResponse{
		AppName:         "Mock Pokémon Server",
		AppVersion:      "v0.1.0",
		ProtocolVersion: "2.0",
		Features: map[string]bool{
			"richContent":        true,
			"actions":            true,
			"progressIndicators": true,
			"confirmations":      true,
			"multiStep":          true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	responseJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal spec response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Sending spec response: %s", string(responseJSON))
	w.Write(responseJSON)

	duration := time.Since(start)
	log.Printf("=== SPEC COMPLETED in %v ===\n", duration)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("=== COMMAND REQUEST ===")
	log.Printf("Method: %s, URL: %s, Remote: %s", r.Method, r.URL.Path, r.RemoteAddr)
	log.Printf("Headers: %+v", r.Header)

	// Read and log the request body
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
		log.Printf("Processing 'look' command - creating structured response")
		collapsed := false // Explicitly set to false for expanded state

		resp.Response.Type = "structured"
		resp.Response.Content = []interfaces.ContentBlock{
			{
				Type:    "text",
				Content: "A wild Pidgey appears!",
				Status:  "info",
			},
			{
				Type:      "collapsible",
				Title:     "Pidgey's Details",
				Collapsed: &collapsed,
				Content: []interfaces.ContentBlock{
					{
						Type:    "text",
						Content: "A tiny bird Pokémon. It is docile and prefers to avoid conflict.",
					},
					{
						Type:    "table",
						Headers: []string{"Stat", "Value"},
						Rows: [][]string{
							{"Level", "5"},
							{"HP", "40/40"},
							{"Type", "Normal/Flying"},
						},
					},
				},
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
			{
				Name:    "Check Bag",
				Command: "check_bag",
				Type:    "info",
				Icon:    "🎒",
			},
		}

	case "throw_ball":
		log.Printf("Processing 'throw_ball' action")
		resp.Response.Type = "structured"
		resp.Response.Content = []interfaces.ContentBlock{
			{
				Type:    "text",
				Content: "You threw a Poké Ball!",
				Status:  "info",
			},
			{
				Type:     "progress",
				Label:    "Capturing Pidgey...",
				Progress: &[]int{75}[0], // 75% progress
			},
			{
				Type:    "text",
				Content: "Gotcha! Pidgey was caught!",
				Status:  "success",
			},
		}
		resp.Actions = []interfaces.Action{
			{
				Name:    "Give nickname",
				Command: "nickname_pidgey",
				Type:    "info",
				Icon:    "✏️",
			},
			{
				Name:    "Continue adventure",
				Command: "continue",
				Type:    "primary",
				Icon:    "➡️",
			},
		}

	case "error":
		log.Printf("Simulating error response")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		errorResp := interfaces.ErrorResponse{}
		errorResp.Error.Message = "The operation failed spectacularly."
		errorResp.Error.Code = "EPIC_FAIL"
		errorResp.Error.RecoveryActions = []interfaces.Action{
			{Name: "Try again", Command: "retry_op", Type: "primary", Icon: "🔄"},
			{Name: "Give up", Command: "abandon_op", Type: "cancel", Icon: "❌"},
		}
		responseJSON, _ := json.MarshalIndent(errorResp, "", "  ")
		log.Printf("Sending error response: %s", string(responseJSON))
		w.Write(responseJSON)
		return

	default:
		log.Printf("Processing default command: '%s'", req.Command)
		resp.Response.Type = "text"
		resp.Response.Content = fmt.Sprintf("You executed command: '%s'. Try 'look' for a rich response!", req.Command)
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

	// For simplicity, delegate to command handler with the action command
	var resp interfaces.CommandResponse
	resp.Response.Type = "text"
	resp.Response.Content = fmt.Sprintf("Executed action: %s", req.Command)

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

	fmt.Println("Mock Compliant Application server starting on :8080...")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /console/spec    - Application metadata")
	fmt.Println("  POST /console/command - Command execution")
	fmt.Println("  POST /console/action  - Action execution")
	fmt.Println()
	fmt.Println("Try these commands after connecting:")
	fmt.Println("  look      - Rich structured response with actions")
	fmt.Println("  error     - Simulated error response")
	fmt.Println("  anything  - Simple text response")
	fmt.Println()

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
