package db

import (
	"strconv"
	"strings"
	"time"
)

// Finding evidence screenshots: files live in dataDir/report_assets/evidence/<node_id>/;
// metadata uses exploration node IDs, shared by findings, reports, and Markdown.

type FindingEvidence struct {
	ID        int64     `json:"id"`
	NodeID    int64     `json:"node_id"`
	Filename  string    `json:"filename"`
	Caption   string    `json:"caption"`
	StepIndex int       `json:"step_index"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateFindingEvidence records an existing file with sort_order set to the current maximum + 1.
func (d *DB) CreateFindingEvidence(nodeID int64, filename, caption string, stepIndex int) (*FindingEvidence, error) {
	var next int
	_ = d.QueryRow(`SELECT COALESCE(MAX(sort_order),0)+1 FROM finding_evidence WHERE node_id=$1`, nodeID).Scan(&next)
	var e FindingEvidence
	err := d.QueryRow(`
INSERT INTO finding_evidence(node_id, filename, caption, step_index, sort_order)
VALUES ($1,$2,$3,$4,$5)
RETURNING id, node_id, filename, caption, step_index, sort_order, created_at`,
		nodeID, filename, strings.TrimSpace(caption), stepIndex, next).
		Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListFindingEvidence returns evidence in display order.
func (d *DB) ListFindingEvidence(nodeID int64) ([]FindingEvidence, error) {
	rows, err := d.Query(`
SELECT id, node_id, filename, caption, step_index, sort_order, created_at
FROM finding_evidence WHERE node_id=$1 ORDER BY step_index, sort_order, id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FindingEvidence
	for rows.Next() {
		var e FindingEvidence
		if err := rows.Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountFindingEvidence batches screenshot counts for finding-list badges.
// Node IDs are int64; inlining them in IN clauses cannot inject SQL.
func (d *DB) CountFindingEvidence(nodeIDs []int64) map[int64]int {
	out := map[int64]int{}
	if len(nodeIDs) == 0 {
		return out
	}
	// Fully parameterized ($1,$2,...), avoiding SQL concatenation even for integer node IDs.
	parts := make([]string, len(nodeIDs))
	args := make([]any, len(nodeIDs))
	for i, id := range nodeIDs {
		parts[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := d.Query(`SELECT node_id, count(*) FROM finding_evidence WHERE node_id IN (`+strings.Join(parts, ",")+`) GROUP BY node_id`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var n int64
		var c int
		if rows.Scan(&n, &c) == nil {
			out[n] = c
		}
	}
	return out
}

// GetFindingEvidence retrieves filename and ownership for deletion checks.
func (d *DB) GetFindingEvidence(id int64) (*FindingEvidence, error) {
	var e FindingEvidence
	err := d.QueryRow(`
SELECT id, node_id, filename, caption, step_index, sort_order, created_at
FROM finding_evidence WHERE id=$1`, id).
		Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// UpdateFindingEvidence changes caption, associated step, or order.
func (d *DB) UpdateFindingEvidence(id int64, caption string, stepIndex, sortOrder int) error {
	_, err := d.Exec(`UPDATE finding_evidence SET caption=$1, step_index=$2, sort_order=$3 WHERE id=$4`,
		strings.TrimSpace(caption), stepIndex, sortOrder, id)
	return err
}

// DeleteFindingEvidence removes metadata; callers delete the returned filename from disk.
func (d *DB) DeleteFindingEvidence(id int64) error {
	_, err := d.Exec(`DELETE FROM finding_evidence WHERE id=$1`, id)
	return err
}
