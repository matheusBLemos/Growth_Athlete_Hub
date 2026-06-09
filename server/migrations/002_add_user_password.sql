-- Adiciona a coluna de hash de senha aos usuários.
-- Hashing é feito com Argon2id + pepper na camada de infra.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
