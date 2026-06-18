package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

const (
	traceHeader   = "X-Trace-ID"
	requestHeader = "X-Request-ID"
)

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(traceHeader)
		if traceID == "" {
			traceID = c.GetHeader(requestHeader)
		}
		if traceID == "" {
			traceID = newTraceID()
		}

		c.Set(response.TraceIDKey, traceID)
		c.Header(traceHeader, traceID)
		c.Next()
	}
}

func newTraceID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	return "req_" + hex.EncodeToString(bytes[:])
}
