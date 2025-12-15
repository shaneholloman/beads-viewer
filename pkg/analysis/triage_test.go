package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestComputeTriage_Empty(t *testing.T) {
	triage := ComputeTriage(nil)

	if triage.Meta.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", triage.Meta.Version)
	}
	if triage.QuickRef.OpenCount != 0 {
		t.Errorf("expected 0 open count, got %d", triage.QuickRef.OpenCount)
	}
	if len(triage.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(triage.Recommendations))
	}
}

func TestComputeTriage_BasicIssues(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "First issue",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:        "test-2",
			Title:     "Second issue",
			Status:    model.StatusOpen,
			Priority:  2,
			IssueType: model.TypeBug,
			UpdatedAt: time.Now().Add(-48 * time.Hour),
		},
		{
			ID:        "test-3",
			Title:     "Closed issue",
			Status:    model.StatusClosed,
			Priority:  1,
			IssueType: model.TypeTask,
		},
	}

	triage := ComputeTriage(issues)

	// Check counts
	if triage.QuickRef.OpenCount != 2 {
		t.Errorf("expected 2 open, got %d", triage.QuickRef.OpenCount)
	}
	if triage.ProjectHealth.Counts.Closed != 1 {
		t.Errorf("expected 1 closed, got %d", triage.ProjectHealth.Counts.Closed)
	}
	if triage.ProjectHealth.Counts.Total != 3 {
		t.Errorf("expected 3 total, got %d", triage.ProjectHealth.Counts.Total)
	}

	// Should have recommendations for open issues
	if len(triage.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// Commands should be populated
	if triage.Commands.ListReady != "bd ready" {
		t.Errorf("expected 'bd ready' command, got %s", triage.Commands.ListReady)
	}
}

func TestComputeTriage_WithDependencies(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "blocker",
			Title:     "Blocker issue",
			Status:    model.StatusOpen,
			Priority:  0,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "blocked",
			Title:     "Blocked issue",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now(),
			Dependencies: []*model.Dependency{
				{DependsOnID: "blocker", Type: model.DepBlocks},
			},
		},
	}

	triage := ComputeTriage(issues)

	// One should be blocked
	if triage.QuickRef.BlockedCount != 1 {
		t.Errorf("expected 1 blocked, got %d", triage.QuickRef.BlockedCount)
	}
	if triage.QuickRef.ActionableCount != 1 {
		t.Errorf("expected 1 actionable, got %d", triage.QuickRef.ActionableCount)
	}

	// Blocker should appear in blockers_to_clear
	found := false
	for _, b := range triage.BlockersToClear {
		if b.ID == "blocker" && b.UnblocksCount == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected blocker to appear in blockers_to_clear")
	}
}

func TestComputeTriage_TopPicks(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "A", Status: model.StatusOpen, Priority: 2, UpdatedAt: time.Now()},
		{ID: "b", Title: "B", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
		{ID: "c", Title: "C", Status: model.StatusOpen, Priority: 0, UpdatedAt: time.Now()},
		{ID: "d", Title: "D", Status: model.StatusOpen, Priority: 3, UpdatedAt: time.Now()},
	}

	triage := ComputeTriage(issues)

	// Should have top picks
	if len(triage.QuickRef.TopPicks) == 0 {
		t.Error("expected top picks")
	}
	if len(triage.QuickRef.TopPicks) > 3 {
		t.Errorf("expected max 3 top picks, got %d", len(triage.QuickRef.TopPicks))
	}
}

func TestComputeTriageWithOptions(t *testing.T) {
	issues := make([]model.Issue, 20)
	for i := 0; i < 20; i++ {
		issues[i] = model.Issue{
			ID:        string(rune('a' + i)),
			Title:     "Issue " + string(rune('A'+i)),
			Status:    model.StatusOpen,
			Priority:  i % 4,
			UpdatedAt: time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		}
	}

	opts := TriageOptions{
		TopN:       5,
		QuickWinN:  3,
		BlockerN:   2,
	}

	triage := ComputeTriageWithOptions(issues, opts)

	if len(triage.Recommendations) > 5 {
		t.Errorf("expected max 5 recommendations, got %d", len(triage.Recommendations))
	}
	if len(triage.QuickWins) > 3 {
		t.Errorf("expected max 3 quick wins, got %d", len(triage.QuickWins))
	}
}

func TestTriageRecommendation_Action(t *testing.T) {
	// Issue in progress for a long time should suggest review
	issues := []model.Issue{
		{
			ID:        "stale-wip",
			Title:     "Stale work in progress",
			Status:    model.StatusInProgress,
			Priority:  1,
			UpdatedAt: time.Now().Add(-20 * 24 * time.Hour), // 20 days old
		},
	}

	triage := ComputeTriage(issues)

	if len(triage.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}

	rec := triage.Recommendations[0]
	if rec.Action != "review" {
		t.Errorf("expected action 'review' for stale in_progress, got %s", rec.Action)
	}
}

func TestTriageHealthCounts(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Status: model.StatusOpen, Priority: 0, IssueType: model.TypeBug},
		{ID: "2", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeBug},
		{ID: "3", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask},
		{ID: "4", Status: model.StatusClosed, Priority: 2, IssueType: model.TypeFeature},
		{ID: "5", Status: model.StatusBlocked, Priority: 2, IssueType: model.TypeTask},
	}

	triage := ComputeTriage(issues)
	counts := triage.ProjectHealth.Counts

	if counts.ByType["bug"] != 2 {
		t.Errorf("expected 2 bugs, got %d", counts.ByType["bug"])
	}
	if counts.ByType["task"] != 2 {
		t.Errorf("expected 2 tasks, got %d", counts.ByType["task"])
	}
	if counts.ByPriority[1] != 2 {
		t.Errorf("expected 2 P1, got %d", counts.ByPriority[1])
	}
}

func TestTriageGraphHealth(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen},
		{ID: "b", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}}},
		{ID: "c", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "b", Type: model.DepBlocks}}},
	}

	triage := ComputeTriage(issues)
	graph := triage.ProjectHealth.Graph

	if graph.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", graph.NodeCount)
	}
	if graph.EdgeCount != 2 {
		t.Errorf("expected 2 edges, got %d", graph.EdgeCount)
	}
	if graph.HasCycles {
		t.Error("expected no cycles")
	}
}

func TestTriageWithCycles(t *testing.T) {
	// Create a cycle: a -> b -> a
	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "b", Type: model.DepBlocks}}},
		{ID: "b", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}}},
	}

	opts := TriageOptions{WaitForPhase2: true}
	triage := ComputeTriageWithOptions(issues, opts)
	graph := triage.ProjectHealth.Graph

	if !graph.HasCycles {
		t.Error("expected cycles to be detected")
	}
	if graph.CycleCount == 0 {
		t.Error("expected cycle count > 0")
	}
}

func TestTriageEmptyCommands(t *testing.T) {
	// When there are no open issues, commands should be gracefully handled
	issues := []model.Issue{
		{ID: "closed-1", Status: model.StatusClosed},
	}

	triage := ComputeTriage(issues)

	// Should not have "bd update  --status" (empty ID)
	if triage.Commands.ClaimTop == "bd update  --status=in_progress" {
		t.Error("ClaimTop should not have empty ID")
	}
	// Should have a fallback message
	if triage.Commands.ClaimTop == "" {
		t.Error("ClaimTop should not be empty")
	}
}

func TestTriageNoRecommendationsCommands(t *testing.T) {
	// Empty project
	triage := ComputeTriage(nil)

	// Commands should be valid even with no recommendations
	if triage.Commands.ListReady != "bd ready" {
		t.Errorf("expected 'bd ready', got %s", triage.Commands.ListReady)
	}
	// ClaimTop should have fallback, not empty ID
	if triage.Commands.ClaimTop == "bd update  --status=in_progress" {
		t.Error("ClaimTop should not have empty ID in command")
	}
}

func TestTriageInProgressAction(t *testing.T) {
	// Test the different staleness thresholds for in_progress items
	tests := []struct {
		name           string
		daysOld        int
		expectedAction string
	}{
		{"fresh in_progress", 5, "work"},      // < 9 days (0.3 * 30)
		{"moderate in_progress", 12, "review"}, // > 9 days, < 15 days
		{"stale in_progress", 20, "review"},    // > 15 days (0.5 * 30)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := []model.Issue{
				{
					ID:        "wip",
					Title:     tt.name,
					Status:    model.StatusInProgress,
					Priority:  1,
					UpdatedAt: time.Now().Add(-time.Duration(tt.daysOld) * 24 * time.Hour),
				},
			}

			triage := ComputeTriage(issues)
			if len(triage.Recommendations) == 0 {
				t.Fatal("expected recommendations")
			}

			rec := triage.Recommendations[0]
			if rec.Action != tt.expectedAction {
				t.Errorf("expected action %q, got %q (staleness: %.2f)",
					tt.expectedAction, rec.Action, rec.Breakdown.StalenessNorm)
			}
		})
	}
}

// ============================================================================
// Tests for bv-146 V2 Types
// ============================================================================

func TestSessionContext_Structure(t *testing.T) {
	session := SessionContext{
		AgentName: "TestAgent",
		Claims: []ClaimInfo{
			{
				BeadID:    "bv-1",
				Title:     "Test Bead",
				ClaimedAt: time.Now(),
				Files:     []string{"main.go", "test.go"},
			},
		},
		PendingHandoffs: []HandoffInfo{
			{
				FromAgent:  "OtherAgent",
				BeadID:     "bv-2",
				Message:    "Please review",
				ReceivedAt: time.Now(),
			},
		},
		RecentActivity: "Claimed bv-1",
	}

	if session.AgentName != "TestAgent" {
		t.Errorf("expected AgentName 'TestAgent', got %s", session.AgentName)
	}
	if len(session.Claims) != 1 {
		t.Errorf("expected 1 claim, got %d", len(session.Claims))
	}
	if len(session.PendingHandoffs) != 1 {
		t.Errorf("expected 1 handoff, got %d", len(session.PendingHandoffs))
	}
}

func TestTeamStatus_Structure(t *testing.T) {
	team := TeamStatus{
		ActiveAgents: []AgentSummary{
			{
				Name:     "Agent1",
				Claims:   []string{"bv-1", "bv-2"},
				LastSeen: "5m ago",
				Track:    "track-A",
			},
			{
				Name:     "Agent2",
				Claims:   []string{"bv-3"},
				LastSeen: "2m ago",
			},
		},
		TotalClaimed: 3,
		FileConflicts: []FileConflict{
			{
				File:       "main.go",
				Agents:     []string{"Agent1", "Agent2"},
				Resolution: "Agent1 should finish first",
			},
		},
		AvailableTracks:  []string{"track-B", "track-C"},
		CoordinationHint: "Focus on track-A items",
	}

	if len(team.ActiveAgents) != 2 {
		t.Errorf("expected 2 active agents, got %d", len(team.ActiveAgents))
	}
	if team.TotalClaimed != 3 {
		t.Errorf("expected 3 total claimed, got %d", team.TotalClaimed)
	}
	if len(team.FileConflicts) != 1 {
		t.Errorf("expected 1 file conflict, got %d", len(team.FileConflicts))
	}
}

func TestAlertsByLevel_Structure(t *testing.T) {
	alerts := AlertsByLevel{
		Critical: []TriageAlert{
			{
				Type:    AlertTypeCycle,
				BeadIDs: []string{"bv-1", "bv-2"},
				Message: "Circular dependency detected",
				Action:  "Break the cycle by removing one dependency",
			},
		},
		Warning: []TriageAlert{
			{
				Type:    AlertTypeStale,
				BeadID:  "bv-3",
				Message: "Bead stale for 30 days",
				Action:  "Review and update or close",
			},
		},
		Info: []TriageAlert{
			{
				Type:    AlertTypeOrphan,
				BeadID:  "bv-4",
				Message: "No dependencies",
				Action:  "Consider adding dependencies",
			},
		},
	}

	if len(alerts.Critical) != 1 {
		t.Errorf("expected 1 critical alert, got %d", len(alerts.Critical))
	}
	if alerts.Critical[0].Type != AlertTypeCycle {
		t.Errorf("expected critical alert type 'cycle', got %s", alerts.Critical[0].Type)
	}
	if len(alerts.Warning) != 1 {
		t.Errorf("expected 1 warning alert, got %d", len(alerts.Warning))
	}
	if len(alerts.Info) != 1 {
		t.Errorf("expected 1 info alert, got %d", len(alerts.Info))
	}
}

func TestSuggestion_Structure(t *testing.T) {
	suggestions := []Suggestion{
		{
			Type: SuggestionMissingDep,
			Bead: "bv-1",
			Hint: "Consider adding dependency on bv-2",
		},
		{
			Type:  SuggestionStaleClaim,
			Bead:  "bv-3",
			Agent: "IdleAgent",
			Hint:  "Claim is 7 days old with no activity",
		},
	}

	if len(suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].Type != SuggestionMissingDep {
		t.Errorf("expected type 'missing_dep', got %s", suggestions[0].Type)
	}
}

func TestTriageResultV2_Structure(t *testing.T) {
	result := TriageResultV2{
		Meta: TriageMetaV2{
			Command:       "--robot-triage",
			GeneratedAt:   time.Now(),
			DataHash:      "abc123",
			AgentIdentity: "TestAgent",
			Phase2Ready:   true,
			IssueCount:    10,
			ComputeTimeMs: 50,
		},
		QuickRef: QuickRefV2{
			TopPick:         "bv-1: Fix bug",
			CriticalAlerts:  1,
			Warnings:        2,
			YourClaims:      1,
			TeamActive:      3,
			ActionableItems: 5,
			BlockedItems:    2,
			Status:          "ready_to_work",
		},
		Alerts: AlertsByLevel{
			Critical: []TriageAlert{},
			Warning:  []TriageAlert{},
			Info:     []TriageAlert{},
		},
		TopPick: &TopPickV2{
			BeadID:       "bv-1",
			Title:        "Fix critical bug",
			Type:         "bug",
			Priority:     "P0",
			Score:        0.95,
			Why:          []string{"High priority", "Unblocks 3 items"},
			Labels:       []string{"critical", "backend"},
			ClaimCommand: "bd update bv-1 --status=in_progress",
		},
		ProjectHealth: ProjectHealthV2{
			Counts: HealthCounts{
				Total:      10,
				Open:       7,
				Closed:     3,
				Blocked:    2,
				Actionable: 5,
			},
			Graph: GraphHealth{
				NodeCount:   10,
				EdgeCount:   8,
				Density:     0.18,
				HasCycles:   false,
				Phase2Ready: true,
			},
		},
		Commands: CommandHelpersV2{
			ClaimTopPick:  "bd update bv-1 --status=in_progress",
			ShowTopPick:   "bd show bv-1",
			SeeFullPlan:   "bv --robot-plan",
			RefreshTriage: "bv --robot-triage",
		},
	}

	if result.Meta.Command != "--robot-triage" {
		t.Errorf("expected command '--robot-triage', got %s", result.Meta.Command)
	}
	if result.QuickRef.Status != "ready_to_work" {
		t.Errorf("expected status 'ready_to_work', got %s", result.QuickRef.Status)
	}
	if result.TopPick == nil {
		t.Error("expected TopPick to be non-nil")
	}
	if result.TopPick.BeadID != "bv-1" {
		t.Errorf("expected TopPick BeadID 'bv-1', got %s", result.TopPick.BeadID)
	}
}
