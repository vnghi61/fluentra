-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------- core.permissions
-- Phase 3 (WO-15) review: money and creator bank details need permissions of
-- their own.
--
-- The payout and reconciliation routes shipped behind `admin.dashboard`, which
-- is the permission that means "may open the back office". It put issuing a
-- payout, and reading a creator's bank account number, behind the weakest
-- permission the system has — and `payment/AGENT.md` had specified
-- `billing.read` and `billing.manage` for exactly this from the start.
--
-- `billing.read` sees money that moved. `billing.manage` moves it: marking a
-- payout sent, marking a refund paid, replaying a webhook.
INSERT INTO core.permissions (name, description) VALUES
    ('billing.read',   'View orders, payments, refunds, payouts and reconciliation.'),
    ('billing.manage', 'Issue refunds, fulfil payouts and replay payment webhooks.')
ON CONFLICT (name) DO NOTHING;

-- Admin keeps everything, as it does for every other permission.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
CROSS JOIN core.permissions p
WHERE r.name = 'admin'
  AND p.name IN ('billing.read', 'billing.manage')
ON CONFLICT DO NOTHING;

-- Moderators deliberately get neither. Reviewing a course and paying for one
-- are different jobs, and a moderator has no reason to read anybody's bank
-- account number (BR-STUDIO-09).

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.role_permissions
WHERE permission_id IN (
    SELECT id FROM core.permissions WHERE name IN ('billing.read', 'billing.manage')
);

DELETE FROM core.permissions WHERE name IN ('billing.read', 'billing.manage');
-- +goose StatementEnd
