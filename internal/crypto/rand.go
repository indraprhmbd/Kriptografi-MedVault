package crypto

import "crypto/rand"

// randRead is a variable so tests can substitute a deterministic source when a
// specific byte pattern is required. Production code always uses crypto/rand.
var randRead = rand.Read
