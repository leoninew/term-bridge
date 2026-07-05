package api

import (
	"context"

	clouddevice "termbridge-go/internal/cloud/repository/user/device"
)

type Device = clouddevice.Device

type DeviceRepository interface {
	UpsertDeviceBinding(ctx context.Context, userId string, device Device) error
	UpsertUserDevice(ctx context.Context, userId string, device Device) error
	UserOwnsDevice(ctx context.Context, userId string, deviceId string) (bool, error)
	DeleteUserDevice(ctx context.Context, userId string, deviceId string) error
	PublicKey(ctx context.Context, deviceId string) (string, error)
	ListDevicesForUser(ctx context.Context, userId string) ([]Device, error)
}
