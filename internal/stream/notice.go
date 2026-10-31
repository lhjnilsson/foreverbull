package stream

import "strings"

// Notice is returned by a command that completed but has lines the user
// must read. It is not a failure: the
// subscriber publishes STATE_COMPLETED with Lines in the state's notices
// field and leaves its error empty, WaitForCompletion returns the Notice,
// and RunOrchestration emits a completed step that carries Lines in its
// notices. Callers that only need success or failure check it with
// errors.As before treating an error as fatal.
type Notice struct {
	Lines []string
}

func (n *Notice) Error() string {
	return strings.Join(n.Lines, "\n")
}
