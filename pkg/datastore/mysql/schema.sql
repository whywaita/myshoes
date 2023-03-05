CREATE TABLE `targets` (
    `uuid` VARCHAR(36) NOT NULL PRIMARY KEY,
    `scope` VARCHAR(255) NOT NULL,
    `ghe_domain` VARCHAR(255),
    `github_token` VARCHAR(255) NOT NULL,
    `token_expired_at` TIMESTAMP NOT NULL,
    `resource_type` ENUM('nano', 'micro', 'small', 'medium', 'large', 'xlarge', '2xlarge', '3xlarge', '4xlarge') NOT NULL,
    `provider_url` VARCHAR(255),
    `status` VARCHAR(255) NOT NULL DEFAULT 'active',
    `status_description` VARCHAR(255),
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp,
    `updated_at` TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp,
    UNIQUE KEY `ghe_domain_scope` (`ghe_domain`, `scope`)
);

CREATE TABLE `runners` (
    `uuid` VARCHAR(36) NOT NULL PRIMARY KEY,
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp
);

CREATE TABLE `runner_detail` (
    `runner_id` VARCHAR(36) NOT NULL,
    `shoes_type` VARCHAR(255) NOT NULL,
    `ip_address` VARCHAR(255) NOT NULL,
    `target_id` VARCHAR(36) NOT NULL,
    `cloud_id` TEXT NOT NULL,
    `resource_type` ENUM('nano', 'micro', 'small', 'medium', 'large', 'xlarge', '2xlarge', '3xlarge', '4xlarge') NOT NULL,
    `runner_user` VARCHAR(255),
    `provider_url` VARCHAR(255),
    `repository_url` VARCHAR(255) NOT NULL,
    `request_webhook` TEXT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp,
    `updated_at` TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp,
    KEY `fk_runner_target_id` (`target_id`),
    CONSTRAINT `runners_ibfk_1` FOREIGN KEY fk_runner_target_id(`target_id`) REFERENCES targets(`uuid`) ON DELETE RESTRICT,
    KEY `fk_runner_detail_id` (`runner_id`),
    CONSTRAINT `runners_ibfk_2` FOREIGN KEY fk_runner_detail_id(`runner_id`) REFERENCES runners(`uuid`) ON DELETE RESTRICT
);

CREATE TABLE `runners_running` (
    `runner_id` VARCHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp,
    KEY `fk_runner_deleted_id` (`runner_id`),
    CONSTRAINT `runners_running_ibfk_1` FOREIGN KEY fk_runner_deleted_id(`runner_id`) REFERENCES runners(`uuid`) ON DELETE CASCADE
);

CREATE TABLE `runners_deleted` (
    `runner_id` VARCHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp,
    `reason` VARCHAR(255) NOT NULL,
    KEY `fk_runner_deleted_id` (`runner_id`),
    CONSTRAINT `runners_deleted_ibfk_1` FOREIGN KEY fk_runner_deleted_id(`runner_id`) REFERENCES runners(`uuid`) ON DELETE CASCADE
);

CREATE TABLE `jobs` (
    `uuid` VARCHAR(36) NOT NULL PRIMARY KEY,
    `ghe_domain` VARCHAR(255),
    `repository` VARCHAR(255) NOT NULL,
    `check_event` TEXT NOT NULL,
    `target_id` VARCHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT current_timestamp,
    `updated_at` TIMESTAMP NOT NULL DEFAULT current_timestamp ON UPDATE current_timestamp,
    KEY `fk_job_target_id` (`target_id`),
    CONSTRAINT `jobs_ibfk_1` FOREIGN KEY fk_job_target_id(`target_id`) REFERENCES targets(`uuid`) ON DELETE RESTRICT
);
