package api

import (
	"context"
	"time"

	clouddevice "termbridge-go/internal/cloud/device"
)

type Device = clouddevice.Device

type DeviceRepository interface {
	UseBindingCode(ctx context.Context, code string) (string, bool, error)
	CreateBindingCode(ctx context.Context, userId string, expiresAt time.Time) (string, error)
	UpsertDeviceBinding(ctx context.Context, userId string, device Device) error
	UpsertUserDevice(ctx context.Context, userId string, device Device) error
	UserOwnsDevice(ctx context.Context, userId string, deviceId string) (bool, error)
	DeleteUserDevice(ctx context.Context, userId string, deviceId string) error
	PublicKey(ctx context.Context, deviceId string) (string, error)
	ListDevicesForUser(ctx context.Context, userId string) ([]Device, error)
}
