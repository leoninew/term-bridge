package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func verifyRepositorySignedTunnelRequest(ctx context.Context, r *http.Request, repo DeviceRepository, audience string) (string, bool) {
	deviceId := strings.TrimSpace(r.Header.Get(tunnel.HeaderDeviceId))
	if deviceId == "" {
		return "", false
	}
	publicKeyText, err := repo.PublicKey(ctx, deviceId)
	if err != nil {
		return "", false
	}
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyText)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", false
	}
	derivedDeviceId, err := security.DeviceIdForEd25519PublicKey(ed25519.PublicKey(publicKey))
	if err != nil || derivedDeviceId != deviceId {
		return "", false
	}
	verifiedDeviceId, err := tunnel.VerifySignedRequest(r, audience, ed25519.PublicKey(publicKey), time.Now())
	return verifiedDeviceId, err == nil
}
