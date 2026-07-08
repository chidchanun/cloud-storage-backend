-- AnuCloud full database schema (corrected for MySQL 8.4).
-- Import this file into the database configured by MYSQL_DATABASE.
--
-- Corrections included:
--   * Multiple login tokens per user are supported.
--   * Duplicate active root-folder names are prevented.
--   * Long storage paths use a SHA-256 generated key for uniqueness.
--   * Plan file-size limits fit signed int64 applications.
--   * ON DUPLICATE KEY UPDATE uses row aliases instead of deprecated VALUES().
--
-- Example:
--   mysql -u root -p your_database < db/schema.sql

SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE TABLE IF NOT EXISTS `user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `first_name` varchar(255) NOT NULL,
  `last_name` varchar(255) NOT NULL,
  `email` varchar(255) NOT NULL,
  `email_verified_at` timestamp NULL DEFAULT NULL,
  `picture_path` varchar(255) DEFAULT NULL,
  `phone` varchar(255) DEFAULT NULL,
  `password_hash` varchar(255) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_email` (`email`),
  KEY `idx_user_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `plan` (
  `id` int NOT NULL AUTO_INCREMENT,
  `plan_name` varchar(100) NOT NULL,
  `plan_code` varchar(50) NOT NULL,
  `description` varchar(500) DEFAULT NULL,
  `storage_limit_bytes` bigint unsigned NOT NULL,
  `max_file_size_bytes` bigint unsigned NOT NULL,
  `max_files` int unsigned DEFAULT NULL,
  `max_share_users_per_file` int unsigned DEFAULT NULL,
  `price` decimal(10,2) NOT NULL DEFAULT 0.00,
  `billing_cycle` enum('free','monthly','yearly') NOT NULL DEFAULT 'free',
  `is_active` tinyint(1) NOT NULL DEFAULT 1,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_plan_code` (`plan_code`),
  KEY `idx_plan_active` (`is_active`),
  KEY `idx_plan_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `user_token` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `token_hash` char(64) NOT NULL,
  `expired_at` timestamp NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `revoked_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_token_user_id` (`user_id`),
  UNIQUE KEY `uq_user_token_hash` (`token_hash`),
  KEY `idx_user_token_expired_at` (`expired_at`),
  KEY `idx_user_token_revoked_at` (`revoked_at`),
  CONSTRAINT `fk_user_token_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `email_verification_token` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `token_hash` char(64) NOT NULL,
  `expires_at` timestamp NOT NULL,
  `used_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_email_verification_token_hash` (`token_hash`),
  KEY `idx_email_verification_user_id` (`user_id`),
  KEY `idx_email_verification_expires_at` (`expires_at`),
  CONSTRAINT `fk_email_verification_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `user_auth_provider` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `provider` varchar(50) NOT NULL,
  `provider_subject` varchar(255) NOT NULL,
  `provider_email` varchar(255) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_auth_provider_subject` (`provider`,`provider_subject`),
  UNIQUE KEY `uq_user_auth_provider_user` (`user_id`,`provider`),
  KEY `idx_user_auth_provider_user_id` (`user_id`),
  CONSTRAINT `fk_user_auth_provider_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `user_folder` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `parent_id` bigint DEFAULT NULL,
  `folder_name` varchar(255) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `parent_key` bigint GENERATED ALWAYS AS (
    IFNULL(`parent_id`, 0)
  ) STORED,
  `active_key` tinyint GENERATED ALWAYS AS (
    CASE WHEN `deleted_at` IS NULL THEN 1 ELSE NULL END
  ) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_folder_active_name` (`user_id`,`parent_key`,`folder_name`,`active_key`),
  KEY `idx_user_folder_user_parent` (`user_id`,`parent_id`,`deleted_at`),
  KEY `idx_user_folder_parent_id` (`parent_id`),
  CONSTRAINT `fk_user_folder_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_user_folder_parent`
    FOREIGN KEY (`parent_id`)
    REFERENCES `user_folder` (`id`)
    ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `user_file` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `folder_id` bigint DEFAULT NULL,
  `original_name` varchar(255) NOT NULL,
  `stored_name` varchar(255) NOT NULL,
  `storage_path` varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `storage_path_hash` binary(32) GENERATED ALWAYS AS (
    UNHEX(SHA2(`storage_path`, 256))
  ) STORED,
  `mime_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream',
  `size_bytes` bigint unsigned NOT NULL,
  `checksum_sha256` char(64) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_file_storage_path_hash` (`storage_path_hash`),
  KEY `idx_user_file_user_folder` (`user_id`,`folder_id`,`deleted_at`),
  KEY `idx_user_file_folder_id` (`folder_id`),
  KEY `idx_user_file_original_name` (`original_name`),
  KEY `idx_user_file_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_user_file_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_user_file_folder`
    FOREIGN KEY (`folder_id`)
    REFERENCES `user_folder` (`id`)
    ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `user_plan` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `plan_id` int NOT NULL,
  `status` enum('pending','trial','active','past_due','cancelled','expired') NOT NULL DEFAULT 'active',
  `started_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` timestamp NULL DEFAULT NULL,
  `cancelled_at` timestamp NULL DEFAULT NULL,
  `auto_renew` tinyint(1) NOT NULL DEFAULT 0,
  `payment_provider` varchar(50) DEFAULT NULL,
  `provider_subscription_id` varchar(255) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `current_plan_key` tinyint GENERATED ALWAYS AS (
    CASE
      WHEN `deleted_at` IS NULL
        AND `status` IN ('trial','active','past_due')
      THEN 1
      ELSE NULL
    END
  ) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_current_plan` (`user_id`,`current_plan_key`),
  UNIQUE KEY `uq_provider_subscription` (`payment_provider`,`provider_subscription_id`),
  KEY `idx_user_plan_user_id` (`user_id`),
  KEY `idx_user_plan_plan_id` (`plan_id`),
  KEY `idx_user_plan_status` (`status`),
  KEY `idx_user_plan_expires_at` (`expires_at`),
  KEY `idx_user_plan_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_user_plan_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_user_plan_plan`
    FOREIGN KEY (`plan_id`)
    REFERENCES `plan` (`id`)
    ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `shared_file` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `file_id` bigint NOT NULL,
  `shared_by_user_id` int NOT NULL,
  `shared_with_user_id` int NOT NULL,
  `permission` enum('viewer','editor') NOT NULL DEFAULT 'viewer',
  `expires_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `active_key` tinyint GENERATED ALWAYS AS (
    CASE WHEN `deleted_at` IS NULL THEN 1 ELSE NULL END
  ) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_shared_file_active` (`file_id`,`shared_with_user_id`,`active_key`),
  KEY `idx_shared_file_file` (`file_id`),
  KEY `idx_shared_file_recipient` (`shared_with_user_id`,`deleted_at`),
  KEY `idx_shared_file_sender` (`shared_by_user_id`,`deleted_at`),
  KEY `idx_shared_file_expires` (`expires_at`),
  CONSTRAINT `fk_shared_file_file`
    FOREIGN KEY (`file_id`)
    REFERENCES `user_file` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_shared_file_sender`
    FOREIGN KEY (`shared_by_user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_shared_file_recipient`
    FOREIGN KEY (`shared_with_user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `chk_shared_file_not_self`
    CHECK (`shared_by_user_id` <> `shared_with_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `shared_folder` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `folder_id` bigint NOT NULL,
  `shared_by_user_id` int NOT NULL,
  `shared_with_user_id` int NOT NULL,
  `permission` enum('viewer','editor') NOT NULL DEFAULT 'viewer',
  `expires_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `active_key` tinyint GENERATED ALWAYS AS (
    CASE WHEN `deleted_at` IS NULL THEN 1 ELSE NULL END
  ) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_shared_folder_active` (`folder_id`,`shared_with_user_id`,`active_key`),
  KEY `idx_shared_folder_folder` (`folder_id`),
  KEY `idx_shared_folder_recipient` (`shared_with_user_id`,`deleted_at`),
  KEY `idx_shared_folder_sender` (`shared_by_user_id`,`deleted_at`),
  KEY `idx_shared_folder_expires` (`expires_at`),
  CONSTRAINT `fk_shared_folder_folder`
    FOREIGN KEY (`folder_id`)
    REFERENCES `user_folder` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_shared_folder_sender`
    FOREIGN KEY (`shared_by_user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_shared_folder_recipient`
    FOREIGN KEY (`shared_with_user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `chk_shared_folder_not_self`
    CHECK (`shared_by_user_id` <> `shared_with_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `public_file_link` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `file_id` bigint NOT NULL,
  `user_id` int NOT NULL,
  `token_hash` char(64) NOT NULL,
  `permission` varchar(32) NOT NULL DEFAULT 'viewer',
  `expires_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_public_file_link_token_hash` (`token_hash`),
  KEY `idx_public_file_link_file_id` (`file_id`),
  KEY `idx_public_file_link_user_id` (`user_id`),
  KEY `idx_public_file_link_expires_at` (`expires_at`),
  CONSTRAINT `fk_public_file_link_file`
    FOREIGN KEY (`file_id`)
    REFERENCES `user_file` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_public_file_link_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO `plan` (
  `plan_name`,
  `plan_code`,
  `description`,
  `storage_limit_bytes`,
  `max_file_size_bytes`,
  `max_files`,
  `max_share_users_per_file`,
  `price`,
  `billing_cycle`,
  `is_active`
) VALUES
  (
    'Free',
    'free',
    'Free plan with 5 GB storage.',
    5368709120,
    5368709120,
    NULL,
    NULL,
    0.00,
    'free',
    1
  ),
  (
    'Pro Monthly',
    'pro_monthly',
    'Pro plan with 25 GB storage, billed monthly.',
    26843545600,
    26843545600,
    NULL,
    NULL,
    50.00,
    'monthly',
    1
  ),
  (
    'Pro Yearly',
    'pro_yearly',
    'Pro plan with 25 GB storage, billed yearly.',
    26843545600,
    26843545600,
    NULL,
    NULL,
    600.00,
    'yearly',
    1
  ) AS new
ON DUPLICATE KEY UPDATE
  `plan_name` = new.`plan_name`,
  `description` = new.`description`,
  `storage_limit_bytes` = new.`storage_limit_bytes`,
  `max_file_size_bytes` = new.`max_file_size_bytes`,
  `max_files` = new.`max_files`,
  `max_share_users_per_file` = new.`max_share_users_per_file`,
  `price` = new.`price`,
  `billing_cycle` = new.`billing_cycle`,
  `is_active` = new.`is_active`,
  `deleted_at` = NULL;

-- Give existing users a current Free plan when they do not have one yet.
INSERT INTO `user_plan` (`user_id`, `plan_id`, `status`, `auto_renew`)
SELECT
  u.id,
  p.id,
  'active',
  0
FROM `user` AS u
INNER JOIN `plan` AS p
  ON p.plan_code = 'free'
WHERE u.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM `user_plan` AS up
    WHERE up.user_id = u.id
      AND up.deleted_at IS NULL
      AND up.status IN ('trial','active','past_due')
  );

DROP TRIGGER IF EXISTS `trg_user_after_insert_default_free_plan`;

DELIMITER $$
CREATE TRIGGER `trg_user_after_insert_default_free_plan`
AFTER INSERT ON `user`
FOR EACH ROW
BEGIN
  INSERT INTO `user_plan` (`user_id`, `plan_id`, `status`, `auto_renew`)
  SELECT NEW.`id`, `id`, 'active', 0
  FROM `plan`
  WHERE `plan_code` = 'free'
    AND `deleted_at` IS NULL
    AND `is_active` = 1
  LIMIT 1;
END$$
DELIMITER ;
