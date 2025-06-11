// Package errors implements error recovery mechanisms for the Universal Application Console.
// This file provides a RecoveryManager to handle error context and guide users through
// resolution workflows, as described in the enhanced error response specification.
package errors

import (
	"fmt"
	"sync"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// RecoverySession holds the context for an active error recovery workflow.
type RecoverySession struct {
	ID        string
	StartTime time.Time
	Error     *ProcessedError
	// In a more complex system, this could track the user's recovery steps.
}

// RecoveryManager manages the state of error recovery workflows.
type RecoveryManager struct {
	activeSession *RecoverySession
	sessionMutex  sync.RWMutex
}

// NewRecoveryManager creates a new recovery manager.
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{}
}

// StartSession begins a new error recovery session. It takes a ProcessedError
// and prepares the manager for handling a recovery workflow.
func (rm *RecoveryManager) StartSession(processedErr *ProcessedError) (*RecoverySession, error) {
	if processedErr == nil {
		return nil, fmt.Errorf("cannot start recovery session with a nil error")
	}

	rm.sessionMutex.Lock()
	defer rm.sessionMutex.Unlock()

	session := &RecoverySession{
		ID:        fmt.Sprintf("recov_%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		Error:     processedErr,
	}

	rm.activeSession = session
	return session, nil
}

// EndSession clears the active recovery session.
func (rm *RecoveryManager) EndSession() {
	rm.sessionMutex.Lock()
	defer rm.sessionMutex.Unlock()

	rm.activeSession = nil
}

// IsActive returns true if there is an active error recovery session.
func (rm *RecoveryManager) IsActive() bool {
	rm.sessionMutex.RLock()
	defer rm.sessionMutex.RUnlock()

	return rm.activeSession != nil
}

// GetRecoveryActions returns the list of actions from the currently active error session.
func (rm *RecoveryManager) GetRecoveryActions() []interfaces.Action {
	rm.sessionMutex.RLock()
	defer rm.sessionMutex.RUnlock()

	if rm.activeSession == nil || rm.activeSession.Error == nil {
		return nil
	}
	return rm.activeSession.Error.RecoveryActions
}
