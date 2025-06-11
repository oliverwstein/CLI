// Package content implements structured content processing for the Universal Application Console.
// This file defines content type structures that correspond to the JSON content specifications
// outlined in section 4.2.1 of the design document, enabling proper parsing and rendering
// of rich application responses.
package content

import (
	"fmt"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// RenderableContent represents content that has been processed for display in the Console
type RenderableContent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Text        string                 `json:"text"`
	Focusable   bool                   `json:"focusable"`
	Expanded    *bool                  `json:"expanded,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Children    []RenderableContent    `json:"children,omitempty"`
	StyleHints  StyleHints             `json:"styleHints"`
	Positioning Positioning            `json:"positioning"`
}

// StyleHints provides visual styling information for rendered content
type StyleHints struct {
	ForegroundColor string            `json:"foregroundColor,omitempty"`
	BackgroundColor string            `json:"backgroundColor,omitempty"`
	Bold            bool              `json:"bold"`
	Italic          bool              `json:"italic"`
	Underline       bool              `json:"underline"`
	Strikethrough   bool              `json:"strikethrough"`
	Borders         BorderStyle       `json:"borders"`
	Padding         Spacing           `json:"padding"`
	Margin          Spacing           `json:"margin"`
	Alignment       string            `json:"alignment"` // "left", "center", "right", "justify"
	CustomStyles    map[string]string `json:"customStyles,omitempty"`
}

// BorderStyle defines border rendering characteristics
type BorderStyle struct {
	Enabled    bool   `json:"enabled"`
	Style      string `json:"style"` // "single", "double", "rounded", "thick"
	Color      string `json:"color,omitempty"`
	Title      string `json:"title,omitempty"`
	TitleAlign string `json:"titleAlign"` // "left", "center", "right"
}

// Spacing defines spacing values for padding and margins
type Spacing struct {
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

// Positioning defines layout positioning for content elements
type Positioning struct {
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	MinWidth   int    `json:"minWidth,omitempty"`
	MaxWidth   int    `json:"maxWidth,omitempty"`
	FlexGrow   int    `json:"flexGrow,omitempty"`
	FlexShrink int    `json:"flexShrink,omitempty"`
	Overflow   string `json:"overflow"` // "visible", "hidden", "scroll", "auto"
}

// TextContent represents plain text content with optional status indicators
type TextContent struct {
	Text     string            `json:"text"`
	Status   string            `json:"status,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CollapsibleContent represents expandable content sections with titles
type CollapsibleContent struct {
	Title       string                    `json:"title"`
	Collapsed   bool                      `json:"collapsed"`
	Content     []interfaces.ContentBlock `json:"content"`
	Icon        string                    `json:"icon,omitempty"`
	Level       int                       `json:"level"` // Nesting level for hierarchical display
	ChildCount  int                       `json:"childCount,omitempty"`
	Expanded    bool                      `json:"expanded"`
	ToggleState CollapsibleState          `json:"toggleState"`
}

// CollapsibleState tracks the state of collapsible content sections
type CollapsibleState struct {
	ID          string    `json:"id"`
	Expanded    bool      `json:"expanded"`
	LastToggled time.Time `json:"lastToggled"`
	ToggleCount int       `json:"toggleCount"`
	HasChildren bool      `json:"hasChildren"`
	ChildrenIDs []string  `json:"childrenIds,omitempty"`
	ParentID    string    `json:"parentId,omitempty"`
	FocusIndex  int       `json:"focusIndex"`
}

// TableContent represents tabular data with headers and alignment options
type TableContent struct {
	Headers     []string      `json:"headers"`
	Rows        [][]string    `json:"rows"`
	Alignment   []string      `json:"alignment,omitempty"`   // Per-column alignment
	ColumnWidth []int         `json:"columnWidth,omitempty"` // Per-column width hints
	Zebra       bool          `json:"zebra"`                 // Alternating row colors
	Borders     bool          `json:"borders"`               // Show table borders
	Sortable    []bool        `json:"sortable,omitempty"`    // Per-column sortability
	Footer      []string      `json:"footer,omitempty"`      // Optional footer row
	Caption     string        `json:"caption,omitempty"`     // Table caption
	Metadata    TableMetadata `json:"metadata"`
}

// TableMetadata provides additional table rendering information
type TableMetadata struct {
	TotalRows    int               `json:"totalRows"`
	FilteredRows int               `json:"filteredRows,omitempty"`
	SortColumn   int               `json:"sortColumn,omitempty"`
	SortOrder    string            `json:"sortOrder,omitempty"` // "asc", "desc"
	Pagination   PaginationInfo    `json:"pagination,omitempty"`
	Summary      map[string]string `json:"summary,omitempty"`
}

// PaginationInfo describes table pagination state
type PaginationInfo struct {
	CurrentPage int `json:"currentPage"`
	TotalPages  int `json:"totalPages"`
	PageSize    int `json:"pageSize"`
	TotalItems  int `json:"totalItems"`
}

// CodeContent represents syntax-highlighted code blocks
type CodeContent struct {
	Code        string           `json:"code"`
	Language    string           `json:"language"`
	Filename    string           `json:"filename,omitempty"`
	LineNumbers bool             `json:"lineNumbers"`
	Highlight   []LineHighlight  `json:"highlight,omitempty"`
	Diff        *DiffInfo        `json:"diff,omitempty"`
	Folding     []FoldingRegion  `json:"folding,omitempty"`
	Annotations []CodeAnnotation `json:"annotations,omitempty"`
	Theme       string           `json:"theme,omitempty"`
}

// LineHighlight specifies highlighted line ranges in code blocks
type LineHighlight struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Type      string `json:"type"` // "error", "warning", "info", "highlight"
	Message   string `json:"message,omitempty"`
}

// DiffInfo provides diff rendering information for code content
type DiffInfo struct {
	OldFile string         `json:"oldFile,omitempty"`
	NewFile string         `json:"newFile,omitempty"`
	Hunks   []DiffHunk     `json:"hunks"`
	Stats   DiffStatistics `json:"stats"`
}

// DiffHunk represents a section of differences in a diff
type DiffHunk struct {
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Lines    []DiffLine `json:"lines"`
}

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type    string `json:"type"` // "context", "add", "remove"
	Content string `json:"content"`
	LineNo  int    `json:"lineNo,omitempty"`
}

// DiffStatistics provides summary information about a diff
type DiffStatistics struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Changes   int `json:"changes"`
}

// FoldingRegion defines collapsible regions within code blocks
type FoldingRegion struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Label     string `json:"label,omitempty"`
	Collapsed bool   `json:"collapsed"`
}

// CodeAnnotation provides additional information for code lines
type CodeAnnotation struct {
	Line    int    `json:"line"`
	Type    string `json:"type"` // "error", "warning", "info", "hint"
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// ProgressContent represents progress indicators with completion status
type ProgressContent struct {
	Label         string            `json:"label"`
	Progress      int               `json:"progress"` // 0-100
	Status        string            `json:"status"`   // "running", "complete", "error", "paused"
	Message       string            `json:"message,omitempty"`
	Indeterminate bool              `json:"indeterminate"` // For unknown duration operations
	ShowPercent   bool              `json:"showPercent"`
	ShowETA       bool              `json:"showETA"`
	Details       ProgressDetails   `json:"details,omitempty"`
	Animation     ProgressAnimation `json:"animation"`
}

// ProgressDetails provides detailed progress information
type ProgressDetails struct {
	Current    int64         `json:"current"`
	Total      int64         `json:"total"`
	Rate       float64       `json:"rate,omitempty"`       // Items per second
	ETA        time.Duration `json:"eta,omitempty"`        // Estimated time remaining
	Elapsed    time.Duration `json:"elapsed"`              // Time elapsed
	Units      string        `json:"units,omitempty"`      // "bytes", "items", "files", etc.
	Throughput string        `json:"throughput,omitempty"` // Human-readable rate
}

// ProgressAnimation defines progress bar animation characteristics
type ProgressAnimation struct {
	Style     string        `json:"style"`     // "smooth", "stepped", "pulse"
	Speed     time.Duration `json:"speed"`     // Animation update interval
	Direction string        `json:"direction"` // "forward", "reverse", "bounce"
	Enabled   bool          `json:"enabled"`
}

// ListContent represents ordered or unordered lists with nesting support
type ListContent struct {
	Items    []ListItem `json:"items"`
	Ordered  bool       `json:"ordered"`
	Style    string     `json:"style,omitempty"`    // "bullet", "number", "alpha", "roman"
	Nested   bool       `json:"nested"`             // Indicates if list contains nested items
	Compact  bool       `json:"compact"`            // Compact rendering style
	MaxDepth int        `json:"maxDepth,omitempty"` // Maximum nesting depth
}

// ListItem represents individual items within lists
type ListItem struct {
	Text     string      `json:"text"`
	Level    int         `json:"level"` // Nesting level
	Children []ListItem  `json:"children,omitempty"`
	Status   string      `json:"status,omitempty"` // "complete", "pending", "error"
	Icon     string      `json:"icon,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// TreeContent represents hierarchical tree structures
type TreeContent struct {
	Root    TreeNode    `json:"root"`
	Options TreeOptions `json:"options"`
	State   TreeState   `json:"state"`
}

// TreeNode represents individual nodes in tree structures
type TreeNode struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Icon       string            `json:"icon,omitempty"`
	Children   []TreeNode        `json:"children,omitempty"`
	Expanded   bool              `json:"expanded"`
	Selectable bool              `json:"selectable"`
	Selected   bool              `json:"selected"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Level      int               `json:"level"`
	IsLeaf     bool              `json:"isLeaf"`
}

// TreeOptions defines tree rendering options
type TreeOptions struct {
	ShowRoot    bool   `json:"showRoot"`
	ShowIcons   bool   `json:"showIcons"`
	ShowLines   bool   `json:"showLines"`
	IndentSize  int    `json:"indentSize"`
	ExpandAll   bool   `json:"expandAll"`
	CollapseAll bool   `json:"collapseAll"`
	SelectMode  string `json:"selectMode"` // "single", "multiple", "none"
}

// TreeState tracks the state of tree interactions
type TreeState struct {
	ExpandedNodes []string `json:"expandedNodes"`
	SelectedNodes []string `json:"selectedNodes"`
	FocusedNode   string   `json:"focusedNode,omitempty"`
	ScrollOffset  int      `json:"scrollOffset"`
}

// SeparatorContent represents visual dividers between content sections
type SeparatorContent struct {
	Style     string `json:"style"`           // "line", "space", "dots", "stars"
	Length    int    `json:"length"`          // Length in characters
	Character string `json:"character"`       // Custom separator character
	Centered  bool   `json:"centered"`        // Center the separator
	Label     string `json:"label,omitempty"` // Optional label within separator
}

// StatusContent represents status indicators with icons and colors
type StatusContent struct {
	Status    string    `json:"status"` // "success", "error", "warning", "info", "pending"
	Message   string    `json:"message"`
	Icon      string    `json:"icon,omitempty"`
	Details   string    `json:"details,omitempty"`
	Code      string    `json:"code,omitempty"` // Error/status code
	Timestamp time.Time `json:"timestamp,omitempty"`
	Severity  string    `json:"severity,omitempty"` // "low", "medium", "high", "critical"
}

// RenderingContext provides context information for content rendering operations
type RenderingContext struct {
	Theme          *interfaces.Theme    `json:"theme"`
	TerminalWidth  int                  `json:"terminalWidth"`
	TerminalHeight int                  `json:"terminalHeight"`
	ColorSupport   bool                 `json:"colorSupport"`
	UnicodeSupport bool                 `json:"unicodeSupport"`
	FocusedElement string               `json:"focusedElement,omitempty"`
	ViewportOffset int                  `json:"viewportOffset"`
	ScrollPosition int                  `json:"scrollPosition"`
	RenderMode     string               `json:"renderMode"` // "full", "partial", "minimal"
	Preferences    RenderingPreferences `json:"preferences"`
}

// RenderingPreferences defines user preferences for content rendering
type RenderingPreferences struct {
	ShowLineNumbers   bool   `json:"showLineNumbers"`
	ShowIcons         bool   `json:"showIcons"`
	CompactMode       bool   `json:"compactMode"`
	AnimationsEnabled bool   `json:"animationsEnabled"`
	HighContrastMode  bool   `json:"highContrastMode"`
	MaxTableRows      int    `json:"maxTableRows"`
	CodeTheme         string `json:"codeTheme"`
	DateFormat        string `json:"dateFormat"`
	TimeFormat        string `json:"timeFormat"`
}

// ContentMetrics provides information about rendered content for layout optimization
type ContentMetrics struct {
	TotalLines       int            `json:"totalLines"`
	MaxLineLength    int            `json:"maxLineLength"`
	FocusableCount   int            `json:"focusableCount"`
	CollapsibleCount int            `json:"collapsibleCount"`
	ElementCounts    map[string]int `json:"elementCounts"`
	RenderTime       time.Duration  `json:"renderTime"`
	MemoryUsage      int64          `json:"memoryUsage"`
	ComplexityScore  int            `json:"complexityScore"`
}

// ValidationResult represents the result of content validation operations
type ValidationResult struct {
	Valid     bool                `json:"valid"`
	Errors    []ValidationError   `json:"errors,omitempty"`
	Warnings  []ValidationWarning `json:"warnings,omitempty"`
	Context   string              `json:"context,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
}

// ValidationError represents errors found during content validation
type ValidationError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`       // JSONPath to problematic content
	Line       int    `json:"line,omitempty"`       // Line number if applicable
	Column     int    `json:"column,omitempty"`     // Column number if applicable
	Severity   string `json:"severity"`             // "error", "warning", "info"
	Suggestion string `json:"suggestion,omitempty"` // Suggested fix
}

// ValidationWarning represents warnings found during content validation
type ValidationWarning struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Helper functions for content type creation and manipulation

// NewRenderableContent creates a new renderable content instance with default values
func NewRenderableContent(contentType, text string) *RenderableContent {
	return &RenderableContent{
		ID:        generateContentID(),
		Type:      contentType,
		Text:      text,
		Focusable: false,
		StyleHints: StyleHints{
			Alignment: "left",
			Borders:   BorderStyle{Enabled: false},
		},
		Positioning: Positioning{
			Overflow: "auto",
		},
		Metadata: make(map[string]interface{}),
	}
}

// NewCollapsibleContent creates a new collapsible content instance
func NewCollapsibleContent(title string, collapsed bool) *CollapsibleContent {
	return &CollapsibleContent{
		Title:     title,
		Collapsed: collapsed,
		Content:   make([]interfaces.ContentBlock, 0),
		Level:     0,
		Expanded:  !collapsed,
		ToggleState: CollapsibleState{
			ID:          generateContentID(),
			Expanded:    !collapsed,
			LastToggled: time.Now(),
			ToggleCount: 0,
			HasChildren: false,
			FocusIndex:  -1,
		},
	}
}

// NewTableContent creates a new table content instance with headers
func NewTableContent(headers []string) *TableContent {
	return &TableContent{
		Headers:   headers,
		Rows:      make([][]string, 0),
		Alignment: make([]string, len(headers)),
		Zebra:     true,
		Borders:   true,
		Metadata: TableMetadata{
			TotalRows: 0,
		},
	}
}

// NewProgressContent creates a new progress content instance
func NewProgressContent(label string, progress int) *ProgressContent {
	return &ProgressContent{
		Label:       label,
		Progress:    progress,
		Status:      "running",
		ShowPercent: true,
		ShowETA:     false,
		Details: ProgressDetails{
			Current: 0,
			Total:   100,
			Elapsed: 0,
		},
		Animation: ProgressAnimation{
			Style:     "smooth",
			Speed:     100 * time.Millisecond,
			Direction: "forward",
			Enabled:   true,
		},
	}
}

// generateContentID creates a unique identifier for content elements
func generateContentID() string {
	return fmt.Sprintf("content_%d", time.Now().UnixNano())
}
