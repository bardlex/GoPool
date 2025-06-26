package stratum

import (
	"reflect"
	"testing"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *Message
		wantErr bool
	}{
		{
			name: "valid request",
			data: []byte(`{"id":1,"method":"mining.subscribe","params":["miner/1.0",null]}`),
			want: &Message{
				ID:     float64(1), // JSON numbers are parsed as float64
				Method: "mining.subscribe",
				Params: []interface{}{"miner/1.0", nil},
			},
			wantErr: false,
		},
		{
			name: "valid response",
			data: []byte(`{"id":1,"result":true,"error":null}`),
			want: &Message{
				ID:     float64(1),
				Result: true,
			},
			wantErr: false,
		},
		{
			name: "valid notification",
			data: []byte(`{"id":null,"method":"mining.notify","params":["job1","prev","cb1","cb2",[],"20000000","1800c29f","5a54a978",true]}`),
			want: &Message{
				ID:     nil,
				Method: "mining.notify",
				Params: []interface{}{"job1", "prev", "cb1", "cb2", []interface{}{}, "20000000", "1800c29f", "5a54a978", true},
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			data:    []byte(`{invalid json}`),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMessage(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalMessage(t *testing.T) {
	msg := &Message{
		ID:     1,
		Method: "mining.subscribe",
		Params: []interface{}{"miner/1.0", nil},
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		t.Errorf("MarshalMessage() error = %v", err)
		return
	}

	// Parse it back to verify
	parsed, err := ParseMessage(data)
	if err != nil {
		t.Errorf("Failed to parse marshaled message: %v", err)
		return
	}

	if parsed.Method != msg.Method {
		t.Errorf("Method mismatch: got %v, want %v", parsed.Method, msg.Method)
	}
}

func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name           string
		msg            *Message
		isRequest      bool
		isResponse     bool
		isNotification bool
	}{
		{
			name: "request",
			msg: &Message{
				ID:     1,
				Method: "mining.subscribe",
				Params: []interface{}{},
			},
			isRequest:      true,
			isResponse:     false,
			isNotification: false,
		},
		{
			name: "response",
			msg: &Message{
				ID:     1,
				Result: true,
			},
			isRequest:      false,
			isResponse:     true,
			isNotification: false,
		},
		{
			name: "notification",
			msg: &Message{
				ID:     nil,
				Method: "mining.notify",
				Params: []interface{}{},
			},
			isRequest:      false,
			isResponse:     false,
			isNotification: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.IsRequest(); got != tt.isRequest {
				t.Errorf("IsRequest() = %v, want %v", got, tt.isRequest)
			}
			if got := tt.msg.IsResponse(); got != tt.isResponse {
				t.Errorf("IsResponse() = %v, want %v", got, tt.isResponse)
			}
			if got := tt.msg.IsNotification(); got != tt.isNotification {
				t.Errorf("IsNotification() = %v, want %v", got, tt.isNotification)
			}
		})
	}
}

func TestParseSubscribeRequest(t *testing.T) {
	tests := []struct {
		name    string
		params  []interface{}
		want    *SubscribeRequest
		wantErr bool
	}{
		{
			name:   "valid with user agent only",
			params: []interface{}{"miner/1.0"},
			want: &SubscribeRequest{
				UserAgent: "miner/1.0",
			},
			wantErr: false,
		},
		{
			name:   "valid with user agent and session",
			params: []interface{}{"miner/1.0", "session123"},
			want: &SubscribeRequest{
				UserAgent: "miner/1.0",
				SessionID: "session123",
			},
			wantErr: false,
		},
		{
			name:    "insufficient parameters",
			params:  []interface{}{},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubscribeRequest(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubscribeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSubscribeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAuthorizeRequest(t *testing.T) {
	tests := []struct {
		name    string
		params  []interface{}
		want    *AuthorizeRequest
		wantErr bool
	}{
		{
			name:   "valid",
			params: []interface{}{"username", "password"},
			want: &AuthorizeRequest{
				Username: "username",
				Password: "password",
			},
			wantErr: false,
		},
		{
			name:    "insufficient parameters",
			params:  []interface{}{"username"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid username type",
			params:  []interface{}{123, "password"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAuthorizeRequest(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseAuthorizeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSubmitRequest(t *testing.T) {
	tests := []struct {
		name    string
		params  []interface{}
		want    *SubmitRequest
		wantErr bool
	}{
		{
			name:   "valid",
			params: []interface{}{"username", "job1", "00000001", "5a54a978", "1a2b3c4d"},
			want: &SubmitRequest{
				Username:    "username",
				JobID:       "job1",
				ExtraNonce2: "00000001",
				NTime:       "5a54a978",
				Nonce:       "1a2b3c4d",
			},
			wantErr: false,
		},
		{
			name:    "insufficient parameters",
			params:  []interface{}{"username", "job1"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid parameter type",
			params:  []interface{}{123, "job1", "00000001", "5a54a978", "1a2b3c4d"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubmitRequest(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubmitRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSubmitRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
