// Healthcheck program
package main

import (
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	c := &http.Client{
		Timeout: 1 * time.Second,
	}

	resp, err := c.Head("http://127.0.0.1:2375/_ping")
	if err != nil {
		log.Fatalf("request error: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Fatalf("error closing response body: %s", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("error reading response body: %s", err)
		}

		log.Fatalf("bad response status: %s, %s", resp.Status, body)
	}
}
