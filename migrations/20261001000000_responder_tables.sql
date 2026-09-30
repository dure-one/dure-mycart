-- +goose Up
-- +goose StatementBegin
CREATE TABLE customer_contact (
    id          TEXT PRIMARY KEY NOT NULL,
    customer_id TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('sms', 'xmpp')),
    address     TEXT NOT NULL,
    active      BOOLEAN DEFAULT TRUE NOT NULL,
    created     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated     TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customer(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_customer_contact_address ON customer_contact(type, address);
CREATE INDEX idx_customer_contact_customer_id ON customer_contact(customer_id);

CREATE TABLE message (
    id              TEXT PRIMARY KEY NOT NULL,
    contact_id      TEXT NOT NULL,
    content         TEXT NOT NULL,
    direction       TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    channel         TEXT NOT NULL CHECK (channel IN ('sms', 'xmpp')),
    delivery_status TEXT CHECK (delivery_status IN ('pending', 'delivered', 'failed')),
    read_status     BOOLEAN DEFAULT FALSE NOT NULL,
    created         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (contact_id) REFERENCES customer_contact(id) ON DELETE CASCADE
);
CREATE INDEX idx_message_contact_id ON message(contact_id);
CREATE INDEX idx_message_created ON message(created DESC);
CREATE INDEX idx_message_channel ON message(channel);

CREATE TABLE message_thread (
    id                   TEXT PRIMARY KEY NOT NULL,
    customer_id          TEXT NOT NULL,
    last_message_at      TIMESTAMP NOT NULL,
    last_message_preview TEXT,
    unread_count         INTEGER DEFAULT 0,
    FOREIGN KEY (customer_id) REFERENCES customer(id) ON DELETE CASCADE
);
CREATE INDEX idx_message_thread_customer_id ON message_thread(customer_id);
CREATE INDEX idx_message_thread_last_message ON message_thread(last_message_at DESC);

CREATE TABLE workflow (
    id          TEXT PRIMARY KEY NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    content     TEXT NOT NULL,
    enabled     BOOLEAN DEFAULT TRUE NOT NULL,
    tags        TEXT DEFAULT '[]' NOT NULL,
    version     TEXT,
    author      TEXT,
    created     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated     TIMESTAMP
);
CREATE INDEX idx_workflow_name ON workflow(name);
CREATE INDEX idx_workflow_enabled ON workflow(enabled);

CREATE TABLE crontab_job (
    id       TEXT PRIMARY KEY NOT NULL,
    job_type TEXT UNIQUE NOT NULL CHECK (job_type IN ('xmpp_check', 'cleanup_inactive')),
    interval TEXT NOT NULL CHECK (interval IN ('5min', '15min', '1hr', '6hr', 'daily')),
    enabled  BOOLEAN DEFAULT TRUE NOT NULL,
    last_run TIMESTAMP,
    next_run TIMESTAMP,
    created  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated  TIMESTAMP
);
CREATE INDEX idx_crontab_job_enabled ON crontab_job(enabled);

INSERT INTO crontab_job (id, job_type, interval, enabled) VALUES
    ('xmpp_check_job', 'xmpp_check', '15min', true),
    ('cleanup_job', 'cleanup_inactive', 'daily', true);

CREATE TABLE archived_customer (
    id              TEXT PRIMARY KEY NOT NULL,
    email           TEXT NOT NULL,
    password        TEXT NOT NULL,
    name            TEXT,
    active          BOOLEAN NOT NULL,
    created         TIMESTAMP,
    updated         TIMESTAMP,
    archived_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    archived_reason TEXT
);

CREATE TABLE archived_message (
    id              TEXT PRIMARY KEY NOT NULL,
    contact_id      TEXT NOT NULL,
    content         TEXT NOT NULL,
    direction       TEXT NOT NULL,
    channel         TEXT NOT NULL,
    delivery_status TEXT,
    read_status     BOOLEAN NOT NULL,
    created         TIMESTAMP,
    archived_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO setting (id, key, value) VALUES
    ('resp_xmpp_jid', 'responder_xmpp_jid', ''),
    ('resp_xmpp_pwd', 'responder_xmpp_password', ''),
    ('resp_xmpp_srv', 'responder_xmpp_server', ''),
    ('resp_xmpp_prt', 'responder_xmpp_port', '5222');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS archived_message;
DROP TABLE IF EXISTS archived_customer;
DROP TABLE IF EXISTS crontab_job;
DROP TABLE IF EXISTS workflow;
DROP TABLE IF EXISTS message_thread;
DROP TABLE IF EXISTS message;
DROP TABLE IF EXISTS customer_contact;
DELETE FROM setting WHERE key IN ('responder_xmpp_jid', 'responder_xmpp_password', 'responder_xmpp_server', 'responder_xmpp_port');
-- +goose StatementEnd
