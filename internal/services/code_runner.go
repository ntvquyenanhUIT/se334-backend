package services

import (
	"HAB/internal/logger"
	"HAB/internal/models"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

type LanguageConfig struct {
	ContainerImage   string
	FileExtension    string
	BuildCommand     []string // Empty for interpreted languages
	RunCommand       []string
	NeedsCompilation bool
}

type TestResult struct {
	TestCaseID     int
	Passed         bool
	ExpectedOutput string
	ActualOutput   string
	Error          string
}

type ExecutionResult struct {
	Status           string
	Results          []TestResult
	CompilationError string
	FailedTestID     *int
	FailedOutput     *string
	ExecutionTime    time.Duration
}

type CodeRunnerRequest struct {
	Submission   models.Submission
	TestCases    []TestCase
	SystemCode   string
	ImportCode   string
	LanguageName string
}

type TestCase struct {
	ID       int
	Input    string
	Expected string
}

type CodeRunnerService struct {
	workDir string
}

func NewCodeRunnerService(workDir string) (*CodeRunnerService, error) {
	// Create working directory if it doesn't exist
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	return &CodeRunnerService{
		workDir: workDir,
	}, nil
}

var languageConfigs = map[string]LanguageConfig{
	"go": {
		ContainerImage:   "go-runner",
		FileExtension:    "go",
		BuildCommand:     []string{"go", "build", "-o", "solution", "main.go"},
		RunCommand:       []string{"./solution"},
		NeedsCompilation: true,
	},
	"python": {
		ContainerImage:   "python-runner",
		FileExtension:    "py",
		BuildCommand:     []string{}, // Python doesn't need compilation
		RunCommand:       []string{"python", "main.py"},
		NeedsCompilation: false,
	},
}

func GetLanguageConfig(languageID int) (string, LanguageConfig, error) {
	var languageName string

	switch languageID {
	case 1:
		languageName = "python"
	case 2:
		languageName = "go"
	default:
		return "", LanguageConfig{}, fmt.Errorf("unsupported language ID: %d", languageID)
	}

	config, ok := languageConfigs[languageName]
	if !ok {
		return "", LanguageConfig{}, fmt.Errorf("configuration not found for language: %s", languageName)
	}

	return languageName, config, nil
}

func (s *CodeRunnerService) Execute(ctx context.Context, req CodeRunnerRequest) (*ExecutionResult, error) {
	startTime := time.Now()
	langConfig, ok := languageConfigs[req.LanguageName]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", req.LanguageName)
	}

	execDir := filepath.Join(s.workDir, fmt.Sprintf("submission_%d", req.Submission.ID))
	if err := os.MkdirAll(execDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create execution directory: %w", err)
	}
	defer os.RemoveAll(execDir) // Clean up when done

	fullCode := combineCode(req.ImportCode, req.Submission.SourceCode, req.SystemCode, req.LanguageName)

	codeFilePath := filepath.Join(execDir, fmt.Sprintf("main.%s", langConfig.FileExtension))
	if err := os.WriteFile(codeFilePath, []byte(fullCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code file: %w", err)
	}

	containerID, err := s.startContainer(codeFilePath, req.LanguageName)
	if err != nil {
		// If compilation error, return immediately
		if strings.Contains(err.Error(), "compilation error") {
			return &ExecutionResult{
				Status:           models.StatusCompilationError,
				CompilationError: err.Error(),
				ExecutionTime:    time.Since(startTime),
			}, nil
		}
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	defer exec.Command("docker", "stop", containerID).Run()

	results := make([]TestResult, 0, len(req.TestCases))

	for _, tc := range req.TestCases {
		result, err := s.executeTestCase(containerID, tc, req.LanguageName)

		if err != nil {
			// Save error output and stop execution
			errorOutput := err.Error()
			return &ExecutionResult{
				Status:        models.StatusCompilationError,
				Results:       results,
				FailedTestID:  &tc.ID,
				FailedOutput:  &errorOutput,
				ExecutionTime: time.Since(startTime),
			}, nil
		}

		results = append(results, result)

		if !result.Passed {
			return &ExecutionResult{
				Status:        models.StatusWrongAnswer,
				Results:       results,
				FailedTestID:  &tc.ID,
				FailedOutput:  &result.ActualOutput,
				ExecutionTime: time.Since(startTime),
			}, nil
		}
	}

	return &ExecutionResult{
		Status:        models.StatusAccepted,
		Results:       results,
		ExecutionTime: time.Since(startTime),
	}, nil
}

func (s *CodeRunnerService) startContainer(codePath, language string) (string, error) {
	langConfig, ok := languageConfigs[language]
	if !ok {
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	absCodePath, err := filepath.Abs(codePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Start container with code mounted
	cmd := exec.Command(
		"docker", "run", "-d", "--rm",
		"-v", fmt.Sprintf("%s:/app/main.%s", absCodePath, langConfig.FileExtension),
		"-w", "/app",
		langConfig.ContainerImage,
		"tail", "-f", "/dev/null",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("container start error: %v, output: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))

	// Compile if necessary (only for compiled languages)
	if langConfig.NeedsCompilation && len(langConfig.BuildCommand) > 0 {
		compileArgs := append([]string{"exec", containerID}, langConfig.BuildCommand...)
		compileCmd := exec.Command("docker", compileArgs...)

		compileOutput, err := compileCmd.CombinedOutput()
		if err != nil {
			// Stop the container if compilation fails
			exec.Command("docker", "stop", containerID).Run()
			return "", fmt.Errorf("compilation error: %v, output: %s", err, compileOutput)
		}
	}

	return containerID, nil
}

// executeTestCase runs a single test case in the container
func (s *CodeRunnerService) executeTestCase(containerID string, tc TestCase, language string) (TestResult, error) {
	langConfig, ok := languageConfigs[language]
	if !ok {
		return TestResult{}, fmt.Errorf("unsupported language: %s", language)
	}

	// Construct the docker exec command with the language-specific run command
	args := append([]string{"exec", "-i", containerID}, langConfig.RunCommand...)
	cmd := exec.Command("docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = strings.NewReader(tc.Input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Log.Debug("Executing test case",
		zap.Int("testcase_id", tc.ID),
		zap.String("container_id", containerID),
		zap.Strings("command", langConfig.RunCommand),
	)

	err := cmd.Run()

	// Check for execution errors
	if err != nil {
		return TestResult{
			TestCaseID:     tc.ID,
			Passed:         false,
			ExpectedOutput: tc.Expected,
			ActualOutput:   stdout.String(),
			Error:          fmt.Sprintf("execution error: %v, stderr: %s", err, stderr.String()),
		}, errors.New(stderr.String())
	}

	// Compare output
	actualOutput := strings.TrimSpace(stdout.String())
	expectedOutput := strings.TrimSpace(tc.Expected)

	return TestResult{
		TestCaseID:     tc.ID,
		Passed:         actualOutput == expectedOutput,
		ExpectedOutput: expectedOutput,
		ActualOutput:   actualOutput,
	}, nil
}

// combineCode combines the import, user and system code into a complete file
func combineCode(importCode, userCode, systemCode, language string) string {
	switch language {
	case "python":
		return importCode + "\n\n" + userCode + "\n\n" + systemCode
	case "go":
		return importCode + "\n\n" + userCode + "\n\n" + systemCode
	default:
		return userCode // Fallback
	}
}
