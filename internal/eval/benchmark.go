package eval

// BenchmarkCase represents a single golden test case.
type BenchmarkCase struct {
	CaseFile    string `json:"case_file"`
	ExpectedRCA string `json:"expected_rca"`
	Category    string `json:"category"` // redis_timeout, db_connection_leak, high_error_rate, etc.
}

// BenchmarkSuite is a collection of golden cases to run.
type BenchmarkSuite struct {
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Cases     []BenchmarkCase `json:"cases"`
	CreatedAt string          `json:"created_at"`
}
