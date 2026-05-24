// Package middleware provides gin middleware for the API server.
package middleware

import (
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Log returns a gin middleware that logs structured request/response information.
// Paths in skipPaths are logged at trace level; all others at info level.
func Log(skipPaths map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		reqID := requestid.Get(c)

		event := log.Info()
		if _, skip := skipPaths[c.Request.URL.Path]; skip {
			event = log.Trace()
		}

		event.
			Str("request_id", reqID).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("request")
	}
}
