-- MySQL engine used is InnoDB (supports all features)

-- Stores information about all users
CREATE TABLE users(
	user_id INT UNSIGNED NOT NULL AUTO_INCREMENT,
	username varchar(32) NOT NULL, -- 32 is max username size
	pubkey varchar(2047), -- 1023 is max argument size
	permission tinyint NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id),
	UNIQUE (username),
	UNIQUE (pubkey)
) ENGINE=InnoDB;

-- Stores messages not yet delivered to the user
CREATE TABLE message_cache(
	src_user INT UNSIGNED NOT NULL,
	dest_user INT UNSIGNED NOT NULL,
	msg varchar(2047) NOT NULL, -- 1023 is max argument size
	stamp datetime NOT NULL DEFAULT CURRENT_TIMESTAMP(),
	FOREIGN KEY (src_user) REFERENCES users(user_id) ON DELETE RESTRICT,
	FOREIGN KEY (dest_user) REFERENCES users(user_id) ON DELETE RESTRICT,
	CHECK (src_user <> dest_user) -- Cannot send messages to self
) ENGINE=InnoDB;