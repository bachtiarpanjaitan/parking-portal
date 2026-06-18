package rules

import "time"

// timeNow returns the current UTC time. Stub-able in tests.
var timeNow = func() time.Time { return time.Now().UTC() }
