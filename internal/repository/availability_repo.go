package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/orbus-digital/agenda/internal/domain"
)

// AvailabilityRepo implements domain.AvailabilityRepository.
type AvailabilityRepo struct {
	db *DB
}

// NewAvailabilityRepo creates a new AvailabilityRepo.
func NewAvailabilityRepo(db *DB) *AvailabilityRepo {
	return &AvailabilityRepo{db: db}
}

func (r *AvailabilityRepo) GetBusySlots(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, start, end time.Time) (map[uuid.UUID][]domain.BusySlot, error) {
	result := make(map[uuid.UUID][]domain.BusySlot)
	for _, uid := range userIDs {
		result[uid] = []domain.BusySlot{}
	}

	err := r.db.WithTenantTx(ctx, tenantID.String(), func(tx pgx.Tx) error {
		// Find events where user is creator or accepted/tentative attendee,
		// event status is confirmed or tentative, within time range.
		rows, err := tx.Query(ctx,
			`SELECT DISTINCT e.id, e.start_time, e.end_time, e.title, e.created_by,
			        a.user_id AS attendee_user_id
			 FROM agenda.events e
			 LEFT JOIN agenda.attendees a ON a.event_id = e.id AND a.user_id = ANY($1)
			 WHERE e.deleted_at IS NULL
			   AND e.status IN ('confirmed', 'tentative')
			   AND e.start_time < $2
			   AND e.end_time > $3
			   AND (
			       e.created_by = ANY($1)
			       OR (a.user_id = ANY($1) AND a.status IN ('accepted', 'tentative'))
			   )
			 ORDER BY e.start_time`,
			userIDs, end, start)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var eventID uuid.UUID
			var startTime, endTime time.Time
			var title string
			var createdBy uuid.UUID
			var attendeeUserID *uuid.UUID

			if err := rows.Scan(&eventID, &startTime, &endTime, &title, &createdBy, &attendeeUserID); err != nil {
				return err
			}

			slot := domain.BusySlot{
				Start:   startTime,
				End:     endTime,
				EventID: eventID,
				Title:   title,
			}

			// Add slot for the creator if they are in the userIDs list
			for _, uid := range userIDs {
				if uid == createdBy {
					result[uid] = appendUniqueBusySlot(result[uid], slot)
				}
			}
			// Add slot for the attendee if matched
			if attendeeUserID != nil {
				result[*attendeeUserID] = appendUniqueBusySlot(result[*attendeeUserID], slot)
			}
		}
		return rows.Err()
	})

	return result, err
}

func appendUniqueBusySlot(slots []domain.BusySlot, slot domain.BusySlot) []domain.BusySlot {
	for _, s := range slots {
		if s.EventID == slot.EventID {
			return slots
		}
	}
	return append(slots, slot)
}
