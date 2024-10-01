package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func TestTracing_UnmarshalKenginefile(t *testing.T) {
	tests := []struct {
		name     string
		spanName string
		d        *kenginefile.Dispenser
		wantErr  bool
	}{
		{
			name:     "Full config",
			spanName: "my-span",
			d: kenginefile.NewTestDispenser(`
tracing {
	span my-span
}`),
			wantErr: false,
		},
		{
			name:     "Only span name in the config",
			spanName: "my-span",
			d: kenginefile.NewTestDispenser(`
tracing {
	span my-span
}`),
			wantErr: false,
		},
		{
			name: "Empty config",
			d: kenginefile.NewTestDispenser(`
tracing {
}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ot := &Tracing{}
			if err := ot.UnmarshalKenginefile(tt.d); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalKenginefile() error = %v, wantErrType %v", err, tt.wantErr)
			}

			if ot.SpanName != tt.spanName {
				t.Errorf("UnmarshalKenginefile() SpanName = %v, want SpanName %v", ot.SpanName, tt.spanName)
			}
		})
	}
}

func TestTracing_UnmarshalKenginefile_Error(t *testing.T) {
	tests := []struct {
		name    string
		d       *kenginefile.Dispenser
		wantErr bool
	}{
		{
			name: "Unknown parameter",
			d: kenginefile.NewTestDispenser(`
		tracing {
			foo bar
		}`),
			wantErr: true,
		},
		{
			name: "Missed argument",
			d: kenginefile.NewTestDispenser(`
tracing {
	span
}`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ot := &Tracing{}
			if err := ot.UnmarshalKenginefile(tt.d); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalKenginefile() error = %v, wantErrType %v", err, tt.wantErr)
			}
		})
	}
}

func TestTracing_ServeHTTP_Propagation_Without_Initial_Headers(t *testing.T) {
	ot := &Tracing{
		SpanName: "mySpan",
	}

	req := createRequestWithContext("GET", "https://example.com/foo")
	w := httptest.NewRecorder()

	var handler kenginehttp.HandlerFunc = func(writer http.ResponseWriter, request *http.Request) error {
		traceparent := request.Header.Get("Traceparent")
		if traceparent == "" || strings.HasPrefix(traceparent, "00-00000000000000000000000000000000-0000000000000000") {
			t.Errorf("Invalid traceparent: %v", traceparent)
		}

		return nil
	}

	ctx, cancel := kengine.NewContext(kengine.Context{Context: context.Background()})
	defer cancel()

	if err := ot.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	if err := ot.ServeHTTP(w, req, handler); err != nil {
		t.Errorf("ServeHTTP error: %v", err)
	}
}

func TestTracing_ServeHTTP_Propagation_With_Initial_Headers(t *testing.T) {
	ot := &Tracing{
		SpanName: "mySpan",
	}

	req := createRequestWithContext("GET", "https://example.com/foo")
	req.Header.Set("traceparent", "00-11111111111111111111111111111111-1111111111111111-01")
	w := httptest.NewRecorder()

	var handler kenginehttp.HandlerFunc = func(writer http.ResponseWriter, request *http.Request) error {
		traceparent := request.Header.Get("Traceparent")
		if !strings.HasPrefix(traceparent, "00-11111111111111111111111111111111") {
			t.Errorf("Invalid traceparent: %v", traceparent)
		}

		return nil
	}

	ctx, cancel := kengine.NewContext(kengine.Context{Context: context.Background()})
	defer cancel()

	if err := ot.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	if err := ot.ServeHTTP(w, req, handler); err != nil {
		t.Errorf("ServeHTTP error: %v", err)
	}
}

func TestTracing_ServeHTTP_Next_Error(t *testing.T) {
	ot := &Tracing{
		SpanName: "mySpan",
	}

	req := createRequestWithContext("GET", "https://example.com/foo")
	w := httptest.NewRecorder()

	expectErr := errors.New("test error")

	var handler kenginehttp.HandlerFunc = func(writer http.ResponseWriter, request *http.Request) error {
		return expectErr
	}

	ctx, cancel := kengine.NewContext(kengine.Context{Context: context.Background()})
	defer cancel()

	if err := ot.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	if err := ot.ServeHTTP(w, req, handler); err == nil || !errors.Is(err, expectErr) {
		t.Errorf("expected error, got: %v", err)
	}
}

func createRequestWithContext(method string, url string) *http.Request {
	r, _ := http.NewRequest(method, url, nil)
	repl := kengine.NewReplacer()
	ctx := context.WithValue(r.Context(), kengine.ReplacerCtxKey, repl)
	r = r.WithContext(ctx)
	return r
}
