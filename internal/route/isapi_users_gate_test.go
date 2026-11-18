package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestISAPIUserWorkflowRequiresProfileBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-admin"}
	workflow := ISAPIUserWorkflow{Transport: transport}
	request := ISAPIUserRequest{Operation: ISAPIUserUpdate, ID: "2", Name: "synthetic", Password: "SyntheticUser1!"}
	result, err := workflow.Configure(context.Background(), serverTarget(t, server), request)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorUnsupported || result.MutationAttempted ||
		result.Verified || calls.Load() != 0 {
		t.Fatalf("result=%+v err=%v requests=%d", result, err, calls.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = workflow.Configure(ctx, serverTarget(t, server), request)
	if !errors.Is(err, context.Canceled) || result.MutationAttempted || calls.Load() != 0 {
		t.Fatalf("canceled result=%+v err=%v requests=%d", result, err, calls.Load())
	}
}
