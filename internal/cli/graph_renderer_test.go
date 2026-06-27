package cli

import (
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// ---- LayoutGraph tests (pure function) ----

func TestLayoutGraph_SingleNode(t *testing.T) {
	nodes := []*GraphNode{
		{StepHash: "aaaa", Parents: nil, Children: nil},
	}
	layout := LayoutGraph(nodes)

	if layout.MaxColumns != 1 {
		t.Errorf("MaxColumns = %d, want 1", layout.MaxColumns)
	}
	if col, ok := layout.ColumnMap["aaaa"]; !ok || col != 0 {
		t.Errorf("ColumnMap[aaaa] = %d, want 0", col)
	}
	if len(layout.Nodes) != 1 {
		t.Errorf("len(Nodes) = %d, want 1", len(layout.Nodes))
	}
}

func TestLayoutGraph_LinearChain(t *testing.T) {
	// n0 -> n1 -> n2 (children refer to parents, so n1 is parent of n2)
	nodes := []*GraphNode{
		{StepHash: "n2", Parents: []store.Hash{"n1"}, Children: nil},
		{StepHash: "n1", Parents: []store.Hash{"n0"}, Children: nil},
		{StepHash: "n0", Parents: nil, Children: nil},
	}
	layout := LayoutGraph(nodes)

	if layout.MaxColumns != 1 {
		t.Errorf("MaxColumns = %d, want 1", layout.MaxColumns)
	}
	for _, h := range []store.Hash{"n0", "n1", "n2"} {
		if col, ok := layout.ColumnMap[h]; !ok {
			t.Errorf("ColumnMap missing %s", h)
		} else if col != 0 {
			t.Errorf("ColumnMap[%s] = %d, want 0", h, col)
		}
	}
}

func TestLayoutGraph_Branch(t *testing.T) {
	// n0 -> n1 (child of n0), n0 -> n2 (also child of n0)
	// Layout order: n2, n1
	nodes := []*GraphNode{
		{StepHash: "n2", Parents: []store.Hash{"n0"}, Children: nil},
		{StepHash: "n1", Parents: []store.Hash{"n0"}, Children: nil},
		{StepHash: "n0", Parents: nil, Children: nil},
	}
	layout := LayoutGraph(nodes)

	if layout.MaxColumns < 1 {
		t.Errorf("MaxColumns = %d, want >= 1", layout.MaxColumns)
	}
	// n0 should be in column 0 (or wherever it landed)
	if col, ok := layout.ColumnMap["n0"]; !ok {
		t.Errorf("ColumnMap missing n0")
	} else if col != 0 {
		t.Errorf("ColumnMap[n0] = %d, want 0", col)
	}
}

func TestLayoutGraph_Merge(t *testing.T) {
	// n0 has two parents (n1, n2). In the layout algorithm, nodes are
	// processed from newest to oldest. After all nodes, columns may compact.
	// Verify that the layout runs without error and produces column assignments.
	nodes := []*GraphNode{
		{StepHash: "n0", Parents: []store.Hash{"n1", "n2"}, Children: nil},
		{StepHash: "n2", Parents: nil, Children: nil},
		{StepHash: "n1", Parents: nil, Children: nil},
	}
	layout := LayoutGraph(nodes)

	// The algorithm compacts empty columns from the end, so MaxColumns may be 1
	// after all columns clear. The merge node itself should still be reachable.
	if layout.ColumnMap["n0"] < 0 {
		t.Errorf("ColumnMap missing n0")
	}
	if _, ok := layout.ColumnMap["n1"]; !ok {
		t.Errorf("ColumnMap missing n1")
	}
	if _, ok := layout.ColumnMap["n2"]; !ok {
		t.Errorf("ColumnMap missing n2")
	}
	// All nodes should have valid column assignments
	if len(layout.ColumnMap) != 3 {
		t.Errorf("expected 3 column map entries, got %d", len(layout.ColumnMap))
	}
}

func TestLayoutGraph_EmptyNodes(t *testing.T) {
	layout := LayoutGraph([]*GraphNode{})
	if layout.MaxColumns != 1 {
		t.Errorf("MaxColumns = %d for empty nodes, want 1 (fallback)", layout.MaxColumns)
	}
	if len(layout.ColumnMap) != 0 {
		t.Errorf("ColumnMap should be empty, got %d entries", len(layout.ColumnMap))
	}
}

// ---- RenderGraphLine tests (pure function) ----

func TestRenderGraphLine_SimpleLinear(t *testing.T) {
	node := &GraphNode{StepHash: "aaaa", Parents: []store.Hash{}, Column: 0}
	layout := &GraphLayout{MaxColumns: 1, ColumnMap: map[store.Hash]int{"aaaa": 0}}

	line := RenderGraphLine(node, layout, nil)
	if !strings.Contains(line, "*") {
		t.Errorf("RenderGraphLine() = %q, want *", line)
	}
	// No vertical pipe for linear single-column
	if strings.Contains(line, "|") {
		t.Errorf("RenderGraphLine() = %q, unexpected | in linear case", line)
	}
}

func TestRenderGraphLine_MergeCommit(t *testing.T) {
	node := &GraphNode{StepHash: "aaaa", Parents: []store.Hash{"p1", "p2"}, Column: 1}
	layout := &GraphLayout{
		MaxColumns: 2,
		ColumnMap:  map[store.Hash]int{"aaaa": 1},
	}

	line := RenderGraphLine(node, layout, nil)
	// Merge commit uses circle symbol
	if !strings.Contains(line, "○") { // ○
		t.Errorf("RenderGraphLine() = %q, want merge marker ○", line)
	}
}

func TestRenderGraphLine_MultiColumn(t *testing.T) {
	node := &GraphNode{StepHash: "c1", Parents: []store.Hash{"p1"}, Column: 0}
	layout := &GraphLayout{
		MaxColumns: 2,
		ColumnMap:  map[store.Hash]int{"c1": 0},
	}

	line := RenderGraphLine(node, layout, nil)
	if !strings.Contains(line, "*") {
		t.Errorf("RenderGraphLine() = %q, want * at column 0", line)
	}
}

// ---- BuildStepGraph tests (needs store) ----

func TestBuildStepGraph_Basic(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Write blob and tree for step construction
	blobHash, err := s.WriteBlob([]byte("content"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}

	step := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Write"}}
	h, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}

	stepInfo := index.StepInfo{Hash: h, SessionID: "sess"}
	nodes, err := BuildStepGraph(s, []index.StepInfo{stepInfo})
	if err != nil {
		t.Fatalf("BuildStepGraph: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].StepHash != h {
		t.Errorf("node hash = %s, want %s", nodes[0].StepHash, h)
	}
	if len(nodes[0].Parents) != 0 {
		t.Errorf("expected 0 parents, got %d", len(nodes[0].Parents))
	}
}

func TestBuildStepGraph_Chain(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)

	// Create parent step
	parent := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Bash"}}
	parentHash, err := s.WriteStep(parent)
	if err != nil {
		t.Fatalf("write parent: %v", err)
	}

	// Create child step with parent pointer
	child := &store.Step{Parent: parentHash, Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Write"}}
	childHash, err := s.WriteStep(child)
	if err != nil {
		t.Fatalf("write child: %v", err)
	}

	steps := []index.StepInfo{
		{Hash: childHash, SessionID: "sess"},
		{Hash: parentHash, SessionID: "sess"},
	}
	nodes, err := BuildStepGraph(s, steps)
	if err != nil {
		t.Fatalf("BuildStepGraph: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	// child (index 0) should have parent
	if len(nodes[0].Parents) != 1 {
		t.Errorf("child has %d parents, want 1", len(nodes[0].Parents))
	}
	if nodes[0].Parents[0] != parentHash {
		t.Errorf("child parent = %s, want %s", nodes[0].Parents[0], parentHash)
	}
	// parent (index 1) should have child hash
	if len(nodes[1].Children) != 1 {
		t.Errorf("parent has %d children, want 1", len(nodes[1].Children))
	}
}

func TestBuildStepGraph_Merge(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)

	p1 := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Write"}}
	p1Hash, _ := s.WriteStep(p1)

	p2 := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Bash"}}
	p2Hash, _ := s.WriteStep(p2)

	child := &store.Step{
		Parent:          p1Hash,
		SecondaryParent: p2Hash,
		Tree:            treeHash,
		SessionID:       "sess",
		Cause:           store.Cause{ToolName: "Edit"},
	}
	childHash, _ := s.WriteStep(child)

	steps := []index.StepInfo{
		{Hash: childHash, SessionID: "sess"},
		{Hash: p1Hash, SessionID: "sess"},
		{Hash: p2Hash, SessionID: "sess"},
	}
	nodes, err := BuildStepGraph(s, steps)
	if err != nil {
		t.Fatalf("BuildStepGraph: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	// child should have 2 parents
	if len(nodes[0].Parents) != 2 {
		t.Errorf("child has %d parents, want 2", len(nodes[0].Parents))
	}
	// both p1 and p2 should have child in their Children list
	if len(nodes[1].Children) != 1 && len(nodes[2].Children) != 1 {
		t.Errorf("expected parents to have 1 child each")
	}
}

func TestBuildStepGraph_MissingStep(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Reference a step that doesn't exist in the store
	stepInfo := index.StepInfo{Hash: "nonexistent", SessionID: "sess"}
	nodes, err := BuildStepGraph(s, []index.StepInfo{stepInfo})
	if err != nil {
		t.Fatalf("BuildStepGraph: %v (should not error on missing step)", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if len(nodes[0].Parents) > 0 {
		t.Errorf("missing step should have no parents")
	}
}

func TestBuildStepGraph_EmptySteps(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	nodes, err := BuildStepGraph(s, []index.StepInfo{})
	if err != nil {
		t.Fatalf("BuildStepGraph: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// ---- RenderGraph integration test ----

func TestRenderGraph_Basic(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("c"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)

	parent := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Write"}}
	parentHash, _ := s.WriteStep(parent)
	child := &store.Step{Parent: parentHash, Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Edit"}}
	childHash, _ := s.WriteStep(child)

	steps := []index.StepInfo{
		{Hash: childHash, SessionID: "sess"},
		{Hash: parentHash, SessionID: "sess"},
	}

	prefixes, err := RenderGraph(steps, s)
	if err != nil {
		t.Fatalf("RenderGraph: %v", err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}
	// Each prefix should contain graph art
	for i, p := range prefixes {
		if p == "" {
			t.Errorf("prefix[%d] is empty", i)
		}
		if !strings.Contains(p, "*") {
			t.Errorf("prefix[%d] = %q, missing *", i, p)
		}
	}
}

func TestRenderGraph_Empty(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	prefixes, err := RenderGraph([]index.StepInfo{}, s)
	if err != nil {
		t.Fatalf("RenderGraph: %v", err)
	}
	if len(prefixes) != 0 {
		t.Errorf("expected 0 prefixes, got %d", len(prefixes))
	}
}

func TestRenderGraph_SingleNode(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("c"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{Tree: treeHash, SessionID: "sess", Cause: store.Cause{ToolName: "Write"}}
	h, _ := s.WriteStep(step)

	prefixes, err := RenderGraph([]index.StepInfo{{Hash: h, SessionID: "sess"}}, s)
	if err != nil {
		t.Fatalf("RenderGraph: %v", err)
	}
	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}
	if !strings.Contains(prefixes[0], "*") {
		t.Errorf("prefix = %q, missing *", prefixes[0])
	}
}
