package repository

// SetSyncTimeouts shortens Sync's lock and idle timeouts for a test and
// returns a func that restores them.
func SetSyncTimeouts(lock, idle string) func() {
	oldLock, oldIdle := syncLockTimeout, syncIdleTimeout
	syncLockTimeout, syncIdleTimeout = lock, idle
	return func() { syncLockTimeout, syncIdleTimeout = oldLock, oldIdle }
}
