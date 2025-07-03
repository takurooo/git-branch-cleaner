package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Branch struct {
	Name           string
	IsMerged       bool
	LastCommitDate time.Time
	LastCommitMsg  string
	CommitsAhead   int
	Author         string
	IsMain         bool
	Selected       bool
}

func isGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

func getRepositoryPath() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository path: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func getDefaultBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err == nil {
		defaultBranch := strings.TrimSpace(string(output))
		defaultBranch = strings.TrimPrefix(defaultBranch, "refs/remotes/origin/")
		
		cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+defaultBranch)
		if cmd.Run() == nil {
			return defaultBranch, nil
		}
	}
	
	branches := []string{"main", "master"}
	for _, branch := range branches {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if cmd.Run() == nil {
			return branch, nil
		}
	}
	return "main", nil
}

func getAllBranches() ([]Branch, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)|%(authorname)|%(committerdate:iso8601)|%(subject)", "refs/heads/")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	defaultBranch, err := getDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}
	var branches []Branch

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		branchName := parts[0]
		author := parts[1]
		dateStr := parts[2]
		subject := parts[3]

		commitDate, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			commitDate = time.Now()
		}

		isMerged := isBranchMerged(branchName, defaultBranch)
		commitsAhead := getCommitsAhead(branchName, defaultBranch)
		isDefault := branchName == defaultBranch

		branch := Branch{
			Name:           branchName,
			IsMerged:       isMerged,
			LastCommitDate: commitDate,
			LastCommitMsg:  subject,
			CommitsAhead:   commitsAhead,
			Author:         author,
			IsMain:         isDefault,
			Selected:       false,
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

func isBranchMerged(branchName, defaultBranch string) bool {
	if branchName == defaultBranch {
		return false
	}

	cmd := exec.Command("git", "merge-base", "--is-ancestor", branchName, defaultBranch)
	err := cmd.Run()
	return err == nil
}

func getCommitsAhead(branchName, defaultBranch string) int {
	if branchName == defaultBranch {
		return 0
	}

	cmd := exec.Command("git", "rev-list", "--count", branchName, "^"+defaultBranch)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}

	return count
}

func deleteBranches(branchNames []string) error {
	for _, branchName := range branchNames {
		cmd := exec.Command("git", "branch", "-d", branchName)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("git", "branch", "-D", branchName)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
			}
		}
	}
	return nil
}

func validateGitEnvironment() error {
	if !isGitRepository() {
		return fmt.Errorf("current directory is not a git repository")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git command not found in PATH")
	}

	return nil
}
