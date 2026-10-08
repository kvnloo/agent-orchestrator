package github

// workflowCheckKey identifies one GitHub Actions job across repeated workflow
// invocations on the same commit. GitHub retains every CheckRun in the rollup;
// the workflow id prevents equally named jobs in unrelated workflows from
// suppressing each other.
type workflowCheckKey struct {
	workflowID int64
	name       string
}

type workflowRunOrder struct {
	runNumber  int64
	runAttempt int64
}

func (o workflowRunOrder) newerThan(other workflowRunOrder) bool {
	return o.runNumber > other.runNumber ||
		(o.runNumber == other.runNumber && o.runAttempt > other.runAttempt)
}

// filterSupersededWorkflowChecks keeps only the newest run/attempt for each
// GitHub Actions workflow + job name. CheckRuns without complete workflow
// metadata and legacy StatusContexts remain untouched: without an identity AO
// cannot prove that one occurrence supersedes another.
func filterSupersededWorkflowChecks(raw []map[string]any) []map[string]any {
	newest := make(map[workflowCheckKey]workflowRunOrder)
	for _, node := range raw {
		key, order, ok := workflowCheckIdentity(node)
		if !ok {
			continue
		}
		if current, found := newest[key]; !found || order.newerThan(current) {
			newest[key] = order
		}
	}

	out := make([]map[string]any, 0, len(raw))
	for _, node := range raw {
		key, order, ok := workflowCheckIdentity(node)
		if !ok || order == newest[key] {
			out = append(out, node)
		}
	}
	return out
}

func workflowCheckIdentity(node map[string]any) (workflowCheckKey, workflowRunOrder, bool) {
	if str(node["__typename"]) != "CheckRun" {
		return workflowCheckKey{}, workflowRunOrder{}, false
	}
	name := str(node["name"])
	checkSuite, _ := node["checkSuite"].(map[string]any)
	workflowRun, _ := checkSuite["workflowRun"].(map[string]any)
	workflow, _ := workflowRun["workflow"].(map[string]any)
	workflowID := int64(num(workflow["databaseId"]))
	runNumber := int64(num(workflowRun["runNumber"]))
	runAttempt := int64(num(workflowRun["runAttempt"]))
	if runAttempt <= 0 {
		runAttempt = 1
	}
	if name == "" || workflowID <= 0 || runNumber <= 0 {
		return workflowCheckKey{}, workflowRunOrder{}, false
	}
	return workflowCheckKey{workflowID: workflowID, name: name}, workflowRunOrder{
		runNumber: runNumber, runAttempt: runAttempt,
	}, true
}
