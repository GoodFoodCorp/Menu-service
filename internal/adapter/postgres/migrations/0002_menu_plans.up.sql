-- Global catalog: the head office (admin) can now own dishes/menu plans too.
-- A NULL tenant_id means "common to every restaurant" instead of belonging
-- to exactly one franchisee.
ALTER TABLE menu_items ALTER COLUMN tenant_id DROP NOT NULL;

-- Free-text ingredient list (sourced, on the franchisee side, from their own
-- stock-service inventory — stock-service stays the source of truth for
-- actual stock levels, this is just a display/composition snapshot).
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS ingredients TEXT[] NOT NULL DEFAULT '{}';

-- "Menus" (formules): a named, priced bundle of dishes — e.g. "Menu Burger"
-- (burger + frites + boisson). Distinct from menu_items ("plats" = the
-- individual dishes a menu is built from).
CREATE TABLE IF NOT EXISTS menu_plans (
    id          UUID PRIMARY KEY,
    tenant_id   UUID,
    name        TEXT             NOT NULL,
    description TEXT             NOT NULL DEFAULT '',
    price_cents BIGINT           NOT NULL CHECK (price_cents > 0),
    emoji       TEXT             NOT NULL DEFAULT '',
    available   BOOLEAN          NOT NULL DEFAULT TRUE,
    sort_order  INT              NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_menu_plans_tenant ON menu_plans (tenant_id, sort_order);

-- Which dishes make up a menu plan.
CREATE TABLE IF NOT EXISTS menu_plan_dishes (
    menu_plan_id UUID NOT NULL REFERENCES menu_plans(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    sort_order   INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (menu_plan_id, menu_item_id)
);
