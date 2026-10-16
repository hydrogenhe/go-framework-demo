-- Apply once to new installations. Existing installations were migrated by cmd/reset-users.
ALTER TABLE `user` ADD UNIQUE INDEX `uk_user_email` (`email`);
