package usercopy

import "fmt"

// ExecutionReadFailed is the sentence for an execution that couldn't be
// read, when it isn't known to be missing.
func ExecutionReadFailed(executionID, command string) string {
	return fmt.Sprintf("Couldn't read execution %s. %s", executionID, RetryStep(command))
}

// ExecutionIDInvalid is the sentence when an execution id cannot be a result key.
// The id itself is left out: a rejected value can be a path or a URL.
func ExecutionIDInvalid() string {
	return "The execution id is not valid."
}

// ExecutionResultMissing is the sentence when the execution exists and its
// stored result does not.
func ExecutionResultMissing() string {
	return "This execution has no stored result."
}

// ExecutionResultTooLarge is the sentence when the stored result is above the
// byte cap. The cap stays off the sentence.
func ExecutionResultTooLarge() string {
	return "The stored result for this execution is too large to return."
}
