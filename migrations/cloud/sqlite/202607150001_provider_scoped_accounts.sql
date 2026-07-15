-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys=OFF;

ALTER TABLE user_identities RENAME TO user_identities_legacy;
ALTER TABLE auth_codes RENAME TO auth_codes_legacy;
DROP INDEX idx_auth_codes_lookup;
ALTER TABLE user_devices RENAME TO user_devices_legacy;
ALTER TABLE users RENAME TO users_legacy;

CREATE TABLE users (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  email_normalized TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  email_verified_at TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_login_at TEXT NULL,
  UNIQUE (provider, email_normalized)
);

INSERT INTO users (id,provider,email_normalized,display_name,status,email_verified_at,created_at,updated_at,last_login_at)
SELECT legacy.id, identities.provider, legacy.email_normalized, legacy.display_name, legacy.status, legacy.email_verified_at, legacy.created_at, legacy.updated_at, legacy.last_login_at
FROM users_legacy AS legacy
JOIN user_identities_legacy AS identities ON identities.user_id = legacy.id;

CREATE TABLE user_identities (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  provider_subject TEXT NOT NULL,
  password_hash TEXT NULL,
  oauth_email TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE (provider, provider_subject)
);

INSERT INTO user_identities (id,user_id,provider,provider_subject,password_hash,oauth_email,created_at,updated_at)
SELECT id,user_id,provider,provider_subject,password_hash,oauth_email,created_at,updated_at FROM user_identities_legacy;

CREATE TABLE auth_codes (
  id TEXT PRIMARY KEY,
  user_id TEXT NULL,
  email_normalized TEXT NOT NULL,
  purpose TEXT NOT NULL,
  code_hash TEXT NOT NULL,
  used_at TEXT NULL,
  expires_at TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  invalidated_at TEXT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO auth_codes (id,user_id,email_normalized,purpose,code_hash,used_at,expires_at,attempts,created_at,invalidated_at)
SELECT id,user_id,email_normalized,purpose,code_hash,used_at,expires_at,attempts,created_at,invalidated_at FROM auth_codes_legacy;
CREATE INDEX idx_auth_codes_lookup ON auth_codes(email_normalized, purpose, created_at);

CREATE TABLE user_devices (
  user_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  role TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, device_id),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

INSERT INTO user_devices (user_id,device_id,role,created_at,updated_at)
SELECT user_id,device_id,role,created_at,updated_at FROM user_devices_legacy;

DROP TABLE user_devices_legacy;
DROP TABLE auth_codes_legacy;
DROP TABLE user_identities_legacy;
DROP TABLE users_legacy;

ALTER TABLE oauth_states ADD COLUMN provider TEXT NOT NULL DEFAULT 'google';
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
ALTER TABLE users RENAME TO users_provider_scoped;
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email_normalized TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  email_verified_at TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_login_at TEXT NULL
);
INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at,last_login_at)
SELECT id,email_normalized,display_name,status,email_verified_at,created_at,updated_at,last_login_at FROM users_provider_scoped;
DROP TABLE users_provider_scoped;
PRAGMA foreign_keys=ON;
