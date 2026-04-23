BEGIN;
CREATE TABLE IF NOT EXISTS cart_items (
  user_id TEXT NOT NULL,
  sku TEXT NOT NULL,
  quantity INTEGER DEFAULT 1,
  PRIMARY KEY (user_id, sku)
);
COMMIT;