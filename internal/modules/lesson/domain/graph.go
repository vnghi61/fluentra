package domain

import "github.com/google/uuid"

// PrerequisiteEdge represents a dependency where LessonID requires RequiresLessonID.
type PrerequisiteEdge struct {
	LessonID         uuid.UUID
	RequiresLessonID uuid.UUID
}

// DetectCycle checks if the directed graph of prerequisites contains any cycle using DFS with 3-color marking.
// An adjacency list maps each lesson to the list of lessons it depends on (its prerequisites).
func DetectCycle(adj map[uuid.UUID][]uuid.UUID) bool {
	// 0: unvisited, 1: visiting (in current recursion stack), 2: visited (fully processed)
	visited := make(map[uuid.UUID]int)

	var hasCycleDFS func(node uuid.UUID) bool
	hasCycleDFS = func(node uuid.UUID) bool {
		visited[node] = 1 // Mark as visiting

		for _, neighbor := range adj[node] {
			if neighbor == node {
				return true // Self-cycle
			}
			state := visited[neighbor]
			if state == 1 {
				return true // Back-edge detected -> cycle
			}
			if state == 0 {
				if hasCycleDFS(neighbor) {
					return true
				}
			}
		}

		visited[node] = 2 // Mark as visited
		return false
	}

	for node := range adj {
		if visited[node] == 0 {
			if hasCycleDFS(node) {
				return true
			}
		}
	}

	return false
}

// WouldCreateCycle determines whether adding an edge (lessonID -> requiresID) to existingEdges would create a cycle.
// If lessonID == requiresID, it immediately returns true.
// Otherwise, it checks if there is already a directed path from requiresID to lessonID in existingEdges.
func WouldCreateCycle(existingEdges []PrerequisiteEdge, lessonID, requiresID uuid.UUID) bool {
	if lessonID == requiresID {
		return true
	}

	// Build adjacency map: node -> dependencies
	adj := make(map[uuid.UUID][]uuid.UUID)
	for _, edge := range existingEdges {
		adj[edge.LessonID] = append(adj[edge.LessonID], edge.RequiresLessonID)
	}

	// Add the candidate edge
	adj[lessonID] = append(adj[lessonID], requiresID)

	return DetectCycle(adj)
}
