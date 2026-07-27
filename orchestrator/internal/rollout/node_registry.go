package rollout

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// NodeRegistry manages active executor nodes and task-to-node assignment.
// Uses round-robin selection for load balancing.
type NodeRegistry struct {
	mu              sync.RWMutex
	nodes           map[string]*RegistryNode
	assignments     map[string]*TaskAssignment // task_id -> assignment
	lastAssignedIdx int                        // for round-robin
}

// RegistryNode represents a registered executor node.
type RegistryNode struct {
	NodeID        string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
	Status        string // "active", "inactive", "deregistering"
}

// TaskAssignment tracks which node is assigned to execute a task.
type TaskAssignment struct {
	TaskID     string
	SessionID  string
	NodeID     string
	AssignedAt time.Time
	Status     string // "pending", "acknowledged", "completed"
}

// NewNodeRegistry creates a new node registry for task assignment.
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes:       make(map[string]*RegistryNode),
		assignments: make(map[string]*TaskAssignment),
	}
}

// RegisterNode adds a new executor node to the registry.
func (nr *NodeRegistry) RegisterNode(nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node_id required")
	}
	nr.mu.Lock()
	defer nr.mu.Unlock()

	now := time.Now().UTC()
	nr.nodes[nodeID] = &RegistryNode{
		NodeID:        nodeID,
		RegisteredAt:  now,
		LastHeartbeat: now,
		Status:        "active",
	}
	return nil
}

// DeregisterNode removes a node from the registry.
func (nr *NodeRegistry) DeregisterNode(nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node_id required")
	}
	nr.mu.Lock()
	defer nr.mu.Unlock()

	node, ok := nr.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	node.Status = "deregistering"
	delete(nr.nodes, nodeID)
	return nil
}

// HeartbeatNode updates node liveness.
func (nr *NodeRegistry) HeartbeatNode(nodeID string) error {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	node, ok := nr.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	node.LastHeartbeat = time.Now().UTC()
	node.Status = "active"
	return nil
}

// GetActiveNodes returns all active nodes.
func (nr *NodeRegistry) GetActiveNodes() []*RegistryNode {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	var active []*RegistryNode
	for _, node := range nr.nodes {
		if node.Status == "active" {
			active = append(active, node)
		}
	}
	return active
}

// AssignTask assigns a task to an available node using round-robin selection.
// Returns the assigned node_id or an error if no nodes are available.
func (nr *NodeRegistry) AssignTask(taskID, sessionID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("task_id required")
	}
	if sessionID == "" {
		return "", fmt.Errorf("session_id required")
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()

	// Get active nodes
	activeNodes := nr.getActiveNodesLocked()
	if len(activeNodes) == 0 {
		return "", fmt.Errorf("no available executor nodes")
	}

	// Round-robin selection
	selectedNode := activeNodes[nr.lastAssignedIdx%len(activeNodes)]
	nr.lastAssignedIdx++

	// Store assignment
	nr.assignments[taskID] = &TaskAssignment{
		TaskID:     taskID,
		SessionID:  sessionID,
		NodeID:     selectedNode.NodeID,
		AssignedAt: time.Now().UTC(),
		Status:     "pending",
	}

	return selectedNode.NodeID, nil
}

// GetAssignedTasks returns all tasks assigned to a specific node that are pending.
func (nr *NodeRegistry) GetAssignedTasks(nodeID string) []*TaskAssignment {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	var result []*TaskAssignment
	for _, assignment := range nr.assignments {
		if assignment.NodeID == nodeID && assignment.Status == "pending" {
			result = append(result, assignment)
		}
	}
	return result
}

// MarkTaskAcknowledged marks a task as acknowledged by the executor node.
func (nr *NodeRegistry) MarkTaskAcknowledged(taskID string) error {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	assignment, ok := nr.assignments[taskID]
	if !ok {
		return fmt.Errorf("task not assigned: %s", taskID)
	}
	assignment.Status = "acknowledged"
	return nil
}

// MarkTaskCompleted marks a task as completed.
func (nr *NodeRegistry) MarkTaskCompleted(taskID string) error {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	assignment, ok := nr.assignments[taskID]
	if !ok {
		return fmt.Errorf("task not assigned: %s", taskID)
	}
	assignment.Status = "completed"
	return nil
}

// NodeCount returns the number of registered nodes.
func (nr *NodeRegistry) NodeCount() int {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return len(nr.nodes)
}

// getActiveNodesLocked returns active nodes without acquiring the lock (for internal use).
// The slice is sorted by NodeID to guarantee stable ordering for round-robin selection.
func (nr *NodeRegistry) getActiveNodesLocked() []*RegistryNode {
	var active []*RegistryNode
	for _, node := range nr.nodes {
		if node.Status == "active" {
			active = append(active, node)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].NodeID < active[j].NodeID })
	return active
}

// DeregisterStaleNodes removes nodes whose last heartbeat is older than maxAge.
// Returns the list of deregistered node IDs. Safe to call concurrently.
func (nr *NodeRegistry) DeregisterStaleNodes(maxAge time.Duration) []string {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	now := time.Now().UTC()
	var removed []string
	for id, node := range nr.nodes {
		if now.Sub(node.LastHeartbeat) > maxAge {
			delete(nr.nodes, id)
			removed = append(removed, id)
		}
	}
	return removed
}
