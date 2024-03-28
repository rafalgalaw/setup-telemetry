package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/sethvargo/go-githubactions"
)

const actionName = "set-up-telemetry"

var (
	BUILD_VERSION string
	BUILD_DATE    string
	COMMIT_ID     string
)

func generateTraceID(runID int64, runAttempt int) string {
	input := fmt.Sprintf("%d%dt", runID, runAttempt)
	hash := sha256.Sum256([]byte(input))
	traceIDHex := hex.EncodeToString(hash[:])
	traceID := traceIDHex[:32]
	return traceID
}

func getGitHubJobInfo(ctx context.Context, token, owner, repo string, runID, attempt int64) (jobID, jobName string, err error) {
	splitRepo := strings.Split(repo, "/")
	if len(splitRepo) != 2 {
		return "", "", fmt.Errorf("GITHUB_REPOSITORY environment variable is malformed: %s", repo)
	}
	owner, repo = splitRepo[0], splitRepo[1]

	client := github.NewClient(nil).WithAuthToken(token)

	opts := &github.ListWorkflowJobsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	runJobs, _, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return "", "", err
	}

	runnerName := os.Getenv("RUNNER_NAME")
	for _, job := range runJobs.Jobs {
		if *job.RunAttempt == attempt && *job.RunnerName == runnerName {
			return strconv.FormatInt(*job.ID, 10), *job.Name, nil
		}
	}

	return "", "", fmt.Errorf("no job found matching the criteria")
}

func main() {
	ctx := context.Background()
	githubactions.Infof("Starting %s version: %s (%s) commit: %s", actionName, BUILD_VERSION, BUILD_DATE, COMMIT_ID)

	if githubactions.GetInput("github-token") == "" {
		githubactions.Fatalf("No GitHub token provided")
	}

	githubToken := githubactions.GetInput("github-token")
	runID, _ := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64)
	runAttempt, _ := strconv.Atoi(os.Getenv("GITHUB_RUN_ATTEMPT"))

	traceID := generateTraceID(runID, runAttempt)
	githubactions.SetOutput("trace-id", traceID)
	githubactions.Infof("Trace ID: %s", traceID)

	jobID, jobName, err := getGitHubJobInfo(ctx, githubToken, os.Getenv("GITHUB_REPOSITORY_OWNER"), os.Getenv("GITHUB_REPOSITORY"), runID, int64(runAttempt))
	if err != nil {
		githubactions.Errorf("Error getting job info: %v", err)
		os.Exit(1)
	}

	githubactions.SetOutput("job-id", jobID)
	githubactions.SetOutput("job-name", jobName)
	githubactions.Infof("Job ID: %s, Job name: %s", jobID, jobName)
}
