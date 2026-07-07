package api

import clouddevice "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/device"

func NewDeviceRepository(repo *clouddevice.Repository) DeviceRepository {
	return repo
}
