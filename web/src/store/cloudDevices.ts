import { defineStore } from 'pinia'
import { ref } from 'vue'
import { deleteDevice, listDevices } from '../features/cloud/api'
import type { DeviceSummary } from '../gen/proto/termbridge/cloud/v1/device'

export const useCloudDevicesStore = defineStore('cloudDevices', () => {
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')

  async function loadDevices() {
    devices.value = await listDevices()
    if (devices.value.some((device) => device.id === selectedDeviceId.value && device.online)) {
      return
    }
    const onlineDevices = devices.value.filter((device) => device.online)
    selectedDeviceId.value = onlineDevices.length === 1 ? onlineDevices[0].id : ''
  }

  function selectDeviceId(deviceId: string) {
    const device = devices.value.find((candidate) => candidate.id === deviceId)
    if (!device?.online || deviceId === selectedDeviceId.value) {
      return false
    }
    selectedDeviceId.value = deviceId
    return true
  }

  async function removeDevice(deviceId: string) {
    const device = devices.value.find((candidate) => candidate.id === deviceId)
    if (!device || device.online) {
      return false
    }
    await deleteDevice(deviceId)
    devices.value = devices.value.filter((candidate) => candidate.id !== deviceId)
    if (selectedDeviceId.value === deviceId) {
      const onlineDevices = devices.value.filter((candidate) => candidate.online)
      selectedDeviceId.value = onlineDevices.length === 1 ? onlineDevices[0].id : ''
    }
    return true
  }

  function reset() {
    devices.value = []
    selectedDeviceId.value = ''
  }

  return {
    devices,
    selectedDeviceId,
    loadDevices,
    selectDeviceId,
    removeDevice,
    reset,
  }
})
