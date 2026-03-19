ALTER TABLE users
    ADD COLUMN email TEXT,
    ADD COLUMN password_hash TEXT;

CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_users_local_username ON users(username) WHERE provider = 'local';
