CREATE TABLE `plan` (
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

CREATE TABLE `user_plan` (
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
    'พื้นที่จัดเก็บ 5 GB ใช้งานฟรี ไม่จำกัดจำนวนไฟล์และจำนวนผู้ใช้ที่แชร์ต่อไฟล์',
    5368709120,
    18446744073709551615,
    NULL,
    NULL,
    0.00,
    'free',
    1
  ),
  (
    'Pro Monthly',
    'pro_monthly',
    'พื้นที่จัดเก็บ 25 GB ราคา 50 บาทต่อเดือน ไม่จำกัดจำนวนไฟล์และจำนวนผู้ใช้ที่แชร์ต่อไฟล์',
    26843545600,
    18446744073709551615,
    NULL,
    NULL,
    50.00,
    'monthly',
    1
  ),
  (
    'Pro Yearly',
    'pro_yearly',
    'พื้นที่จัดเก็บ 25 GB ราคา 600 บาทต่อปี ไม่จำกัดจำนวนไฟล์และจำนวนผู้ใช้ที่แชร์ต่อไฟล์',
    26843545600,
    18446744073709551615,
    NULL,
    NULL,
    600.00,
    'yearly',
    1
  )
ON DUPLICATE KEY UPDATE
  `plan_name` = VALUES(`plan_name`),
  `description` = VALUES(`description`),
  `storage_limit_bytes` = VALUES(`storage_limit_bytes`),
  `max_file_size_bytes` = VALUES(`max_file_size_bytes`),
  `max_files` = VALUES(`max_files`),
  `max_share_users_per_file` = VALUES(`max_share_users_per_file`),
  `price` = VALUES(`price`),
  `billing_cycle` = VALUES(`billing_cycle`),
  `is_active` = VALUES(`is_active`),
  `deleted_at` = NULL;
