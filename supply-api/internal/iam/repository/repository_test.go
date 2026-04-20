package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"lijiaoqiao/supply-api/internal/iam/model"
)

type stubIAMDB struct {
	execSQL  string
	execArgs []any
	execTag  pgconn.CommandTag
	execErr  error

	querySQL  string
	queryArgs []any
	queryRows pgx.Rows
	queryErr  error

	queryRowSQL  string
	queryRowArgs []any
	queryRow     pgx.Row
}

func (s *stubIAMDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.execSQL = sql
	s.execArgs = args
	return s.execTag, s.execErr
}

func (s *stubIAMDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.querySQL = sql
	s.queryArgs = args
	return s.queryRows, s.queryErr
}

func (s *stubIAMDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	s.queryRowSQL = sql
	s.queryRowArgs = args
	return s.queryRow
}

type stubIAMRow struct {
	values []any
	err    error
}

func (r stubIAMRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignIAMScan(dest, r.values)
}

type stubIAMRows struct {
	rows  [][]any
	index int
}

func (r *stubIAMRows) Close()                                       {}
func (r *stubIAMRows) Err() error                                   { return nil }
func (r *stubIAMRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *stubIAMRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *stubIAMRows) RawValues() [][]byte                          { return nil }
func (r *stubIAMRows) Values() ([]any, error)                       { return nil, nil }
func (r *stubIAMRows) Conn() *pgx.Conn                              { return nil }

func (r *stubIAMRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *stubIAMRows) Scan(dest ...any) error {
	return assignIAMScan(dest, r.rows[r.index-1])
}

func assignIAMScan(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan arity mismatch")
	}
	for i, value := range values {
		switch d := dest[i].(type) {
		case *int64:
			if value == nil {
				return fmt.Errorf("cannot scan NULL into *int64")
			}
			*d = value.(int64)
		case *string:
			if value == nil {
				return fmt.Errorf("cannot scan NULL into *string")
			}
			*d = value.(string)
		case *int:
			if value == nil {
				return fmt.Errorf("cannot scan NULL into *int")
			}
			*d = value.(int)
		case *bool:
			if value == nil {
				return fmt.Errorf("cannot scan NULL into *bool")
			}
			*d = value.(bool)
		case **int64:
			if value == nil {
				*d = nil
				continue
			}
			v := value.(int64)
			*d = &v
		case **string:
			if value == nil {
				*d = nil
				continue
			}
			v := value.(string)
			*d = &v
		case **time.Time:
			if value == nil {
				*d = nil
				continue
			}
			v := value.(time.Time)
			*d = &v
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func TestCreateRoleMapsDuplicateKeyAndInitializesDefaults(t *testing.T) {
	db := &stubIAMDB{
		execErr: errors.New("duplicate key value violates unique constraint"),
	}
	repo := newPostgresIAMRepositoryWithDB(db)
	role := &model.Role{
		Code:      "viewer",
		Name:      "Viewer",
		Type:      model.RoleTypePlatform,
		Level:     10,
		IsActive:  true,
		RequestID: "req-1",
		Version:   1,
	}

	err := repo.CreateRole(context.Background(), role)
	if !errors.Is(err, ErrDuplicateRoleCode) {
		t.Fatalf("CreateRole() error = %v, want %v", err, ErrDuplicateRoleCode)
	}
	if role.CreatedAt == nil || role.UpdatedAt == nil {
		t.Fatal("CreateRole() should initialize timestamps before persistence")
	}
	parentID, ok := db.execArgs[3].(*int64)
	if !ok || parentID != nil {
		t.Fatalf("parent_role_id arg = %#v, want typed nil *int64", db.execArgs[3])
	}
	if db.execArgs[8] != nil || db.execArgs[9] != nil {
		t.Fatalf("expected empty audit IPs to be stored as nil, got %#v %#v", db.execArgs[8], db.execArgs[9])
	}
}

func TestGetRoleByCodeMapsNoRowsToErrRoleNotFound(t *testing.T) {
	db := &stubIAMDB{
		queryRow: stubIAMRow{err: pgx.ErrNoRows},
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	role, err := repo.GetRoleByCode(context.Background(), "missing")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("GetRoleByCode() error = %v, want %v", err, ErrRoleNotFound)
	}
	if role != nil {
		t.Fatalf("expected nil role, got %#v", role)
	}
}

func TestListRolesAppliesRoleTypeFilter(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	db := &stubIAMDB{
		queryRows: &stubIAMRows{
			rows: [][]any{
				{int64(1), "viewer", "Viewer", model.RoleTypePlatform, nil, 10, "readonly", true, "req-1", nil, nil, 1, now, now},
			},
		},
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	roles, err := repo.ListRoles(context.Background(), model.RoleTypePlatform)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("role count = %d, want 1", len(roles))
	}
	if roles[0].Code != "viewer" || roles[0].Level != 10 {
		t.Fatalf("unexpected role payload: %#v", roles[0])
	}
	if len(db.queryArgs) != 1 || db.queryArgs[0] != model.RoleTypePlatform {
		t.Fatalf("query args = %#v, want [%q]", db.queryArgs, model.RoleTypePlatform)
	}
	if !strings.Contains(db.querySQL, "WHERE type = $1 AND is_active = true") {
		t.Fatalf("query SQL should include role type filter, got %s", db.querySQL)
	}
}

func TestRevokeRoleReturnsErrUserRoleNotFoundWhenNothingIsUpdated(t *testing.T) {
	db := &stubIAMDB{
		queryRow: stubIAMRow{values: []any{int64(99)}},
		execTag:  pgconn.NewCommandTag("UPDATE 0"),
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	err := repo.RevokeRole(context.Background(), 7, "viewer", 42)
	if !errors.Is(err, ErrUserRoleNotFound) {
		t.Fatalf("RevokeRole() error = %v, want %v", err, ErrUserRoleNotFound)
	}
	if len(db.execArgs) != 3 || db.execArgs[0] != int64(7) || db.execArgs[1] != int64(99) || db.execArgs[2] != int64(42) {
		t.Fatalf("unexpected revoke args: %#v", db.execArgs)
	}
}

func TestListRolesAcceptsNullRequestID(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	db := &stubIAMDB{
		queryRows: &stubIAMRows{
			rows: [][]any{
				{int64(1), "viewer", "Viewer", model.RoleTypePlatform, nil, 10, "readonly", true, nil, nil, nil, 1, now, now},
			},
		},
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	roles, err := repo.ListRoles(context.Background(), "")
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("role count = %d, want 1", len(roles))
	}
	if roles[0].RequestID != "" {
		t.Fatalf("expected null request_id to map to empty string, got %q", roles[0].RequestID)
	}
}

func TestGetUserRolesWithCodeAcceptsNullableTenantAndGrantFields(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	db := &stubIAMDB{
		queryRows: &stubIAMRows{
			rows: [][]any{
				{int64(11), int64(7), "viewer", nil, true, nil, nil, nil, now, now},
			},
		},
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	roles, err := repo.GetUserRolesWithCode(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetUserRolesWithCode() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("role count = %d, want 1", len(roles))
	}
	if roles[0].TenantID != 0 {
		t.Fatalf("expected null tenant_id to map to 0, got %d", roles[0].TenantID)
	}
	if roles[0].GrantedBy != 0 {
		t.Fatalf("expected null granted_by to map to 0, got %d", roles[0].GrantedBy)
	}
	if roles[0].RequestID != "" {
		t.Fatalf("expected null request_id to map to empty string, got %q", roles[0].RequestID)
	}
}

func TestUpdateRoleStoresEmptyUpdatedIPAsNil(t *testing.T) {
	db := &stubIAMDB{
		execTag: pgconn.NewCommandTag("UPDATE 1"),
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	err := repo.UpdateRole(context.Background(), &model.Role{
		Code:        "viewer",
		Name:        "Viewer",
		Description: "readonly",
		IsActive:    true,
		UpdatedIP:   "",
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if len(db.execArgs) != 5 {
		t.Fatalf("unexpected exec args length: got=%d want=5", len(db.execArgs))
	}
	if db.execArgs[4] != nil {
		t.Fatalf("expected empty updated_ip to be stored as nil, got %#v", db.execArgs[4])
	}
}

func TestUpdateRolePreservesExplicitUpdatedIP(t *testing.T) {
	db := &stubIAMDB{
		execTag: pgconn.NewCommandTag("UPDATE 1"),
	}
	repo := newPostgresIAMRepositoryWithDB(db)

	err := repo.UpdateRole(context.Background(), &model.Role{
		Code:        "viewer",
		Name:        "Viewer",
		Description: "readonly",
		IsActive:    true,
		UpdatedIP:   "10.0.0.8",
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if len(db.execArgs) != 5 {
		t.Fatalf("unexpected exec args length: got=%d want=5", len(db.execArgs))
	}
	if db.execArgs[4] != "10.0.0.8" {
		t.Fatalf("expected explicit updated_ip to be preserved, got %#v", db.execArgs[4])
	}
}
