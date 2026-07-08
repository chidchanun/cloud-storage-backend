ALTER TABLE `user`
  ADD COLUMN email_verified_at TIMESTAMP NULL DEFAULT NULL AFTER email;

CREATE TABLE `email_verification_token` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` INT NOT NULL,
  `token_hash` CHAR(64) NOT NULL,
  `expires_at` TIMESTAMP NOT NULL,
  `used_at` TIMESTAMP NULL DEFAULT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_email_verification_token_hash` (`token_hash`),
  KEY `idx_email_verification_user_id` (`user_id`),
  KEY `idx_email_verification_expires_at` (`expires_at`),
  CONSTRAINT `fk_email_verification_user`
    FOREIGN KEY (`user_id`)
    REFERENCES `user` (`id`)
    ON DELETE CASCADE
);
