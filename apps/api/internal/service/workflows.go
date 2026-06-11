package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkflowSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type WorkflowVariable struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type WorkflowStep struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Agent          string `json:"agent,omitempty"`
	ApproverPolicy string `json:"approverPolicy,omitempty"`
}

type WorkflowDetail struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Path        string             `json:"path"`
	Variables   []WorkflowVariable `json:"variables"`
	Steps       []WorkflowStep     `json:"steps"`
}

type WorkflowValidationIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorkflowValidationResult struct {
	WorkflowID string                    `json:"workflowId"`
	Valid      bool                      `json:"valid"`
	Errors     []WorkflowValidationIssue `json:"errors"`
	Warnings   []WorkflowValidationIssue `json:"warnings"`
}

type WorkflowDryRunStep struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Agent    string `json:"agent,omitempty"`
	Blocking bool   `json:"blocking"`
}

type WorkflowDryRunResult struct {
	WorkflowID       string                `json:"workflowId"`
	Ready            bool                  `json:"ready"`
	MissingVariables []string              `json:"missingVariables"`
	Warnings         []WorkflowValidationIssue `json:"warnings"`
	Steps            []WorkflowDryRunStep  `json:"steps"`
}

type WorkflowService struct {
	root string
}

func NewWorkflowService(repoRoot string) WorkflowService {
	root := repoRoot
	if root == "" {
		root = discoverRepoRoot()
	}

	if filepath.Base(root) != "workflows" {
		root = filepath.Join(root, "workflows")
	}
	return WorkflowService{root: root}
}

func (s WorkflowService) List() ([]WorkflowSummary, error) {
	base := s.root
	var results []WorkflowSummary

	if _, err := os.Stat(base); err != nil {
		return []WorkflowSummary{}, nil
	}

	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Name() != "workflow.yaml" {
			return err
		}

		detail, readErr := parseWorkflowDetail(path)
		if readErr != nil {
			return readErr
		}

		results = append(results, WorkflowSummary{
			ID:          detail.ID,
			Name:        detail.Name,
			Version:     detail.Version,
			Description: detail.Description,
			Path:        detail.Path,
		})

		return nil
	})

	return results, err
}

func (s WorkflowService) Get(id string) (WorkflowDetail, bool, error) {
	workflowID := strings.TrimSpace(id)
	if workflowID == "" {
		return WorkflowDetail{}, false, nil
	}

	base := s.root
	if _, err := os.Stat(base); err != nil {
		return WorkflowDetail{}, false, nil
	}

	var found WorkflowDetail
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Name() != "workflow.yaml" {
			return err
		}

		detail, readErr := parseWorkflowDetail(path)
		if readErr != nil {
			return readErr
		}

		if detail.ID == workflowID {
			found = detail
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil && !errors.Is(err, filepath.SkipDir) {
		return WorkflowDetail{}, false, err
	}

	if found.ID == "" {
		return WorkflowDetail{}, false, nil
	}

	return found, true, nil
}

func (s WorkflowService) Exists(id string) (bool, error) {
	workflowID := strings.TrimSpace(id)
	if workflowID == "" {
		return false, nil
	}

	items, err := s.List()
	if err != nil {
		return false, err
	}

	for _, item := range items {
		if item.ID == workflowID {
			return true, nil
		}
	}

	return false, nil
}

func (s WorkflowService) Validate(id string) (WorkflowValidationResult, bool, error) {
	workflow, found, err := s.Get(id)
	if err != nil || !found {
		return WorkflowValidationResult{}, found, err
	}

	result := validateWorkflowDetail(workflow)
	return result, true, nil
}

func (s WorkflowService) DryRun(id string, variables map[string]string) (WorkflowDryRunResult, bool, error) {
	workflow, found, err := s.Get(id)
	if err != nil || !found {
		return WorkflowDryRunResult{}, found, err
	}

	validation := validateWorkflowDetail(workflow)
	missingKeys := missingRequiredVariableKeys(workflow, normalizeInputVariables(variables))
	steps := make([]WorkflowDryRunStep, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		steps = append(steps, WorkflowDryRunStep{
			ID:       step.ID,
			Kind:     step.Kind,
			Agent:    step.Agent,
			Blocking: step.Kind == "human_approval",
		})
	}

	warnings := append([]WorkflowValidationIssue{}, validation.Warnings...)
	warnings = append(warnings, WorkflowValidationIssue{
		Field:   "runtime",
		Code:    "execution_not_available",
		Message: "dry-run reports contract readiness only; live step execution is not implemented yet",
	})

	return WorkflowDryRunResult{
		WorkflowID:       workflow.ID,
		Ready:            validation.Valid && len(missingKeys) == 0,
		MissingVariables: missingKeys,
		Warnings:         warnings,
		Steps:            steps,
	}, true, nil
}

func validateWorkflowDetail(workflow WorkflowDetail) WorkflowValidationResult {
	result := WorkflowValidationResult{
		WorkflowID: workflow.ID,
		Valid:      true,
		Errors:     []WorkflowValidationIssue{},
		Warnings:   []WorkflowValidationIssue{},
	}

	allowedKinds := map[string]struct{}{
		"agent_task":       {},
		"human_approval":   {},
		"asset_transform":  {},
		"shell_command":    {},
		"git_checkpoint":   {},
		"artifact_publish": {},
		"noop":             {},
	}

	if len(workflow.Steps) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, WorkflowValidationIssue{
			Field:   "steps",
			Code:    "missing_steps",
			Message: "workflow must declare at least one step",
		})
	}

	for index, step := range workflow.Steps {
		fieldPrefix := fmt.Sprintf("steps[%d]", index)
		if _, ok := allowedKinds[step.Kind]; !ok {
			result.Valid = false
			result.Errors = append(result.Errors, WorkflowValidationIssue{
				Field:   fieldPrefix + ".kind",
				Code:    "unknown_step_kind",
				Message: fmt.Sprintf("unsupported step kind %q", step.Kind),
			})
		}

		if step.Kind == "human_approval" && strings.TrimSpace(step.ApproverPolicy) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, WorkflowValidationIssue{
				Field:   fieldPrefix + ".approverPolicy",
				Code:    "missing_approver_policy",
				Message: "human_approval steps require approverPolicy",
			})
		}
	}

	return result
}

type workflowYAML struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Variables   []struct {
		Key         string `yaml:"key"`
		Type        string `yaml:"type"`
		Required    bool   `yaml:"required"`
		Description string `yaml:"description"`
	} `yaml:"variables"`
	Steps []struct {
		ID             string `yaml:"id"`
		Kind           string `yaml:"kind"`
		Agent          string `yaml:"agent"`
		ApproverPolicy string `yaml:"approverPolicy"`
	} `yaml:"steps"`
	Metadata struct {
		ID          string `yaml:"id"`
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Description string `yaml:"description"`
	} `yaml:"metadata"`
}

func parseWorkflowDetail(path string) (WorkflowDetail, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return WorkflowDetail{}, err
	}

	var cfg workflowYAML
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return WorkflowDetail{}, fmt.Errorf("parse workflow yaml %s: %w", path, err)
	}

	id := firstNonEmpty(cfg.Metadata.ID, cfg.ID)
	if id == "" {
		id = filepath.Base(filepath.Dir(path))
	}

	variables := make([]WorkflowVariable, 0, len(cfg.Variables))
	for _, variable := range cfg.Variables {
		variables = append(variables, WorkflowVariable{
			Key:         variable.Key,
			Type:        variable.Type,
			Required:    variable.Required,
			Description: variable.Description,
		})
	}

	steps := make([]WorkflowStep, 0, len(cfg.Steps))
	for _, step := range cfg.Steps {
		steps = append(steps, WorkflowStep{
			ID:             step.ID,
			Kind:           step.Kind,
			Agent:          step.Agent,
			ApproverPolicy: step.ApproverPolicy,
		})
	}

	return WorkflowDetail{
		ID:          id,
		Name:        firstNonEmpty(cfg.Metadata.Name, cfg.Name),
		Version:     firstNonEmpty(cfg.Metadata.Version, cfg.Version),
		Description: firstNonEmpty(cfg.Metadata.Description, cfg.Description),
		Path:        path,
		Variables:   variables,
		Steps:       steps,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func discoverRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("..", "..", "..")
	}

	current := wd
	for {
		if dirExists(filepath.Join(current, "workflows")) {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return filepath.Join("..", "..", "..")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
