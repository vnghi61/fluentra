package domain

import (
	"cmp"
	"slices"

	"github.com/google/uuid"
)

// PrerequisiteEdge represents an edge in the DAG where NodeID requires RequiresNodeID.
type PrerequisiteEdge struct {
	NodeID         uuid.UUID
	RequiresNodeID uuid.UUID
}

// DetectCycle checks if the directed graph of prerequisites contains any cycle using DFS with 3-color marking.
// adj maps each node to the list of nodes it depends on (its prerequisites).
// Returns true and the cycle chain if a cycle is detected.
func DetectCycle(adj map[uuid.UUID][]uuid.UUID) (bool, []uuid.UUID) {
	// 0: unvisited, 1: visiting (in current recursion stack), 2: visited (fully processed)
	visited := make(map[uuid.UUID]int)
	stack := make([]uuid.UUID, 0)
	stackPos := make(map[uuid.UUID]int)

	var cycle []uuid.UUID

	var dfs func(node uuid.UUID) bool
	dfs = func(node uuid.UUID) bool {
		visited[node] = 1
		stackPos[node] = len(stack)
		stack = append(stack, node)

		for _, neighbor := range adj[node] {
			if neighbor == node {
				cycle = []uuid.UUID{node, node}
				return true
			}
			state := visited[neighbor]
			if state == 1 {
				// Back-edge found -> extract cycle
				pos := stackPos[neighbor]
				cycle = append(cycle, stack[pos:]...)
				cycle = append(cycle, neighbor)
				return true
			}
			if state == 0 {
				if dfs(neighbor) {
					return true
				}
			}
		}

		visited[node] = 2
		delete(stackPos, node)
		stack = stack[:len(stack)-1]
		return false
	}

	for node := range adj {
		if visited[node] == 0 {
			if dfs(node) {
				return true, cycle
			}
		}
	}

	return false, nil
}

// CheckProposedEdges tests whether replacing targetNode's prerequisites with newPrereqs would form a cycle.
func CheckProposedEdges(
	existingEdges []PrerequisiteEdge, targetNode uuid.UUID, newPrereqs []uuid.UUID,
) (bool, []uuid.UUID) {
	adj := make(map[uuid.UUID][]uuid.UUID)

	// Keep edges for other nodes
	for _, edge := range existingEdges {
		if edge.NodeID != targetNode {
			adj[edge.NodeID] = append(adj[edge.NodeID], edge.RequiresNodeID)
		}
	}

	// Add new proposed prerequisites for targetNode
	for _, reqID := range newPrereqs {
		if reqID == targetNode {
			return true, []uuid.UUID{targetNode, targetNode}
		}
		adj[targetNode] = append(adj[targetNode], reqID)
	}

	return DetectCycle(adj)
}

// TopologicalOrder produces a stable, deterministic ordering of nodes where prerequisites appear before
// the nodes that require them. Ties between independent nodes are broken deterministically by (position ASC, code ASC).
func TopologicalOrder(nodes []Taxonomy, edges []PrerequisiteEdge) ([]Taxonomy, error) {
	if len(nodes) == 0 {
		return []Taxonomy{}, nil
	}

	nodeMap := make(map[uuid.UUID]Taxonomy, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	// Calculate in-degrees and outgoing edges for forward learning order:
	// If Node requires RequiresNode, then RequiresNode -> Node.
	// That is, RequiresNode must be completed before Node can be started.
	inDegree := make(map[uuid.UUID]int, len(nodes))
	outgoing := make(map[uuid.UUID][]uuid.UUID, len(nodes))

	for _, n := range nodes {
		inDegree[n.ID] = 0
	}

	for _, edge := range edges {
		// Only consider edges where both ends are in the provided node set
		if _, ok1 := nodeMap[edge.NodeID]; ok1 {
			if _, ok2 := nodeMap[edge.RequiresNodeID]; ok2 {
				inDegree[edge.NodeID]++
				outgoing[edge.RequiresNodeID] = append(outgoing[edge.RequiresNodeID], edge.NodeID)
			}
		}
	}

	// Find initial nodes with inDegree == 0
	ready := make([]Taxonomy, 0)
	for _, n := range nodes {
		if inDegree[n.ID] == 0 {
			ready = append(ready, n)
		}
	}

	// Sort ready nodes by (position ASC, code ASC)
	sortNodes(ready)

	result := make([]Taxonomy, 0, len(nodes))

	for len(ready) > 0 {
		// Pop the lowest (position, code)
		curr := ready[0]
		ready = ready[1:]
		result = append(result, curr)

		// For all nodes depending on curr, decrement inDegree
		for _, depID := range outgoing[curr.ID] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				ready = append(ready, nodeMap[depID])
			}
		}

		sortNodes(ready)
	}

	if len(result) < len(nodes) {
		return nil, ErrTaxonomyCycle
	}

	return result, nil
}

func sortNodes(slice []Taxonomy) {
	slices.SortFunc(slice, func(a, b Taxonomy) int {
		if c := cmp.Compare(a.Position, b.Position); c != 0 {
			return c
		}
		return cmp.Compare(a.Code, b.Code)
	})
}

// PathTo returns the transitive closure of prerequisites for target in topological order, ending at target.
func PathTo(target Taxonomy, allNodes map[uuid.UUID]Taxonomy, edges []PrerequisiteEdge) ([]Taxonomy, error) {
	// Build map: node -> prerequisites (RequiresNodeID)
	prereqsMap := make(map[uuid.UUID][]uuid.UUID)
	for _, edge := range edges {
		prereqsMap[edge.NodeID] = append(prereqsMap[edge.NodeID], edge.RequiresNodeID)
	}

	// Compute transitive closure of target's prerequisites
	closureSet := make(map[uuid.UUID]bool)
	queue := []uuid.UUID{target.ID}
	closureSet[target.ID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, prereqID := range prereqsMap[curr] {
			if !closureSet[prereqID] {
				closureSet[prereqID] = true
				queue = append(queue, prereqID)
			}
		}
	}

	// Filter nodes and edges to closure.
	//
	// A prerequisite missing from allNodes is the ordinary case, not a fault:
	// allNodes excludes deprecated nodes (BR-FOUNDATION-07), and deprecating one
	// that others still require must shorten the path rather than break it. This
	// used to return an error, so deprecating a single node turned every path
	// through it into a 500.
	subNodes := make([]Taxonomy, 0, len(closureSet))
	for id := range closureSet {
		if node, ok := allNodes[id]; ok {
			subNodes = append(subNodes, node)
			continue
		}
		delete(closureSet, id)
	}

	subEdges := make([]PrerequisiteEdge, 0)
	for _, edge := range edges {
		if closureSet[edge.NodeID] && closureSet[edge.RequiresNodeID] {
			subEdges = append(subEdges, edge)
		}
	}

	return TopologicalOrder(subNodes, subEdges)
}
