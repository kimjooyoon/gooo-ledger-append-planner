package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	profileSchema       = "gooo/self-improvement-portfolio/contract/v1"
	assessmentSchema    = "gooo/self-improvement-portfolio/assessment/v1"
	verificationSchema  = "gooo/self-improvement-portfolio/release-verification/v1"
	runtimeSchema       = "gooo/self-improvement-portfolio/runtime/v1"
	reportSchema        = "gooo/self-improvement-portfolio/report/v1"
	stateClosed         = "CLOSED"
	stateUnknown        = "UNKNOWN"
	stateRefuted        = "REFUTED"
	activityLinePattern = `(?m)^activity ([A-Za-z0-9_]+)\(`
)

type Profile struct {
	Schema          string         `json:"schema"`
	ProfileID       string         `json:"profile_id"`
	TotalCells      int            `json:"total_cells"`
	ProofTotals     map[string]int `json:"proof_totals"`
	IndicatorTotals map[string]int `json:"indicator_totals"`
	Precedence      []string       `json:"precedence"`
	Policy          Policy         `json:"policy"`
	Cells           []Cell         `json:"cells"`
}

type Policy struct {
	DenominatorMutationDuringRun bool `json:"denominator_mutation_during_run"`
	StatusInferenceFromMissing   bool `json:"status_inference_from_missing_evidence"`
	RuntimeRepositoryWrites      int  `json:"runtime_repository_writes"`
	CallerOwnedTempOutputOnly    bool `json:"caller_owned_temp_output_only"`
	CrossProjectRequiredGates    int  `json:"cross_project_required_gates"`
	AggregatePercentage          bool `json:"aggregate_percentage"`
	AggregateScore               bool `json:"aggregate_score"`
}

type Cell struct {
	Ordinal           int      `json:"ordinal"`
	ID                string   `json:"id"`
	Axis              string   `json:"axis"`
	Proof             string   `json:"proof"`
	Indicator         string   `json:"indicator"`
	Activity          string   `json:"activity"`
	Source            string   `json:"source"`
	IR                string   `json:"ir"`
	GeneratedArtifact string   `json:"generated_artifact"`
	Evaluator         string   `json:"evaluator"`
	MetricID          string   `json:"metric_id"`
	MetricDenominator int      `json:"metric_denominator"`
	ReleaseKey        string   `json:"release_key"`
	DependsOn         []string `json:"depends_on"`
}

type Assessment struct {
	Schema       string           `json:"schema"`
	ProfileID    string           `json:"profile_id"`
	AssessmentID string           `json:"assessment_id"`
	Cells        []AssessmentCell `json:"cells"`
}

type AssessmentCell struct {
	CellID     string   `json:"cell_id"`
	State      string   `json:"state"`
	ReleaseKey string   `json:"release_key"`
	Evidence   []string `json:"evidence"`
	Unknown    *Unknown `json:"unknown"`
	Refutation *Unknown `json:"refutation"`
}

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type ReleaseVerification struct {
	Schema   string                   `json:"schema"`
	Releases map[string]ReleaseResult `json:"releases"`
	Summary  ReleaseSummary           `json:"summary"`
	Timing   Timing                   `json:"timing"`
}

type ReleaseResult struct {
	State           string        `json:"state"`
	Verified        bool          `json:"verified"`
	Repository      string        `json:"repository"`
	Tag             string        `json:"tag"`
	ReleaseURL      string        `json:"release_url"`
	TargetCommitSHA string        `json:"target_commit_sha"`
	Assets          []AssetResult `json:"assets"`
	Fetch           Measurement   `json:"fetch"`
	Verify          Measurement   `json:"verify"`
	Reason          string        `json:"reason"`
}

type AssetResult struct {
	Name        string `json:"name"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"download_url"`
	Verified    bool   `json:"verified"`
}

type ReleaseSummary struct {
	Total    int `json:"total"`
	Verified int `json:"verified"`
	Unknown  int `json:"unknown"`
	Refuted  int `json:"refuted"`
}

type RuntimeInput struct {
	Schema              string              `json:"schema"`
	SubjectSHA          string              `json:"subject_sha"`
	GoVersion           string              `json:"go_version"`
	Timing              Timing              `json:"timing"`
	Authority           Authority           `json:"authority"`
	LocalExecutionCount LocalExecutionCount `json:"local_execution_counts"`
}

type Measurement struct {
	WallMS     int64 `json:"wall_ms"`
	DurationNS int64 `json:"duration_ns"`
	PeakRSSKiB int64 `json:"peak_rss_kib"`
}

type Timing struct {
	Fetch  Measurement `json:"fetch"`
	Verify Measurement `json:"verify"`
	Report Measurement `json:"report"`
}

type Authority struct {
	RuntimeRepositoryWrites   int  `json:"runtime_repository_writes"`
	CallerOwnedTempOutput     bool `json:"caller_owned_temp_output"`
	CrossProjectRequiredGates int  `json:"cross_project_required_gates"`
}

type LocalExecutionCount struct {
	Gofmt       int `json:"gofmt"`
	Build       int `json:"build"`
	Test        int `json:"test"`
	Vet         int `json:"vet"`
	Conformance int `json:"conformance"`
}

type Inventory struct {
	DescendantDirs     int  `json:"dirs"`
	RegularFiles       int  `json:"files"`
	GoFiles            int  `json:"go_files"`
	GoLines            int  `json:"go_lines"`
	GoooFiles          int  `json:"gooo_files"`
	GoooLines          int  `json:"gooo_lines"`
	RootReadmeExcluded bool `json:"root_readme_excluded"`
}

type ArtifactStats struct {
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
	Scope string `json:"scope"`
}

type ReportCell struct {
	Ordinal           int      `json:"ordinal"`
	ID                string   `json:"id"`
	Axis              string   `json:"axis"`
	Proof             string   `json:"proof"`
	Indicator         string   `json:"indicator"`
	Activity          string   `json:"activity"`
	Source            string   `json:"source"`
	IR                string   `json:"ir"`
	GeneratedArtifact string   `json:"generated_artifact"`
	Evaluator         string   `json:"evaluator"`
	MetricID          string   `json:"metric_id"`
	Numerator         int      `json:"numerator"`
	Denominator       int      `json:"denominator"`
	State             string   `json:"state"`
	ReleaseKey        string   `json:"release_key,omitempty"`
	Evidence          []string `json:"evidence"`
	Unknown           *Unknown `json:"unknown,omitempty"`
	Refutation        *Unknown `json:"refutation,omitempty"`
}

type BucketSummary struct {
	Denominator int `json:"denominator"`
	Closed      int `json:"closed"`
	Unknown     int `json:"unknown"`
	Refuted     int `json:"refuted"`
}

type StatusCounts struct {
	Total   int `json:"total"`
	Closed  int `json:"closed"`
	Unknown int `json:"unknown"`
	Refuted int `json:"refuted"`
}

type PortfolioReport struct {
	Schema              string                   `json:"schema"`
	ProfileID           string                   `json:"profile_id"`
	AssessmentID        string                   `json:"assessment_id"`
	SubjectSHA          string                   `json:"subject_sha"`
	GoVersion           string                   `json:"go_version"`
	Decision            string                   `json:"decision"`
	Precedence          []string                 `json:"precedence"`
	Summary             StatusCounts             `json:"summary"`
	ProofCounts         map[string]BucketSummary `json:"proof_counts"`
	IndicatorCounts     map[string]BucketSummary `json:"indicator_counts"`
	Cells               []ReportCell             `json:"cells"`
	Bindings            BindingSummary           `json:"bindings"`
	Inventory           Inventory                `json:"inventory"`
	Performance         Timing                   `json:"performance"`
	Artifact            ArtifactStats            `json:"artifact"`
	ReleaseSummary      ReleaseSummary           `json:"releases"`
	Authority           Authority                `json:"authority"`
	LocalExecutionCount LocalExecutionCount      `json:"local_execution_counts"`
	Policy              Policy                   `json:"policy"`
}

type BindingSummary struct {
	OneToOne          bool `json:"one_to_one"`
	Cells             int  `json:"cells"`
	Activities        int  `json:"activities"`
	UniqueAxes        int  `json:"unique_axes"`
	UniqueMetrics     int  `json:"unique_metrics"`
	SourceBindings    int  `json:"source_bindings"`
	IRBindings        int  `json:"ir_bindings"`
	ArtifactBindings  int  `json:"generated_artifact_bindings"`
	EvaluatorBindings int  `json:"evaluator_bindings"`
}

func main() {
	profilePath := flag.String("profile", "", "fixed capability profile JSON")
	activitiesPath := flag.String("activities", "", "authoritative Gooo activity file")
	assessmentPath := flag.String("assessment", "", "current assessment JSON")
	verificationPath := flag.String("verification", "", "release verification JSON")
	runtimePath := flag.String("runtime", "", "runtime measurements JSON")
	repositoryRoot := flag.String("repository-root", ".", "repository root for inventory")
	artifactRoot := flag.String("artifact-root", ".", "caller-owned evidence output root")
	outputJSON := flag.String("output-json", "", "report JSON output")
	outputMarkdown := flag.String("output-markdown", "", "human-readable report output")
	flag.Parse()

	for name, value := range map[string]string{
		"profile": *profilePath, "activities": *activitiesPath, "assessment": *assessmentPath,
		"verification": *verificationPath, "runtime": *runtimePath, "output-json": *outputJSON,
		"output-markdown": *outputMarkdown,
	} {
		if value == "" {
			fatalf("-%s is required", name)
		}
	}

	var profile Profile
	readJSON(*profilePath, &profile)
	validateProfile(profile)
	activityNames := readActivities(*activitiesPath)
	validateActivityBindings(profile, activityNames)

	assessment := readAssessment(*assessmentPath)
	validateAssessment(profile, assessment)

	var verification ReleaseVerification
	readJSON(*verificationPath, &verification)
	if verification.Schema != verificationSchema {
		fatalf("verification schema is %q, want %q", verification.Schema, verificationSchema)
	}

	var runtime RuntimeInput
	readJSON(*runtimePath, &runtime)
	if runtime.Schema != runtimeSchema {
		fatalf("runtime schema is %q, want %q", runtime.Schema, runtimeSchema)
	}

	inventory := collectInventory(*repositoryRoot)
	artifact := collectArtifacts(*artifactRoot)
	report := buildReport(profile, assessment, verification, runtime, inventory, artifact)

	if err := writeJSON(*outputJSON, report); err != nil {
		fatalf("write report JSON: %v", err)
	}
	if err := writeMarkdown(*outputMarkdown, report); err != nil {
		fatalf("write report Markdown: %v", err)
	}
}

func readJSON(path string, destination any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		fatalf("decode %s: %v", path, err)
	}
}

func validateProfile(profile Profile) {
	if profile.Schema != profileSchema || profile.ProfileID == "" {
		fatalf("invalid profile identity")
	}
	if profile.TotalCells != 39 || len(profile.Cells) != profile.TotalCells {
		fatalf("profile must contain exactly 39 cells")
	}
	if !equalStringSlice(profile.Precedence, []string{stateRefuted, stateUnknown, stateClosed}) {
		fatalf("profile precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	if profile.Policy.DenominatorMutationDuringRun || profile.Policy.StatusInferenceFromMissing || profile.Policy.RuntimeRepositoryWrites != 0 || !profile.Policy.CallerOwnedTempOutputOnly || profile.Policy.CrossProjectRequiredGates != 0 || profile.Policy.AggregatePercentage || profile.Policy.AggregateScore {
		fatalf("profile policy violates fixed denominator or authority boundary")
	}
	if !equalIntMap(profile.ProofTotals, map[string]int{"FOUNDATION": 4, "COHERENCE": 30, "REGRESSION": 5}) {
		fatalf("proof totals must be FOUNDATION4/COHERENCE30/REGRESSION5")
	}
	if !equalIntMap(profile.IndicatorTotals, map[string]int{"DRIVER": 4, "OUTCOME": 30, "GUARDRAIL": 5}) {
		fatalf("indicator totals must be DRIVER4/OUTCOME30/GUARDRAIL5")
	}
	seenIDs := map[string]bool{}
	seenAxes := map[string]bool{}
	seenActivities := map[string]bool{}
	seenMetrics := map[string]bool{}
	actualProof := map[string]int{}
	actualIndicator := map[string]int{}
	for index, cell := range profile.Cells {
		if cell.Ordinal != index+1 || cell.ID == "" || cell.Axis == "" || seenIDs[cell.ID] || seenAxes[cell.Axis] || cell.Activity == "" || seenActivities[cell.Activity] || cell.MetricID == "" || seenMetrics[cell.MetricID] || cell.MetricDenominator != 1 {
			fatalf("profile cell %d is not a unique, denominator-1 binding", index+1)
		}
		if cell.Source == "" || cell.IR == "" || cell.GeneratedArtifact == "" || cell.Evaluator == "" {
			fatalf("profile cell %s is missing a binding edge", cell.ID)
		}
		seenIDs[cell.ID] = true
		seenAxes[cell.Axis] = true
		seenActivities[cell.Activity] = true
		seenMetrics[cell.MetricID] = true
		actualProof[cell.Proof]++
		actualIndicator[cell.Indicator]++
	}
	if !equalIntMap(actualProof, profile.ProofTotals) || !equalIntMap(actualIndicator, profile.IndicatorTotals) {
		fatalf("profile cell classification totals do not match declared 4/30/5")
	}
}

func readActivities(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read activities: %v", err)
	}
	matches := regexp.MustCompile(activityLinePattern).FindAllSubmatch(raw, -1)
	activities := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := string(match[1])
		if seen[name] {
			fatalf("duplicate Gooo activity %s", name)
		}
		seen[name] = true
		activities = append(activities, name)
	}
	return activities
}

func validateActivityBindings(profile Profile, activities []string) {
	if len(activities) != profile.TotalCells {
		fatalf("Gooo activity count is %d, want %d", len(activities), profile.TotalCells)
	}
	declared := map[string]bool{}
	for _, cell := range profile.Cells {
		declared[cell.Activity] = true
	}
	for _, activity := range activities {
		if !declared[activity] {
			fatalf("unbound Gooo activity %s", activity)
		}
	}
}

func readAssessment(path string) Assessment {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read assessment: %v", err)
	}
	var envelope struct {
		Schema       string            `json:"schema"`
		ProfileID    string            `json:"profile_id"`
		AssessmentID string            `json:"assessment_id"`
		Cells        []json.RawMessage `json:"cells"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		fatalf("decode assessment: %v", err)
	}
	assessment := Assessment{Schema: envelope.Schema, ProfileID: envelope.ProfileID, AssessmentID: envelope.AssessmentID}
	for _, cellRaw := range envelope.Cells {
		var cell AssessmentCell
		if err := json.Unmarshal(cellRaw, &cell); err != nil {
			fatalf("decode assessment cell: %v", err)
		}
		if cell.State == stateUnknown {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(cellRawField(cellRaw, "unknown"), &fields); err != nil || len(fields) != 6 {
				fatalf("UNKNOWN cell %s must contain exactly six fields", cell.CellID)
			}
			for _, key := range []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"} {
				if _, ok := fields[key]; !ok {
					fatalf("UNKNOWN cell %s is missing %s", cell.CellID, key)
				}
			}
			if cell.Unknown == nil || cell.Unknown.Stage == "" || cell.Unknown.Step == "" || cell.Unknown.Reason == "" || cell.Unknown.UnknownClass == "" || cell.Unknown.NextOperation == "" || len(cell.Unknown.BlockedBy) == 0 {
				fatalf("UNKNOWN cell %s has an incomplete six-field frontier", cell.CellID)
			}
		}
		assessment.Cells = append(assessment.Cells, cell)
	}
	return assessment
}

func cellRawField(raw json.RawMessage, field string) json.RawMessage {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		fatalf("decode assessment cell fields: %v", err)
	}
	value, ok := fields[field]
	if !ok || string(value) == "null" {
		return []byte(`{}`)
	}
	return value
}

func validateAssessment(profile Profile, assessment Assessment) {
	if assessment.Schema != assessmentSchema || assessment.ProfileID != profile.ProfileID || len(assessment.Cells) != profile.TotalCells {
		fatalf("assessment does not bind the fixed profile")
	}
	byID := map[string]AssessmentCell{}
	for _, cell := range assessment.Cells {
		if cell.CellID == "" || byID[cell.CellID].CellID != "" {
			fatalf("assessment has duplicate or empty cell identity")
		}
		if cell.State != stateClosed && cell.State != stateUnknown && cell.State != stateRefuted {
			fatalf("assessment cell %s has invalid state %q", cell.CellID, cell.State)
		}
		if cell.State == stateRefuted && cell.Refutation == nil {
			fatalf("REFUTED cell %s is missing its refutation", cell.CellID)
		}
		byID[cell.CellID] = cell
	}
	for _, cell := range profile.Cells {
		assessmentCell, ok := byID[cell.ID]
		if !ok {
			fatalf("assessment is missing %s", cell.ID)
		}
		if assessmentCell.ReleaseKey != cell.ReleaseKey {
			fatalf("assessment release binding for %s disagrees with profile", cell.ID)
		}
	}
}

func buildReport(profile Profile, assessment Assessment, verification ReleaseVerification, runtime RuntimeInput, inventory Inventory, artifact ArtifactStats) PortfolioReport {
	assessmentByID := map[string]AssessmentCell{}
	for _, cell := range assessment.Cells {
		assessmentByID[cell.CellID] = cell
	}

	reportedCells := make([]ReportCell, 0, len(profile.Cells))
	proofCounts := map[string]BucketSummary{}
	indicatorCounts := map[string]BucketSummary{}
	for key := range profile.ProofTotals {
		proofCounts[key] = BucketSummary{Denominator: profile.ProofTotals[key]}
	}
	for key := range profile.IndicatorTotals {
		indicatorCounts[key] = BucketSummary{Denominator: profile.IndicatorTotals[key]}
	}

	for _, cell := range profile.Cells {
		assessmentCell := assessmentByID[cell.ID]
		state := assessmentCell.State
		unknown := assessmentCell.Unknown
		refutation := assessmentCell.Refutation
		if cell.ReleaseKey != "" {
			release, ok := verification.Releases[cell.ReleaseKey]
			if !ok {
				fatalf("release verification is missing %s", cell.ReleaseKey)
			}
			switch release.State {
			case stateRefuted:
				state = stateRefuted
				refutation = &Unknown{Stage: "RELEASE_VERIFICATION", Step: "VERIFY_LOCKED_RELEASE_ASSET", Reason: release.Reason, UnknownClass: "CONTRADICTION", NextOperation: "RESTORE_EXACT_LOCKED_RELEASE_ASSET", BlockedBy: []string{cell.ReleaseKey}}
			case stateUnknown:
				if state == stateClosed {
					state = stateUnknown
					unknown = &Unknown{Stage: "RELEASE_VERIFICATION", Step: "FETCH_LOCKED_RELEASE_ASSET", Reason: "LOCKED_RELEASE_ASSET_NOT_AVAILABLE", UnknownClass: "DIRECT_MISSING", NextOperation: "PROVIDE_LOCKED_RELEASE_ASSET", BlockedBy: []string{cell.ReleaseKey}}
				}
			case stateClosed:
			default:
				fatalf("release %s has invalid state %q", cell.ReleaseKey, release.State)
			}
		}
		if state == stateUnknown && unknown == nil {
			fatalf("effective UNKNOWN cell %s lost its six-field frontier", cell.ID)
		}
		numerator := 0
		if state == stateClosed {
			numerator = 1
		}
		reported := ReportCell{
			Ordinal: cell.Ordinal, ID: cell.ID, Axis: cell.Axis, Proof: cell.Proof, Indicator: cell.Indicator,
			Activity: cell.Activity, Source: cell.Source, IR: cell.IR, GeneratedArtifact: cell.GeneratedArtifact,
			Evaluator: cell.Evaluator, MetricID: cell.MetricID, Numerator: numerator, Denominator: 1,
			State: state, ReleaseKey: cell.ReleaseKey, Evidence: append([]string(nil), assessmentCell.Evidence...),
			Unknown: unknown, Refutation: refutation,
		}
		reportedCells = append(reportedCells, reported)
		proofSummary := proofCounts[cell.Proof]
		addState(&proofSummary, state)
		proofCounts[cell.Proof] = proofSummary
		indicatorSummary := indicatorCounts[cell.Indicator]
		addState(&indicatorSummary, state)
		indicatorCounts[cell.Indicator] = indicatorSummary
	}

	counts := StatusCounts{Total: len(reportedCells)}
	for _, cell := range reportedCells {
		switch cell.State {
		case stateClosed:
			counts.Closed++
		case stateUnknown:
			counts.Unknown++
		case stateRefuted:
			counts.Refuted++
		}
	}
	releaseSummary := summarizeReleases(verification)
	if releaseSummary.Total == 0 {
		releaseSummary = verification.Summary
	}

	return PortfolioReport{
		Schema: reportSchema, ProfileID: profile.ProfileID, AssessmentID: assessment.AssessmentID,
		SubjectSHA: runtime.SubjectSHA, GoVersion: runtime.GoVersion, Decision: "REPORT_ONLY", Precedence: profile.Precedence,
		Summary: counts, ProofCounts: proofCounts, IndicatorCounts: indicatorCounts, Cells: reportedCells,
		Bindings:  BindingSummary{OneToOne: true, Cells: len(profile.Cells), Activities: len(profile.Cells), UniqueAxes: len(profile.Cells), UniqueMetrics: len(profile.Cells), SourceBindings: len(profile.Cells), IRBindings: len(profile.Cells), ArtifactBindings: len(profile.Cells), EvaluatorBindings: len(profile.Cells)},
		Inventory: inventory, Performance: runtime.Timing, Artifact: artifact, ReleaseSummary: releaseSummary,
		Authority: runtime.Authority, LocalExecutionCount: runtime.LocalExecutionCount, Policy: profile.Policy,
	}
}

func summarizeReleases(verification ReleaseVerification) ReleaseSummary {
	result := ReleaseSummary{Total: len(verification.Releases)}
	for _, release := range verification.Releases {
		switch release.State {
		case stateClosed:
			result.Verified++
		case stateUnknown:
			result.Unknown++
		case stateRefuted:
			result.Refuted++
		}
	}
	return result
}

func addState(summary *BucketSummary, state string) {
	switch state {
	case stateClosed:
		summary.Closed++
	case stateUnknown:
		summary.Unknown++
	case stateRefuted:
		summary.Refuted++
	}
}

func collectInventory(root string) Inventory {
	var result Inventory
	result.RootReadmeExcluded = true
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if path != root {
				result.DescendantDirs++
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		result.RegularFiles++
		extension := filepath.Ext(path)
		switch extension {
		case ".go":
			result.GoFiles++
			result.GoLines += physicalLines(path)
		case ".gooo":
			result.GoooFiles++
			result.GoooLines += physicalLines(path)
		}
		return nil
	})
	if err != nil {
		fatalf("inventory %s: %v", root, err)
	}
	return result
}

func collectArtifacts(root string) ArtifactStats {
	var result ArtifactStats
	result.Scope = "caller-owned evidence files present before report emission"
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result.Files++
		result.Bytes += info.Size()
		return nil
	})
	if err != nil {
		fatalf("artifact inventory %s: %v", root, err)
	}
	return result
}

func physicalLines(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read line-count input %s: %v", path, err)
	}
	if len(raw) == 0 {
		return 0
	}
	lines := bytes.Count(raw, []byte{'\n'})
	if raw[len(raw)-1] != '\n' {
		lines++
	}
	return lines
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeMarkdown(path string, report PortfolioReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# self-improvement-portfolio-v1\n\n")
	fmt.Fprintf(&b, "Decision: `%s`\n\n", report.Decision)
	fmt.Fprintf(&b, "Go toolchain: `%s`\n\n", report.GoVersion)
	fmt.Fprintf(&b, "The fixed denominator is `%d` cells. This report measures the named capability profile only; it does not infer whole-language completeness.\n\n", report.Summary.Total)
	fmt.Fprintf(&b, "Status counts: `CLOSED %d`, `UNKNOWN %d`, `REFUTED %d`. No percentage or score is emitted.\n\n", report.Summary.Closed, report.Summary.Unknown, report.Summary.Refuted)
	fmt.Fprintf(&b, "## %d-cell report\n\n| # | axis | proof | indicator | state | numerator/denominator | activity | release |\n|---:|---|---|---|---|---:|---|---|\n", report.Summary.Total)
	for _, cell := range report.Cells {
		fmt.Fprintf(&b, "| %d | `%s` | `%s` | `%s` | **%s** | %d/%d | `%s` | `%s` |\n", cell.Ordinal, cell.Axis, cell.Proof, cell.Indicator, cell.State, cell.Numerator, cell.Denominator, cell.Activity, cell.ReleaseKey)
	}
	fmt.Fprintf(&b, "\n## REFUTED process and release frontiers\n\n")
	fmt.Fprintf(&b, "| cell | stage | step | reason | next_operation | blocked_by |\n|---|---|---|---|---|---|\n")
	for _, cell := range report.Cells {
		if cell.State != stateRefuted || cell.Refutation == nil {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` |\n", cell.ID, cell.Refutation.Stage, cell.Refutation.Step, cell.Refutation.Reason, cell.Refutation.NextOperation, strings.Join(cell.Refutation.BlockedBy, ", "))
	}
	fmt.Fprintf(&b, "\n## UNKNOWN frontier\n\n")
	fmt.Fprintf(&b, "| cell | stage | step | reason | unknown_class | next_operation | blocked_by |\n|---|---|---|---|---|---|---|\n")
	for _, cell := range report.Cells {
		if cell.State != stateUnknown || cell.Unknown == nil {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | `%s` |\n", cell.ID, cell.Unknown.Stage, cell.Unknown.Step, cell.Unknown.Reason, cell.Unknown.UnknownClass, cell.Unknown.NextOperation, strings.Join(cell.Unknown.BlockedBy, ", "))
	}
	fmt.Fprintf(&b, "\n## Bindings and runtime evidence\n\n")
	fmt.Fprintf(&b, "- one-to-one activity binding: `%t` (%d cells / %d activities)\n", report.Bindings.OneToOne, report.Bindings.Cells, report.Bindings.Activities)
	fmt.Fprintf(&b, "- proof buckets: FOUNDATION %d/%d, COHERENCE %d/%d, REGRESSION %d/%d\n", report.ProofCounts["FOUNDATION"].Closed, report.ProofCounts["FOUNDATION"].Denominator, report.ProofCounts["COHERENCE"].Closed, report.ProofCounts["COHERENCE"].Denominator, report.ProofCounts["REGRESSION"].Closed, report.ProofCounts["REGRESSION"].Denominator)
	fmt.Fprintf(&b, "- indicator buckets: DRIVER %d/%d, OUTCOME %d/%d, GUARDRAIL %d/%d\n", report.IndicatorCounts["DRIVER"].Closed, report.IndicatorCounts["DRIVER"].Denominator, report.IndicatorCounts["OUTCOME"].Closed, report.IndicatorCounts["OUTCOME"].Denominator, report.IndicatorCounts["GUARDRAIL"].Closed, report.IndicatorCounts["GUARDRAIL"].Denominator)
	fmt.Fprintf(&b, "- repository dirs/files: %d/%d; Go files/lines: %d/%d; Gooo files/lines: %d/%d; root README excluded from line accounting: `%t`\n", report.Inventory.DescendantDirs, report.Inventory.RegularFiles, report.Inventory.GoFiles, report.Inventory.GoLines, report.Inventory.GoooFiles, report.Inventory.GoooLines, report.Inventory.RootReadmeExcluded)
	fmt.Fprintf(&b, "- fetch wall_ms/raw duration_ns: %d/%d; verify wall_ms/raw duration_ns: %d/%d; report wall_ms/raw duration_ns: %d/%d\n", report.Performance.Fetch.WallMS, report.Performance.Fetch.DurationNS, report.Performance.Verify.WallMS, report.Performance.Verify.DurationNS, report.Performance.Report.WallMS, report.Performance.Report.DurationNS)
	fmt.Fprintf(&b, "- peak RSS KiB fetch/verify/report: %d/%d/%d\n", report.Performance.Fetch.PeakRSSKiB, report.Performance.Verify.PeakRSSKiB, report.Performance.Report.PeakRSSKiB)
	fmt.Fprintf(&b, "- caller-owned artifact files/bytes: %d/%d\n", report.Artifact.Files, report.Artifact.Bytes)
	fmt.Fprintf(&b, "- releases verified/unknown/refuted: %d/%d/%d\n", report.ReleaseSummary.Verified, report.ReleaseSummary.Unknown, report.ReleaseSummary.Refuted)
	fmt.Fprintf(&b, "- runtime repository writes/cross-project required gates: %d/%d; caller-owned temp output: `%t`\n", report.Authority.RuntimeRepositoryWrites, report.Authority.CrossProjectRequiredGates, report.Authority.CallerOwnedTempOutput)
	fmt.Fprintf(&b, "- developer-local gofmt/build/test/vet/conformance executions: %d/%d/%d/%d/%d\n", report.LocalExecutionCount.Gofmt, report.LocalExecutionCount.Build, report.LocalExecutionCount.Test, report.LocalExecutionCount.Vet, report.LocalExecutionCount.Conformance)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func equalStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalIntMap(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
