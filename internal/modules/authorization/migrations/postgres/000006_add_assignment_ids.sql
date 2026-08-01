-- +goose Up
ALTER TABLE system_role_assignments
    ADD COLUMN assignment_id UUID;

UPDATE system_role_assignments
SET assignment_id = md5(user_id::text || ':' || role_id::text)::uuid
WHERE assignment_id IS NULL;

ALTER TABLE system_role_assignments
    ALTER COLUMN assignment_id SET NOT NULL,
    ADD CONSTRAINT system_role_assignments_assignment_id_unique
        UNIQUE (assignment_id);

ALTER TABLE membership_roles
    ADD COLUMN assignment_id UUID;

UPDATE membership_roles
SET assignment_id = md5(
    account_id::text || ':' || membership_id::text || ':' || role_id::text
)::uuid
WHERE assignment_id IS NULL;

ALTER TABLE membership_roles
    ALTER COLUMN assignment_id SET NOT NULL,
    ADD CONSTRAINT membership_roles_assignment_id_unique
        UNIQUE (assignment_id);

-- +goose Down
ALTER TABLE membership_roles
    DROP CONSTRAINT membership_roles_assignment_id_unique,
    DROP COLUMN assignment_id;

ALTER TABLE system_role_assignments
    DROP CONSTRAINT system_role_assignments_assignment_id_unique,
    DROP COLUMN assignment_id;

