package httpapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parseLimitOffset(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	q := r.URL.Query()

	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}

func parseRFC3339Ptr(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, fmt.Errorf("invalid time (RFC3339 expected)")
	}
	return &t, nil
}

func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if v := strings.TrimSpace(r.Header.Get("X-Real-Ip")); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
