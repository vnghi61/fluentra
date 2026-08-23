package domain_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
)

func TestDetectCycle(t *testing.T) {
	nodeA := uuid.New()
	nodeB := uuid.New()
	nodeC := uuid.New()
	nodeD := uuid.New()

	tests := []struct {
		name      string
		adj       map[uuid.UUID][]uuid.UUID
		wantCycle bool
	}{
		{
			name:      "no edges (empty graph)",
			adj:       map[uuid.UUID][]uuid.UUID{},
			wantCycle: false,
		},
		{
			name: "single node without edges",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {},
			},
			wantCycle: false,
		},
		{
			name: "linear chain A -> B -> C",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB},
				nodeB: {nodeC},
				nodeC: {},
			},
			wantCycle: false,
		},
		{
			name: "diamond A -> B, A -> C, B -> D, C -> D",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB, nodeC},
				nodeB: {nodeD},
				nodeC: {nodeD},
				nodeD: {},
			},
			wantCycle: false,
		},
		{
			name: "two-node cycle A -> B -> A",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB},
				nodeB: {nodeA},
			},
			wantCycle: true,
		},
		{
			name: "three-node cycle A -> B -> C -> A",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB},
				nodeB: {nodeC},
				nodeC: {nodeA},
			},
			wantCycle: true,
		},
		{
			name: "self-edge A -> A",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeA},
			},
			wantCycle: true,
		},
		{
			name: "disconnected components without cycle",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB},
				nodeC: {nodeD},
			},
			wantCycle: false,
		},
		{
			name: "disconnected components with cycle in second component",
			adj: map[uuid.UUID][]uuid.UUID{
				nodeA: {nodeB},
				nodeC: {nodeD},
				nodeD: {nodeC},
			},
			wantCycle: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.DetectCycle(tc.adj)
			if got != tc.wantCycle {
				t.Errorf("DetectCycle() = %v, want %v", got, tc.wantCycle)
			}
		})
	}
}

func TestWouldCreateCycle(t *testing.T) {
	nodeA := uuid.New()
	nodeB := uuid.New()
	nodeC := uuid.New()
	nodeD := uuid.New()

	existingChain := []domain.PrerequisiteEdge{
		{LessonID: nodeA, RequiresLessonID: nodeB},
		{LessonID: nodeB, RequiresLessonID: nodeC},
	}

	tests := []struct {
		name      string
		edges     []domain.PrerequisiteEdge
		newLesson uuid.UUID
		newReq    uuid.UUID
		wantCycle bool
	}{
		{
			name:      "self edge",
			edges:     nil,
			newLesson: nodeA,
			newReq:    nodeA,
			wantCycle: true,
		},
		{
			name:      "extending chain without cycle A->B->C and C->D",
			edges:     existingChain,
			newLesson: nodeC,
			newReq:    nodeD,
			wantCycle: false,
		},
		{
			name:      "closing chain into cycle A->B->C and C->A",
			edges:     existingChain,
			newLesson: nodeC,
			newReq:    nodeA,
			wantCycle: true,
		},
		{
			name:      "closing chain into cycle B->A",
			edges:     existingChain,
			newLesson: nodeB,
			newReq:    nodeA,
			wantCycle: true,
		},
		{
			name:      "independent edge D->A",
			edges:     existingChain,
			newLesson: nodeD,
			newReq:    nodeA,
			wantCycle: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.WouldCreateCycle(tc.edges, tc.newLesson, tc.newReq)
			if got != tc.wantCycle {
				t.Errorf("WouldCreateCycle() = %v, want %v", got, tc.wantCycle)
			}
		})
	}
}
