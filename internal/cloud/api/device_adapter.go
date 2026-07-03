package api

import clouddevice "termbridge-go/internal/cloud/device"

func NewDeviceRepository(repo *clouddevice.Repository) DeviceRepository {
	return repo
}
