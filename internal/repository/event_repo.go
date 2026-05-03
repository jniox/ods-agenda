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

// EventRepo implements domain.EventRepository using PostgreSQL.
type EventRepo struct {
	db *DB
}

// NewEventRepo creates a new EventRepo.
func NewEventRepo(db *DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) Create(ctx context.Context, ev *domain.Event) error {
	return r.db.WithTenantTx(ctx, ev.TenantID.String(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO agenda.events (id, tenant_id, calendar_id, title, description, location,
			 start_time, end_time, all_day, timezone, status, recurrence_rule, created_by, created_at, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			ev.ID, ev.TenantID, ev.CalendarID, ev.Title, ev.Description, ev.Location,
			ev.StartTime, ev.EndTime, ev.AllDay, ev.Timezone, ev.Status, ev.RecurrenceRule,
			ev.CreatedBy, ev.CreatedAt, ev.UpdatedAt)
		return err
	})
}

func (r *EventRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Event, error) {
	var ev domain.Event
	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT id, tenant_id, calendar_id, title, description, location,
			 start_time, end_time, all_day, timezone, status, recurrence_rule,
			 created_by, deleted_at, created_at, updated_at
			 FROM agenda.events WHERE id = $1 AND deleted_at IS NULL`, id)
		return scanEvent(row, &ev)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *EventRepo) ListByCalendar(ctx context.Context, tenantID, calendarID uuid.UUID, start, end time.Time, status *domain.EventStatus, limit, offset int) ([]*domain.Event, int, error) {
	var events []*domain.Event
	var total int

	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		baseWhere := `WHERE calendar_id = $1 AND start_time < $2 AND end_time > $3 AND deleted_at IS NULL`
		args := []interface{}{calendarID, end, start}
		argIdx := 4

		if status != nil {
			baseWhere += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, string(*status))
			argIdx++
		}

		// Count
		countQ := `SELECT COUNT(*) FROM agenda.events ` + baseWhere
		if err := tx.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return err
		}

		// List
		listQ := fmt.Sprintf(`SELECT id, tenant_id, calendar_id, title, description, location,
			start_time, end_time, all_day, timezone, status, recurrence_rule,
			created_by, deleted_at, created_at, updated_at
			FROM agenda.events %s ORDER BY start_time ASC LIMIT $%d OFFSET $%d`,
			baseWhere, argIdx, argIdx+1)
		listArgs := append(args, limit, offset)

		rows, err := tx.Query(ctx, listQ, listArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var ev domain.Event
			if err := scanEventRows(rows, &ev); err != nil {
				return err
			}
			events = append(events, &ev)
		}
		return rows.Err()
	})

	return events, total, err
}

func (r *EventRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, calendarID, createdBy *uuid.UUID, start, end time.Time, status *domain.EventStatus, limit, offset int) ([]*domain.Event, int, error) {
	var events []*domain.Event
	var total int

	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		baseWhere := `WHERE start_time < $1 AND end_time > $2 AND deleted_at IS NULL`
		args := []interface{}{end, start}
		argIdx := 3

		if calendarID != nil {
			baseWhere += fmt.Sprintf(" AND calendar_id = $%d", argIdx)
			args = append(args, *calendarID)
			argIdx++
		}
		if createdBy != nil {
			baseWhere += fmt.Sprintf(" AND created_by = $%d", argIdx)
			args = append(args, *createdBy)
			argIdx++
		}
		if status != nil {
			baseWhere += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, string(*status))
			argIdx++
		}

		countQ := `SELECT COUNT(*) FROM agenda.events ` + baseWhere
		if err := tx.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return err
		}

		listQ := fmt.Sprintf(`SELECT id, tenant_id, calendar_id, title, description, location,
			start_time, end_time, all_day, timezone, status, recurrence_rule,
			created_by, deleted_at, created_at, updated_at
			FROM agenda.events %s ORDER BY start_time ASC LIMIT $%d OFFSET $%d`,
			baseWhere, argIdx, argIdx+1)
		listArgs := append(args, limit, offset)

		rows, err := tx.Query(ctx, listQ, listArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var ev domain.Event
			if err := scanEventRows(rows, &ev); err != nil {
				return err
			}
			events = append(events, &ev)
		}
		return rows.Err()
	})

	return events, total, err
}

func (r *EventRepo) Update(ctx context.Context, ev *domain.Event) error {
	ev.UpdatedAt = time.Now().UTC()
	return r.db.WithTenantTx(ctx, ev.TenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE agenda.events SET title=$1, description=$2, location=$3,
			 start_time=$4, end_time=$5, all_day=$6, timezone=$7, status=$8,
			 recurrence_rule=$9, updated_at=$10
			 WHERE id=$11 AND deleted_at IS NULL`,
			ev.Title, ev.Description, ev.Location,
			ev.StartTime, ev.EndTime, ev.AllDay, ev.Timezone, ev.Status,
			ev.RecurrenceRule, ev.UpdatedAt, ev.ID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *EventRepo) Cancel(ctx context.Context, tenantID, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE agenda.events SET status='cancelled', deleted_at=$1, updated_at=$1
			 WHERE id=$2 AND deleted_at IS NULL`, now, id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func scanEvent(row scannable, ev *domain.Event) error {
	return row.Scan(
		&ev.ID, &ev.TenantID, &ev.CalendarID, &ev.Title, &ev.Description, &ev.Location,
		&ev.StartTime, &ev.EndTime, &ev.AllDay, &ev.Timezone, &ev.Status, &ev.RecurrenceRule,
		&ev.CreatedBy, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt,
	)
}

func scanEventRows(rows pgx.Rows, ev *domain.Event) error {
	return rows.Scan(
		&ev.ID, &ev.TenantID, &ev.CalendarID, &ev.Title, &ev.Description, &ev.Location,
		&ev.StartTime, &ev.EndTime, &ev.AllDay, &ev.Timezone, &ev.Status, &ev.RecurrenceRule,
		&ev.CreatedBy, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt,
	)
}
