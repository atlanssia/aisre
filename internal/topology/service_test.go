package topology

import (
	"context"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// mockTopologyRepo implements store.TopologyRepo for testing.
type mockTopologyRepo struct {
	edges  []store.TopologyEdge
	nextID int64
}

func (m *mockTopologyRepo) Create(_ context.Context, edge *store.TopologyEdge) (int64, error) {
	m.nextID++
	edge.ID = m.nextID
	m.edges = append(m.edges, *edge)
	return m.nextID, nil
}

func (m *mockTopologyRepo) List(_ context.Context) ([]store.TopologyEdge, error) {
	return m.edges, nil
}

func (m *mockTopologyRepo) ListBySource(_ context.Context, source string) ([]store.TopologyEdge, error) {
	var result []store.TopologyEdge
	for _, e := range m.edges {
		if e.Source == source {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockTopologyRepo) Delete(_ context.Context, id int64) error {
	for i, e := range m.edges {
		if e.ID == id {
			m.edges = append(m.edges[:i], m.edges[i+1:]...)
			break
		}
	}
	return nil
}

func seedEdges(repo *mockTopologyRepo, edges [][3]string) {
	for _, e := range edges {
		repo.nextID++
		repo.edges = append(repo.edges, store.TopologyEdge{
			ID:       repo.nextID,
			Source:   e[0],
			Target:   e[1],
			Relation: e[2],
		})
	}
}

func TestComputeBlastRadius_Linear(t *testing.T) {
	repo := &mockTopologyRepo{}
	seedEdges(repo, [][3]string{
		{"A", "B", "calls"},
		{"B", "C", "calls"},
		{"C", "D", "calls"},
	})
	svc := NewService(repo, nil)

	result, err := svc.ComputeBlastRadius(context.Background(), "A", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 affected services, got %d", len(result))
	}

	// Verify BFS order: B (depth 1), C (depth 2), D (depth 3)
	assertAffected(t, result[0], "B", 1, "A -> B")
	assertAffected(t, result[1], "C", 2, "A -> B -> C")
	assertAffected(t, result[2], "D", 3, "A -> B -> C -> D")
}

func TestComputeBlastRadius_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D
	repo := &mockTopologyRepo{}
	seedEdges(repo, [][3]string{
		{"A", "B", "calls"},
		{"A", "C", "calls"},
		{"B", "D", "calls"},
		{"C", "D", "calls"},
	})
	svc := NewService(repo, nil)

	result, err := svc.ComputeBlastRadius(context.Background(), "A", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find B, C, D — D only once
	seen := map[string]bool{}
	for _, r := range result {
		if seen[r.Service] {
			t.Errorf("duplicate service: %s", r.Service)
		}
		seen[r.Service] = true
	}
	if !seen["B"] || !seen["C"] || !seen["D"] {
		t.Errorf("expected B, C, D; got %v", seen)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 unique services, got %d", len(result))
	}
}

func TestComputeBlastRadius_Cycle(t *testing.T) {
	// A -> B -> C -> A (cycle)
	repo := &mockTopologyRepo{}
	seedEdges(repo, [][3]string{
		{"A", "B", "calls"},
		{"B", "C", "calls"},
		{"C", "A", "calls"},
	})
	svc := NewService(repo, nil)

	result, err := svc.ComputeBlastRadius(context.Background(), "A", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range result {
		if r.Service == "A" {
			t.Error("should not re-visit starting service A")
		}
	}
}

func TestComputeBlastRadius_Empty(t *testing.T) {
	repo := &mockTopologyRepo{}
	svc := NewService(repo, nil)

	result, err := svc.ComputeBlastRadius(context.Background(), "A", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 affected, got %d", len(result))
	}
}

func TestComputeBlastRadius_DepthLimit(t *testing.T) {
	repo := &mockTopologyRepo{}
	seedEdges(repo, [][3]string{
		{"A", "B", "calls"},
		{"B", "C", "calls"},
		{"C", "D", "calls"},
	})
	svc := NewService(repo, nil)

	result, err := svc.ComputeBlastRadius(context.Background(), "A", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 affected, got %d", len(result))
	}
	if result[0].Service != "B" || result[1].Service != "C" {
		t.Errorf("expected [B, C], got %v", result)
	}
}

func TestGetTopology(t *testing.T) {
	repo := &mockTopologyRepo{}
	seedEdges(repo, [][3]string{
		{"svc-a", "svc-b", "calls"},
		{"svc-b", "svc-c", "depends_on"},
	})
	svc := NewService(repo, nil)

	graph, err := svc.GetTopology(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(graph.Edges))
	}
	// Should have 3 nodes: svc-a, svc-b, svc-c
	nodeSet := map[string]bool{}
	for _, n := range graph.Nodes {
		nodeSet[n.Name] = true
	}
	if len(nodeSet) != 3 || !nodeSet["svc-a"] || !nodeSet["svc-b"] || !nodeSet["svc-c"] {
		t.Errorf("expected 3 unique nodes, got %v", nodeSet)
	}
}

func TestAddEdge(t *testing.T) {
	repo := &mockTopologyRepo{}
	svc := NewService(repo, nil)

	id, err := svc.AddEdge(context.Background(), contract.AddEdgeRequest{
		Source:   "svc-a",
		Target:   "svc-b",
		Relation: "calls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
	if len(repo.edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(repo.edges))
	}
	if repo.edges[0].Source != "svc-a" || repo.edges[0].Target != "svc-b" {
		t.Errorf("unexpected edge: %+v", repo.edges[0])
	}
}

func assertAffected(t *testing.T, got contract.BlastRadiusAffected, wantSvc string, wantDepth int, wantPath string) {
	t.Helper()
	if got.Service != wantSvc {
		t.Errorf("Service = %q, want %q", got.Service, wantSvc)
	}
	if got.Depth != wantDepth {
		t.Errorf("Depth = %d, want %d", got.Depth, wantDepth)
	}
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
}
