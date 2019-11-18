package referees

type MockExecutor struct{}

func (me *MockExecutor) GetMetricsLabelValue() string {
	return "value"
}
