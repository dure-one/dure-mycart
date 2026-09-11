CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE setting (
	id     TEXT PRIMARY KEY NOT NULL,
	key    TEXT UNIQUE NOT NULL,
	value  TEXT DEFAULT NULL
);
CREATE INDEX idx_setting_key ON setting (key);
CREATE TABLE session (
	key      TEXT UNIQUE NOT NULL,
	value    TEXT DEFAULT NULL,
	expires  INTEGER
);
CREATE INDEX idx_session_key ON session (key);
CREATE TABLE subdomain (
	id    TEXT PRIMARY KEY NOT NULL,
	name  TEXT UNIQUE NOT NULL,
	desc  TEXT DEFAULT NULL
);
CREATE INDEX idx_subdomain_name ON subdomain (name);
CREATE TABLE page (
	id 				TEXT PRIMARY KEY NOT NULL,
	name 			TEXT NOT NULL,
	slug 			TEXT UNIQUE NOT NULL,
	content 	TEXT DEFAULT NULL,
	position  TEXT NOT NULL CHECK (position == 'header' OR position == 'footer'),
	active    BOOLEAN DEFAULT FALSE NOT NULL,
	created 	TIMESTAMP DEFAULT (datetime('now')),
	updated 	TIMESTAMP
, "seo" JSON DEFAULT '{}' NOT NULL);
CREATE INDEX idx_page_name ON page (name);
CREATE INDEX idx_page_slug ON page (slug);
CREATE INDEX idx_page_group ON page (position);
CREATE TABLE product (
	id         TEXT PRIMARY KEY NOT NULL,
	name       TEXT NOT NULL,
	desc       TEXT NOT NULL,
	slug       TEXT UNIQUE NOT NULL,
	amount     NUMERC NOT NULL,
	metadata   JSON DEFAULT '[]' NOT NULL,
	attribute  JSON DEFAULT '[]' NOT NULL,
	digital    TEXT CHECK (digital == 'file' OR digital == 'data' OR digital == 'api'),
	active     BOOLEAN DEFAULT TRUE NOT NULL,
	deleted    BOOLEAN DEFAULT FALSE NOT NULL,
	created    TIMESTAMP DEFAULT (datetime('now')),
	updated    TIMESTAMP
, "seo" JSON DEFAULT '{}' NOT NULL, "brief" TEXT NOT NULL DEFAULT '', quantity INTEGER DEFAULT 0, sku TEXT, has_variants BOOLEAN DEFAULT FALSE);
CREATE INDEX idx_product_id ON product (id);
CREATE INDEX idx_product_name ON product (name);
CREATE INDEX idx_product_slug ON product (slug);
CREATE TABLE digital_file (
	id            TEXT PRIMARY KEY NOT NULL,
	product_id    TEXT NOT NULL,
	name          TEXT NOT NULL,
	ext           TEXT NOT NULL,
	orig_name     TEXT NOT NULL,
	FOREIGN KEY (product_id) REFERENCES product(id) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_digital_file_product_id ON digital_file (product_id);
CREATE TABLE digital_data (
	id            TEXT PRIMARY KEY NOT NULL,
	product_id    TEXT NOT NULL,
	content       TEXT NOT NULL,
	cart_id       TEXT DEFAULT NULL,
	FOREIGN KEY (product_id) REFERENCES product(id) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_digital_data_product_id ON digital_data (product_id);
CREATE TABLE product_image (
	id          TEXT PRIMARY KEY NOT NULL,
	product_id  TEXT NOT NULL,
	name        TEXT NOT NULL,
	ext         TEXT NOT NULL,
	orig_name   TEXT NOT NULL, position INTEGER DEFAULT 0,
	FOREIGN KEY (product_id) REFERENCES product(id) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_product_image_product_id ON product_image (product_id);
CREATE TABLE cart (
	id              TEXT PRIMARY KEY NOT NULL,
	email           TEXT DEFAULT NULL,
	amount_total    NUMERC NOT NULL,
	currency        TEXT NOT NULL,
	payment_id      TEXT DEFAULT NULL,
	payment_status  TEXT DEFAULT NULL,
	cart            JSON DEFAULT '[]' NOT NULL,
	created         TIMESTAMP DEFAULT (datetime('now')),
	updated         TIMESTAMP
, payment_system TEXT NOT NULL DEFAULT '');
CREATE UNIQUE INDEX idx_product_sku ON product (sku) WHERE sku IS NOT NULL;
CREATE TABLE product_option (
    id          TEXT PRIMARY KEY NOT NULL,
    product_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    position    INTEGER DEFAULT 0,
    created     TIMESTAMP DEFAULT (datetime('now')),
    FOREIGN KEY (product_id) REFERENCES product(id) ON DELETE CASCADE
);
CREATE INDEX idx_product_option_product_id ON product_option (product_id);
CREATE TABLE product_option_value (
    id          TEXT PRIMARY KEY NOT NULL,
    option_id   TEXT NOT NULL,
    value       TEXT NOT NULL,
    position    INTEGER DEFAULT 0,
    FOREIGN KEY (option_id) REFERENCES product_option(id) ON DELETE CASCADE
);
CREATE INDEX idx_product_option_value_option_id ON product_option_value (option_id);
CREATE TABLE product_variant (
    id              TEXT PRIMARY KEY NOT NULL,
    product_id      TEXT NOT NULL,
    sku             TEXT,
    price_surcharge NUMERIC DEFAULT 0,
    quantity        INTEGER DEFAULT 0,
    option_values   TEXT NOT NULL DEFAULT '{}',
    active          BOOLEAN DEFAULT TRUE,
    deleted         BOOLEAN DEFAULT FALSE,
    created         TIMESTAMP DEFAULT (datetime('now')),
    updated         TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES product(id) ON DELETE CASCADE
);
CREATE INDEX idx_product_variant_product_id ON product_variant (product_id);
CREATE UNIQUE INDEX idx_product_variant_sku ON product_variant (sku) WHERE sku IS NOT NULL;
CREATE TABLE product_variant_option (
    variant_id      TEXT NOT NULL,
    option_value_id TEXT NOT NULL,
    PRIMARY KEY (variant_id, option_value_id),
    FOREIGN KEY (variant_id) REFERENCES product_variant(id) ON DELETE CASCADE,
    FOREIGN KEY (option_value_id) REFERENCES product_option_value(id) ON DELETE CASCADE
);
CREATE INDEX idx_product_variant_option_variant ON product_variant_option (variant_id);
CREATE INDEX idx_product_variant_option_value ON product_variant_option (option_value_id);
CREATE TABLE product_variant_image (
    id          TEXT PRIMARY KEY NOT NULL,
    variant_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    ext         TEXT NOT NULL,
    orig_name   TEXT NOT NULL,
    position    INTEGER DEFAULT 0,
    FOREIGN KEY (variant_id) REFERENCES product_variant(id) ON DELETE CASCADE
);
CREATE INDEX idx_product_variant_image_variant_id ON product_variant_image (variant_id);
CREATE TABLE users (
    id         TEXT PRIMARY KEY NOT NULL,
    email      TEXT UNIQUE NOT NULL,
    password   TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT (datetime('now')),
    updated_at TIMESTAMP
);
CREATE INDEX idx_users_email ON users (email);
CREATE TABLE new_carts (
    id         TEXT PRIMARY KEY NOT NULL,
    session_id TEXT NOT NULL,
    status     TEXT NOT NULL,
    total      TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT (datetime('now')),
    updated_at TIMESTAMP
);
CREATE INDEX idx_new_carts_session_id ON new_carts (session_id);
CREATE TABLE cart_items (
    id         TEXT PRIMARY KEY NOT NULL,
    cart_id    TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity   INTEGER NOT NULL,
    price      TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT (datetime('now')),
    FOREIGN KEY (cart_id) REFERENCES new_carts(id) ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES product(id) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_cart_items_cart_id ON cart_items (cart_id);
CREATE INDEX idx_cart_items_product_id ON cart_items (product_id);
