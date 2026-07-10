CREATE TABLE IF NOT EXISTS `user_folder_star` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `folder_id` bigint NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (`id`),

  UNIQUE KEY `uq_user_folder_star` (`user_id`, `folder_id`),
  KEY `idx_user_folder_star_user_id` (`user_id`),
  KEY `idx_user_folder_star_folder_id` (`folder_id`),
  KEY `idx_user_folder_star_created_at` (`created_at`),

  CONSTRAINT `fk_user_folder_star_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE,

  CONSTRAINT `fk_user_folder_star_folder`
    FOREIGN KEY (`folder_id`)
    REFERENCES `user_folder` (`id`)
    ON DELETE CASCADE
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;
