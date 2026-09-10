package logging

import "testing"

func TestLoggingBeforeInitializationIsSafe(t *testing.T) {
	t.Parallel()
	Infof("startup test")
}
