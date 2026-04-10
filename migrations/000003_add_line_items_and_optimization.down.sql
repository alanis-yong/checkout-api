BEGIN;
DROP INDEX IF EXISTS idx_orders_pagination;
DROP TABLE IF EXISTS line_items;
COMMIT;