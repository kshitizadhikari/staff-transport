-- 0002_push_tokens.up.sql
-- Device push tokens used to deliver notifications to a user's devices.

CREATE TABLE push_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token      text NOT NULL UNIQUE,
    platform   text NOT NULL DEFAULT 'unknown'
        CHECK (platform IN ('ios', 'android', 'web', 'unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX push_tokens_user_id_idx ON push_tokens (user_id);

CREATE TRIGGER push_tokens_set_updated_at
    BEFORE UPDATE ON push_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
