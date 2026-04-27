package handlers

// stopLeakedSSECoalescer stops the coalescer goroutine attached to the global
// SSE handler, if any. Test setup functions call this before re-initializing
// shared globals (logger, gin mode, sseHandler) so a coalescer goroutine left
// over from a previous test does not race with those mutations.
func stopLeakedSSECoalescer() {
	if sseHandler != nil && sseHandler.coalescer != nil {
		sseHandler.coalescer.stop()
	}
}
