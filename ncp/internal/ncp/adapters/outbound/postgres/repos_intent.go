package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type IntentRepo struct{ DB *DB }

func (r IntentRepo) CreateIntent(ctx context.Context, it outbound.Intent) error {
	labels, _ := json.Marshal(it.Labels)

	var cur any = nil
	if it.CurrentRevision != nil {
		cur = *it.CurrentRevision
	}

	q := `INSERT INTO intents(id,key,title,owner_team,status,current_revision,labels,created_by,created_at,updated_at)
	      VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),now())`

	return exec(ctx, r.DB, q, it.ID, it.Key, it.Title, it.OwnerTeam, it.Status, cur, labels, it.CreatedBy)
}

func (r IntentRepo) GetIntent(ctx context.Context, id uuid.UUID) (outbound.Intent, error) {
	q := `SELECT id,key,title,owner_team,status,current_revision,labels,created_by,
	             extract(epoch from created_at)::bigint,
	             extract(epoch from updated_at)::bigint
	      FROM intents WHERE id=$1`

	row := queryRow(ctx, r.DB, q, id)

	var it outbound.Intent
	var labelsBytes []byte
	var cur *int64

	if err := row.Scan(&it.ID, &it.Key, &it.Title, &it.OwnerTeam, &it.Status, &cur, &labelsBytes, &it.CreatedBy, &it.CreatedAtUnix, &it.UpdatedAtUnix); err != nil {
		return outbound.Intent{}, err
	}
	it.CurrentRevision = cur
	_ = json.Unmarshal(labelsBytes, &it.Labels)
	return it, nil
}

func (r IntentRepo) CreateRevision(ctx context.Context, rev outbound.Revision) (int, error) {
	tx, ok := getTx(ctx)
	if !ok {
		return 0, errors.New("CreateRevision must run within tx")
	}

	if _, err := tx.Exec(ctx, `SELECT 1 FROM intents WHERE id=$1 FOR UPDATE`, rev.IntentID); err != nil {
		return 0, err
	}

	var next int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0)+1 FROM intent_revisions WHERE intent_id=$1`, rev.IntentID).Scan(&next); err != nil {
		return 0, err
	}

	specBytes, err := json.Marshal(rev.Spec)
	if err != nil {
		return 0, err
	}
	h := sha256.Sum256(specBytes)
	specHash := hex.EncodeToString(h[:])

	var ttl any = nil
	if rev.TTLSeconds != nil {
		ttl = *rev.TTLSeconds
	}
	if rev.TTLSeconds == nil {
		log.Printf("DEBUG: CreateRevision TTLSeconds=nil intent=%s ticket=%s", rev.IntentID, rev.TicketRef)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO intent_revisions(
			intent_id,revision,spec,spec_hash,state,justification,ticket_ref,ttl_seconds,not_after,created_by,created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULL,$9,now())`,
		rev.IntentID, next, specBytes, specHash, rev.State, rev.Justification, rev.TicketRef, ttl, rev.CreatedBy,
	)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `UPDATE intents SET current_revision=$2, updated_at=now() WHERE id=$1`, rev.IntentID, next)
	return next, err
}

func (r IntentRepo) GetRevision(ctx context.Context, intentID uuid.UUID, revision int) (outbound.Revision, error) {
	q := `SELECT id,intent_id,revision,spec,spec_hash,state,ticket_ref,justification,ttl_seconds,
	             CASE WHEN not_after IS NULL THEN NULL ELSE extract(epoch from not_after)::bigint END,
	             created_by,extract(epoch from created_at)::bigint
	      FROM intent_revisions WHERE intent_id=$1 AND revision=$2`

	row := queryRow(ctx, r.DB, q, intentID, revision)

	var rev outbound.Revision
	var specBytes []byte
	var ttl *int
	var notAfter *int64

	if err := row.Scan(&rev.ID, &rev.IntentID, &rev.Revision, &specBytes, &rev.SpecHash, &rev.State,
		&rev.TicketRef, &rev.Justification, &ttl, &notAfter, &rev.CreatedBy, &rev.CreatedAtUnix); err != nil {
		return outbound.Revision{}, err
	}

	rev.TTLSeconds = ttl
	rev.NotAfterUnix = notAfter
	_ = json.Unmarshal(specBytes, &rev.Spec)
	return rev, nil
}

func (r IntentRepo) RefreshNotAfterOnApply(ctx context.Context, revisionID int64) error {
	q := `
		UPDATE intent_revisions
		SET not_after = CASE
			WHEN ttl_seconds IS NULL THEN NULL
			ELSE now() + (ttl_seconds || ' seconds')::interval
		END
		WHERE id = $1
	`
	return exec(ctx, r.DB, q, revisionID)
}

func (r *IntentRepo) SetNotAfterOnFirstApply(ctx context.Context, revisionID int64, appliedAt time.Time) error {
	q := `
		UPDATE intent_revisions
		SET not_after = now() + (ttl_seconds::bigint * INTERVAL '1 second')
		WHERE id = $1
		  AND ttl_seconds IS NOT NULL
		  AND ttl_seconds > 0
		  AND not_after IS NULL
	`
	return exec(ctx, r.DB, q, revisionID)
}
