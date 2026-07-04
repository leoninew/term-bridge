package api

import clouddevice "termbridge-go/internal/cloud/repository/user/device"

func NewDeviceRepository(repo *clouddevice.Repository) DeviceRepository {
	return repo
}
