-- SQLite client database table creation script
CREATE TABLE user (
    id_u INTEGER NOT NULL,
	username TEXT NOT NULL,
	p_key TEXT,
	PRIMARY KEY(id_u AUTOINCREMENT),
	UNIQUE(username)
);

CREATE TABLE message (
    id_source_u INTEGER NOT NULL,
	id_destination_u INTEGER NOT NULL,
	unix_stamp TEXT NOT NULL,
	msg TEXT,
	PRIMARY KEY(id_source_u, unix_stamp),
	FOREIGN KEY(id_source_u) REFERENCES user(id_u) ON DELETE RESTRICT,
	FOREIGN KEY(id_destination_u) REFERENCES user(id_u) ON DELETE RESTRICT
);
