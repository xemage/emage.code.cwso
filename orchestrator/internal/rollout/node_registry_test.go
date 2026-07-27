package rollout

import (
	"testing"
	"time"
)

// TestNodeRegistry_AssignTask verifies round-robin task assignment (Phase 3.1).
func TestNodeRegistry_AssignTask(t *testing.T) {
	nr := NewNodeRegistry()

	// Register 2 nodes
	if err := nr.RegisterNode("executor-1"); err != nil {
		t.Fatal(err)
	}
	if err := nr.RegisterNode("executor-2"); err != nil {
		t.Fatal(err)
	}

	// Assign multiple tasks and collect assignments
	// Due to map iteration randomness, we just verify:
	// 1. Tasks can be assigned to available nodes
	// 2. Different nodes are used when available
	assignments := make(map[string]int) // node -> count

	for i := 1; i <= 4; i++ {
		nodeID, err := nr.AssignTask(
			"task-"+string(rune('0'+i)),
			"session-"+string(rune('0'+i)),
		)
		if err != nil {
			t.Fatalf("task %d assignment failed: %v", i, err)
		}
		assignments[nodeID]++
	}

	// Verify both nodes received some assignments (round-robin should spread load)
	if len(assignments) < 2 {
		t.Logf("assignments: %v", assignments)
		t.Fatal("expected tasks assigned to at least 2 nodes")
	}

	// Verify total assignments
	total := 0
	for _, count := range assignments {
		total += count
	}
	if total != 4 {
		t.Fatalf("expected 4 total assignments, got %d", total)
	}
}

// TestNodeRegistry_GetAssignedTasks verifies task retrieval by node (Phase 3.1).
func TestNodeRegistry_GetAssignedTasks(t *testing.T) {
	nr := NewNodeRegistry()

	// Register node
	nr.RegisterNode("executor-1")

	// Assign task
	nodeID, _ := nr.AssignTask("task-1", "session-1")
	if nodeID != "executor-1" {
		t.Fatal("task not assigned to expected node")
	}

	// Retrieve assigned tasks for node
	tasks := nr.GetAssignedTasks("executor-1")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.TaskID != "task-1" || task.SessionID != "session-1" {
		t.Fatalf("unexpected task: %+v", task)
	}

	if task.Status != "pending" {
		t.Fatalf("expected status 'pending', got '%s'", task.Status)
	}
}

// TestNodeRegistry_HeartbeatKeepsNodeActive verifies node liveness update.
func TestNodeRegistry_HeartbeatKeepsNodeActive(t *testing.T) {
	nr := NewNodeRegistry()
	nr.RegisterNode("executor-1")

	// Get active nodes before heartbeat
	active1 := nr.GetActiveNodes()
	if len(active1) != 1 {
		t.Fatal("expected 1 active node")
	}

	oldHeartbeat := active1[0].LastHeartbeat

	// Wait a bit and send heartbeat
	time.Sleep(100 * time.Millisecond)
	if err := nr.HeartbeatNode("executor-1"); err != nil {
		t.Fatal(err)
	}

	// Verify heartbeat was updated
	active2 := nr.GetActiveNodes()
	if len(active2) != 1 {
		t.Fatal("expected 1 active node after heartbeat")
	}

	if active2[0].LastHeartbeat.Before(oldHeartbeat.Add(50 * time.Millisecond)) {
		t.Fatal("heartbeat timestamp not updated")
	}
}

// TestNodeRegistry_NoAvailableNodes verifies error when no executors available.
func TestNodeRegistry_NoAvailableNodes(t *testing.T) {
	nr := NewNodeRegistry()

	// Try to assign task with no nodes
	_, err := nr.AssignTask("task-1", "session-1")
	if err == nil {
		t.Fatal("expected error when no nodes available")
	}
}

// TestNodeRegistry_MarkTaskAcknowledged verifies task status transition.
func TestNodeRegistry_MarkTaskAcknowledged(t *testing.T) {
	nr := NewNodeRegistry()
	nr.RegisterNode("executor-1")

	// Assign task
	_, err := nr.AssignTask("task-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}

	// Mark as acknowledged
	if err := nr.MarkTaskAcknowledged("task-1"); err != nil {
		t.Fatal(err)
	}

	// Task should no longer appear in pending list (since status is "acknowledged")
	tasks := nr.GetAssignedTasks("executor-1")
	if len(tasks) != 0 {
		t.Fatalf("expected 0 pending tasks after acknowledgement, got %d", len(tasks))
	}
}

// TestNodeRegistry_DeregisterStaleNodes verifies Phase 3.2 stale-node reaping.
func TestNodeRegistry_DeregisterStaleNodes(t *testing.T) {
	nr := NewNodeRegistry()

	if err := nr.RegisterNode("fresh-node"); err != nil {
		t.Fatal(err)
	}
	if err := nr.RegisterNode("stale-node"); err != nil {
		t.Fatal(err)
	}

	// Back-date the stale node's heartbeat beyond the maxAge threshold
	nr.mu.Lock()
	nr.nodes["stale-node"].LastHeartbeat = time.Now().UTC().Add(-120 * time.Second)
	nr.mu.Unlock()

	removed := nr.DeregisterStaleNodes(90 * time.Second)

	if len(removed) != 1 || removed[0] != "stale-node" {
		t.Fatalf("expected [stale-node] removed, got %v", removed)
	}

	// fresh-node must still be active
	active := nr.GetActiveNodes()
	if len(active) != 1 || active[0].NodeID != "fresh-node" {
		t.Fatalf("expected fresh-node still active, got %v", active)
	}
}
