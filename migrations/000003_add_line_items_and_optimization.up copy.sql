-- 1. Create the orders table
CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  total INTEGER NOT NULL,
  status VARCHAR(20) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- 2. Create the line_items table
CREATE TABLE line_items (
  id SERIAL PRIMARY KEY,
  order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  item_id INTEGER NOT NULL,
  price INTEGER NOT NULL,
  quantity INTEGER NOT NULL
);
-- 3. Create the index for your cursor pagination
CREATE INDEX idx_orders_pagination ON orders (user_id, id DESC);