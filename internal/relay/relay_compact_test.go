package relay

import "testing"

func TestShouldMarkCompactUnsupported(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "model_not_found_503",
			err:  assertErr("upstream error: 503: {\"error\":{\"code\":\"model_not_found\",\"message\":\"No available channel for model gpt-5.4-openai-compact\"}}"),
			want: true,
		},
		{
			name: "unsupported_path_404",
			err:  assertErr("upstream error: 404: endpoint /v1/responses/compact not found"),
			want: true,
		},
		{
			name: "disabled_key_401_not_marked",
			err:  assertErr("upstream error: 401: {\"code\":\"API_KEY_DISABLED\"}"),
			want: false,
		},
		{
			name: "context_canceled_not_marked",
			err:  assertErr("failed to send request: Post \"https://a/v1/responses/compact\": context canceled"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldMarkCompactUnsupported(tt.err); got != tt.want {
				t.Fatalf("shouldMarkCompactUnsupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

func assertErr(v string) error { return testErr(v) }
