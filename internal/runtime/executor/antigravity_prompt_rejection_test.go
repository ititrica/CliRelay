package executor

import (
	"net/http"
	"testing"
)

func TestIsAntigravityPromptRejectionRequiresBareGeneric429(t *testing.T) {
	bare := []byte(`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`)
	if !isAntigravityPromptRejection(http.StatusTooManyRequests, bare) {
		t.Fatal("bare generic RESOURCE_EXHAUSTED response was not classified as prompt rejection")
	}
	withDetails := []byte(`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo"}]}}`)
	if isAntigravityPromptRejection(http.StatusTooManyRequests, withDetails) {
		t.Fatal("quota details were misclassified as a prompt rejection")
	}
	if isAntigravityPromptRejection(http.StatusBadRequest, bare) {
		t.Fatal("non-429 response was classified as a prompt rejection")
	}
}

func TestNewAntigravityStatusErrMakesPromptRejectionRequestScoped(t *testing.T) {
	body := []byte(`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`)
	err := newAntigravityStatusErr(http.StatusTooManyRequests, body)
	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", err.StatusCode())
	}
	if err.RetryAfter() != nil {
		t.Fatal("prompt rejection retained a retry delay")
	}
	if string(err.UpstreamErrorBody()) != string(body) {
		t.Fatal("original upstream body was not retained for diagnostics")
	}
}
