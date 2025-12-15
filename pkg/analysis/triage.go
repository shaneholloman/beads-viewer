package analysis

import (
	"fmt"
	"sort"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// TriageResult is the unified output for --robot-triage
// Designed as a single entry point for AI agents to get everything they need
type TriageResult struct {
	Meta            TriageMeta         `json:"meta"`
	QuickRef        QuickRef           `json:"quick_ref"`
	Recommendations []Recommendation   `json:"recommendations"`
	QuickWins       []QuickWin         `json:"quick_wins"`
	BlockersToClear []BlockerItem      `json:"blockers_to_clear"`
	ProjectHealth   ProjectHealth      `json:"project_health"`
	Alerts          []Alert            `json:"alerts,omitempty"`
	Commands        CommandHelpers     `json:"commands"`
}

// TriageMeta contains metadata about the triage computation
type TriageMeta struct {
	Version      string    `json:"version"`
	GeneratedAt  time.Time `json:"generated_at"`
	Phase2Ready  bool      `json:"phase2_ready"`
	IssueCount   int       `json:"issue_count"`
	ComputeTimeMs int64    `json:"compute_time_ms"`
}

// QuickRef provides at-a-glance summary for fast decisions
type QuickRef struct {
	OpenCount       int       `json:"open_count"`
	ActionableCount int       `json:"actionable_count"`
	BlockedCount    int       `json:"blocked_count"`
	InProgressCount int       `json:"in_progress_count"`
	TopPicks        []TopPick `json:"top_picks"` // Top 3 recommended items
}

// TopPick is a condensed recommendation for quick reference
type TopPick struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Score    float64  `json:"score"`
	Reasons  []string `json:"reasons"`
	Unblocks int      `json:"unblocks"` // How many items this unblocks
}

// Recommendation is an actionable item with full context
type Recommendation struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	Priority    int            `json:"priority"`
	Labels      []string       `json:"labels"`
	Score       float64        `json:"score"`
	Breakdown   ScoreBreakdown `json:"breakdown"`
	Action      string         `json:"action"`   // "work", "review", "unblock"
	Reasons     []string       `json:"reasons"`
	UnblocksIDs []string       `json:"unblocks_ids,omitempty"`
	BlockedBy   []string       `json:"blocked_by,omitempty"`
}

// QuickWin represents a low-effort, high-impact item
type QuickWin struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
	UnblocksIDs []string `json:"unblocks_ids,omitempty"`
}

// BlockerItem represents an item that blocks significant downstream work
type BlockerItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	UnblocksCount int      `json:"unblocks_count"`
	UnblocksIDs   []string `json:"unblocks_ids"`
	Actionable    bool     `json:"actionable"` // Can we work on this now?
	BlockedBy     []string `json:"blocked_by,omitempty"`
}

// ProjectHealth provides overall project status
type ProjectHealth struct {
	Counts     HealthCounts `json:"counts"`
	Graph      GraphHealth  `json:"graph"`
	Velocity   *Velocity    `json:"velocity,omitempty"`   // nil until labels view ready
	Staleness  *Staleness   `json:"staleness,omitempty"`  // nil until history ready
}

// HealthCounts is basic issue statistics
type HealthCounts struct {
	Total      int            `json:"total"`
	Open       int            `json:"open"`
	Closed     int            `json:"closed"`
	Blocked    int            `json:"blocked"`
	Actionable int            `json:"actionable"`
	ByStatus   map[string]int `json:"by_status"`
	ByType     map[string]int `json:"by_type"`
	ByPriority map[int]int    `json:"by_priority"`
}

// GraphHealth summarizes dependency graph metrics
type GraphHealth struct {
	NodeCount    int     `json:"node_count"`
	EdgeCount    int     `json:"edge_count"`
	Density      float64 `json:"density"`
	HasCycles    bool    `json:"has_cycles"`
	CycleCount   int     `json:"cycle_count,omitempty"`
	Phase2Ready  bool    `json:"phase2_ready"`
}

// Velocity tracks work completion rate (future: from labels view)
type Velocity struct {
	ClosedLast7Days  int     `json:"closed_last_7_days"`
	ClosedLast30Days int     `json:"closed_last_30_days"`
	AvgDaysToClose   float64 `json:"avg_days_to_close"`
}

// Staleness tracks stale issues (future: from history)
type Staleness struct {
	StaleCount       int      `json:"stale_count"`        // Issues with no activity > threshold
	StalestIssueID   string   `json:"stalest_issue_id"`
	StalestIssueDays int      `json:"stalest_issue_days"`
	ThresholdDays    int      `json:"threshold_days"`
}

// Alert represents a proactive warning (future: from alerts engine)
type Alert struct {
	Type     string `json:"type"`     // "stale", "velocity_drop", "cycle", "duplicate"
	Severity string `json:"severity"` // "info", "warning", "error"
	Message  string `json:"message"`
	IssueID  string `json:"issue_id,omitempty"`
	IssueIDs []string `json:"issue_ids,omitempty"`
}

// CommandHelpers provides copy-paste commands for common actions
type CommandHelpers struct {
	ClaimTop       string `json:"claim_top"`        // bd update <id> --status=in_progress
	ShowTop        string `json:"show_top"`         // bd show <id>
	ListReady      string `json:"list_ready"`       // bd ready
	ListBlocked    string `json:"list_blocked"`     // bd blocked
	RefreshTriage  string `json:"refresh_triage"`   // bv --robot-triage
}

// ComputeTriage generates a unified triage result from issues
func ComputeTriage(issues []model.Issue) TriageResult {
	return ComputeTriageWithOptions(issues, TriageOptions{})
}

// TriageOptions configures triage computation
type TriageOptions struct {
	TopN           int  // Number of recommendations (default 10)
	QuickWinN      int  // Number of quick wins (default 5)
	BlockerN       int  // Number of blockers to show (default 5)
	WaitForPhase2  bool // Block until Phase 2 metrics ready
}

// ComputeTriageWithOptions generates triage with custom options
func ComputeTriageWithOptions(issues []model.Issue, opts TriageOptions) TriageResult {
	start := time.Now()

	// Set defaults
	if opts.TopN <= 0 {
		opts.TopN = 10
	}
	if opts.QuickWinN <= 0 {
		opts.QuickWinN = 5
	}
	if opts.BlockerN <= 0 {
		opts.BlockerN = 5
	}

	// Build analyzer
	analyzer := NewAnalyzer(issues)
	stats := analyzer.AnalyzeAsync()

	if opts.WaitForPhase2 {
		stats.WaitForPhase2()
	}

	// Compute impact scores
	impactScores := analyzer.ComputeImpactScores()

	// Get execution plan for unblock analysis (currently unused but kept for future phases)
	_ = analyzer.GetExecutionPlan()

	// Build unblocks map
	unblocksMap := buildUnblocksMap(analyzer, issues)

	// Compute counts
	counts := computeCounts(issues, analyzer)

	// Build recommendations
	recommendations := buildRecommendations(impactScores, analyzer, unblocksMap, opts.TopN)

	// Build quick wins
	quickWins := buildQuickWins(impactScores, unblocksMap, opts.QuickWinN)

	// Build blockers to clear
	blockersToClear := buildBlockersToClear(analyzer, unblocksMap, opts.BlockerN)

	// Build top picks for quick ref
	topPicks := buildTopPicks(recommendations, 3)

	// Determine top issue for commands
	topID := ""
	if len(recommendations) > 0 {
		topID = recommendations[0].ID
	}

	elapsed := time.Since(start)

	return TriageResult{
		Meta: TriageMeta{
			Version:       "1.0.0",
			GeneratedAt:   time.Now(),
			Phase2Ready:   stats.IsPhase2Ready(),
			IssueCount:    len(issues),
			ComputeTimeMs: elapsed.Milliseconds(),
		},
		QuickRef: QuickRef{
			OpenCount:       counts.Open,
			ActionableCount: counts.Actionable,
			BlockedCount:    counts.Blocked,
			InProgressCount: counts.ByStatus["in_progress"],
			TopPicks:        topPicks,
		},
		Recommendations: recommendations,
		QuickWins:       quickWins,
		BlockersToClear: blockersToClear,
		ProjectHealth: ProjectHealth{
			Counts: counts,
			Graph:  buildGraphHealth(stats),
			// Velocity and Staleness are nil until those features are implemented
		},
		Commands: buildCommands(topID),
	}
}

// buildUnblocksMap computes what each issue unblocks
func buildUnblocksMap(analyzer *Analyzer, issues []model.Issue) map[string][]string {
	unblocksMap := make(map[string][]string)
	for _, issue := range issues {
		if issue.Status == model.StatusClosed {
			continue
		}
		unblocksMap[issue.ID] = analyzer.computeUnblocks(issue.ID)
	}
	return unblocksMap
}

// computeCounts tallies issues by various dimensions
func computeCounts(issues []model.Issue, analyzer *Analyzer) HealthCounts {
	counts := HealthCounts{
		Total:      len(issues),
		ByStatus:   make(map[string]int),
		ByType:     make(map[string]int),
		ByPriority: make(map[int]int),
	}

	actionable := analyzer.GetActionableIssues()
	actionableSet := make(map[string]bool, len(actionable))
	for _, a := range actionable {
		actionableSet[a.ID] = true
	}

	for _, issue := range issues {
		counts.ByStatus[string(issue.Status)]++
		counts.ByType[string(issue.IssueType)]++
		counts.ByPriority[issue.Priority]++

		if issue.Status == model.StatusClosed {
			counts.Closed++
		} else {
			counts.Open++
			if actionableSet[issue.ID] {
				counts.Actionable++
			} else {
				counts.Blocked++
			}
		}
	}

	return counts
}

// buildRecommendations creates detailed recommendations from impact scores
func buildRecommendations(scores []ImpactScore, analyzer *Analyzer, unblocksMap map[string][]string, limit int) []Recommendation {
	if len(scores) > limit {
		scores = scores[:limit]
	}

	recommendations := make([]Recommendation, 0, len(scores))
	for _, score := range scores {
		issue := analyzer.GetIssue(score.IssueID)
		if issue == nil {
			continue
		}

		// Determine action and reasons
		action, reasons := determineAction(score, unblocksMap[score.IssueID], issue)

		// Get labels (already strings in model.Issue)
		labels := issue.Labels

		// Get blocked by
		blockedBy := analyzer.GetOpenBlockers(score.IssueID)

		rec := Recommendation{
			ID:          score.IssueID,
			Title:       score.Title,
			Type:        string(issue.IssueType),
			Status:      score.Status,
			Priority:    score.Priority,
			Labels:      labels,
			Score:       score.Score,
			Breakdown:   score.Breakdown,
			Action:      action,
			Reasons:     reasons,
			UnblocksIDs: unblocksMap[score.IssueID],
		}
		if len(blockedBy) > 0 {
			rec.BlockedBy = blockedBy
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// determineAction decides what action to take and why
func determineAction(score ImpactScore, unblocks []string, issue *model.Issue) (string, []string) {
	var reasons []string
	action := "work"

	// High PageRank = central to project
	if score.Breakdown.PageRankNorm > 0.3 {
		reasons = append(reasons, fmt.Sprintf("High centrality (PageRank: %.2f)", score.Breakdown.PageRankNorm))
	}

	// High Betweenness = bottleneck
	if score.Breakdown.BetweennessNorm > 0.5 {
		reasons = append(reasons, fmt.Sprintf("Critical bottleneck (Betweenness: %.2f)", score.Breakdown.BetweennessNorm))
	}

	// High blocker ratio = unblocks many
	if len(unblocks) >= 3 {
		reasons = append(reasons, fmt.Sprintf("Unblocks %d downstream items", len(unblocks)))
		action = "unblock" // Priority action
	} else if len(unblocks) > 0 {
		reasons = append(reasons, fmt.Sprintf("Unblocks %d item(s)", len(unblocks)))
	}

	// Staleness - check if issue is stale
	isStale := score.Breakdown.StalenessNorm > 0.5
	if isStale {
		days := int(score.Breakdown.StalenessNorm * 30)
		reasons = append(reasons, fmt.Sprintf("Stale for %d+ days", days))
	}

	// In progress items may need review
	if issue.Status == model.StatusInProgress {
		if isStale {
			// Very stale in_progress - definitely needs review
			action = "review"
			reasons = append(reasons, "In progress but appears stuck")
		} else if score.Breakdown.StalenessNorm > 0.3 {
			// Moderately stale in_progress - might need attention
			action = "review"
			reasons = append(reasons, "In progress - may need attention")
		}
	}

	// Priority consideration
	if score.Priority <= 1 {
		reasons = append(reasons, fmt.Sprintf("High priority (P%d)", score.Priority))
	}

	// Default reason if none
	if len(reasons) == 0 {
		reasons = append(reasons, "Good candidate for work")
	}

	return action, reasons
}

// buildQuickWins finds low-complexity, high-impact items
func buildQuickWins(scores []ImpactScore, unblocksMap map[string][]string, limit int) []QuickWin {
	// Quick wins: high score but likely simple (no deep dependency chains)
	// Heuristic: items that unblock others but have low blocker ratio themselves

	type candidate struct {
		score   ImpactScore
		unblocks []string
		quickWinScore float64
	}

	var candidates []candidate
	for _, score := range scores {
		unblocks := unblocksMap[score.IssueID]
		// Quick win score: benefits unblocking, penalizes complexity
		// - High unblock count = good (helps project progress)
		// - Low BlockerRatioNorm = few things depend on this = safer to work on
		// - High priority number (P3, P4) = likely simpler tasks
		qwScore := float64(len(unblocks)) * 0.5
		if score.Breakdown.BlockerRatioNorm < 0.3 {
			qwScore += 0.3 // Bonus: not a critical bottleneck (fewer downstream deps)
		}
		if score.Priority >= 3 {
			qwScore += 0.2 // Bonus: lower priority often means simpler
		}
		candidates = append(candidates, candidate{score, unblocks, qwScore})
	}

	// Sort by quick win score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].quickWinScore > candidates[j].quickWinScore
	})

	quickWins := make([]QuickWin, 0, limit)
	for i := 0; i < len(candidates) && i < limit; i++ {
		c := candidates[i]
		reason := "Low complexity"
		if len(c.unblocks) > 0 {
			reason = fmt.Sprintf("Unblocks %d items", len(c.unblocks))
		}

		quickWins = append(quickWins, QuickWin{
			ID:          c.score.IssueID,
			Title:       c.score.Title,
			Score:       c.quickWinScore,
			Reason:      reason,
			UnblocksIDs: c.unblocks,
		})
	}

	return quickWins
}

// buildBlockersToClear finds items that block the most downstream work
func buildBlockersToClear(analyzer *Analyzer, unblocksMap map[string][]string, limit int) []BlockerItem {
	type blocker struct {
		id       string
		title    string
		unblocks []string
	}

	actionable := analyzer.GetActionableIssues()
	actionableSet := make(map[string]bool, len(actionable))
	for _, a := range actionable {
		actionableSet[a.ID] = true
	}

	var blockers []blocker
	for id, unblocks := range unblocksMap {
		if len(unblocks) == 0 {
			continue
		}
		issue := analyzer.GetIssue(id)
		if issue == nil || issue.Status == model.StatusClosed {
			continue
		}
		blockers = append(blockers, blocker{
			id:       id,
			title:    issue.Title,
			unblocks: unblocks,
		})
	}

	// Sort by unblocks count descending
	sort.Slice(blockers, func(i, j int) bool {
		return len(blockers[i].unblocks) > len(blockers[j].unblocks)
	})

	result := make([]BlockerItem, 0, limit)
	for i := 0; i < len(blockers) && i < limit; i++ {
		b := blockers[i]
		item := BlockerItem{
			ID:            b.id,
			Title:         b.title,
			UnblocksCount: len(b.unblocks),
			UnblocksIDs:   b.unblocks,
			Actionable:    actionableSet[b.id],
		}
		if !item.Actionable {
			item.BlockedBy = analyzer.GetOpenBlockers(b.id)
		}
		result = append(result, item)
	}

	return result
}

// buildTopPicks creates condensed top picks from recommendations
func buildTopPicks(recommendations []Recommendation, limit int) []TopPick {
	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	picks := make([]TopPick, 0, len(recommendations))
	for _, rec := range recommendations {
		picks = append(picks, TopPick{
			ID:       rec.ID,
			Title:    rec.Title,
			Score:    rec.Score,
			Reasons:  rec.Reasons,
			Unblocks: len(rec.UnblocksIDs),
		})
	}

	return picks
}

// buildGraphHealth constructs graph health metrics from stats
func buildGraphHealth(stats *GraphStats) GraphHealth {
	// Call Cycles() once to avoid duplicate work (it makes a copy each time)
	cycles := stats.Cycles()
	cycleCount := 0
	if cycles != nil {
		cycleCount = len(cycles)
	}

	return GraphHealth{
		NodeCount:   stats.NodeCount,
		EdgeCount:   stats.EdgeCount,
		Density:     stats.Density,
		HasCycles:   cycleCount > 0,
		CycleCount:  cycleCount,
		Phase2Ready: stats.IsPhase2Ready(),
	}
}

// buildCommands constructs helper commands, handling empty topID gracefully
func buildCommands(topID string) CommandHelpers {
	claimTop := "bd ready  # No top pick available"
	showTop := "bd ready  # No top pick available"
	if topID != "" {
		claimTop = fmt.Sprintf("bd update %s --status=in_progress", topID)
		showTop = fmt.Sprintf("bd show %s", topID)
	}

	return CommandHelpers{
		ClaimTop:      claimTop,
		ShowTop:       showTop,
		ListReady:     "bd ready",
		ListBlocked:   "bd blocked",
		RefreshTriage: "bv --robot-triage",
	}
}

// ============================================================================
// Multi-Agent Coordination Types (bv-146)
// These types enable team awareness and conflict detection for agent swarms
// ============================================================================

// SessionContext provides the current agent's session information
type SessionContext struct {
	AgentName       string       `json:"agent_name"`
	Claims          []ClaimInfo  `json:"claims"`
	PendingHandoffs []HandoffInfo `json:"pending_handoffs"`
	RecentActivity  string       `json:"recent_activity,omitempty"`
}

// ClaimInfo represents an issue claimed by an agent
type ClaimInfo struct {
	BeadID    string    `json:"bead_id"`
	Title     string    `json:"title"`
	ClaimedAt time.Time `json:"claimed_at"`
	Files     []string  `json:"files,omitempty"`
}

// HandoffInfo represents a handoff waiting for an agent
type HandoffInfo struct {
	FromAgent  string    `json:"from_agent"`
	BeadID     string    `json:"bead_id"`
	Message    string    `json:"message"`
	ReceivedAt time.Time `json:"received_at"`
}

// TeamStatus provides visibility into other agents working on the project
type TeamStatus struct {
	ActiveAgents     []AgentSummary `json:"active_agents"`
	TotalClaimed     int            `json:"total_claimed"`
	FileConflicts    []FileConflict `json:"file_conflicts,omitempty"`
	AvailableTracks  []string       `json:"available_tracks,omitempty"`
	CoordinationHint string         `json:"coordination_hint,omitempty"`
}

// AgentSummary provides basic info about another agent
type AgentSummary struct {
	Name     string   `json:"name"`
	Claims   []string `json:"claims"`
	LastSeen string   `json:"last_seen"`
	Track    string   `json:"track,omitempty"`
}

// FileConflict indicates potential conflicts between agents
type FileConflict struct {
	File       string   `json:"file"`
	Agents     []string `json:"agents"`
	Resolution string   `json:"resolution,omitempty"` // Suggested resolution
}

// ============================================================================
// Alert Types (bv-146)
// Structured alerts by severity for proactive issue detection
// ============================================================================

// AlertsByLevel groups alerts by severity for prioritized display
type AlertsByLevel struct {
	Critical []TriageAlert `json:"critical"`
	Warning  []TriageAlert `json:"warning"`
	Info     []TriageAlert `json:"info"`
}

// TriageAlert is a specific alert with actionable guidance
type TriageAlert struct {
	Type    string   `json:"type"`              // "cycle", "stale", "velocity_drop", "duplicate", etc.
	BeadID  string   `json:"bead_id,omitempty"` // Single bead alerts
	BeadIDs []string `json:"bead_ids,omitempty"` // Multi-bead alerts (e.g., cycles)
	Message string   `json:"message"`
	Action  string   `json:"action"` // What to do about it
}

// AlertType constants for alert classification
const (
	AlertTypeCycle         = "cycle"
	AlertTypeStale         = "stale"
	AlertTypeVelocityDrop  = "velocity_drop"
	AlertTypeDuplicate     = "duplicate"
	AlertTypeOrphan        = "orphan"
	AlertTypePriorityDrift = "priority_drift"
	AlertTypeBlockedChain  = "blocked_chain"
)

// AlertSeverity constants
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// ============================================================================
// Extended Project Health Types (bv-146)
// Additional health metrics for comprehensive project status
// ============================================================================

// LabelAttention flags labels that need attention
type LabelAttention struct {
	Label  string `json:"label"`
	Health int    `json:"health"` // 0-100 health score
	Issue  string `json:"issue"`  // Description of the problem
}

// VelocityInfo tracks work completion trends (extended from Velocity)
type VelocityInfo struct {
	ClosedPerWeek int    `json:"closed_per_week"`
	Trend         string `json:"trend"` // "improving", "stable", "declining"
}

// DriftSummary provides baseline drift information
type DriftSummary struct {
	BaselineAge        string `json:"baseline_age,omitempty"`
	SignificantChanges bool   `json:"significant_changes"`
	AlertCount         int    `json:"alert_count,omitempty"`
}

// Suggestion provides hygiene hints for the project
type Suggestion struct {
	Type  string `json:"type"`  // "missing_dep", "stale_claim", "duplicate", "orphan"
	Bead  string `json:"bead,omitempty"`
	Agent string `json:"agent,omitempty"`
	Hint  string `json:"hint"`
}

// SuggestionType constants
const (
	SuggestionMissingDep  = "missing_dep"
	SuggestionStaleClaim  = "stale_claim"
	SuggestionDuplicate   = "duplicate"
	SuggestionOrphanBead  = "orphan_bead"
	SuggestionPriorityGap = "priority_gap"
)

// ============================================================================
// Extended TriageResult (bv-146)
// Full triage result with multi-agent and enhanced alert support
// ============================================================================

// TriageResultV2 is the extended triage output with multi-agent support
// This is designed for future use when agent swarm features are fully implemented
type TriageResultV2 struct {
	Meta            TriageMetaV2     `json:"meta"`
	QuickRef        QuickRefV2       `json:"quick_ref"`
	Alerts          AlertsByLevel    `json:"alerts"`
	TopPick         *TopPickV2       `json:"top_pick"`         // nil if nothing to work on
	YourSession     *SessionContext  `json:"your_session"`     // nil if no agent identity
	Team            TeamStatus       `json:"team"`
	Recommendations []Recommendation `json:"recommendations"`
	QuickWins       []QuickWin       `json:"quick_wins"`
	BlockersToClear []BlockerItem    `json:"blockers_to_clear"`
	ProjectHealth   ProjectHealthV2  `json:"project_health"`
	Suggestions     []Suggestion     `json:"suggestions"`
	Commands        CommandHelpersV2 `json:"commands"`
}

// TriageMetaV2 is the extended metadata for triage v2
type TriageMetaV2 struct {
	Command       string    `json:"command"`
	GeneratedAt   time.Time `json:"generated_at"`
	DataHash      string    `json:"data_hash"`
	AgentIdentity string    `json:"agent_identity,omitempty"`
	Phase2Ready   bool      `json:"phase2_ready"`
	IssueCount    int       `json:"issue_count"`
	ComputeTimeMs int64     `json:"compute_time_ms"`
}

// QuickRefV2 is the extended quick reference
type QuickRefV2 struct {
	TopPick         string `json:"top_pick"`          // "bv-43: Title"
	CriticalAlerts  int    `json:"critical_alerts"`
	Warnings        int    `json:"warnings"`
	YourClaims      int    `json:"your_claims"`
	TeamActive      int    `json:"team_active"`
	ActionableItems int    `json:"actionable_items"`
	BlockedItems    int    `json:"blocked_items"`
	Status          string `json:"status"` // "ready_to_work", "alerts_pending", "all_claimed"
}

// TopPickV2 is the extended top pick with full context
type TopPickV2 struct {
	BeadID          string   `json:"bead_id"`
	Title           string   `json:"title"`
	Type            string   `json:"type"`
	Priority        string   `json:"priority"`
	Score           float64  `json:"score"`
	Why             []string `json:"why"` // Human-readable reasons with emoji
	Labels          []string `json:"labels"`
	EstimatedImpact string   `json:"estimated_impact,omitempty"`
	FilesLikely     []string `json:"files_likely,omitempty"`
	ClaimCommand    string   `json:"claim_command"`
}

// ProjectHealthV2 is the extended project health with more metrics
type ProjectHealthV2 struct {
	Counts          HealthCounts     `json:"counts"`
	Graph           GraphHealth      `json:"graph"`
	LabelsAttention []LabelAttention `json:"labels_attention,omitempty"`
	Velocity        *VelocityInfo    `json:"velocity,omitempty"`
	Drift           *DriftSummary    `json:"drift,omitempty"`
}

// CommandHelpersV2 is the extended command helpers
type CommandHelpersV2 struct {
	ClaimTopPick     string `json:"claim_top_pick"`
	ShowTopPick      string `json:"show_top_pick"`
	ContinueYourWork string `json:"continue_your_work,omitempty"`
	SeeFullPlan      string `json:"see_full_plan"`
	ReleaseClaims    string `json:"release_claims,omitempty"`
	SendHandoff      string `json:"send_handoff,omitempty"`
	RefreshTriage    string `json:"refresh_triage"`
}
