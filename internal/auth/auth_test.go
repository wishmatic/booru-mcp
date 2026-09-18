package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

const testKey = "secret-key-123"

func TestBearerAuth(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{name: "valid token", authHeader: "Bearer " + testKey, wantCode: http.StatusOK},
		{name: "missing header", authHeader: "", wantCode: http.StatusUnauthorized},
		{name: "wrong token", authHeader: "Bearer wrong-key", wantCode: http.StatusUnauthorized},
		{name: "empty token", authHeader: "Bearer ", wantCode: http.StatusUnauthorized},
		{name: "basic scheme rejected", authHeader: "Basic " + testKey, wantCode: http.StatusUnauthorized},
		{name: "no space after bearer", authHeader: "Bearer" + testKey, wantCode: http.StatusUnauthorized},
		{name: "token with trailing space", authHeader: "Bearer " + testKey + " ", wantCode: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, guarded(), tt.authHeader)
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d (body: %q)", rec.Code, tt.wantCode, rec.Body.String())
			}
		})
	}
}

func TestUnauthorizedIncludesChallenge(t *testing.T) {
	rec := do(t, guarded(), "")

	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header on 401 response")
	}
}

func guarded() http.Handler {
	mw := Middleware(zap.NewNop(), testKey)

	return mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
}

func do(t *testing.T, h http.Handler, authHeader string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}
