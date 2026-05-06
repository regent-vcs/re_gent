package cli

import (
	"strings"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
)

// GraphNode represents a step in the graph with its relationships
type GraphNode struct {
	StepHash  store.Hash
	Parents   []store.Hash // [primary, secondary if exists]
	Children  []store.Hash // Derived from parent pointers
	Column    int          // Display column (0-based)
	SessionID string
}

// GraphLayout holds the column assignments for rendering
type GraphLayout struct {
	Nodes      []*GraphNode
	ColumnMap  map[store.Hash]int // stepHash → column
	MaxColumns int
}

// RenderGraph generates ASCII graph prefixes for each step
// Returns a slice of prefixes, one per step, in the same order as input
func RenderGraph(steps []index.StepInfo, s *store.Store) ([]string, error) {
	if len(steps) == 0 {
		return []string{}, nil
	}

	// Build graph nodes with parent/child relationships
	nodes, err := BuildStepGraph(s, steps)
	if err != nil {
		return nil, err
	}

	// Assign columns to nodes
	layout := LayoutGraph(nodes)

	// Render ASCII art for each node
	prefixes := make([]string, len(nodes))
	for i, node := range nodes {
		var nextNode *GraphNode
		if i < len(nodes)-1 {
			nextNode = nodes[i+1]
		}
		prefixes[i] = RenderGraphLine(node, layout, nextNode)
	}

	return prefixes, nil
}

// BuildStepGraph constructs parent→children relationships from steps
func BuildStepGraph(s *store.Store, steps []index.StepInfo) ([]*GraphNode, error) {
	nodes := make([]*GraphNode, len(steps))
	nodeMap := make(map[store.Hash]*GraphNode)

	// First pass: create nodes
	for i, stepInfo := range steps {
		node := &GraphNode{
			StepHash:  stepInfo.Hash,
			SessionID: stepInfo.SessionID,
			Children:  []store.Hash{},
		}

		// Read full step to get parent pointers
		step, err := s.ReadStep(stepInfo.Hash)
		if err != nil {
			// Skip if we can't read, treat as root
			nodes[i] = node
			nodeMap[stepInfo.Hash] = node
			continue
		}

		// Collect parents
		if step.Parent != "" {
			node.Parents = append(node.Parents, step.Parent)
		}
		if step.SecondaryParent != "" {
			node.Parents = append(node.Parents, step.SecondaryParent)
		}

		nodes[i] = node
		nodeMap[stepInfo.Hash] = node
	}

	// Second pass: build children relationships
	for _, node := range nodes {
		for _, parentHash := range node.Parents {
			if parent, ok := nodeMap[parentHash]; ok {
				parent.Children = append(parent.Children, node.StepHash)
			}
		}
	}

	return nodes, nil
}

// LayoutGraph assigns columns to each node using a topological layout
func LayoutGraph(nodes []*GraphNode) *GraphLayout {
	layout := &GraphLayout{
		Nodes:     nodes,
		ColumnMap: make(map[store.Hash]int),
	}

	// Track active columns (each holds a hash that continues in that column)
	currentColumns := []store.Hash{}

	for _, node := range nodes {
		// Find if this node is already in a column (as a continuation)
		column := -1
		for i, hash := range currentColumns {
			if hash == node.StepHash {
				column = i
				break
			}
		}

		// If not found, allocate a new column
		if column == -1 {
			column = len(currentColumns)
			currentColumns = append(currentColumns, node.StepHash)
		}

		node.Column = column
		layout.ColumnMap[node.StepHash] = column

		// Update column to continue with primary parent
		if len(node.Parents) > 0 {
			currentColumns[column] = node.Parents[0]
		} else {
			// Root node - clear this column
			currentColumns[column] = ""
		}

		// Handle secondary parent (merge)
		if len(node.Parents) > 1 {
			// Find or allocate column for secondary parent
			found := false
			for _, hash := range currentColumns {
				if hash == node.Parents[1] {
					found = true
					break
				}
			}
			if !found {
				currentColumns = append(currentColumns, node.Parents[1])
			}
		}

		// Compact columns (remove empties from the end)
		for len(currentColumns) > 0 && currentColumns[len(currentColumns)-1] == "" {
			currentColumns = currentColumns[:len(currentColumns)-1]
		}
	}

	layout.MaxColumns = len(currentColumns)
	if layout.MaxColumns == 0 {
		layout.MaxColumns = 1
	}

	return layout
}

// RenderGraphLine generates the ASCII art prefix for a single step
func RenderGraphLine(node *GraphNode, layout *GraphLayout, nextNode *GraphNode) string {
	if layout.MaxColumns == 1 && len(node.Parents) <= 1 {
		// Simple linear case - just commit marker and vertical line
		return style.DimText("* ")
	}

	var line strings.Builder

	// Build the line with columns
	for col := 0; col < layout.MaxColumns; col++ {
		if col == node.Column {
			// This is the commit column
			if len(node.Parents) > 1 {
				line.WriteString(style.DimText("○ ")) // Merge commit
			} else {
				line.WriteString(style.DimText("* ")) // Regular commit
			}
		} else if col < node.Column {
			// Column to the left
			line.WriteString(style.DimText("│ "))
		} else {
			// Column to the right
			line.WriteString(style.DimText("  "))
		}
	}

	return line.String()
}
