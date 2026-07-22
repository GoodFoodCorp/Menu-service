CREATE TABLE IF NOT EXISTS menu_items (
    id          UUID PRIMARY KEY,
    tenant_id   UUID             NOT NULL,
    name        TEXT             NOT NULL,
    description TEXT             NOT NULL DEFAULT '',
    price_cents BIGINT           NOT NULL CHECK (price_cents > 0),
    category    TEXT             NOT NULL,
    emoji       TEXT             NOT NULL DEFAULT '',
    rating      DOUBLE PRECISION NOT NULL DEFAULT 0,
    available   BOOLEAN          NOT NULL DEFAULT TRUE,
    sort_order  INT              NOT NULL DEFAULT 0
);

-- Menus are always queried per restaurant → index the tenant boundary.
CREATE INDEX IF NOT EXISTS idx_menu_items_tenant ON menu_items (tenant_id, sort_order);
