package lint

import (
	"context"
	"encoding/json"
	"fmt"
	"go/build"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Issues []Issue `json:"Issues,omitempty"`
	Report Report  `json:"Report,omitempty"`
}

type Issue struct {
	FromLinter  string      `json:"FromLinter,omitempty"`
	Text        string      `json:"Text,omitempty"`
	Severity    string      `json:"Severity,omitempty"`
	SourceLines []string    `json:"SourceLines,omitempty"`
	Replacement interface{} `json:"Replacement,omitempty"`
	Pos         struct {
		Filename string `json:"Filename,omitempty"`
		Offset   int    `json:"Offset,omitempty"`
		Line     int    `json:"Line,omitempty"`
		Column   int    `json:"Column,omitempty"`
	} `json:"Pos,omitempty"`
	ExpectNoLint         bool   `json:"ExpectNoLint,omitempty"`
	ExpectedNoLintLinter string `json:"ExpectedNoLintLinter,omitempty"`
}

type Report struct {
	Linters []struct {
		Name             string `json:"Name,omitempty"`
		Enabled          bool   `json:"Enabled,omitempty"`
		EnabledByDefault bool   `json:"EnabledByDefault,omitempty"`
	} `json:"Linters,omitempty"`
}

func Lint(ctx context.Context, repositoryDirectory string) (*Result, error) {
	_, err := installMissing("golangci-lint", "github.com/golangci/golangci-lint", "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.0")
	if err != nil {
		return nil, fmt.Errorf("install golangci-lint: %w", err)
	}

	// TODO: we might have to tweak the skip dirs when running it in a server.
	cmd := exec.CommandContext(ctx, "golangci-lint",
		"run",
		"--skip-dirs", "opt/homebrew,go/pkg",
		"--out-format", "json",
		"--issues-exit-code", "42")
	cmd.Dir = repositoryDirectory
	b, err := cmd.CombinedOutput()

	if err != nil && !strings.HasPrefix(err.Error(), "exit status 42") {
		return nil, fmt.Errorf("%s: %s", err.Error(), string(b))
	}

	var result Result
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func installMissing(bin, getPath, importPath string) (string, error) {
	if b, err := findBin(bin); err == nil {
		return b, nil
	}
	if data, err := exec.Command("go", "get", getPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to get %s: %v: %s", importPath, err, string(data))
	}

	if data, err := exec.Command("go", "install", importPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to install %s: %v: %s", importPath, err, string(data))
	}
	b, err := findBin(bin)
	if err != nil {
		return "", fmt.Errorf("failed to lookup %v after install: %v", bin, err)
	}
	return b, nil
}

func findBin(bin string) (string, error) {
	if _, err := exec.LookPath(bin); err == nil {
		return bin, nil
	}
	srcDirs := build.Default.SrcDirs()
	for _, src := range srcDirs {
		binFile := filepath.Join(filepath.Dir(src), "bin", bin)
		if _, err := exec.LookPath(binFile); err == nil {
			return binFile, nil
		}
	}
	return "", fmt.Errorf("failed to find binary: %v", bin)
}
