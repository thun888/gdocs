-- +goose Up
ALTER TABLE users ALTER COLUMN subject TYPE varchar(255);

-- +goose Down
ALTER TABLE users ALTER COLUMN subject TYPE varchar(32);
