package executor

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

const antigravityPromptRejectionResponse = `{"error":{"code":"antigravity_prompt_rejected","message":"Antigravity rejected the system prompt as a coding-agent fingerprint. This is not an account quota error; rewrite the system prompt and retry.","type":"invalid_request_error"}}`

// isAntigravityPromptRejection identifies Google's misleading bare 429 that
// is caused by a fingerprinted agent prompt. Real quota responses include
// ErrorInfo/RetryInfo (or another non-empty details field); this response does
// not. Keeping the distinction here prevents the scheduler from cooling an
// otherwise usable account and retrying the same body several times.
func isAntigravityPromptRejection(statusCode int, body []byte) bool {
	if statusCode != http.StatusTooManyRequests || len(body) == 0 {
		return false
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(root.Get("error.status").String()), "RESOURCE_EXHAUSTED") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(root.Get("error.message").String()), "Resource has been exhausted (e.g. check quota).") {
		return false
	}
	return !hasAntigravityErrorDetails(root.Get("error.details")) && !hasAntigravityErrorDetails(root.Get("details"))
}

func hasAntigravityErrorDetails(value gjson.Result) bool {
	if !value.Exists() || value.Raw == "null" || strings.TrimSpace(value.Raw) == "\"\"" {
		return false
	}
	if value.IsArray() {
		return len(value.Array()) > 0
	}
	if value.IsObject() {
		return len(value.Map()) > 0
	}
	return strings.TrimSpace(value.String()) != ""
}

func newAntigravityStatusErr(statusCode int, body []byte) statusErr {
	if isAntigravityPromptRejection(statusCode, body) {
		return statusErr{
			code:         http.StatusBadRequest,
			msg:          antigravityPromptRejectionResponse,
			upstreamBody: append([]byte(nil), body...),
		}
	}
	sErr := statusErr{code: statusCode, msg: string(body), upstreamBody: append([]byte(nil), body...)}
	if statusCode == http.StatusTooManyRequests {
		if retryAfter, parseErr := parseRetryDelay(body); parseErr == nil && retryAfter != nil {
			sErr.retryAfter = retryAfter
		}
	}
	return sErr
}
