package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func renderTemplate(meta MetaCode, name string, context map[string]any) ([]byte, error) {
	metaDir := filepath.Dir(meta.Path)
	root := filepath.Clean(filepath.Join(metaDir, ".."))
	path := filepath.Join(root, filepath.FromSlash(meta.Templates[name]))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s template: %w", name, err)
	}
	funcs := template.FuncMap{"json": func(value any) (string, error) {
		data, err := json.Marshal(value)
		return string(data), err
	}}
	parsed, err := template.New(name).Funcs(funcs).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse %s template: %w", name, err)
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, context); err != nil {
		return nil, fmt.Errorf("render %s template: %w", name, err)
	}
	return output.Bytes(), nil
}

func templateContext(state *runState, files []FileMutation) map[string]any {
	tx := state.Transaction
	return map[string]any{
		"TransactionID":     tx.TransactionID,
		"Operation":         state.Meta.Operation,
		"OperationDecision": state.Meta.TerminalState,
		"PortfolioDecision": expectedPortfolioDecision(state),
		"Precedence":        state.Meta.Precedence,
		"Baseline":          tx.Baseline,
		"Migration":         state.Plan.Migration,
		"NewCell":           tx.Cell,
		"NewActivity":       tx.Activity,
		"ReleaseKey":        tx.Cell.ReleaseKey,
		"Summary":           map[string]any{"total": sumMap(tx.Expected.StatusCounts), "closed": tx.Expected.StatusCounts[DecisionClosed], "unknown": tx.Expected.StatusCounts[DecisionUnknown], "refuted": tx.Expected.StatusCounts[DecisionRefuted]},
		"ProofTotals":       tx.Expected.ProofTotals,
		"IndicatorTotals":   tx.Expected.IndicatorTotals,
		"BeforeDigest":      state.BeforeDigest,
		"AfterDigest":       state.AfterDigest,
		"Files":             files,
		"Metrics":           state.Plan.Metrics,
		"Authority":         authorityValue(state.Meta),
		"Process":           state.Meta.Process,
		"Claims":            state.Meta.Claims,
		"HistoryEvent":      historyEvent(state),
	}
}

func historyEvent(state *runState) map[string]any {
	tx := state.Transaction
	event := map[string]any{
		"event_id":                tx.TransactionID,
		"event_type":              "ADOPTION_TRANSACTION",
		"append_only":             true,
		"cell_id":                 tx.Cell.ID,
		"activity":                tx.Activity.Name,
		"release_key":             tx.Cell.ReleaseKey,
		"state":                   tx.Outcome.State,
		"portfolio_decision":      expectedPortfolioDecision(state),
		"canonical_before_digest": state.BeforeDigest,
		"canonical_after_digest":  state.AfterDigest,
		"migration":               state.Plan.Migration,
		"improvement":             map[string]any{"state": state.Meta.Claims["whole_language_improvement"], "evidence": "UNKNOWN until matched manual/tool before-after under the same digest"},
		"motivation":              map[string]any{"prior_local_validation_executions": 1, "prior_process_state": "REFUTED", "wrong_insertion_point_attempts": "preserved as historical motivation"},
	}
	if tx.Outcome.Unknown != nil {
		event["unknown"] = tx.Outcome.Unknown
	}
	if tx.Outcome.Refutation != nil {
		event["refutation"] = tx.Outcome.Refutation
	}
	return event
}

func makeDossier(state *runState) string {
	decision := state.Plan.OperationDecision
	if decision == "" {
		decision = reduceFindings(state.Meta, state.Findings)
	}
	var builder strings.Builder
	builder.WriteString("# Gooo ledger append transaction dossier\n\n")
	builder.WriteString("This dossier describes one structural append transaction. The input repository is read-only; any successful result is materialized only in a caller-owned temporary copy.\n\n")
	builder.WriteString("## Decision\n\n")
	fmt.Fprintf(&builder, "- operation: `%s`\n- transaction: `%s`\n- operation decision: `%s`\n- portfolio decision: `%s`\n- canonical subject before: `%s`\n- canonical subject after: `%s`\n\n", state.Meta.Operation, state.Transaction.TransactionID, decision, state.Plan.PortfolioDecision, state.BeforeDigest, state.AfterDigest)
	builder.WriteString("## Structural change\n\n")
	fmt.Fprintf(&builder, "The transaction appends cell `%s`, activity `%s`, and release-map key `%s`; migration is ADD1/RETIRE0/SPLIT0. Existing JSON objects are compared canonically and existing Gooo bytes remain a prefix of the appended activity file.\n\n", state.Transaction.Cell.ID, state.Transaction.Activity.Name, state.Transaction.Cell.ReleaseKey)
	builder.WriteString("## Historical motivation\n\n")
	builder.WriteString("The v0.31.0 ledger records `local_validation_executions=1` with process state `REFUTED`, alongside multiple uncommitted wrong insertion-point attempts. Those observations are preserved as motivation and are not a measured time improvement.\n\n")
	builder.WriteString("## Authority and process\n\n")
	builder.WriteString("The `.gooo` operation graph is the semantic authority consumed by the Go AST executor. This implementation performs no commit, push, merge, upstream write, or input-repository write. Bootstrap policy is exactly one initial bootstrap commit; post-bootstrap direct-main is zero; implementation changes are intended to move through one open PR at a time with GitHub Actions as the verification authority.\n\n")
	fmt.Fprintf(&builder, "The development observation ledger records local validation attempts `%s`, Go test executions `%s`, Go build executions `%s`, `go run` compilations `%s`, Go vet executions `%s`, and conformance executions `%s`; therefore `development_process=REFUTED`. These local results are not product conformance authority.\n\n", state.Meta.Process["local_validation_executions"], state.Meta.Process["local_go_test_executions"], state.Meta.Process["local_go_build_executions"], state.Meta.Process["local_go_run_compile_executions"], state.Meta.Process["local_go_vet_executions"], state.Meta.Process["local_conformance_executions"])
	builder.WriteString("## Non-claims\n\n")
	builder.WriteString("Whole-language improvement remains UNKNOWN until a future real ledger adoption supplies matched manual/tool before-after evidence under the same digest. External utility remains UNKNOWN/NOT_MADE.\n\n")
	if len(state.Findings) > 0 {
		builder.WriteString("## Findings\n\n")
		for _, finding := range state.Findings {
			fmt.Fprintf(&builder, "- `%s` at `%s/%s`: %s (%s)\n", finding.State, finding.Stage, finding.Step, finding.Reason, finding.Counterexample)
		}
	}
	return builder.String()
}

func sumMap(values map[string]int) int {
	result := 0
	for _, value := range values {
		result += value
	}
	return result
}
