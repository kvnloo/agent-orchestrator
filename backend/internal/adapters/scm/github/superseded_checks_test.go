package github

import (
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func workflowCheck(name, conclusion string, workflowID, runNumber, runAttempt int) map[string]any {
	return map[string]any{
		"__typename": "CheckRun",
		"name":       name,
		"status":     "COMPLETED",
		"conclusion": conclusion,
		"detailsUrl": "https://github.com/o/r/actions/runs/1/job/1",
		"checkSuite": map[string]any{
			"workflowRun": map[string]any{
				"databaseId": float64(runNumber * 100),
				"runNumber":  float64(runNumber),
				"runAttempt": float64(runAttempt),
				"workflow": map[string]any{
					"databaseId": float64(workflowID),
				},
			},
		},
	}
}

func prWithCheckRuns(checks ...map[string]any) map[string]any {
	nodes := make([]any, 0, len(checks))
	for _, check := range checks {
		nodes = append(nodes, check)
	}
	return map[string]any{
		"number":     float64(1),
		"url":        "https://github.com/o/r/pull/1",
		"state":      "OPEN",
		"headRefOid": "head-1",
		"commits": map[string]any{
			"nodes": []any{
				map[string]any{
					"commit": map[string]any{
						"oid": "head-1",
						"statusCheckRollup": map[string]any{
							"state": "FAILURE",
							"contexts": map[string]any{
								"nodes":    nodes,
								"pageInfo": map[string]any{"hasNextPage": false},
							},
						},
					},
				},
			},
		},
	}
}

func TestCISummaryIgnoresCanceledRunSupersededByNewerSuccess(t *testing.T) {
	pr := prWithCheckRuns(
		workflowCheck("build-test", "CANCELLED", 301525012, 5585, 1),
		workflowCheck("build-test", "SUCCESS", 301525012, 5586, 1),
	)
	if got := ciSummaryFromGraphQL(pr); got != domain.CIPassing {
		t.Fatalf("CI = %q, want passing after newer successful workflow run", got)
	}
}

func TestCISummaryKeepsNewestCanceledRunFailing(t *testing.T) {
	pr := prWithCheckRuns(
		workflowCheck("build-test", "SUCCESS", 301525012, 5585, 1),
		workflowCheck("build-test", "CANCELLED", 301525012, 5586, 1),
	)
	if got := ciSummaryFromGraphQL(pr); got != domain.CIFailing {
		t.Fatalf("CI = %q, want failing when newest occurrence is canceled", got)
	}
}

func TestCISummaryDoesNotSuppressSameJobAcrossDifferentWorkflows(t *testing.T) {
	pr := prWithCheckRuns(
		workflowCheck("build-test", "CANCELLED", 301525012, 5585, 1),
		workflowCheck("build-test", "SUCCESS", 999999999, 5586, 1),
	)
	if got := ciSummaryFromGraphQL(pr); got != domain.CIFailing {
		t.Fatalf("CI = %q, want failing because a different workflow cannot supersede the cancellation", got)
	}
}

func TestCISummaryUsesNewestAttemptWithinWorkflowRun(t *testing.T) {
	pr := prWithCheckRuns(
		workflowCheck("build-test", "CANCELLED", 301525012, 5585, 1),
		workflowCheck("build-test", "SUCCESS", 301525012, 5585, 2),
	)
	if got := ciSummaryFromGraphQL(pr); got != domain.CIPassing {
		t.Fatalf("CI = %q, want passing after newer successful rerun attempt", got)
	}
}

func TestSCMObservationFiltersSupersededCancellationFromSummaryAndDetails(t *testing.T) {
	pr := prWithCheckRuns(
		workflowCheck("build-test", "CANCELLED", 301525012, 5585, 1),
		workflowCheck("build-test", "SUCCESS", 301525012, 5586, 1),
	)
	obs := scmObservationFromGraphQL(ports.SCMPRRef{
		Repo: ports.SCMRepo{Provider: "github", Host: "github.com", Owner: "o", Name: "r", Repo: "o/r"},
		Number: 1,
		URL:    "https://github.com/o/r/pull/1",
	}, pr)
	if obs.CI.Summary != string(domain.CIPassing) {
		t.Fatalf("summary = %q, want passing", obs.CI.Summary)
	}
	if len(obs.CI.FailedChecks) != 0 {
		t.Fatalf("failed checks = %#v, want none after supersession", obs.CI.FailedChecks)
	}
	if len(obs.CI.Checks) != 1 || obs.CI.Checks[0].Status != string(domain.PRCheckPassed) {
		t.Fatalf("checks = %#v, want only the newer successful occurrence", obs.CI.Checks)
	}
}

func TestCISummaryPreservesCanceledCheckWithoutWorkflowIdentity(t *testing.T) {
	check := workflowCheck("external-check", "CANCELLED", 0, 0, 0)
	delete(check, "checkSuite")
	pr := prWithCheckRuns(check)
	if got := ciSummaryFromGraphQL(pr); got != domain.CIFailing {
		t.Fatalf("CI = %q, want conservative failing state without workflow identity", got)
	}
}
