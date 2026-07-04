package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderDeviceId        = "X-TermBridge-Device-ID"
	HeaderDeviceTimestamp = "X-TermBridge-Device-Timestamp"
	HeaderDeviceNonce     = "X-TermBridge-Device-Nonce"
	HeaderDeviceSignature = "X-TermBridge-Device-Signature"
)

const MaxSignatureSkew = 5 * time.Minute

type SignatureInput struct {
	Method    string
	Path      string
	DeviceId  string
	Timestamp int64
	Nonce     string
	Audience  string
}

func SignedTunnelHeader(method string, path string, audience string, deviceId string, privateKey ed25519.PrivateKey, now time.Time, nonce string) (http.Header, error) {
	if strings.TrimSpace(nonce) == "" {
		generated, err := randomNonce()
		if err != nil {
			return nil, err
		}
		nonce = generated
	}
	input := SignatureInput{Method: method, Path: path, DeviceId: deviceId, Timestamp: now.UTC().Unix(), Nonce: nonce, Audience: audience}
	signature := ed25519.Sign(privateKey, []byte(canonicalSignatureMessage(input)))
	header := http.Header{}
	header.Set(HeaderDeviceId, deviceId)
	header.Set(HeaderDeviceTimestamp, strconv.FormatInt(input.Timestamp, 10))
	header.Set(HeaderDeviceNonce, nonce)
	header.Set(HeaderDeviceSignature, base64.StdEncoding.EncodeToString(signature))
	return header, nil
}

func VerifySignedRequest(r *http.Request, audience string, publicKey ed25519.PublicKey, now time.Time) (string, error) {
	deviceId := strings.TrimSpace(r.Header.Get(HeaderDeviceId))
	timestampText := strings.TrimSpace(r.Header.Get(HeaderDeviceTimestamp))
	nonce := strings.TrimSpace(r.Header.Get(HeaderDeviceNonce))
	signatureText := strings.TrimSpace(r.Header.Get(HeaderDeviceSignature))
	if deviceId == "" || timestampText == "" || nonce == "" || signatureText == "" {
		return "", fmt.Errorf("missing device signature headers")
	}
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid device signature timestamp")
	}
	signedAt := time.Unix(timestamp, 0).UTC()
	if signedAt.Before(now.UTC().Add(-MaxSignatureSkew)) || signedAt.After(now.UTC().Add(MaxSignatureSkew)) {
		return "", fmt.Errorf("device signature timestamp outside allowed skew")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return "", fmt.Errorf("invalid device signature encoding")
	}
	input := SignatureInput{Method: r.Method, Path: r.URL.Path, DeviceId: deviceId, Timestamp: timestamp, Nonce: nonce, Audience: audience}
	if !ed25519.Verify(publicKey, []byte(canonicalSignatureMessage(input)), signature) {
		return "", fmt.Errorf("invalid device signature")
	}
	return deviceId, nil
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func canonicalSignatureMessage(input SignatureInput) string {
	return strings.ToUpper(strings.TrimSpace(input.Method)) + "\n" +
		strings.TrimSpace(input.Path) + "\n" +
		strings.TrimSpace(input.DeviceId) + "\n" +
		strconv.FormatInt(input.Timestamp, 10) + "\n" +
		strings.TrimSpace(input.Nonce) + "\n" +
		strings.TrimRight(strings.TrimSpace(input.Audience), "/")
}
