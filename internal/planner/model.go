package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	DecisionClosed  = "CLOSED"
	DecisionUnknown = "UNKNOWN"
	DecisionRefuted = "REFUTED"
)

type MetaCode struct {
	Path                    string
	Digest                  string
	Package                 string
	Namespace               string
	Operation               string
	Version                 string
	Activities              []ActivitySpec
	Edges                   []EdgeSpec
	Paths                   map[string]string
	Templates               map[string]string
	Required                []string
	UnknownFields           []string
	Precedence              []string
	States                  []string
	TerminalState           string
	ProofClasses            []string
	IndicatorClasses        []string
	ReleaseLockFields       []string
	Migration               Migration
	StatesOfRun             []string
	Invariants              []string
	Authority               map[string]int
	Process                 map[string]string
	Claims                  map[string]string
	Metrics                 []string
	Cases                   []CaseSpec
	TransactionManifestPath string
}

type ActivitySpec struct {
	Name       string
	InputType  string
	OutputType string
	Computes   string
	Kind       string
	Options    map[string]string
}

type EdgeSpec struct {
	From      string
	To        string
	ValueType string
}

type CaseSpec struct {
	Name  string
	State string
}

type Migration struct {
	Operation string `json:"operation"`
	From      int    `json:"from"`
	To        int    `json:"to"`
	Add       int    `json:"add"`
	Retire    int    `json:"retire"`
	Split     int    `json:"split"`
}

type Transaction struct {
	Schema            string         `json:"schema"`
	TransactionID     string         `json:"transaction_id"`
	Baseline          Baseline       `json:"baseline"`
	Cell              Cell           `json:"cell"`
	Activity          Activity       `json:"activity"`
	ReleaseKey        string         `json:"release_key"`
	ReleaseLock       map[string]any `json:"release_lock"`
	Outcome           Outcome        `json:"outcome"`
	Expected          Expected       `json:"expected"`
	Migration         Migration      `json:"migration"`
	RegistryEntry     map[string]any `json:"registry_entry"`
	InsertionStrategy string         `json:"insertion_strategy"`
	ManifestKey       string         `json:"manifest_key,omitempty"`
}

type Baseline struct {
	Repository          string `json:"repository"`
	Tag                 string `json:"tag"`
	ReleaseID           int64  `json:"release_id"`
	TagObjectSHA        string `json:"tag_object_sha"`
	TargetCommitSHA     string `json:"target_commit_sha"`
	Immutable           *bool  `json:"immutable"`
	SourceArchiveSHA256 string `json:"source_archive_sha256"`
	SourceTreeSHA256    string `json:"source_tree_sha256"`
	ReleaseAssetID      int64  `json:"release_asset_id"`
	ReleaseAssetSHA256  string `json:"release_asset_sha256"`
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

type Activity struct {
	Name       string `json:"name"`
	InputType  string `json:"input_type"`
	OutputType string `json:"output_type"`
	Computes   string `json:"computes"`
}

type Outcome struct {
	State      string           `json:"state"`
	Evidence   []string         `json:"evidence"`
	Unknown    *UnknownFrontier `json:"unknown"`
	Refutation *Refutation      `json:"refutation"`
}

type Expected struct {
	ProofTotals       map[string]int `json:"proof_totals"`
	IndicatorTotals   map[string]int `json:"indicator_totals"`
	StatusCounts      map[string]int `json:"status_counts"`
	PortfolioDecision string         `json:"portfolio_decision"`
}

type UnknownFrontier struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type Refutation struct {
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	Reason         string `json:"reason"`
	Counterexample string `json:"counterexample"`
}

type Finding struct {
	State          string   `json:"state"`
	Stage          string   `json:"stage"`
	Step           string   `json:"step"`
	Reason         string   `json:"reason"`
	UnknownClass   string   `json:"unknown_class,omitempty"`
	NextOperation  string   `json:"next_operation,omitempty"`
	BlockedBy      []string `json:"blocked_by,omitempty"`
	Counterexample string   `json:"counterexample,omitempty"`
}

type FileMutation struct {
	Path         string   `json:"path"`
	Action       string   `json:"action"`
	BeforeExists bool     `json:"before_exists"`
	AfterExists  bool     `json:"after_exists"`
	BeforeDigest string   `json:"before_digest"`
	AfterDigest  string   `json:"after_digest"`
	BeforeBytes  int64    `json:"before_bytes"`
	AfterBytes   int64    `json:"after_bytes"`
	ChangedPaths []string `json:"changed_paths,omitempty"`
	Description  string   `json:"description"`
}

type Metrics struct {
	ExactFilesChanged  int64 `json:"exact_files_changed"`
	ExactFilesPlanned  int64 `json:"exact_files_planned"`
	ASTNodesAdded      int64 `json:"ast_nodes_added"`
	BytesBefore        int64 `json:"bytes_before"`
	BytesAfter         int64 `json:"bytes_after"`
	ReplayMismatches   int64 `json:"replay_mismatches"`
	RepositoryWrites   int64 `json:"repository_writes"`
	GeneratedFiles     int64 `json:"generated_files"`
	GeneratedBytes     int64 `json:"generated_bytes"`
	WallMS             int64 `json:"wall_ms"`
	PeakRSSKiB         int64 `json:"peak_rss_kib"`
	TestsTotal         int64 `json:"tests_total"`
	TestsExecuted      int64 `json:"tests_executed"`
	TestsReused        int64 `json:"tests_reused"`
	TestsFailed        int64 `json:"tests_failed"`
	TestsUnknown       int64 `json:"tests_unknown"`
	GoFiles            int64 `json:"go_files"`
	GoLines            int64 `json:"go_lines"`
	GoooFiles          int64 `json:"gooo_files"`
	GoooLines          int64 `json:"gooo_lines"`
	RegularFiles       int64 `json:"regular_files"`
	Directories        int64 `json:"directories"`
	RootReadmeExcluded bool  `json:"root_readme_excluded"`
}

type PatchPlan struct {
	Schema                  string            `json:"schema"`
	TransactionID           string            `json:"transaction_id"`
	Operation               string            `json:"operation"`
	OperationDecision       string            `json:"decision"`
	PortfolioDecision       string            `json:"portfolio_decision"`
	Findings                []Finding         `json:"findings,omitempty"`
	Baseline                Baseline          `json:"baseline"`
	Migration               Migration         `json:"migration"`
	NewCell                 Cell              `json:"new_cell"`
	NewActivity             Activity          `json:"new_activity"`
	ReleaseKey              string            `json:"release_key"`
	ProofTotals             map[string]int    `json:"proof_totals"`
	IndicatorTotals         map[string]int    `json:"indicator_totals"`
	StatusCounts            map[string]int    `json:"status_counts"`
	Files                   []FileMutation    `json:"files"`
	BeforeDigest            string            `json:"canonical_before_digest"`
	AfterDigest             string            `json:"canonical_after_digest"`
	Metrics                 Metrics           `json:"metrics"`
	Authority               map[string]any    `json:"authority"`
	Process                 map[string]string `json:"process"`
	Claims                  map[string]string `json:"claims"`
	RepositoryOutput        string            `json:"repository_output,omitempty"`
	InputRepositoryMutated  bool              `json:"input_repository_mutated"`
	ManifestKey             string            `json:"manifest_key,omitempty"`
	TargetBeforeDigest      string            `json:"target_before_digest,omitempty"`
	ManifestPlannedFiles    []string          `json:"manifest_planned_files,omitempty"`
	ManifestAfterInvariants []string          `json:"manifest_after_invariants,omitempty"`
}

type ReplayReceipt struct {
	Schema              string   `json:"schema"`
	TransactionID       string   `json:"transaction_id"`
	State               string   `json:"state"`
	BeforeDigest        string   `json:"canonical_before_digest"`
	AfterDigest         string   `json:"canonical_after_digest"`
	ObservedAfterDigest string   `json:"observed_after_digest"`
	Mismatches          []string `json:"mismatches"`
	RollbackReady       bool     `json:"rollback_ready"`
	RepositoryWrites    int      `json:"repository_writes"`
}

type RollbackBundle struct {
	Schema        string         `json:"schema"`
	TransactionID string         `json:"transaction_id"`
	BeforeDigest  string         `json:"canonical_before_digest"`
	AfterDigest   string         `json:"canonical_after_digest"`
	Files         []RollbackFile `json:"files"`
	Replay        ReplayReceipt  `json:"replay"`
}

type RollbackFile struct {
	Path         string `json:"path"`
	BeforeExists bool   `json:"before_exists"`
	BeforeDigest string `json:"before_digest"`
	AfterDigest  string `json:"after_digest"`
	BeforeBase64 string `json:"before_base64,omitempty"`
}

func (t Transaction) ValidateShape() error {
	if t.Schema != "gooo/ledger-append-transaction/v1" && t.Schema != "gooo/ledger-append-transaction/v2" {
		return fmt.Errorf("transaction schema is %q", t.Schema)
	}
	if t.TransactionID == "" {
		return fmt.Errorf("transaction_id is required")
	}
	return nil
}

func decodeJSON(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(target)
}
