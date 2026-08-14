package main

import "testing"

func TestEnvironmentValue(t *testing.T) {
	const environmentVariable = "CADANA_TEST_ENVIRONMENT_VALUE"
	testCases := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{name: "configured value", value: "configured", fallback: "fallback", want: "configured"},
		{name: "fallback value", fallback: "fallback", want: "fallback"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(environmentVariable, testCase.value)
			if got := environmentValue(environmentVariable, testCase.fallback); got != testCase.want {
				t.Errorf(
					"environmentValue(%q, %q) = %q, want %q",
					environmentVariable,
					testCase.fallback,
					got,
					testCase.want,
				)
			}
		})
	}
}
