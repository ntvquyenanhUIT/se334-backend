package models

type TestCase struct {
	ID             int
	ProblemID      int
	Input          string
	ExpectedOutput string
}
