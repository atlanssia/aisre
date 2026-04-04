package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// TopologyFinder computes blast radius from the topology graph.
// Defined in consuming package per CLAUDE.md convention.
type TopologyFinder interface {
	GetTopology(ctx context.Context) (*contract.TopologyGraph, error)
	ComputeBlastRadius(ctx context.Context, service string, depth int) ([]contract.BlastRadiusAffected, error)
}

// Service manages service topology and blast radius computation.
type Service struct {
	topoRepo  store.TopologyRepo
	incRepo   store.IncidentRepo
	logger    *slog.Logger
}

// NewService creates a new topology service.
func NewService(topoRepo store.TopologyRepo, incRepo store.IncidentRepo) *Service {
	return &Service{
		topoRepo: topoRepo,
		incRepo:  incRepo,
		logger:   slog.Default(),
	}
}

// GetTopology returns the full service topology graph.
func (s *Service) GetTopology(ctx context.Context) (*contract.TopologyGraph, error) {
	edges, err := s.topoRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("topology: get: %w", err)
	}

	nodeSet := make(map[string]string) // name -> type
	var contractEdges []contract.TopologyEdge
	for _, e := range edges {
		if _, ok := nodeSet[e.Source]; !ok {
			nodeSet[e.Source] = "service"
		}
		if _, ok := nodeSet[e.Target]; !ok {
			nodeSet[e.Target] = "service"
		}
		contractEdges = append(contractEdges, contract.TopologyEdge{
			ID:       e.ID,
			Source:   e.Source,
			Target:   e.Target,
			Relation: e.Relation,
		})
	}

	nodes := make([]contract.TopologyNode, 0, len(nodeSet))
	for name, typ := range nodeSet {
		nodes = append(nodes, contract.TopologyNode{
			ID:   name,
			Name: name,
			Type: typ,
		})
	}

	if contractEdges == nil {
		contractEdges = []contract.TopologyEdge{}
	}

	return &contract.TopologyGraph{
		Nodes: nodes,
		Edges: contractEdges,
	}, nil
}

// ComputeBlastRadius performs BFS from a service to find all downstream dependents.
func (s *Service) ComputeBlastRadius(ctx context.Context, service string, depth int) ([]contract.BlastRadiusAffected, error) {
	if depth <= 0 {
		depth = 3
	}

	edges, err := s.topoRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("topology: blast radius: %w", err)
	}

	// Build adjacency list (source -> targets)
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}

	// BFS
	visited := map[string]bool{service: true}
	queue := []bfsNode{{name: service, depth: 0, path: service}}
	var result []contract.BlastRadiusAffected

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, neighbor := range adj[cur.name] {
			if visited[neighbor] {
				continue
			}
			visited[neighbor] = true
			nextDepth := cur.depth + 1
			nextPath := cur.path + " -> " + neighbor

			result = append(result, contract.BlastRadiusAffected{
				Service: neighbor,
				Depth:   nextDepth,
				Path:    nextPath,
			})

			if nextDepth < depth {
				queue = append(queue, bfsNode{name: neighbor, depth: nextDepth, path: nextPath})
			}
		}
	}

	if result == nil {
		result = []contract.BlastRadiusAffected{}
	}
	return result, nil
}

// ComputeBlastRadiusForIncident computes blast radius for the service involved in an incident.
func (s *Service) ComputeBlastRadiusForIncident(ctx context.Context, incidentID int64, depth int) (*contract.BlastRadiusResponse, error) {
	inc, err := s.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("topology: get incident: %w", err)
	}

	affected, err := s.ComputeBlastRadius(ctx, inc.ServiceName, depth)
	if err != nil {
		return nil, fmt.Errorf("topology: compute blast radius: %w", err)
	}

	if depth <= 0 {
		depth = 3
	}

	return &contract.BlastRadiusResponse{
		IncidentID: incidentID,
		Service:    inc.ServiceName,
		Affected:   affected,
		Depth:      depth,
	}, nil
}

// AddEdge adds a topology edge.
func (s *Service) AddEdge(ctx context.Context, req contract.AddEdgeRequest) (int64, error) {
	if req.Relation == "" {
		req.Relation = "calls"
	}
	metadata, _ := json.Marshal(map[string]any{})
	edge := &store.TopologyEdge{
		Source:   req.Source,
		Target:   req.Target,
		Relation: req.Relation,
		Metadata: string(metadata),
	}
	id, err := s.topoRepo.Create(ctx, edge)
	if err != nil {
		return 0, fmt.Errorf("topology: add edge: %w", err)
	}
	s.logger.Info("topology edge added", "id", id, "source", req.Source, "target", req.Target)
	return id, nil
}

type bfsNode struct {
	name  string
	depth int
	path  string
}
