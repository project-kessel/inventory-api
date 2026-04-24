package model

// HealthResult contains the outcome of a health check.
type HealthResult struct {
	status string
	code   int
}

func NewHealthResult(status string, code int) HealthResult {
	return HealthResult{status: status, code: code}
}

func (h HealthResult) Status() string { return h.status }
func (h HealthResult) Code() int      { return h.code }
