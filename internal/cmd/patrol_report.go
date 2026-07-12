package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/formula"
	"github.com/steveyegge/gastown/internal/style"
)

var (
	patrolReportSummary string
	patrolReportSteps   string
)

var patrolReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Close patrol cycle with summary and start next cycle",
	Long: `Close the current patrol cycle, recording a summary of observations,
then automatically start a new patrol cycle.

This replaces the old squash+new pattern with a single command that:
  1. Closes the current patrol root wisp with the summary
  2. Creates a new patrol wisp for the next cycle

The summary is stored on the patrol root wisp for audit purposes.
The --steps flag records which patrol steps were executed vs skipped,
making shortcutting visible in the ledger.

Example shape (replace the audit with every step shown by gt prime):
	gt patrol report --summary "All clear" --steps "<step-1>:OK,<step-2>:SKIP,..."`,
	RunE: runPatrolReport,
}

func init() {
	patrolReportCmd.Flags().StringVar(&patrolReportSummary, "summary", "", "Brief summary of patrol observations (required)")
	patrolReportCmd.Flags().StringVar(&patrolReportSteps, "steps", "", "Step audit: comma-separated step:STATUS pairs (e.g., heartbeat:OK,inbox-check:OK)")
	_ = patrolReportCmd.MarkFlagRequired("summary")
	_ = patrolReportCmd.MarkFlagRequired("steps")
}

func runPatrolReport(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(patrolReportSteps) == "" {
		return fmt.Errorf("--steps is required: report every patrol step as step_id:OK or step_id:SKIP")
	}

	// Resolve role
	roleInfo, err := GetRole()
	if err != nil {
		return fmt.Errorf("detecting role: %w", err)
	}

	roleName := string(roleInfo.Role)

	// Build config based on role
	var cfg PatrolConfig
	switch roleInfo.Role {
	case RoleDeacon:
		cfg = PatrolConfig{
			RoleName:      "deacon",
			PatrolMolName: constants.MolDeaconPatrol,
			BeadsDir:      roleInfo.TownRoot,
			Assignee:      deaconPatrolAssignee(),
		}
	case RoleWitness:
		cfg = PatrolConfig{
			RoleName:      "witness",
			PatrolMolName: constants.MolWitnessPatrol,
			BeadsDir:      roleInfo.TownRoot,
			Assignee:      roleInfo.Rig + "/witness",
		}
	case RoleRefinery:
		cfg = PatrolConfig{
			RoleName:      "refinery",
			PatrolMolName: constants.MolRefineryPatrol,
			BeadsDir:      roleInfo.TownRoot,
			Assignee:      roleInfo.Rig + "/refinery",
			ExtraVars:     buildRefineryPatrolVars(roleInfo),
		}
	default:
		return fmt.Errorf("unsupported role for patrol report: %q", roleName)
	}
	if err := validateStepAudit(cfg.PatrolMolName, patrolReportSteps); err != nil {
		return err
	}

	// Find the active patrol
	patrolID, _, hasPatrol, findErr := findActivePatrol(cfg)
	if findErr != nil {
		return fmt.Errorf("finding active patrol: %w", findErr)
	}
	if !hasPatrol {
		return fmt.Errorf("no active patrol found for %s", cfg.RoleName)
	}

	// Close the current patrol root with the summary
	b := beads.New(cfg.BeadsDir)

	// Build step audit checklist
	stepAudit := buildStepAudit(cfg.PatrolMolName, patrolReportSteps)

	// Update the description with the patrol summary and step audit
	desc := fmt.Sprintf("Patrol report: %s\n\n%s", patrolReportSummary, stepAudit)
	currentRoot, err := b.Show(patrolID)
	if err != nil {
		return fmt.Errorf("loading current patrol %s before audit persistence: %w", patrolID, err)
	}
	if attachment := beads.ParseAttachmentFields(currentRoot); attachment != nil {
		desc = beads.SetAttachmentFields(&beads.Issue{Description: desc}, attachment)
	}
	if err := b.Update(patrolID, beads.UpdateOptions{
		Description: &desc,
	}); err != nil {
		return fmt.Errorf("persisting patrol summary and step audit: %w", err)
	}

	// Print the step audit for visibility
	fmt.Println(stepAudit)

	// Create and hook the successor before closing the current patrol. The current
	// root is exempt from pre-spawn cleanup, so a spawn failure leaves a usable
	// hook that can retry instead of creating a patrol gap.
	spawnCfg := cfg
	spawnCfg.BurnExemptIDs = append(spawnCfg.BurnExemptIDs, patrolID)
	newPatrolID, err := autoSpawnPatrol(spawnCfg)
	if err != nil {
		return fmt.Errorf("starting next patrol cycle; current patrol %s remains active: %w", patrolID, err)
	}

	// Close all materialized patrol steps, then the reported root. If either
	// close fails, roll back the successor so the current patrol remains the
	// single authoritative cycle and can retry safely.
	closed, closeDescErr := forceClosePatrolSteps(b, patrolID)
	if closeDescErr != nil {
		return rollbackPatrolSpawn(spawnCfg, newPatrolID,
			fmt.Errorf("closing steps of patrol %s (closed %d): %w", patrolID, closed, closeDescErr))
	}

	// Close the patrol root
	if err := b.ForceCloseWithReason("patrol cycle complete: "+patrolReportSummary, patrolID); err != nil {
		return fmt.Errorf("successor %s remains active after failing to retire patrol %s: %w", newPatrolID, patrolID, err)
	}

	fmt.Printf("%s Closed patrol %s\n", style.Success.Render("✓"), patrolID)
	fmt.Printf("%s Started new patrol: %s\n", style.Success.Render("✓"), newPatrolID)
	return nil
}

func validateStepAudit(formulaName, stepsFlag string) error {
	content, err := formula.GetEmbeddedFormulaContent(formulaName)
	if err != nil {
		return fmt.Errorf("validating --steps: loading formula %s: %w", formulaName, err)
	}
	f, err := formula.Parse(content)
	if err != nil {
		return fmt.Errorf("validating --steps: parsing formula %s: %w", formulaName, err)
	}

	canonical := f.GetAllIDs()
	known := make(map[string]bool, len(canonical))
	for _, stepID := range canonical {
		known[stepID] = true
	}

	seen := make(map[string]bool, len(canonical))
	for _, entry := range strings.Split(stepsFlag, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return fmt.Errorf("invalid --steps entry %q: expected step_id:OK or step_id:SKIP", entry)
		}
		stepID := strings.TrimSpace(parts[0])
		status := strings.ToUpper(strings.TrimSpace(parts[1]))
		if !known[stepID] {
			return fmt.Errorf("unknown step %q in --steps for %s", stepID, formulaName)
		}
		if seen[stepID] {
			return fmt.Errorf("duplicate step %q in --steps", stepID)
		}
		if status != "OK" && status != "SKIP" &&
			!(strings.HasPrefix(status, "OK(") && strings.HasSuffix(status, ")")) &&
			!(strings.HasPrefix(status, "SKIP(") && strings.HasSuffix(status, ")")) {
			return fmt.Errorf("invalid status %q for step %q: use OK or SKIP", status, stepID)
		}
		seen[stepID] = true
	}
	for _, stepID := range canonical {
		if !seen[stepID] {
			return fmt.Errorf("missing step %q in --steps for %s", stepID, formulaName)
		}
	}
	return nil
}

// buildStepAudit builds a step checklist from the formula's steps and the
// reported step results. Format:
//
//	Steps: heartbeat OK | inbox-check OK | orphan-cleanup SKIP | ... (14/25)
//
// If stepsFlag is empty, returns a line indicating the audit was not reported.
func buildStepAudit(formulaName string, stepsFlag string) string {
	// Load the formula to get the canonical step list
	content, err := formula.GetEmbeddedFormulaContent(formulaName)
	if err != nil {
		if stepsFlag == "" {
			return "Steps: NOT REPORTED (formula not found)"
		}
		// Can't validate without the formula, but still show what was reported
		return fmt.Sprintf("Steps: %s (unvalidated — formula not found)", stepsFlag)
	}

	f, err := formula.Parse(content)
	if err != nil {
		if stepsFlag == "" {
			return "Steps: NOT REPORTED (formula parse error)"
		}
		return fmt.Sprintf("Steps: %s (unvalidated — formula parse error)", stepsFlag)
	}

	allStepIDs := f.GetAllIDs()
	if len(allStepIDs) == 0 {
		return ""
	}

	if stepsFlag == "" {
		return fmt.Sprintf("Steps: NOT REPORTED (?/%d)", len(allStepIDs))
	}

	// Parse the reported step results
	reported := parseStepResults(stepsFlag)

	// Build the audit line: map each formula step to its reported status
	var parts []string
	okCount := 0
	for _, stepID := range allStepIDs {
		status, ok := reported[stepID]
		if !ok {
			status = "SKIP"
		}
		if isPatrolStepOK(status) {
			okCount++
		}
		parts = append(parts, stepID+" "+status)
	}

	return fmt.Sprintf("Steps: %s (%d/%d)", strings.Join(parts, " | "), okCount, len(allStepIDs))
}

func isPatrolStepOK(status string) bool {
	status = strings.TrimSpace(strings.ToUpper(status))
	return status == "OK" || strings.HasPrefix(status, "OK(")
}

// parseStepResults parses a comma-separated string of step:STATUS pairs.
// Returns a map of step ID to uppercase status.
// Example input: "heartbeat:OK,inbox-check:OK,orphan-cleanup:SKIP"
func parseStepResults(stepsFlag string) map[string]string {
	results := make(map[string]string)
	for _, entry := range strings.Split(stepsFlag, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) == 2 {
			results[strings.TrimSpace(parts[0])] = strings.ToUpper(strings.TrimSpace(parts[1]))
		}
	}
	return results
}
