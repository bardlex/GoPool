// Package stratum implements the Stratum V1 mining protocol for GOMP mining pool.
// It provides session management, message parsing, and connection handling.
package stratum

import (
	"strings"
	"sync"
)

// Object pools for hot path optimizations
var (
	// messagePool reuses Message structs to reduce allocations
	messagePool = sync.Pool{
		New: func() any {
			return &Message{}
		},
	}

	// bufferPool reuses byte buffers for network I/O
	bufferPool = sync.Pool{
		New: func() any {
			return make([]byte, 4096) // 4KB buffer
		},
	}

	// stringBuilderPool reuses string builders for JSON construction
	stringBuilderPool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}
)

// GetMessage gets a Message from the pool
func GetMessage() *Message {
	msg := messagePool.Get().(*Message)
	// Reset the message
	msg.ID = nil
	msg.Method = ""
	msg.Params = nil
	msg.Result = nil
	msg.Error = nil
	return msg
}

// PutMessage returns a Message to the pool
func PutMessage(msg *Message) {
	if msg != nil {
		messagePool.Put(msg)
	}
}

// GetBuffer gets a byte buffer from the pool
func GetBuffer() []byte {
	return bufferPool.Get().([]byte)
}

// PutBuffer returns a byte buffer to the pool
func PutBuffer(buf []byte) {
	if buf != nil {
		bufferPool.Put(buf)
	}
}

// GetStringBuilder gets a string builder from the pool
func GetStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// PutStringBuilder returns a string builder to the pool
func PutStringBuilder(sb *strings.Builder) {
	if sb != nil {
		stringBuilderPool.Put(sb)
	}
}
