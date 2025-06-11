// Package content implements progressive disclosure mechanisms for the Universal Application Console.
// This file creates collapsible section components with expand and collapse functionality,
// keyboard navigation support, and visual indicators as specified in section 3.3.1
// of the design document.
package content

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// CollapsibleManager handles the state and operations of all collapsible content sections
type CollapsibleManager struct {
	sections      map[string]*CollapsibleContent
	stateHistory  []StateSnapshot
	focusIndex    int
	totalSections int
	mutex         sync.RWMutex
	preferences   CollapsiblePreferences
}

// StateSnapshot captures the state of all collapsible sections at a point in time
type StateSnapshot struct {
	Timestamp  time.Time                   `json:"timestamp"`
	SectionIDs []string                    `json:"sectionIds"`
	States     map[string]CollapsibleState `json:"states"`
	FocusIndex int                         `json:"focusIndex"`
	Operation  string                      `json:"operation"`
}

// CollapsiblePreferences defines user preferences for collapsible behavior
type CollapsiblePreferences struct {
	AnimateToggle      bool          `json:"animateToggle"`
	RememberState      bool          `json:"rememberState"`
	AutoFocus          bool          `json:"autoFocus"`
	ExpandOnFocus      bool          `json:"expandOnFocus"`
	MaxHistorySize     int           `json:"maxHistorySize"`
	DefaultExpanded    bool          `json:"defaultExpanded"`
	KeyboardNavigation bool          `json:"keyboardNavigation"`
	ToggleAnimation    time.Duration `json:"toggleAnimation"`
}

// NavigationDirection represents navigation directions for collapsible sections
type NavigationDirection int

const (
	NavigationNext NavigationDirection = iota
	NavigationPrevious
	NavigationParent
	NavigationChild
	NavigationFirst
	NavigationLast
)

// NewCollapsibleManager creates a new collapsible content manager with default preferences
func NewCollapsibleManager() *CollapsibleManager {
	return &CollapsibleManager{
		sections:     make(map[string]*CollapsibleContent),
		stateHistory: make([]StateSnapshot, 0),
		focusIndex:   -1,
		preferences: CollapsiblePreferences{
			AnimateToggle:      true,
			RememberState:      true,
			AutoFocus:          false,
			ExpandOnFocus:      false,
			MaxHistorySize:     50,
			DefaultExpanded:    false,
			KeyboardNavigation: true,
			ToggleAnimation:    200 * time.Millisecond,
		},
	}
}

// RegisterSection adds a new collapsible section to the manager
func (cm *CollapsibleManager) RegisterSection(sectionID string, content *CollapsibleContent) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if sectionID == "" {
		return fmt.Errorf("section ID cannot be empty")
	}

	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	// Initialize content state if not set
	if content.ToggleState.ID == "" {
		content.ToggleState.ID = sectionID
		content.ToggleState.Expanded = content.Expanded
		content.ToggleState.LastToggled = time.Now()
		content.ToggleState.HasChildren = len(content.Content) > 0
		content.ToggleState.FocusIndex = cm.totalSections
	}

	// Update parent-child relationships
	cm.updateParentChildRelationships(sectionID, content)

	// Register the section
	cm.sections[sectionID] = content
	cm.totalSections++

	// Create state snapshot
	cm.createStateSnapshot("register", sectionID)

	return nil
}

// ToggleSection expands or collapses a specific collapsible section
func (cm *CollapsibleManager) ToggleSection(sectionID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	section, exists := cm.sections[sectionID]
	if !exists {
		return fmt.Errorf("section '%s' not found", sectionID)
	}

	// Toggle the expanded state
	section.Expanded = !section.Expanded
	section.Collapsed = !section.Expanded

	// Update toggle state
	section.ToggleState.Expanded = section.Expanded
	section.ToggleState.LastToggled = time.Now()
	section.ToggleState.ToggleCount++

	// Create state snapshot
	cm.createStateSnapshot("toggle", sectionID)

	// Handle child sections if collapsing parent
	if !section.Expanded && len(section.ToggleState.ChildrenIDs) > 0 {
		cm.collapseChildSections(section.ToggleState.ChildrenIDs)
	}

	return nil
}

// ExpandAll expands all collapsible sections
func (cm *CollapsibleManager) ExpandAll() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for _, section := range cm.sections {
		section.Expanded = true
		section.Collapsed = false
		section.ToggleState.Expanded = true
		section.ToggleState.LastToggled = time.Now()
		section.ToggleState.ToggleCount++
	}

	cm.createStateSnapshot("expand_all", "")
	return nil
}

// CollapseAll collapses all collapsible sections
func (cm *CollapsibleManager) CollapseAll() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for _, section := range cm.sections {
		section.Expanded = false
		section.Collapsed = true
		section.ToggleState.Expanded = false
		section.ToggleState.LastToggled = time.Now()
		section.ToggleState.ToggleCount++
	}

	cm.createStateSnapshot("collapse_all", "")
	return nil
}

// NavigateSections handles keyboard navigation between collapsible sections
func (cm *CollapsibleManager) NavigateSections(direction NavigationDirection) (string, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if len(cm.sections) == 0 {
		return "", fmt.Errorf("no sections available for navigation")
	}

	// Get ordered list of section IDs
	sectionIDs := cm.getOrderedSectionIDs()

	switch direction {
	case NavigationNext:
		return cm.navigateNext(sectionIDs), nil
	case NavigationPrevious:
		return cm.navigatePrevious(sectionIDs), nil
	case NavigationFirst:
		cm.focusIndex = 0
		return sectionIDs[0], nil
	case NavigationLast:
		cm.focusIndex = len(sectionIDs) - 1
		return sectionIDs[cm.focusIndex], nil
	case NavigationParent:
		return cm.navigateToParent(sectionIDs)
	case NavigationChild:
		return cm.navigateToChild(sectionIDs)
	default:
		return "", fmt.Errorf("unsupported navigation direction")
	}
}

// GetSectionState returns the current state of a specific section
func (cm *CollapsibleManager) GetSectionState(sectionID string) (*CollapsibleState, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	section, exists := cm.sections[sectionID]
	if !exists {
		return nil, fmt.Errorf("section '%s' not found", sectionID)
	}

	// Return a copy to prevent external modification
	state := section.ToggleState
	return &state, nil
}

// GetAllSectionStates returns the current state of all sections
func (cm *CollapsibleManager) GetAllSectionStates() map[string]CollapsibleState {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	states := make(map[string]CollapsibleState)
	for id, section := range cm.sections {
		states[id] = section.ToggleState
	}

	return states
}

// RestoreFromSnapshot restores all sections to a previous state
func (cm *CollapsibleManager) RestoreFromSnapshot(timestamp time.Time) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Find the snapshot closest to the requested timestamp
	var targetSnapshot *StateSnapshot
	for i := len(cm.stateHistory) - 1; i >= 0; i-- {
		if cm.stateHistory[i].Timestamp.Before(timestamp) || cm.stateHistory[i].Timestamp.Equal(timestamp) {
			targetSnapshot = &cm.stateHistory[i]
			break
		}
	}

	if targetSnapshot == nil {
		return fmt.Errorf("no snapshot found for timestamp %v", timestamp)
	}

	// Restore section states
	for sectionID, state := range targetSnapshot.States {
		if section, exists := cm.sections[sectionID]; exists {
			section.Expanded = state.Expanded
			section.Collapsed = !state.Expanded
			section.ToggleState = state
		}
	}

	cm.focusIndex = targetSnapshot.FocusIndex

	// Create new snapshot for the restore operation
	cm.createStateSnapshot("restore", "")

	return nil
}

// GetStateHistory returns the history of state changes
func (cm *CollapsibleManager) GetStateHistory() []StateSnapshot {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// Return a copy to prevent external modification
	history := make([]StateSnapshot, len(cm.stateHistory))
	copy(history, cm.stateHistory)
	return history
}

// UpdatePreferences updates the collapsible manager preferences
func (cm *CollapsibleManager) UpdatePreferences(preferences CollapsiblePreferences) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.preferences = preferences
}

// GetCollapsibleSummary returns summary information about all collapsible sections
func (cm *CollapsibleManager) GetCollapsibleSummary() CollapsibleSummary {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	summary := CollapsibleSummary{
		TotalSections:    len(cm.sections),
		ExpandedSections: 0,
		FocusedSection:   "",
		MaxNestingLevel:  0,
	}

	var focusedSectionID string
	if cm.focusIndex >= 0 {
		sectionIDs := cm.getOrderedSectionIDs()
		if cm.focusIndex < len(sectionIDs) {
			focusedSectionID = sectionIDs[cm.focusIndex]
		}
	}
	summary.FocusedSection = focusedSectionID

	for _, section := range cm.sections {
		if section.Expanded {
			summary.ExpandedSections++
		}
		if section.Level > summary.MaxNestingLevel {
			summary.MaxNestingLevel = section.Level
		}
	}

	return summary
}

// CollapsibleSummary provides overview information about collapsible sections
type CollapsibleSummary struct {
	TotalSections    int    `json:"totalSections"`
	ExpandedSections int    `json:"expandedSections"`
	FocusedSection   string `json:"focusedSection"`
	MaxNestingLevel  int    `json:"maxNestingLevel"`
}

// Helper methods for internal operations

// updateParentChildRelationships establishes parent-child relationships between sections
func (cm *CollapsibleManager) updateParentChildRelationships(sectionID string, content *CollapsibleContent) {
	// This is a simplified implementation
	// In a full implementation, this would analyze content hierarchy
	// and establish proper parent-child relationships

	// For now, we'll use the level to determine relationships
	for existingID, existingSection := range cm.sections {
		if existingSection.Level == content.Level-1 {
			// Found potential parent
			content.ToggleState.ParentID = existingID
			existingSection.ToggleState.ChildrenIDs = append(existingSection.ToggleState.ChildrenIDs, sectionID)
			existingSection.ToggleState.HasChildren = true
			break
		}
	}
}

// collapseChildSections recursively collapses child sections
func (cm *CollapsibleManager) collapseChildSections(childIDs []string) {
	for _, childID := range childIDs {
		if child, exists := cm.sections[childID]; exists {
			child.Expanded = false
			child.Collapsed = true
			child.ToggleState.Expanded = false
			child.ToggleState.LastToggled = time.Now()

			// Recursively collapse grandchildren
			if len(child.ToggleState.ChildrenIDs) > 0 {
				cm.collapseChildSections(child.ToggleState.ChildrenIDs)
			}
		}
	}
}

// getOrderedSectionIDs returns section IDs in display order
func (cm *CollapsibleManager) getOrderedSectionIDs() []string {
	type sectionInfo struct {
		ID    string
		Index int
	}

	var sections []sectionInfo
	for id, section := range cm.sections {
		sections = append(sections, sectionInfo{
			ID:    id,
			Index: section.ToggleState.FocusIndex,
		})
	}

	// Sort by focus index
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Index < sections[j].Index
	})

	var ids []string
	for _, section := range sections {
		ids = append(ids, section.ID)
	}

	return ids
}

// navigateNext moves focus to the next section
func (cm *CollapsibleManager) navigateNext(sectionIDs []string) string {
	if cm.focusIndex < len(sectionIDs)-1 {
		cm.focusIndex++
	} else {
		cm.focusIndex = 0 // Wrap around
	}
	return sectionIDs[cm.focusIndex]
}

// navigatePrevious moves focus to the previous section
func (cm *CollapsibleManager) navigatePrevious(sectionIDs []string) string {
	if cm.focusIndex > 0 {
		cm.focusIndex--
	} else {
		cm.focusIndex = len(sectionIDs) - 1 // Wrap around
	}
	return sectionIDs[cm.focusIndex]
}

// navigateToParent moves focus to the parent section
func (cm *CollapsibleManager) navigateToParent(sectionIDs []string) (string, error) {
	if cm.focusIndex < 0 || cm.focusIndex >= len(sectionIDs) {
		return "", fmt.Errorf("invalid focus index")
	}

	currentID := sectionIDs[cm.focusIndex]
	currentSection := cm.sections[currentID]

	if currentSection.ToggleState.ParentID == "" {
		return "", fmt.Errorf("current section has no parent")
	}

	// Find parent index
	for i, id := range sectionIDs {
		if id == currentSection.ToggleState.ParentID {
			cm.focusIndex = i
			return id, nil
		}
	}

	return "", fmt.Errorf("parent section not found")
}

// navigateToChild moves focus to the first child section
func (cm *CollapsibleManager) navigateToChild(sectionIDs []string) (string, error) {
	if cm.focusIndex < 0 || cm.focusIndex >= len(sectionIDs) {
		return "", fmt.Errorf("invalid focus index")
	}

	currentID := sectionIDs[cm.focusIndex]
	currentSection := cm.sections[currentID]

	if len(currentSection.ToggleState.ChildrenIDs) == 0 {
		return "", fmt.Errorf("current section has no children")
	}

	// Find first child index
	firstChildID := currentSection.ToggleState.ChildrenIDs[0]
	for i, id := range sectionIDs {
		if id == firstChildID {
			cm.focusIndex = i
			return id, nil
		}
	}

	return "", fmt.Errorf("child section not found")
}

// createStateSnapshot creates a snapshot of the current state
func (cm *CollapsibleManager) createStateSnapshot(operation, sectionID string) {
	if !cm.preferences.RememberState {
		return
	}

	snapshot := StateSnapshot{
		Timestamp:  time.Now(),
		SectionIDs: cm.getOrderedSectionIDs(),
		States:     make(map[string]CollapsibleState),
		FocusIndex: cm.focusIndex,
		Operation:  operation,
	}

	// Copy current states
	for id, section := range cm.sections {
		snapshot.States[id] = section.ToggleState
	}

	cm.stateHistory = append(cm.stateHistory, snapshot)

	// Trim history if it exceeds maximum size
	if len(cm.stateHistory) > cm.preferences.MaxHistorySize {
		cm.stateHistory = cm.stateHistory[1:]
	}
}
