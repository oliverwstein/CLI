// Package content implements sophisticated content rendering for the Universal Application Console.
// This file provides the concrete implementation of the ContentRenderer interface that transforms
// structured content responses into Lipgloss-styled components as specified in section 3.3
// of the design document.
package content

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/universal-console/console/internal/interfaces"
)

// Renderer implements the ContentRenderer interface with comprehensive content processing capabilities
type Renderer struct {
	collapsibleManager *CollapsibleManager
	syntaxHighlighter  *SyntaxHighlighter
	themeManager       *ThemeManager
	cache              *RenderCache
	mutex              sync.RWMutex
	preferences        RenderingPreferences
	metrics            ContentMetrics
}

// RenderCache provides intelligent caching of rendered content for performance optimization
type RenderCache struct {
	renderedContent map[string]string
	contentHashes   map[string]string
	lastAccessed    map[string]time.Time
	mutex           sync.RWMutex
	maxSize         int
	ttl             time.Duration
}

// SyntaxHighlighter provides code syntax highlighting capabilities using Chroma
type SyntaxHighlighter struct {
	formatter chroma.Formatter
	style     *chroma.Style
	theme     string
}

// ThemeManager handles theme-specific styling and color management
type ThemeManager struct {
	currentTheme   *interfaces.Theme
	lipglossStyles map[string]lipgloss.Style
	colorPalette   map[string]string
	darkMode       bool
	highContrast   bool
}

// NewRenderer creates a new content renderer with comprehensive rendering capabilities
func NewRenderer() (*Renderer, error) {
	// Initialize collapsible content manager
	collapsibleManager := NewCollapsibleManager()

	// Initialize syntax highlighter with default settings
	highlighter, err := NewSyntaxHighlighter("github", "terminal256")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize syntax highlighter: %w", err)
	}

	// Initialize theme manager with default theme
	themeManager := NewThemeManager()

	// Initialize render cache with reasonable defaults
	cache := &RenderCache{
		renderedContent: make(map[string]string),
		contentHashes:   make(map[string]string),
		lastAccessed:    make(map[string]time.Time),
		maxSize:         1000,
		ttl:             15 * time.Minute,
	}

	// Set default rendering preferences
	preferences := RenderingPreferences{
		ShowLineNumbers:   true,
		ShowIcons:         true,
		CompactMode:       false,
		AnimationsEnabled: true,
		HighContrastMode:  false,
		MaxTableRows:      50,
		CodeTheme:         "github",
		DateFormat:        "2006-01-02",
		TimeFormat:        "15:04:05",
	}

	renderer := &Renderer{
		collapsibleManager: collapsibleManager,
		syntaxHighlighter:  highlighter,
		themeManager:       themeManager,
		cache:              cache,
		preferences:        preferences,
		metrics: ContentMetrics{
			ElementCounts: make(map[string]int),
		},
	}

	return renderer, nil
}

// RenderContent transforms structured content into display-ready format
func (r *Renderer) RenderContent(content interface{}, theme *interfaces.Theme) ([]interfaces.RenderedContent, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		r.metrics.RenderTime = time.Since(startTime)
	}()

	// Update theme if provided
	if theme != nil {
		r.themeManager.SetTheme(theme)
	}

	// Parse content structure
	contentBlocks, err := r.parseContentStructure(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content structure: %w", err)
	}

	// Render each content block
	var renderedBlocks []interfaces.RenderedContent
	for i, block := range contentBlocks {
		rendered, err := r.renderContentBlock(block, i)
		if err != nil {
			return nil, fmt.Errorf("failed to render content block %d: %w", i, err)
		}
		renderedBlocks = append(renderedBlocks, rendered...)
	}

	// Update metrics
	r.updateRenderingMetrics(renderedBlocks)

	return renderedBlocks, nil
}

// RenderActions formats actions for the Actions Pane
func (r *Renderer) RenderActions(actions []interfaces.Action, theme *interfaces.Theme) (string, error) {
	if len(actions) == 0 {
		return "", nil
	}

	// Update theme if provided
	if theme != nil {
		r.themeManager.SetTheme(theme)
	}

	// Create actions pane styling
	var actionLines []string

	for i, action := range actions {
		actionStyle := r.getActionStyle(action.Type)

		// Format action with number and icon
		actionText := fmt.Sprintf("[%d] %s %s", i+1, action.Icon, action.Name)
		styledAction := actionStyle.Render(actionText)

		actionLines = append(actionLines, styledAction)
	}

	// Create bordered actions pane
	actionsContent := strings.Join(actionLines, "\n")
	actionsPane := r.themeManager.GetBorderStyle("actions").
		Render(actionsContent)

	return actionsPane, nil
}

// RenderError formats error responses with recovery options
func (r *Renderer) RenderError(errorResp *interfaces.ErrorResponse, theme *interfaces.Theme) (string, error) {
	if errorResp == nil {
		return "", fmt.Errorf("error response cannot be nil")
	}

	// Update theme if provided
	if theme != nil {
		r.themeManager.SetTheme(theme)
	}

	// Create error styling
	errorStyle := r.themeManager.GetErrorStyle()

	// Render main error message
	errorHeader := errorStyle.Render(fmt.Sprintf("❌ Error: %s", errorResp.Error.Message))

	var errorComponents []string
	errorComponents = append(errorComponents, errorHeader)

	// Add error code if present
	if errorResp.Error.Code != "" {
		codeText := fmt.Sprintf("Code: %s", errorResp.Error.Code)
		errorComponents = append(errorComponents, r.themeManager.GetInfoStyle().Render(codeText))
	}

	// Render error details if present
	if errorResp.Error.Details != nil {
		detailsRendered, err := r.renderContentBlock(*errorResp.Error.Details, 0)
		if err == nil && len(detailsRendered) > 0 {
			errorComponents = append(errorComponents, detailsRendered[0].Text)
		}
	}

	// Render recovery actions if present
	if len(errorResp.Error.RecoveryActions) > 0 {
		recoveryActions, err := r.RenderActions(errorResp.Error.RecoveryActions, theme)
		if err == nil {
			errorComponents = append(errorComponents, "\nRecovery Options:")
			errorComponents = append(errorComponents, recoveryActions)
		}
	}

	return strings.Join(errorComponents, "\n"), nil
}

// RenderProgress formats progress indicators
func (r *Renderer) RenderProgress(progress *interfaces.ProgressResponse, theme *interfaces.Theme) (string, error) {
	if progress == nil {
		return "", fmt.Errorf("progress response cannot be nil")
	}

	// Update theme if provided
	if theme != nil {
		r.themeManager.SetTheme(theme)
	}

	// Create progress content
	progressContent := &ProgressContent{
		Label:    progress.Message,
		Progress: progress.Progress,
		Status:   progress.Status,
		Message:  fmt.Sprintf("%d/%d completed", progress.Details.Completed, progress.Details.Total),
		Details: ProgressDetails{
			Current: int64(progress.Details.Completed),
			Total:   int64(progress.Details.Total),
			Units:   "items",
		},
	}

	return r.renderProgressBar(progressContent), nil
}

// RenderWorkflow formats workflow breadcrumbs
func (r *Renderer) RenderWorkflow(workflow *interfaces.Workflow, theme *interfaces.Theme) (string, error) {
	if workflow == nil {
		return "", nil
	}

	// Update theme if provided
	if theme != nil {
		r.themeManager.SetTheme(theme)
	}

	// Create workflow breadcrumb
	breadcrumb := fmt.Sprintf("%s (%d/%d)", workflow.Title, workflow.Step, workflow.TotalSteps)

	// Create progress indicator for workflow steps
	progressBar := r.createWorkflowProgressBar(workflow)

	workflowStyle := r.themeManager.GetWorkflowStyle()
	return workflowStyle.Render(breadcrumb + "\n" + progressBar), nil
}

// Content parsing and structure analysis

// parseContentStructure analyzes and parses the content structure
func (r *Renderer) parseContentStructure(content interface{}) ([]interfaces.ContentBlock, error) {
	switch v := content.(type) {
	case string:
		// Simple text content
		return []interfaces.ContentBlock{{
			Type:    "text",
			Content: v,
		}}, nil
	case []interface{}:
		// Array of content blocks
		var blocks []interfaces.ContentBlock
		for _, item := range v {
			if blockData, err := r.parseContentItem(item); err == nil {
				blocks = append(blocks, blockData)
			}
		}
		return blocks, nil
	case map[string]interface{}:
		// Single content block
		block, err := r.parseContentItem(v)
		if err != nil {
			return nil, err
		}
		return []interfaces.ContentBlock{block}, nil
	default:
		return nil, fmt.Errorf("unsupported content type: %T", content)
	}
}

// parseContentItem converts interface{} to ContentBlock
func (r *Renderer) parseContentItem(item interface{}) (interfaces.ContentBlock, error) {
	// Convert to JSON and back to ensure proper structure
	jsonData, err := json.Marshal(item)
	if err != nil {
		return interfaces.ContentBlock{}, fmt.Errorf("failed to marshal content item: %w", err)
	}

	var block interfaces.ContentBlock
	if err := json.Unmarshal(jsonData, &block); err != nil {
		return interfaces.ContentBlock{}, fmt.Errorf("failed to unmarshal content block: %w", err)
	}

	return block, nil
}

// Content rendering methods for specific types

// renderContentBlock renders a single content block based on its type
func (r *Renderer) renderContentBlock(block interfaces.ContentBlock, index int) ([]interfaces.RenderedContent, error) {
	switch block.Type {
	case "text":
		return r.renderTextContent(block)
	case "code":
		return r.renderCodeContent(block)
	case "table":
		return r.renderTableContent(block)
	case "collapsible":
		return r.renderCollapsibleContent(block)
	case "progress":
		return r.renderProgressContent(block)
	case "list":
		return r.renderListContent(block)
	case "tree":
		return r.renderTreeContent(block)
	case "separator":
		return r.renderSeparatorContent(block)
	default:
		// Fallback to text rendering for unknown types
		return r.renderTextContent(block)
	}
}

// renderTextContent handles plain text content with status indicators
func (r *Renderer) renderTextContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	content := interfaces.RenderedContent{
		Text:      fmt.Sprintf("%v", block.Content),
		Focusable: false,
	}

	// Apply status styling if present
	if block.Status != "" {
		statusStyle := r.themeManager.GetStatusStyle(block.Status)
		content.Text = statusStyle.Render(content.Text)
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderCodeContent handles syntax-highlighted code blocks
func (r *Renderer) renderCodeContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var codeContent CodeContent

	// Parse code block structure
	if err := r.parseBlockContent(block.Content, &codeContent); err != nil {
		return nil, fmt.Errorf("failed to parse code content: %w", err)
	}

	// Apply syntax highlighting
	highlightedCode, err := r.syntaxHighlighter.Highlight(codeContent.Code, codeContent.Language)
	if err != nil {
		// Fallback to plain text if highlighting fails
		highlightedCode = codeContent.Code
	}

	// Add line numbers if requested
	if codeContent.LineNumbers && r.preferences.ShowLineNumbers {
		highlightedCode = r.addLineNumbers(highlightedCode)
	}

	// Create bordered code block
	codeStyle := r.themeManager.GetCodeStyle()
	renderedCode := codeStyle.Render(highlightedCode)

	content := interfaces.RenderedContent{
		Text:      renderedCode,
		Focusable: false,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderTableContent handles tabular data formatting
func (r *Renderer) renderTableContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var tableContent TableContent

	// Parse table structure
	if err := r.parseBlockContent(block.Content, &tableContent); err != nil {
		return nil, fmt.Errorf("failed to parse table content: %w", err)
	}

	// Render table with proper formatting
	tableText := r.formatTable(&tableContent)

	content := interfaces.RenderedContent{
		Text:      tableText,
		Focusable: false,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderCollapsibleContent handles expandable content sections
func (r *Renderer) renderCollapsibleContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var collapsibleContent CollapsibleContent

	// Parse collapsible structure
	if err := r.parseBlockContent(block.Content, &collapsibleContent); err != nil {
		return nil, fmt.Errorf("failed to parse collapsible content: %w", err)
	}

	contentID := generateContentID()

	// Register with collapsible manager
	r.collapsibleManager.RegisterSection(contentID, &collapsibleContent)

	// Create header with toggle indicator
	toggleIcon := "▶"
	if collapsibleContent.Expanded {
		toggleIcon = "▼"
	}

	headerText := fmt.Sprintf("%s %s", toggleIcon, collapsibleContent.Title)
	headerStyle := r.themeManager.GetCollapsibleHeaderStyle()

	var result []interfaces.RenderedContent

	// Add collapsible header
	header := interfaces.RenderedContent{
		Text:      headerStyle.Render(headerText),
		Focusable: true,
		Expanded:  &collapsibleContent.Expanded,
		ID:        contentID,
	}
	result = append(result, header)

	// Add content if expanded
	if collapsibleContent.Expanded {
		for _, childBlock := range collapsibleContent.Content {
			childRendered, err := r.renderContentBlock(childBlock, 0)
			if err == nil {
				result = append(result, childRendered...)
			}
		}
	}

	return result, nil
}

// renderProgressContent handles progress indicators
func (r *Renderer) renderProgressContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var progressContent ProgressContent

	if err := r.parseBlockContent(block.Content, &progressContent); err != nil {
		return nil, fmt.Errorf("failed to parse progress content: %w", err)
	}

	progressText := r.renderProgressBar(&progressContent)

	content := interfaces.RenderedContent{
		Text:      progressText,
		Focusable: false,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderListContent handles ordered and unordered lists
func (r *Renderer) renderListContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var listContent ListContent

	if err := r.parseBlockContent(block.Content, &listContent); err != nil {
		return nil, fmt.Errorf("failed to parse list content: %w", err)
	}

	listText := r.formatList(&listContent)

	content := interfaces.RenderedContent{
		Text:      listText,
		Focusable: false,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderTreeContent handles hierarchical tree structures
func (r *Renderer) renderTreeContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var treeContent TreeContent

	if err := r.parseBlockContent(block.Content, &treeContent); err != nil {
		return nil, fmt.Errorf("failed to parse tree content: %w", err)
	}

	treeText := r.formatTree(&treeContent)

	content := interfaces.RenderedContent{
		Text:      treeText,
		Focusable: true,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// renderSeparatorContent handles visual dividers
func (r *Renderer) renderSeparatorContent(block interfaces.ContentBlock) ([]interfaces.RenderedContent, error) {
	var separatorContent SeparatorContent

	if err := r.parseBlockContent(block.Content, &separatorContent); err != nil {
		return nil, fmt.Errorf("failed to parse separator content: %w", err)
	}

	separatorText := r.formatSeparator(&separatorContent)

	content := interfaces.RenderedContent{
		Text:      separatorText,
		Focusable: false,
		ID:        generateContentID(),
	}

	return []interfaces.RenderedContent{content}, nil
}

// Helper methods for formatting and styling

// parseBlockContent converts interface{} content to specific content type
func (r *Renderer) parseBlockContent(content interface{}, target interface{}) error {
	jsonData, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal content: %w", err)
	}

	return nil
}

// formatTable creates formatted table output
func (r *Renderer) formatTable(table *TableContent) string {
	if len(table.Headers) == 0 {
		return ""
	}

	// Calculate column widths
	columnWidths := r.calculateColumnWidths(table)

	var lines []string

	// Create header
	headerLine := r.formatTableRow(table.Headers, columnWidths, true)
	lines = append(lines, headerLine)

	// Create separator
	separatorLine := r.createTableSeparator(columnWidths)
	lines = append(lines, separatorLine)

	// Create data rows
	maxRows := r.preferences.MaxTableRows
	for i, row := range table.Rows {
		if i >= maxRows {
			lines = append(lines, fmt.Sprintf("... and %d more rows", len(table.Rows)-maxRows))
			break
		}
		rowLine := r.formatTableRow(row, columnWidths, false)
		lines = append(lines, rowLine)
	}

	return strings.Join(lines, "\n")
}

// calculateColumnWidths determines optimal column widths for tables
func (r *Renderer) calculateColumnWidths(table *TableContent) []int {
	widths := make([]int, len(table.Headers))

	// Initialize with header widths
	for i, header := range table.Headers {
		widths[i] = len(header)
	}

	// Check data row widths
	for _, row := range table.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Apply minimum and maximum width constraints
	for i := range widths {
		if widths[i] < 8 {
			widths[i] = 8
		}
		if widths[i] > 40 {
			widths[i] = 40
		}
	}

	return widths
}

// formatTableRow creates a formatted table row
func (r *Renderer) formatTableRow(cells []string, widths []int, isHeader bool) string {
	var formattedCells []string

	for i, cell := range cells {
		width := widths[i]
		if i < len(widths) {
			// Truncate if too long
			if len(cell) > width {
				cell = cell[:width-3] + "..."
			}
			// Pad to width
			formatted := fmt.Sprintf("%-*s", width, cell)
			if isHeader {
				formatted = r.themeManager.GetTableHeaderStyle().Render(formatted)
			}
			formattedCells = append(formattedCells, formatted)
		}
	}

	return "│ " + strings.Join(formattedCells, " │ ") + " │"
}

// createTableSeparator creates table separator lines
func (r *Renderer) createTableSeparator(widths []int) string {
	var parts []string
	for _, width := range widths {
		parts = append(parts, strings.Repeat("─", width))
	}
	return "├─" + strings.Join(parts, "─┼─") + "─┤"
}

// formatList creates formatted list output
func (r *Renderer) formatList(list *ListContent) string {
	var lines []string

	for i, item := range list.Items {
		marker := r.getListMarker(list, i, item.Level)
		indent := strings.Repeat("  ", item.Level)
		line := fmt.Sprintf("%s%s %s", indent, marker, item.Text)

		if item.Status != "" {
			statusStyle := r.themeManager.GetStatusStyle(item.Status)
			line = statusStyle.Render(line)
		}

		lines = append(lines, line)

		// Render nested items
		if len(item.Children) > 0 {
			nestedList := &ListContent{
				Items:   item.Children,
				Ordered: list.Ordered,
				Style:   list.Style,
			}
			nestedText := r.formatList(nestedList)
			lines = append(lines, nestedText)
		}
	}

	return strings.Join(lines, "\n")
}

// getListMarker returns appropriate list markers based on style and order
func (r *Renderer) getListMarker(list *ListContent, index, level int) string {
	if list.Ordered {
		return fmt.Sprintf("%d.", index+1)
	}

	markers := []string{"•", "◦", "▪", "▫"}
	return markers[level%len(markers)]
}

// formatTree creates formatted tree output
func (r *Renderer) formatTree(tree *TreeContent) string {
	return r.formatTreeNode(&tree.Root, "", true, &tree.Options)
}

// formatTreeNode recursively formats tree nodes
func (r *Renderer) formatTreeNode(node *TreeNode, prefix string, isLast bool, options *TreeOptions) string {
	var lines []string

	// Create node line
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	icon := ""
	if options.ShowIcons && node.Icon != "" {
		icon = node.Icon + " "
	}

	nodeLine := prefix + connector + icon + node.Label
	lines = append(lines, nodeLine)

	// Process children if expanded
	if node.Expanded && len(node.Children) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		for i, child := range node.Children {
			isLastChild := i == len(node.Children)-1
			childLines := r.formatTreeNode(&child, childPrefix, isLastChild, options)
			lines = append(lines, childLines)
		}
	}

	return strings.Join(lines, "\n")
}

// formatSeparator creates visual separators
func (r *Renderer) formatSeparator(separator *SeparatorContent) string {
	char := separator.Character
	if char == "" {
		switch separator.Style {
		case "line":
			char = "─"
		case "dots":
			char = "·"
		case "stars":
			char = "*"
		default:
			char = " "
		}
	}

	length := separator.Length
	if length <= 0 {
		length = 50
	}

	line := strings.Repeat(char, length)

	if separator.Label != "" {
		labelPos := (length - len(separator.Label)) / 2
		if labelPos > 0 {
			line = line[:labelPos] + separator.Label + line[labelPos+len(separator.Label):]
		}
	}

	return line
}

// renderProgressBar creates visual progress indicators
func (r *Renderer) renderProgressBar(progress *ProgressContent) string {
	barWidth := 40
	filledWidth := int(float64(barWidth) * float64(progress.Progress) / 100.0)

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", barWidth-filledWidth)

	progressBar := fmt.Sprintf("[%s%s] %d%%", filled, empty, progress.Progress)

	if progress.Label != "" {
		progressBar = progress.Label + ": " + progressBar
	}

	if progress.Status != "" {
		statusStyle := r.themeManager.GetStatusStyle(progress.Status)
		progressBar = statusStyle.Render(progressBar)
	}

	return progressBar
}

// createWorkflowProgressBar creates progress indicators for workflows
func (r *Renderer) createWorkflowProgressBar(workflow *interfaces.Workflow) string {
	totalSteps := workflow.TotalSteps
	currentStep := workflow.Step

	var steps []string
	for i := 1; i <= totalSteps; i++ {
		if i < currentStep {
			steps = append(steps, "●")
		} else if i == currentStep {
			steps = append(steps, "◉")
		} else {
			steps = append(steps, "○")
		}
	}

	return strings.Join(steps, "─")
}

// addLineNumbers adds line numbers to code blocks
func (r *Renderer) addLineNumbers(code string) string {
	lines := strings.Split(code, "\n")
	var numberedLines []string

	for i, line := range lines {
		lineNumber := fmt.Sprintf("%3d │ ", i+1)
		numberedLines = append(numberedLines, lineNumber+line)
	}

	return strings.Join(numberedLines, "\n")
}

// getActionStyle returns appropriate styling for different action types
func (r *Renderer) getActionStyle(actionType string) lipgloss.Style {
	switch actionType {
	case "confirmation":
		return r.themeManager.GetConfirmationStyle()
	case "cancel":
		return r.themeManager.GetCancelStyle()
	case "info":
		return r.themeManager.GetInfoStyle()
	case "alternative":
		return r.themeManager.GetAlternativeStyle()
	default:
		return r.themeManager.GetPrimaryStyle()
	}
}

// updateRenderingMetrics updates rendering performance metrics
func (r *Renderer) updateRenderingMetrics(rendered []interfaces.RenderedContent) {
	r.metrics.TotalLines = 0
	r.metrics.MaxLineLength = 0
	r.metrics.FocusableCount = 0
	r.metrics.CollapsibleCount = 0

	for _, content := range rendered {
		lines := strings.Split(content.Text, "\n")
		r.metrics.TotalLines += len(lines)

		for _, line := range lines {
			if len(line) > r.metrics.MaxLineLength {
				r.metrics.MaxLineLength = len(line)
			}
		}

		if content.Focusable {
			r.metrics.FocusableCount++
		}

		if content.Expanded != nil {
			r.metrics.CollapsibleCount++
		}
	}
}

// NewSyntaxHighlighter creates a new syntax highlighter with specified theme and format
func NewSyntaxHighlighter(themeName, formatterName string) (*SyntaxHighlighter, error) {
	// Get the formatter
	formatter := formatters.Get(formatterName)
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Get the style
	style := styles.Get(themeName)
	if style == nil {
		style = styles.GitHub
	}

	return &SyntaxHighlighter{
		formatter: formatter,
		style:     style,
		theme:     themeName,
	}, nil
}

// Highlight applies syntax highlighting to code
func (sh *SyntaxHighlighter) Highlight(code, language string) (string, error) {
	// Get lexer for the language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Ensure lexer is configured
	lexer = chroma.Coalesce(lexer)

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code, err
	}

	// Format the tokens
	var highlighted strings.Builder
	err = sh.formatter.Format(&highlighted, sh.style, iterator)
	if err != nil {
		return code, err
	}

	return highlighted.String(), nil
}

// SetTheme updates the syntax highlighting theme
func (sh *SyntaxHighlighter) SetTheme(themeName string) error {
	style := styles.Get(themeName)
	if style == nil {
		return fmt.Errorf("theme '%s' not found", themeName)
	}

	sh.style = style
	sh.theme = themeName
	return nil
}

// NewThemeManager creates a new theme manager with default settings
func NewThemeManager() *ThemeManager {
	tm := &ThemeManager{
		lipglossStyles: make(map[string]lipgloss.Style),
		colorPalette:   make(map[string]string),
		darkMode:       false,
		highContrast:   false,
	}

	tm.initializeDefaultStyles()
	return tm
}

// SetTheme updates the current theme and rebuilds styles
func (tm *ThemeManager) SetTheme(theme *interfaces.Theme) {
	tm.currentTheme = theme
	tm.buildColorPalette()
	tm.buildLipglossStyles()
}

// GetBorderStyle returns a border style for specific contexts
func (tm *ThemeManager) GetBorderStyle(context string) lipgloss.Style {
	if style, exists := tm.lipglossStyles["border_"+context]; exists {
		return style
	}
	return tm.lipglossStyles["border_default"]
}

// GetStatusStyle returns styling for status indicators
func (tm *ThemeManager) GetStatusStyle(status string) lipgloss.Style {
	if style, exists := tm.lipglossStyles["status_"+status]; exists {
		return style
	}
	return tm.lipglossStyles["status_default"]
}

// Style getter methods for different components
func (tm *ThemeManager) GetErrorStyle() lipgloss.Style {
	return tm.lipglossStyles["error"]
}

func (tm *ThemeManager) GetInfoStyle() lipgloss.Style {
	return tm.lipglossStyles["info"]
}

func (tm *ThemeManager) GetCodeStyle() lipgloss.Style {
	return tm.lipglossStyles["code"]
}

func (tm *ThemeManager) GetCollapsibleHeaderStyle() lipgloss.Style {
	return tm.lipglossStyles["collapsible_header"]
}

func (tm *ThemeManager) GetTableHeaderStyle() lipgloss.Style {
	return tm.lipglossStyles["table_header"]
}

func (tm *ThemeManager) GetWorkflowStyle() lipgloss.Style {
	return tm.lipglossStyles["workflow"]
}

func (tm *ThemeManager) GetConfirmationStyle() lipgloss.Style {
	return tm.lipglossStyles["confirmation"]
}

func (tm *ThemeManager) GetCancelStyle() lipgloss.Style {
	return tm.lipglossStyles["cancel"]
}

func (tm *ThemeManager) GetAlternativeStyle() lipgloss.Style {
	return tm.lipglossStyles["alternative"]
}

func (tm *ThemeManager) GetPrimaryStyle() lipgloss.Style {
	return tm.lipglossStyles["primary"]
}

// initializeDefaultStyles creates default Lipgloss styles
func (tm *ThemeManager) initializeDefaultStyles() {
	tm.lipglossStyles = map[string]lipgloss.Style{
		"border_default":     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()),
		"border_actions":     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#888888")),
		"status_default":     lipgloss.NewStyle(),
		"status_success":     lipgloss.NewStyle().Foreground(lipgloss.Color("#28a745")),
		"status_error":       lipgloss.NewStyle().Foreground(lipgloss.Color("#dc3545")),
		"status_warning":     lipgloss.NewStyle().Foreground(lipgloss.Color("#ffc107")),
		"status_info":        lipgloss.NewStyle().Foreground(lipgloss.Color("#17a2b8")),
		"error":              lipgloss.NewStyle().Foreground(lipgloss.Color("#dc3545")).Bold(true),
		"info":               lipgloss.NewStyle().Foreground(lipgloss.Color("#17a2b8")),
		"code":               lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1),
		"collapsible_header": lipgloss.NewStyle().Bold(true),
		"table_header":       lipgloss.NewStyle().Bold(true).Underline(true),
		"workflow":           lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		"confirmation":       lipgloss.NewStyle().Foreground(lipgloss.Color("#28a745")),
		"cancel":             lipgloss.NewStyle().Foreground(lipgloss.Color("#dc3545")),
		"alternative":        lipgloss.NewStyle().Foreground(lipgloss.Color("#6c757d")),
		"primary":            lipgloss.NewStyle().Foreground(lipgloss.Color("#007bff")),
	}
}

// buildColorPalette creates a color palette from the current theme
func (tm *ThemeManager) buildColorPalette() {
	if tm.currentTheme == nil {
		return
	}

	tm.colorPalette = map[string]string{
		"success": tm.currentTheme.Success,
		"error":   tm.currentTheme.Error,
		"warning": tm.currentTheme.Warning,
		"info":    tm.currentTheme.Info,
	}
}

// buildLipglossStyles creates Lipgloss styles based on the current theme
func (tm *ThemeManager) buildLipglossStyles() {
	if tm.currentTheme == nil {
		return
	}

	// Update styles with theme colors
	tm.lipglossStyles["status_success"] = tm.lipglossStyles["status_success"].Foreground(lipgloss.Color(tm.currentTheme.Success))
	tm.lipglossStyles["status_error"] = tm.lipglossStyles["status_error"].Foreground(lipgloss.Color(tm.currentTheme.Error))
	tm.lipglossStyles["status_warning"] = tm.lipglossStyles["status_warning"].Foreground(lipgloss.Color(tm.currentTheme.Warning))
	tm.lipglossStyles["status_info"] = tm.lipglossStyles["status_info"].Foreground(lipgloss.Color(tm.currentTheme.Info))
	tm.lipglossStyles["error"] = tm.lipglossStyles["error"].Foreground(lipgloss.Color(tm.currentTheme.Error))
	tm.lipglossStyles["info"] = tm.lipglossStyles["info"].Foreground(lipgloss.Color(tm.currentTheme.Info))
}

// Interface implementation methods for collapsible management

// ToggleCollapsible expands or collapses a collapsible section
func (r *Renderer) ToggleCollapsible(contentID string) error {
	return r.collapsibleManager.ToggleSection(contentID)
}

// ExpandAll expands all collapsible sections
func (r *Renderer) ExpandAll() error {
	return r.collapsibleManager.ExpandAll()
}

// CollapseAll collapses all collapsible sections
func (r *Renderer) CollapseAll() error {
	return r.collapsibleManager.CollapseAll()
}
