package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"termbridge-go/internal/shared/tunnel"
)

func verifyRepositorySignedTunnelRequest(ctx context.Context, r *http.Request, repo DeviceRepository, audience string) (string, bool) {
	deviceID := strings.TrimSpace(r.Header.Get(tunnel.HeaderDeviceId))
	if deviceID == "" {
		return "", false
	}
	publicKeyText, err := repo.PublicKey(ctx, deviceID)
	if err != nil {
		return "", false
	}
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyText)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", false
	}
	verifiedDeviceID, err := tunnel.VerifySignedRequest(r, audience, ed25519.PublicKey(publicKey), time.Now())
	return verifiedDeviceID, err == nil
}
