-- =============================================================
-- checkout-api — consolidated schema + seed data
-- Mirrors the final state of all migrations (001 → 010)
-- =============================================================

BEGIN;

-- ── items ─────────────────────────────────────────────────────
CREATE TABLE items (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    price       BIGINT       NOT NULL,
    stock       BIGINT       NOT NULL DEFAULT 0,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_items_price ON items (price);
CREATE INDEX idx_items_name  ON items (name);

-- ── orders ────────────────────────────────────────────────────
CREATE TABLE orders (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    total      BIGINT      NOT NULL,
    status     VARCHAR(20) NOT NULL,
    created_at TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id    ON orders (user_id);
CREATE INDEX idx_orders_status     ON orders (status);
CREATE INDEX idx_orders_created_at ON orders (created_at DESC);

-- ── line_items ────────────────────────────────────────────────
CREATE TABLE line_items (
    id       BIGSERIAL PRIMARY KEY,
    order_id BIGINT    NOT NULL REFERENCES orders (id),
    item_id  BIGINT    NOT NULL REFERENCES items  (id),
    price    BIGINT    NOT NULL CHECK (price >= 0),
    quantity BIGINT    NOT NULL CHECK (quantity > 0)
);

CREATE INDEX idx_line_items_order_id ON line_items (order_id);
CREATE INDEX idx_line_items_item_id  ON line_items (item_id);

-- ── carts ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS carts (
    id       BIGSERIAL,
    user_id  BIGINT NOT NULL,
    item_id  BIGINT NOT NULL REFERENCES items (id),
    quantity BIGINT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    UNIQUE (user_id, item_id)
);

CREATE INDEX idx_user_id_item_id ON carts (user_id, item_id);

-- ── users ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL    PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    hash       BYTEA        NOT NULL,
    created_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    UNIQUE (email)
);

CREATE INDEX idx_email_hash ON users (email, hash);

-- ── seed: items ───────────────────────────────────────────────
INSERT INTO items (name, description, price, stock) VALUES
    ('Xsolla T-Shirt',      'Classic cotton tee with Xsolla logo. Unisex fit.',          2500,  120),
    ('Developer Hoodie',    'Heavyweight pullover hoodie. Perfect for late-night PRs.',   6000,   45),
    ('Sticker Pack',        '10-pack of Xsolla and open-source themed stickers.',          500,  300),
    ('Mechanical Keyboard', 'Tenkeyless, Cherry MX Brown switches. USB-C.',             18000,   18),
    ('Laptop Stand',        'Aluminium adjustable stand. Folds flat for travel.',         4500,   60),
    ('USB-C Hub',           '7-in-1 hub: HDMI 4K, 3× USB-A, SD, microSD, PD.',          3200,   75),
    ('Notebook (A5)',       'Dot-grid, 200 pages, lay-flat binding.',                     1200,  200),
    ('Cable Organiser',     'Leather magnetic cable ties, pack of 6.',                     800,  150);

COMMIT;
