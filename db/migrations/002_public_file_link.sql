CREATE TABLE `public_file_link` (
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
);
