package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/orbus-digital/agenda/internal/domain"
)

// CalendarRepo implements domain.CalendarRepository using PostgreSQL.
type CalendarRepo struct {
	db *DB
}

// NewCalendarRepo creates a new CalendarRepo.
func NewCalendarRepo(db *DB) *CalendarRepo {
	return &CalendarRepo{db: db}
}

func (r *CalendarRepo) Create(ctx context.Context, cal *domain.Calendar) error {
	return r.db.WithTenantTx(ctx, cal.TenantID.String(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO agenda.calendars (id, tenant_id, name, description, color, owner_id, is_default, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			cal.ID, cal.TenantID, cal.Name, cal.Description, cal.Color, cal.OwnerID, cal.IsDefault, cal.CreatedAt, cal.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: calendar name already exists for this owner", domain.ErrConflict)
			}
			return err
		}
		return nil
	})
}

func (r *CalendarRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Calendar, error) {
	var cal domain.Calendar
	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT id, tenant_id, name, description, color, owner_id, is_default, deleted_at, created_at, updated_at
			 FROM agenda.calendars
			 WHERE id = $1 AND deleted_at IS NULL`, id)
		return scanCalendar(row, &cal)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &cal, nil
}

func (r *CalendarRepo) List(ctx context.Context, tenantID uuid.UUID, ownerID *uuid.UUID, limit, offset int) ([]*domain.Calendar, int, error) {
	var calendars []*domain.Calendar
	var total int

	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		// Count
		countQ := `SELECT COUNT(*) FROM agenda.calendars WHERE deleted_at IS NULL`
		args := []interface{}{}
		argIdx := 1

		if ownerID != nil {
			countQ += fmt.Sprintf(" AND owner_id = $%d", argIdx)
			args = append(args, *ownerID)
			argIdx++
		}

		if err := tx.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return err
		}

		// List
		listQ := `SELECT id, tenant_id, name, description, color, owner_id, is_default, deleted_at, created_at, updated_at
				   FROM agenda.calendars WHERE deleted_at IS NULL`
		listArgs := []interface{}{}
		listIdx := 1

		if ownerID != nil {
			listQ += fmt.Sprintf(" AND owner_id = $%d", listIdx)
			listArgs = append(listArgs, *ownerID)
			listIdx++
		}

		listQ += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", listIdx, listIdx+1)
		listArgs = append(listArgs, limit, offset)

		rows, err := tx.Query(ctx, listQ, listArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var cal domain.Calendar
			if err := scanCalendarRows(rows, &cal); err != nil {
				return err
			}
			calendars = append(calendars, &cal)
		}
		return rows.Err()
	})

	return calendars, total, err
}

func (r *CalendarRepo) Update(ctx context.Context, cal *domain.Calendar) error {
	cal.UpdatedAt = time.Now().UTC()
	return r.db.WithTenantTx(ctx, cal.TenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE agenda.calendars SET name = $1, description = $2, color = $3, updated_at = $4
			 WHERE id = $5 AND deleted_at IS NULL`,
			cal.Name, cal.Description, cal.Color, cal.UpdatedAt, cal.ID)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: calendar name already exists for this owner", domain.ErrConflict)
			}
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *CalendarRepo) SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		// Soft-delete the calendar
		ct, err := tx.Exec(ctx,
			`UPDATE agenda.calendars SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
			now, id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		// Soft-delete all events in this calendar (BR-CAL-004)
		_, err = tx.Exec(ctx,
			`UPDATE agenda.events SET deleted_at = $1, status = 'cancelled', updated_at = $1
			 WHERE calendar_id = $2 AND deleted_at IS NULL`,
			now, id)
		return err
	})
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanCalendar(row scannable, cal *domain.Calendar) error {
	return row.Scan(
		&cal.ID, &cal.TenantID, &cal.Name, &cal.Description, &cal.Color,
		&cal.OwnerID, &cal.IsDefault, &cal.DeletedAt, &cal.CreatedAt, &cal.UpdatedAt,
	)
}

func scanCalendarRows(rows pgx.Rows, cal *domain.Calendar) error {
	return rows.Scan(
		&cal.ID, &cal.TenantID, &cal.Name, &cal.Description, &cal.Color,
		&cal.OwnerID, &cal.IsDefault, &cal.DeletedAt, &cal.CreatedAt, &cal.UpdatedAt,
	)
}
