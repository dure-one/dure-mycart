-- Current database schema for sqlc
-- This represents the state after all migrations

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

CREATE TABLE page (
	id        TEXT PRIMARY KEY NOT NULL,
	name      TEXT NOT NULL,
	slug      TEXT UNIQUE NOT NULL,
	content   TEXT DEFAULT NULL,
	position  TEXT NOT NULL CHECK (position IN ('header', 'footer')),
	active    BOOLEAN DEFAULT FALSE NOT NULL,
	created   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated   TIMESTAMP
);

CREATE INDEX idx_page_name ON page (name);
CREATE INDEX idx_page_slug ON page (slug);
CREATE INDEX idx_page_group ON page (position);

CREATE TABLE product (
	id         TEXT PRIMARY KEY NOT NULL,
	name       TEXT NOT NULL,
	"desc"     TEXT NOT NULL,
	slug       TEXT UNIQUE NOT NULL,
	amount     NUMERIC NOT NULL,
	metadata   TEXT DEFAULT '[]' NOT NULL,
	attribute  TEXT DEFAULT '[]' NOT NULL,
	digital    TEXT CHECK (digital IN ('file', 'data', 'api')),
	active     BOOLEAN DEFAULT TRUE NOT NULL,
	deleted    BOOLEAN DEFAULT FALSE NOT NULL,
	created    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated    TIMESTAMP
);

CREATE INDEX idx_product_id ON product (id);
CREATE INDEX idx_product_name ON product (name);
CREATE INDEX idx_product_slug ON product (slug);

CREATE TABLE cart (
	id           TEXT PRIMARY KEY NOT NULL,
	customer_id  TEXT DEFAULT NULL,
	email        TEXT DEFAULT NULL,
	status       TEXT NOT NULL CHECK (status IN ('processing', 'paid', 'delivered', 'cancelled')) DEFAULT 'processing',
	stripe_id    TEXT DEFAULT NULL,
	amount       NUMERIC DEFAULT 0,
	created      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated      TIMESTAMP
);

CREATE INDEX idx_cart_customer_id ON cart (customer_id);
CREATE INDEX idx_cart_email ON cart (email);
CREATE INDEX idx_cart_status ON cart (status);

CREATE TABLE customer (
	id       TEXT PRIMARY KEY NOT NULL,
	email    TEXT UNIQUE NOT NULL,
	name     TEXT DEFAULT NULL,
	created  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated  TIMESTAMP
);

CREATE INDEX idx_customer_email ON customer (email);
