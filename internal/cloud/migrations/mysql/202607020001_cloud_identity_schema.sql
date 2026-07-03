-- +goose Up
CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(32) PRIMARY KEY,
  email_normalized VARCHAR(320) NOT NULL UNIQUE,
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  email_verified_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  last_login_at DATETIME(6) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_identities (
  id VARCHAR(32) PRIMARY KEY,
  user_id VARCHAR(32) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  provider_subject VARCHAR(320) NOT NULL,
  password_hash VARCHAR(255) NULL,
  oauth_email VARCHAR(320) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_user_identities_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE KEY uq_user_identities_provider_subject (provider, provider_subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS auth_codes (
  id VARCHAR(32) PRIMARY KEY,
  user_id VARCHAR(32) NULL,
  email_normalized VARCHAR(320) NOT NULL,
  purpose VARCHAR(64) NOT NULL,
  code_hash VARCHAR(255) NOT NULL,
  used_at DATETIME(6) NULL,
  expires_at DATETIME(6) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  invalidated_at DATETIME(6) NULL,
  CONSTRAINT fk_auth_codes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX idx_auth_codes_lookup (email_normalized, purpose, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS email_delivery_logs (
  id VARCHAR(32) PRIMARY KEY,
  email_normalized VARCHAR(320) NOT NULL,
  purpose VARCHAR(64) NOT NULL,
  provider_message_id VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  response_body TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS oauth_states (
  state VARCHAR(255) PRIMARY KEY,
  expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS devices (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  public_key TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_devices (
  user_id VARCHAR(32) NOT NULL,
  device_id VARCHAR(128) NOT NULL,
  role VARCHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (user_id, device_id),
  CONSTRAINT fk_user_devices_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_devices_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS device_binding_codes (
  code_hash VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(32) NOT NULL,
  used_at DATETIME(6) NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_device_binding_codes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS device_binding_codes;
DROP TABLE IF EXISTS user_devices;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS oauth_states;
DROP TABLE IF EXISTS email_delivery_logs;
DROP TABLE IF EXISTS auth_codes;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
