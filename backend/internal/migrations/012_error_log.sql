-- 012_error_log.sql : pencatatan error aplikasi (error tracking).
CREATE TABLE IF NOT EXISTS error_log (
  id BIGSERIAL PRIMARY KEY,
  request_id TEXT NOT NULL DEFAULT '',
  user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  username TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  stack TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_error_log_created ON error_log(created_at DESC);
