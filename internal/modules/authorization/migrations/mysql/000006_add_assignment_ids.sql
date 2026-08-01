-- +goose Up
ALTER TABLE system_role_assignments
    ADD COLUMN assignment_id BINARY(16) NULL;

UPDATE system_role_assignments
SET assignment_id = UNHEX(MD5(CONCAT(HEX(user_id), ':', HEX(role_id))))
WHERE assignment_id IS NULL;

ALTER TABLE system_role_assignments
    MODIFY COLUMN assignment_id BINARY(16) NOT NULL,
    ADD CONSTRAINT system_role_assignments_assignment_id_unique
        UNIQUE (assignment_id);

ALTER TABLE membership_roles
    ADD COLUMN assignment_id BINARY(16) NULL;

UPDATE membership_roles
SET assignment_id = UNHEX(MD5(
    CONCAT(HEX(account_id), ':', HEX(membership_id), ':', HEX(role_id)
)))
WHERE assignment_id IS NULL;

ALTER TABLE membership_roles
    MODIFY COLUMN assignment_id BINARY(16) NOT NULL,
    ADD CONSTRAINT membership_roles_assignment_id_unique
        UNIQUE (assignment_id);

-- +goose Down
ALTER TABLE membership_roles
    DROP INDEX membership_roles_assignment_id_unique,
    DROP COLUMN assignment_id;

ALTER TABLE system_role_assignments
    DROP INDEX system_role_assignments_assignment_id_unique,
    DROP COLUMN assignment_id;
