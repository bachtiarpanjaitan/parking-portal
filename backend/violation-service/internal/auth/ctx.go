package auth

import "context"

// nilCtx is a tiny helper used by Service.Login so the repository method
// signature can stay context-agnostic at the auth layer. In production the
// handler will pass the request context instead.
func nilCtx() context.Context { return context.Background() }
