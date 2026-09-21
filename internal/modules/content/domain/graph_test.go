package domain_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

// Spine codes used across these tests, spelled once.
const (
	codeSentenceStructure = "SENTENCE_STRUCTURE"
	codePresentSimple     = "PRESENT_SIMPLE"
	codePresentContinuous = "PRESENT_CONTINUOUS"
	codePastSimple        = "PAST_SIMPLE"
	codePresentPerfect    = "PRESENT_PERFECT"
	codeFutureSimple      = "FUTURE_SIMPLE"
)

func TestDetectCycle(t *testing.T) {
	t.Parallel()

	nodeA := uuid.New()
	nodeB := uuid.New()
	nodeC := uuid.New()
	nodeD := uuid.New()

	t.Run("self-edge", func(t *testing.T) {
		adj := map[uuid.UUID][]uuid.UUID{
			nodeA: {nodeA},
		}
		hasCycle, cycle := domain.DetectCycle(adj)
		if !hasCycle {
			t.Fatal("expected cycle on self-edge")
		}
		if len(cycle) != 2 || cycle[0] != nodeA || cycle[1] != nodeA {
			t.Errorf("unexpected cycle chain: %v", cycle)
		}
	})

	t.Run("two-node cycle", func(t *testing.T) {
		adj := map[uuid.UUID][]uuid.UUID{
			nodeA: {nodeB},
			nodeB: {nodeA},
		}
		hasCycle, cycle := domain.DetectCycle(adj)
		if !hasCycle {
			t.Fatal("expected cycle between nodeA and nodeB")
		}
		if len(cycle) < 3 {
			t.Errorf("cycle chain too short: %v", cycle)
		}
	})

	t.Run("three-node cycle", func(t *testing.T) {
		adj := map[uuid.UUID][]uuid.UUID{
			nodeA: {nodeB},
			nodeB: {nodeC},
			nodeC: {nodeA},
		}
		hasCycle, cycle := domain.DetectCycle(adj)
		if !hasCycle {
			t.Fatal("expected cycle in 3-node loop")
		}
		if len(cycle) < 4 {
			t.Errorf("cycle chain too short: %v", cycle)
		}
	})

	t.Run("diamond graph is acyclic", func(t *testing.T) {
		// D requires B and C; both B and C require A
		adj := map[uuid.UUID][]uuid.UUID{
			nodeD: {nodeB, nodeC},
			nodeB: {nodeA},
			nodeC: {nodeA},
			nodeA: {},
		}
		hasCycle, _ := domain.DetectCycle(adj)
		if hasCycle {
			t.Fatal("diamond graph should not have a cycle")
		}
	})
}

func TestCheckProposedEdges(t *testing.T) {
	t.Parallel()

	nodeA := uuid.New()
	nodeB := uuid.New()
	nodeC := uuid.New()

	// Existing: B requires A, C requires B
	existing := []domain.PrerequisiteEdge{
		{NodeID: nodeB, RequiresNodeID: nodeA},
		{NodeID: nodeC, RequiresNodeID: nodeB},
	}

	// Propose A requires C -> cycle (C -> B -> A -> C)
	hasCycle, _ := domain.CheckProposedEdges(existing, nodeA, []uuid.UUID{nodeC})
	if !hasCycle {
		t.Fatal("expected cycle when proposing A requires C")
	}

	// Propose A requires self
	hasCycle, _ = domain.CheckProposedEdges(existing, nodeA, []uuid.UUID{nodeA})
	if !hasCycle {
		t.Fatal("expected cycle on self-prerequisite")
	}

	// Propose C requires only A -> valid DAG
	hasCycle, _ = domain.CheckProposedEdges(existing, nodeC, []uuid.UUID{nodeA})
	if hasCycle {
		t.Fatal("expected valid DAG when C requires A")
	}
}

func TestTopologicalOrderStability(t *testing.T) {
	t.Parallel()

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()

	nodes := []domain.Taxonomy{
		{ID: id1, Code: "B_TOPIC", Position: 20},
		{ID: id2, Code: "A_TOPIC", Position: 20},
		{ID: id3, Code: "Z_TOPIC", Position: 10},
		{ID: id4, Code: "C_TOPIC", Position: 30},
	}

	// No edges: tie breaking should produce Z_TOPIC (pos 10), then A_TOPIC (pos 20), B_TOPIC (pos 20), C_TOPIC (pos 30)
	order1, err1 := domain.TopologicalOrder(nodes, nil)
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	order2, err2 := domain.TopologicalOrder(nodes, nil)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}

	if len(order1) != len(nodes) || len(order2) != len(nodes) {
		t.Fatalf("wrong length: %d and %d", len(order1), len(order2))
	}

	for i := range order1 {
		if order1[i].Code != order2[i].Code {
			t.Fatalf("non-deterministic order at index %d: %s vs %s", i, order1[i].Code, order2[i].Code)
		}
	}

	expectedCodes := []string{"Z_TOPIC", "A_TOPIC", "B_TOPIC", "C_TOPIC"}
	for i, exp := range expectedCodes {
		if order1[i].Code != exp {
			t.Errorf("at %d got %s, want %s", i, order1[i].Code, exp)
		}
	}
}

func TestAcceptancePathTo(t *testing.T) {
	t.Parallel()

	// 1. Acceptance test:
	// SENTENCE_STRUCTURE -> PRESENT_SIMPLE -> PRESENT_CONTINUOUS -> PAST_SIMPLE -> PRESENT_PERFECT -> FUTURE_SIMPLE
	ssID := uuid.New()
	psID := uuid.New()
	pcID := uuid.New()
	pastID := uuid.New()
	perfID := uuid.New()
	futID := uuid.New()

	nodes := map[uuid.UUID]domain.Taxonomy{
		ssID:   {ID: ssID, Code: codeSentenceStructure, Position: 10},
		psID:   {ID: psID, Code: codePresentSimple, Position: 20},
		pcID:   {ID: pcID, Code: codePresentContinuous, Position: 30},
		pastID: {ID: pastID, Code: codePastSimple, Position: 40},
		perfID: {ID: perfID, Code: codePresentPerfect, Position: 50},
		futID:  {ID: futID, Code: codeFutureSimple, Position: 60},
	}

	edges := []domain.PrerequisiteEdge{
		{NodeID: psID, RequiresNodeID: ssID},
		{NodeID: pcID, RequiresNodeID: psID},
		{NodeID: pastID, RequiresNodeID: pcID},
		{NodeID: perfID, RequiresNodeID: pastID},
		{NodeID: futID, RequiresNodeID: perfID},
	}

	path, err := domain.PathTo(nodes[perfID], nodes, edges)
	if err != nil {
		t.Fatalf("PathTo(PRESENT_PERFECT) failed: %v", err)
	}

	expected := []string{
		codeSentenceStructure, codePresentSimple, codePresentContinuous, codePastSimple, codePresentPerfect,
	}
	if len(path) != len(expected) {
		t.Fatalf("got %d nodes, want %d", len(path), len(expected))
	}
	for i, exp := range expected {
		if path[i].Code != exp {
			t.Errorf("path[%d] = %s, want %s", i, path[i].Code, exp)
		}
	}

	// 2. Acceptance test:
	// SENTENCE_STRUCTURE -> RELATIVE_CLAUSES -> NOUN_CLAUSES -> ADVERBIAL_CLAUSES
	rcID := uuid.New()
	ncID := uuid.New()
	acID := uuid.New()

	nodes[rcID] = domain.Taxonomy{ID: rcID, Code: "RELATIVE_CLAUSES", Position: 70}
	nodes[ncID] = domain.Taxonomy{ID: ncID, Code: "NOUN_CLAUSES", Position: 80}
	nodes[acID] = domain.Taxonomy{ID: acID, Code: "ADVERBIAL_CLAUSES", Position: 90}

	edges = append(edges,
		domain.PrerequisiteEdge{NodeID: rcID, RequiresNodeID: ssID},
		domain.PrerequisiteEdge{NodeID: ncID, RequiresNodeID: rcID},
		domain.PrerequisiteEdge{NodeID: acID, RequiresNodeID: ncID},
	)

	pathClauses, err := domain.PathTo(nodes[acID], nodes, edges)
	if err != nil {
		t.Fatalf("PathTo(ADVERBIAL_CLAUSES) failed: %v", err)
	}

	expectedClauses := []string{codeSentenceStructure, "RELATIVE_CLAUSES", "NOUN_CLAUSES", "ADVERBIAL_CLAUSES"}
	if len(pathClauses) != len(expectedClauses) {
		t.Fatalf("got %d nodes, want %d", len(pathClauses), len(expectedClauses))
	}
	for i, exp := range expectedClauses {
		if pathClauses[i].Code != exp {
			t.Errorf("pathClauses[%d] = %s, want %s", i, pathClauses[i].Code, exp)
		}
	}
}

// TestPathTo_SkipsDeprecatedPrerequisites is BR-FOUNDATION-07 at the one place
// it is easy to get wrong.
//
// The catalogue handed to PathTo excludes deprecated nodes, so a prerequisite
// that has been deprecated is simply absent from it. Treating that absence as a
// fault turned deprecating a single node into a 500 on every path that ran
// through it — which is the opposite of what deprecating a node is for.
func TestPathTo_SkipsDeprecatedPrerequisites(t *testing.T) {
	t.Parallel()

	rootID := uuid.New()
	goneID := uuid.New()
	midID := uuid.New()
	targetID := uuid.New()

	// goneID is deprecated, so it is not in the catalogue the service passes in.
	live := map[uuid.UUID]domain.Taxonomy{
		rootID:   {ID: rootID, Code: codeSentenceStructure, Position: 10},
		midID:    {ID: midID, Code: codePastSimple, Position: 30},
		targetID: {ID: targetID, Code: codePresentPerfect, Position: 40},
	}

	edges := []domain.PrerequisiteEdge{
		{NodeID: midID, RequiresNodeID: rootID},
		{NodeID: targetID, RequiresNodeID: midID},
		{NodeID: targetID, RequiresNodeID: goneID},
		{NodeID: goneID, RequiresNodeID: rootID},
	}

	path, err := domain.PathTo(live[targetID], live, edges)
	if err != nil {
		t.Fatalf("a deprecated prerequisite should shorten the path, not break it: %v", err)
	}

	want := []string{codeSentenceStructure, codePastSimple, codePresentPerfect}
	if len(path) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(path), len(want))
	}
	for i, code := range want {
		if path[i].Code != code {
			t.Errorf("path[%d] = %s, want %s", i, path[i].Code, code)
		}
	}
}
