package terminalproto

import "testing"

func TestBrowserDisconnectError(t *testing.T) {
	cases := []struct {
		reason  string
		code    string
		message string
	}{
		{reason: DisconnectReasonDeviceDisconnected, code: ErrorCodeDeviceDisconnected, message: ErrorMessageDeviceDisconnected},
		{reason: DisconnectReasonDeviceReconnected, code: ErrorCodeDeviceDisconnected, message: ErrorMessageDeviceReconnected},
		{reason: DisconnectReasonDeviceDeleted, code: ErrorCodeDeviceDisconnected, message: ErrorMessageDeviceDeleted},
		{reason: "unexpected free text", code: ErrorCodeDeviceDisconnected, message: ErrorMessageDeviceDisconnected},
	}
	for _, tc := range cases {
		code, message := BrowserDisconnectError(tc.reason)
		if code != tc.code || message != tc.message {
			t.Fatalf("BrowserDisconnectError(%q) = (%q, %q), want (%q, %q)", tc.reason, code, message, tc.code, tc.message)
		}
	}
}
