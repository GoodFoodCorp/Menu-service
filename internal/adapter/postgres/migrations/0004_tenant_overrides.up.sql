-- A franchisee cannot edit or delete a global (head-office) dish/menu — but
-- they can hide it from their own restaurant without affecting anyone else.
-- Presence of a row means "hidden by this tenant"; there is nothing to store
-- beyond the pair, so no extra column is needed.
CREATE TABLE IF NOT EXISTS menu_item_tenant_overrides (
    tenant_id    UUID NOT NULL,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, menu_item_id)
);

CREATE TABLE IF NOT EXISTS menu_plan_tenant_overrides (
    tenant_id    UUID NOT NULL,
    menu_plan_id UUID NOT NULL REFERENCES menu_plans(id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, menu_plan_id)
);
