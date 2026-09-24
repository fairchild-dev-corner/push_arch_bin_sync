package utils

import (
	"log"
	"net/http"
	"time"
)

// CheckServiceUp polls url (a plain GET) up to maxWait, retrying once a
// second, and logs whether name came up in that window. It never fails
// startup - Prometheus/Grafana are observability tooling, not a dependency
// this daemon needs to run. Intermediate retries aren't logged, only the
// final outcome - Grafana in particular can take a few seconds to boot, so
// a single immediate check would false-negative on a normal startup race.
func CheckServiceUp(name, url string, maxWait time.Duration) bool {
	client := http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(maxWait)

	var lastErr error
	var lastStatus int

	for {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
		} else {
			lastStatus = resp.StatusCode
			resp.Body.Close()
			if lastStatus >= 200 && lastStatus < 300 {
				log.Printf("%s is running (%s)", name, url)
				return true
			}
			lastErr = nil
		}

		if time.Now().After(deadline) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		log.Printf("%s is not reachable at %s: %v", name, url, lastErr)
	} else {
		log.Printf("%s responded with HTTP %d at %s", name, lastStatus, url)
	}
	return false
}
