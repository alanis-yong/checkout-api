CREATE TABLE IF NOT EXISTS user_inventory (
  id SERIAL PRIMARY KEY,
  user_id TEXT NOT NULL,
  sku TEXT NOT NULL,
  quantity INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  -- This constraint is CRITICAL for the "ON CONFLICT" code to work
  UNIQUE(user_id, sku)
);