package contract

// TopologyNode represents a service in the topology graph.
type TopologyNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "service", "database", "queue", "external"
}

// TopologyEdge represents a directed dependency between two nodes.
type TopologyEdge struct {
	ID       int64  `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"` // "calls", "depends_on", "publishes"
}

// TopologyGraph is the full service topology.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// BlastRadiusResponse is the API response for blast radius computation.
type BlastRadiusResponse struct {
	IncidentID int64                 `json:"incident_id"`
	Service    string                `json:"service"`
	Affected   []BlastRadiusAffected `json:"affected"`
	Depth      int                   `json:"depth"`
	Graph      *TopologyGraph        `json:"graph,omitempty"`
}

// BlastRadiusAffected describes a service affected by a blast radius.
type BlastRadiusAffected struct {
	Service string `json:"service"`
	Depth   int    `json:"depth"`
	Path    string `json:"path"` // e.g. "svc-a -> svc-b -> svc-c"
}

// AddEdgeRequest is the API request to add a topology edge.
type AddEdgeRequest struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

// InferFromTracesRequest is the API request to infer topology from trace data.
type InferFromTracesRequest struct {
	IncidentID int64 `json:"incident_id"`
}
