package health

import "context"

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func Evaluate(ctx context.Context, checkers []Checker) (bool, []CheckResult) {
	if len(checkers) == 0 {
		return true, nil
	}
	results := make([]CheckResult, 0, len(checkers))
	healthy := true
	for _, checker := range checkers {
		if checker == nil {
			continue
		}
		if err := checker.Check(ctx); err != nil {
			healthy = false
			results = append(results, CheckResult{Name: checker.Name(), Status: "DOWN", Error: err.Error()})
			continue
		}
		results = append(results, CheckResult{Name: checker.Name(), Status: "UP"})
	}
	return healthy, results
}
