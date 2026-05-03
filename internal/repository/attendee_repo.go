package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/orbus-digital/agenda/internal/domain"
)

// AttendeeRepo implements domain.AttendeeRepository using PostgreSQL.
type AttendeeRepo struct {
	db *DB
}

// NewAttendeeRepo creates a new AttendeeRepo.
func NewAttendeeRepo(db *DB) *AttendeeRepo {
	return &AttendeeRepo{db: db}
}

func (r *AttendeeRepo) Create(ctx context.Context, att *domain.Attendee) error {
	return r.db.WithTenantTx(ctx, att.TenantID.String(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO agenda.attendees (id, tenant_id, event_id, user_id, email, status, role, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			att.ID, att.TenantID, att.EventID, nullableUUID(att.UserID), att.Email, att.Status, att.Role, att.CreatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: attendee already exists for this event", domain.ErrConflict)
			}
			return err
		}
		return nil
	})
}

func (r *AttendeeRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Attendee, error) {
	var att domain.Attendee
	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT id, tenant_id, event_id, user_id, email, status, role, responded_at, created_at
			 FROM agenda.attendees WHERE id = $1`, id)
		return scanAttendee(row, &att)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &att, nil
}

func (r *AttendeeRepo) ListByEvent(ctx context.Context, tenantID, eventID uuid.UUID) ([]*domain.Attendee, error) {
	var attendees []*domain.Attendee
	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, tenant_id, event_id, user_id, email, status, role, responded_at, created_at
			 FROM agenda.attendees WHERE event_id = $1 ORDER BY created_at ASC`, eventID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var att domain.Attendee
			if err := scanAttendeeRows(rows, &att); err != nil {
				return err
			}
			attendees = append(attendees, &att)
		}
		return rows.Err()
	})
	return attendees, err
}

func (r *AttendeeRepo) UpdateStatus(ctx context.Context, att *domain.Attendee) error {
	return r.db.WithTenantTx(ctx, att.TenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE agenda.attendees SET status = $1, responded_at = $2 WHERE id = $3`,
			att.Status, att.RespondedAt, att.ID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *AttendeeRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `DELETE FROM agenda.attendees WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func scanAttendee(row scannable, att *domain.Attendee) error {
	var userID *uuid.UUID
	err := row.Scan(
		&att.ID, &att.TenantID, &att.EventID, &userID, &att.Email,
		&att.Status, &att.Role, &att.RespondedAt, &att.CreatedAt,
	)
	if userID != nil {
		att.UserID = *userID
	}
	return err
}

func scanAttendeeRows(rows pgx.Rows, att *domain.Attendee) error {
	var userID *uuid.UUID
	err := rows.Scan(
		&att.ID, &att.TenantID, &att.EventID, &userID, &att.Email,
		&att.Status, &att.Role, &att.RespondedAt, &att.CreatedAt,
	)
	if userID != nil {
		att.UserID = *userID
	}
	return err
}

// nullableUUID returns nil if the UUID is zero, otherwise a pointer to the UUID.
func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
