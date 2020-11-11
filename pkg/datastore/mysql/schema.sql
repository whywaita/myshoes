CREATE DATABASE IF NOT EXISTS myshoes;

CREATE TABLE IF NOT EXISTS targets (
    uuid VARCHAR(255) NOT NULL PRIMARY KEY,
    scope VARCHAR(255) NOT NULL UNIQUE,
    github_personal_token VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp
);