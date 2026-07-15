-- +goose Up
ALTER TABLE users ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT 'email' AFTER id;
UPDATE users AS users
JOIN user_identities AS identities ON identities.user_id = users.id
SET users.provider = identities.provider;
ALTER TABLE users DROP INDEX email_normalized;
ALTER TABLE users ADD UNIQUE KEY uq_users_provider_email (provider, email_normalized);
ALTER TABLE oauth_states ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT 'google' AFTER state;

-- +goose Down
ALTER TABLE oauth_states DROP COLUMN provider;
ALTER TABLE users DROP INDEX uq_users_provider_email;
ALTER TABLE users ADD UNIQUE KEY email_normalized (email_normalized);
ALTER TABLE users DROP COLUMN provider;
