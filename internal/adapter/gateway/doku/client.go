package doku

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// dokuTimeFormat adalah format Request-Timestamp DOKU: ISO-8601 UTC presisi detik.
const dokuTimeFormat = "2006-01-02T15:04:05Z"

// signedResponse adalah hasil mentah pemanggilan Direct API DOKU.
type signedResponse struct {
	statusCode int
	body       []byte
	requestID  string // Request-Id yang KITA kirim (dipakai sbg GatewayRequestID)
}

// postSigned membangun request POST bertanda tangan ke Direct API DOKU dan
// mengeksekusinya. Skema signature: HMAC-SHA256 berbasis component string
// (doku-integration-spec §1) — sama dengan verifier webhook.
//
// target adalah Request-Target (path, mis. "/checkout/v1/payment") yang ikut
// ditandatangani; harus persis sama dengan path pada URL.
func (a *Adapter) postSigned(ctx context.Context, target string, body []byte) (signedResponse, error) {
	ts := a.now().UTC().Format(dokuTimeFormat)
	reqID := a.newRequestID()
	digest := Digest(body)
	signature := Sign(a.cfg.SecretKey, ComponentString(a.cfg.ClientID, reqID, ts, target, digest))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.BaseURL+target, bytes.NewReader(body))
	if err != nil {
		return signedResponse{}, fmt.Errorf("doku: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Client-Id", a.cfg.ClientID)
	httpReq.Header.Set("Request-Id", reqID)
	httpReq.Header.Set("Request-Timestamp", ts)
	httpReq.Header.Set("Digest", digest)
	httpReq.Header.Set("Signature", signature)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return signedResponse{}, fmt.Errorf("doku: kirim request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return signedResponse{}, fmt.Errorf("doku: baca response: %w", err)
	}
	return signedResponse{statusCode: resp.StatusCode, body: respBody, requestID: reqID}, nil
}

// Error adalah error terstruktur dari DOKU Direct API (HTTP non-2xx).
type Error struct {
	HTTPStatus int
	Code       string
	Message    string
	Raw        []byte
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("doku: %s (http %d, code %s)", e.Message, e.HTTPStatus, e.Code)
	}
	return fmt.Sprintf("doku: %s (http %d)", e.Message, e.HTTPStatus)
}

// parseError mengekstrak pesan error dari body DOKU. Bentuk error DOKU bervariasi
// ({"error":{"code","message":[...]}} atau {"message":[...]}), jadi parser dibuat toleran.
func parseError(statusCode int, body []byte) *Error {
	var env struct {
		Error *struct {
			Code    string          `json:"code"`
			Message json.RawMessage `json:"message"`
		} `json:"error"`
		Message json.RawMessage `json:"message"`
	}
	e := &Error{HTTPStatus: statusCode, Raw: body}
	if err := json.Unmarshal(body, &env); err != nil {
		e.Message = "response error tidak dapat di-parse"
		return e
	}
	if env.Error != nil {
		e.Code = env.Error.Code
		e.Message = firstMessage(env.Error.Message)
		return e
	}
	e.Message = firstMessage(env.Message)
	if e.Message == "" {
		e.Message = "gateway mengembalikan error"
	}
	return e
}

// firstMessage menormalkan field message DOKU yang bisa string ATAU array string.
func firstMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) > 0 {
			return arr[0]
		}
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}
