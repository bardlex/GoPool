package stratum

import (
	"encoding/json"
	"fmt"
)

// Message represents a Stratum JSON-RPC message
type Message struct {
	ID     any    `json:"id"`
	Method string `json:"method,omitempty"`
	Params []any  `json:"params,omitempty"`
	Result any    `json:"result,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// Error represents a Stratum error response
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Common Stratum error codes
const (
	ErrorOther          = 20
	ErrorJobNotFound    = 21
	ErrorDuplicateShare = 22
	ErrorLowDifficulty  = 23
	ErrorUnauthorized   = 24
	ErrorNotSubscribed  = 25
	ErrorInvalidRequest = -32600
	ErrorMethodNotFound = -32601
	ErrorInvalidParams  = -32602
	ErrorParseError     = -32700
)

// SubscribeRequest represents a mining.subscribe request
type SubscribeRequest struct {
	UserAgent string
	SessionID string
}

// SubscribeResponse represents a mining.subscribe response
type SubscribeResponse struct {
	Subscriptions   [][]string `json:"subscriptions"`
	ExtraNonce1     string     `json:"extranonce1"`
	ExtraNonce2Size int        `json:"extranonce2_size"`
}

// AuthorizeRequest represents a mining.authorize request
type AuthorizeRequest struct {
	Username string
	Password string
}

// SubmitRequest represents a mining.submit request
type SubmitRequest struct {
	Username    string
	JobID       string
	ExtraNonce2 string
	NTime       string
	Nonce       string
}

// NotifyParams represents mining.notify parameters
type NotifyParams struct {
	JobID        string   `json:"job_id"`
	PrevHash     string   `json:"prevhash"`
	Coinb1       string   `json:"coinb1"`
	Coinb2       string   `json:"coinb2"`
	MerkleBranch []string `json:"merkle_branch"`
	Version      string   `json:"version"`
	NBits        string   `json:"nbits"`
	NTime        string   `json:"ntime"`
	CleanJobs    bool     `json:"clean_jobs"`
}

// SetDifficultyParams represents mining.set_difficulty parameters
type SetDifficultyParams struct {
	Difficulty float64 `json:"difficulty"`
}

// ParseMessage parses a JSON-RPC message from bytes
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &msg, nil
}

// MarshalMessage marshals a message to JSON bytes
func MarshalMessage(msg *Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// NewRequest creates a new request message
func NewRequest(id any, method string, params []any) *Message {
	return &Message{
		ID:     id,
		Method: method,
		Params: params,
	}
}

// NewResponse creates a new response message
func NewResponse(id any, result any) *Message {
	return &Message{
		ID:     id,
		Result: result,
	}
}

// NewErrorResponse creates a new error response message
func NewErrorResponse(id any, code int, message string) *Message {
	return &Message{
		ID: id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}

// NewNotification creates a new notification message
func NewNotification(method string, params []any) *Message {
	return &Message{
		ID:     nil,
		Method: method,
		Params: params,
	}
}

// IsRequest returns true if the message is a request
func (m *Message) IsRequest() bool {
	return m.Method != "" && m.ID != nil
}

// IsResponse returns true if the message is a response
func (m *Message) IsResponse() bool {
	return m.Method == "" && m.ID != nil && (m.Result != nil || m.Error != nil)
}

// IsNotification returns true if the message is a notification
func (m *Message) IsNotification() bool {
	return m.Method != "" && m.ID == nil
}

// ParseSubscribeRequest parses mining.subscribe parameters
func ParseSubscribeRequest(params []any) (*SubscribeRequest, error) {
	if len(params) < 1 {
		return nil, fmt.Errorf("insufficient parameters")
	}

	req := &SubscribeRequest{}

	if userAgent, ok := params[0].(string); ok {
		req.UserAgent = userAgent
	}

	if len(params) > 1 {
		if sessionID, ok := params[1].(string); ok {
			req.SessionID = sessionID
		}
	}

	return req, nil
}

// ParseAuthorizeRequest parses mining.authorize parameters
func ParseAuthorizeRequest(params []any) (*AuthorizeRequest, error) {
	if len(params) < 2 {
		return nil, fmt.Errorf("insufficient parameters")
	}

	username, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("username must be string")
	}

	password, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("password must be string")
	}

	return &AuthorizeRequest{
		Username: username,
		Password: password,
	}, nil
}

// ParseSubmitRequest parses mining.submit parameters
func ParseSubmitRequest(params []any) (*SubmitRequest, error) {
	if len(params) < 5 {
		return nil, fmt.Errorf("insufficient parameters")
	}

	username, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("username must be string")
	}

	jobID, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("job_id must be string")
	}

	extraNonce2, ok := params[2].(string)
	if !ok {
		return nil, fmt.Errorf("extranonce2 must be string")
	}

	nTime, ok := params[3].(string)
	if !ok {
		return nil, fmt.Errorf("ntime must be string")
	}

	nonce, ok := params[4].(string)
	if !ok {
		return nil, fmt.Errorf("nonce must be string")
	}

	return &SubmitRequest{
		Username:    username,
		JobID:       jobID,
		ExtraNonce2: extraNonce2,
		NTime:       nTime,
		Nonce:       nonce,
	}, nil
}
