CREATE DATABASE IF NOT EXISTS myshoes;

CREATE TABLE IF NOT EXISTS targets (
    uuid VARCHAR(36) NOT NULL PRIMARY KEY,
    scope VARCHAR(255) NOT NULL,
    ghe_domain VARCHAR(255),
    github_personal_token VARCHAR(255) NOT NULL,
    UNIQUE (ghe_domain, scope),
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp
);

CREATE TABLE IF NOT EXISTS runners (
    uuid VARCHAR(36) NOT NULL PRIMARY KEY,
    shoes_type VARCHAR(255) NOT NULL,
    ip_address VARCHAR(255) NOT NULL,
    target_id VARCHAR(36) NOT NULL,
    deleted bool DEFAULT false,
    FOREIGN KEY fk_target_id(target_id) REFERENCES targets(uuid) ON DELETE RESTRICT,
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp,
    deleted_at TIMESTAMP
)