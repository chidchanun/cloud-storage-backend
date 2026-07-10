ALTER TABLE `user_token`
  DROP INDEX `uq_user_token_user_id`,
  ADD INDEX `idx_user_token_user_id` (`user_id`);
