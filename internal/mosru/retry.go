package mosru

import (
"fmt"
"log"
"time"
)

const (
maxRetries  = 5
baseBackoff = 2 * time.Second
)

// withRetry retries fn up to maxRetries times on error, with exponential
// backoff (2s, 4s, 8s, 16s, 32s). Intended for transient failures like
// upstream 5xx errors or network blips.
func withRetry(description string, fn func() error) error {
var lastErr error

for attempt := 1; attempt <= maxRetries; attempt++ {
err := fn()
if err == nil {
return nil
}

lastErr = err
if attempt < maxRetries {
backoff := baseBackoff * time.Duration(1<<uint(attempt-1))
log.Printf("%s: attempt %d/%d failed: %v (retrying in %s)", description, attempt, maxRetries, err, backoff)
time.Sleep(backoff)
}
}

return fmt.Errorf("%s: all %d attempts failed: %w", description, maxRetries, lastErr)
}
