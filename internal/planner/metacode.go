package planner

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadMetaCode(path string) (MetaCode, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MetaCode{}, err
	}
	meta := MetaCode{Path: path, Digest: fileDigest(raw), Paths: map[string]string{}, Templates: map[string]string{}, Authority: map[string]int{}, Process: map[string]string{}, Claims: map[string]string{}}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "package "):
			meta.Package = strings.TrimSpace(strings.TrimPrefix(line, "package "))
		case strings.HasPrefix(line, "namespace "):
			meta.Namespace = strings.TrimSpace(strings.TrimPrefix(line, "namespace "))
		case strings.HasPrefix(line, "entity "):
			// Entities are intentionally descriptive. Activities and edges form the executable graph.
		case strings.HasPrefix(line, "activity "):
			activity, ok := parseMetaActivity(line)
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed activity", path, lineNumber)
			}
			meta.Activities = append(meta.Activities, activity)
		case strings.HasPrefix(line, "edge "):
			edge, ok := parseMetaEdge(strings.TrimSpace(strings.TrimPrefix(line, "edge ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed edge", path, lineNumber)
			}
			meta.Edges = append(meta.Edges, edge)
		case strings.HasPrefix(line, "gooo "):
			fields := strings.Fields(line)
			if len(fields) != 3 {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed gooo declaration", path, lineNumber)
			}
			meta.Operation, meta.Version = fields[1], fields[2]
		case strings.HasPrefix(line, "operation "):
			meta.Operation = strings.TrimSpace(strings.TrimPrefix(line, "operation "))
		case strings.HasPrefix(line, "path "):
			key, value, ok := parseQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "path ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed path", path, lineNumber)
			}
			meta.Paths[key] = value
		case strings.HasPrefix(line, "template "):
			key, value, ok := parseQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "template ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed template", path, lineNumber)
			}
			meta.Templates[key] = value
		case strings.HasPrefix(line, "required "):
			meta.Required = append(meta.Required, strings.TrimSpace(strings.TrimPrefix(line, "required ")))
		case strings.HasPrefix(line, "unknown-field "):
			meta.UnknownFields = append(meta.UnknownFields, strings.TrimSpace(strings.TrimPrefix(line, "unknown-field ")))
		case strings.HasPrefix(line, "precedence "):
			meta.Precedence = strings.Fields(strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(line, "precedence ")), ">", " "))
		case strings.HasPrefix(line, "states "):
			meta.States = strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "states ")))
		case strings.HasPrefix(line, "terminal-state "):
			meta.TerminalState = strings.TrimSpace(strings.TrimPrefix(line, "terminal-state "))
		case strings.HasPrefix(line, "proof-class "):
			meta.ProofClasses = append(meta.ProofClasses, strings.TrimSpace(strings.TrimPrefix(line, "proof-class ")))
		case strings.HasPrefix(line, "indicator-class "):
			meta.IndicatorClasses = append(meta.IndicatorClasses, strings.TrimSpace(strings.TrimPrefix(line, "indicator-class ")))
		case strings.HasPrefix(line, "release-lock-field "):
			meta.ReleaseLockFields = append(meta.ReleaseLockFields, strings.TrimSpace(strings.TrimPrefix(line, "release-lock-field ")))
		case strings.HasPrefix(line, "migration "):
			migration, ok := parseOptions(strings.TrimSpace(strings.TrimPrefix(line, "migration ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed migration", path, lineNumber)
			}
			meta.Migration.Operation = migration["operation"]
			meta.Migration.Add = parseIntOption(migration, "add")
			meta.Migration.Retire = parseIntOption(migration, "retire")
			meta.Migration.Split = parseIntOption(migration, "split")
		case strings.HasPrefix(line, "transaction-state "):
			meta.StatesOfRun = append(meta.StatesOfRun, strings.TrimSpace(strings.TrimPrefix(line, "transaction-state ")))
		case strings.HasPrefix(line, "invariant "):
			meta.Invariants = append(meta.Invariants, strings.TrimSpace(strings.TrimPrefix(line, "invariant ")))
		case strings.HasPrefix(line, "authority "):
			options, ok := parseOptions(strings.TrimSpace(strings.TrimPrefix(line, "authority ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed authority", path, lineNumber)
			}
			for key, value := range options {
				meta.Authority[key] = parseIntOption(map[string]string{key: value}, key)
			}
		case strings.HasPrefix(line, "process "):
			options, ok := parseOptions(strings.TrimSpace(strings.TrimPrefix(line, "process ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed process", path, lineNumber)
			}
			for key, value := range options {
				meta.Process[key] = value
			}
		case strings.HasPrefix(line, "claim "):
			options, ok := parseOptions(strings.TrimSpace(strings.TrimPrefix(line, "claim ")))
			if !ok {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed claim", path, lineNumber)
			}
			for key, value := range options {
				meta.Claims[key] = value
			}
		case strings.HasPrefix(line, "metric "):
			meta.Metrics = append(meta.Metrics, strings.TrimSpace(strings.TrimPrefix(line, "metric ")))
		case strings.HasPrefix(line, "case "):
			fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "case ")))
			if len(fields) != 2 {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed canonical case", path, lineNumber)
			}
			meta.Cases = append(meta.Cases, CaseSpec{Name: fields[0], State: fields[1]})
		}
	}
	if err := scanner.Err(); err != nil {
		return MetaCode{}, err
	}
	if err := validateMetaCode(meta); err != nil {
		return MetaCode{}, err
	}
	return meta, nil
}

func parseMetaActivity(line string) (ActivitySpec, bool) {
	computedAt := strings.Index(line, " computes ")
	if computedAt < 0 {
		return ActivitySpec{}, false
	}
	left := strings.TrimSpace(strings.TrimPrefix(line[:computedAt], "activity "))
	open := strings.IndexByte(left, '(')
	if open < 1 {
		return ActivitySpec{}, false
	}
	close := strings.IndexByte(left[open+1:], ')')
	if close < 0 {
		return ActivitySpec{}, false
	}
	close += open + 1
	name := strings.TrimSpace(left[:open])
	input := strings.TrimSpace(left[open+1 : close])
	rest := strings.TrimSpace(left[close+1:])
	if !strings.HasPrefix(rest, "->") {
		return ActivitySpec{}, false
	}
	output := strings.TrimSpace(strings.TrimPrefix(rest, "->"))
	if name == "" || input == "" || output == "" || strings.Contains(output, " ") {
		return ActivitySpec{}, false
	}
	program, err := strconv.Unquote(strings.TrimSpace(line[computedAt+len(" computes "):]))
	if err != nil || program == "" {
		return ActivitySpec{}, false
	}
	options, ok := parseProgram(program)
	if !ok {
		return ActivitySpec{}, false
	}
	return ActivitySpec{Name: name, InputType: input, OutputType: output, Computes: program, Kind: options["kind"], Options: options}, true
}

func parseMetaEdge(value string) (EdgeSpec, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return EdgeSpec{}, false
	}
	roles := strings.Fields(strings.TrimSpace(parts[0]))
	if len(roles) != 3 || roles[1] != "->" || strings.TrimSpace(parts[1]) == "" {
		return EdgeSpec{}, false
	}
	return EdgeSpec{From: roles[0], To: roles[2], ValueType: strings.TrimSpace(parts[1])}, true
}

func parseQuotedPair(value string) (string, string, bool) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return "", "", false
	}
	parsed, err := strconv.Unquote(fields[1])
	if err != nil || parsed == "" {
		return "", "", false
	}
	return fields[0], parsed, true
}

func parseOptions(value string) (map[string]string, bool) {
	result := map[string]string{}
	for _, field := range strings.Fields(value) {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, false
		}
		result[parts[0]] = parts[1]
	}
	return result, true
}

func parseProgram(program string) (map[string]string, bool) {
	parts := strings.Split(program, ";")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}
	options, ok := parseOptions(strings.Join(parts[1:], " "))
	if !ok && len(parts) > 1 {
		return nil, false
	}
	if options == nil {
		options = map[string]string{}
	}
	options["program"] = parts[0]
	return options, true
}

func parseIntOption(options map[string]string, key string) int {
	value, err := strconv.Atoi(options[key])
	if err != nil {
		return 0
	}
	return value
}

func validateMetaCode(meta MetaCode) error {
	if meta.Package == "" || meta.Namespace == "" || meta.Operation == "" || meta.Version == "" {
		return fmt.Errorf("metacode identity is incomplete")
	}
	if len(meta.Activities) == 0 || len(meta.Edges) != len(meta.Activities)-1 {
		return fmt.Errorf("metacode activity graph is incomplete")
	}
	if len(meta.Precedence) == 0 || len(meta.States) == 0 || meta.TerminalState == "" || !contains(meta.States, meta.TerminalState) || len(meta.UnknownFields) == 0 || len(meta.ProofClasses) == 0 || len(meta.IndicatorClasses) == 0 || len(meta.ReleaseLockFields) == 0 {
		return fmt.Errorf("metacode status contract is incomplete")
	}
	if meta.Migration.Operation == "" {
		return fmt.Errorf("metacode migration is missing")
	}
	for _, path := range []string{"activity_file", "profile", "release_locks", "assessment", "registry", "report", "history"} {
		if meta.Paths[path] == "" {
			return fmt.Errorf("metacode path %q is missing", path)
		}
	}
	if meta.Templates["report"] == "" || meta.Templates["history"] == "" {
		return fmt.Errorf("projection templates are missing")
	}
	if len(meta.Metrics) == 0 {
		return fmt.Errorf("metacode metrics are missing")
	}
	seen := map[string]bool{}
	for _, activity := range meta.Activities {
		if activity.Name == "" || activity.Kind == "" || seen[activity.Name] {
			return fmt.Errorf("metacode activity identity is invalid")
		}
		seen[activity.Name] = true
	}
	for _, edge := range meta.Edges {
		if !seen[edge.From] || !seen[edge.To] || edge.ValueType == "" {
			return fmt.Errorf("metacode edge references an unknown activity")
		}
	}
	return nil
}

func executionOrder(meta MetaCode) ([]ActivitySpec, error) {
	byName := map[string]ActivitySpec{}
	for _, activity := range meta.Activities {
		byName[activity.Name] = activity
	}
	adjacency := map[string][]string{}
	indegree := map[string]int{}
	for name := range byName {
		indegree[name] = 0
	}
	for _, edge := range meta.Edges {
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		indegree[edge.To]++
	}
	queue := make([]string, 0)
	for _, activity := range meta.Activities {
		if indegree[activity.Name] == 0 {
			queue = append(queue, activity.Name)
		}
	}
	order := make([]ActivitySpec, 0, len(meta.Activities))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		order = append(order, byName[name])
		for _, next := range adjacency[name] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != len(meta.Activities) {
		return nil, fmt.Errorf("metacode graph contains a cycle")
	}
	return order, nil
}
