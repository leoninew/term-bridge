-- +goose Up
CREATE TABLE devices (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_devices (
  user_id VARCHAR(32) NOT NULL,
  device_id VARCHAR(128) NOT NULL,
  role VARCHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (user_id, device_id),
  CONSTRAINT fk_user_devices_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_devices_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE device_keys (
  id VARCHAR(32) PRIMARY KEY,
  device_id VARCHAR(128) NOT NULL,
  public_key TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_device_keys_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_device_keys_device (device_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE device_binding_codes (
  code_hash VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(32) NOT NULL,
  used_at DATETIME(6) NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_device_binding_codes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS device_binding_codes;
DROP TABLE IF EXISTS device_keys;
DROP TABLE IF EXISTS user_devices;
DROP TABLE IF EXISTS devices;
