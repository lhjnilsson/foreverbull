package stream

// Notice is returned by a command that completed but has a sentence the
// user must read, such as prices the ingest filled. It is not a failure:
// the subscriber publishes STATE_COMPLETED with Text in the state's error
// field, WaitForCompletion returns the Notice, and RunOrchestration emits
// a completed step whose message is Text. Callers that only need success
// or failure check it with errors.As before treating an error as fatal.
type Notice struct {
	Text string
}

func (n *Notice) Error() string {
	return n.Text
}
