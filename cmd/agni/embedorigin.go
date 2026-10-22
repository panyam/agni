package main

import (
	"fmt"
	"net/url"
)

// checkEmbedOrigins refuses an --embed-origin that is not an origin. The page compares each against
// MessageEvent.origin as a string, so a value with a path or a trailing slash would match no page
// and read as a viewer that ignores its host, and a "*" would let any page drive it.
func checkEmbedOrigins(origins []string) error {
	for _, o := range origins {
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || o != u.Scheme+"://"+u.Host {
			return fmt.Errorf("--embed-origin %q is not an origin; write it as scheme://host[:port], with no path, such as https://portal.example", o)
		}
	}
	return nil
}
