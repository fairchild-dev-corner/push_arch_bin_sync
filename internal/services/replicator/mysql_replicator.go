package replicator

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"push_arch_bin_sync/internal/models/change"
	"push_arch_bin_sync/internal/models/table"
)

// MySQLReplicator mirrors detected source changes into same-named tables on
// a destination MySQL database.
type MySQLReplicator struct {
	db *sql.DB
}

func NewMySQLReplicator(db *sql.DB) *MySQLReplicator {
	return &MySQLReplicator{db: db}
}

// ApplyBatch applies all changes from one poll cycle inside a single
// transaction, in their original order. On any failure the whole
// transaction is rolled back, preserving cross-table consistency within
// the batch.
func (r *MySQLReplicator) ApplyBatch(changes []change.TableChange) error {
	if len(changes) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin destination tx: %w", err)
	}

	for _, c := range changes {
		var applyErr error
		switch c.Operation {
		case "INSERT", "UPDATE":
			applyErr = applyUpsert(tx, c)
		case "DELETE":
			applyErr = applyDelete(tx, c)
		default:
			applyErr = fmt.Errorf("unknown operation %q", c.Operation)
		}
		if applyErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to apply %s on %s (record %v): %w", c.Operation, c.TableName, c.RecordID, applyErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit destination tx: %w", err)
	}
	return nil
}

func (r *MySQLReplicator) Close() error {
	return r.db.Close()
}

// applyUpsert relies on MySQL resolving ON DUPLICATE KEY UPDATE against the
// destination table's real (possibly composite) primary key - no special
// handling for composite keys is needed here.
//
// Table/column identifiers are not attacker-controlled: they come from
// go-mysql's own binlog metadata, and table names are already filtered
// against a fixed allowlist (table.GetTableNames()) before a TableChange is
// ever constructed. Backtick-quoting below is defense-in-depth /
// reserved-word safety, not injection prevention.
func applyUpsert(tx *sql.Tx, c change.TableChange) error {
	if len(c.NewValues) == 0 {
		return fmt.Errorf("no new values to apply")
	}

	cols := make([]string, 0, len(c.NewValues))
	for col := range c.NewValues {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	quotedCols := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	updateClauses := make([]string, len(cols))
	args := make([]interface{}, len(cols))
	for i, col := range cols {
		quotedCols[i] = "`" + col + "`"
		placeholders[i] = "?"
		updateClauses[i] = fmt.Sprintf("`%s` = VALUES(`%s`)", col, col)
		args[i] = c.NewValues[col]
	}

	query := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		c.TableName, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "), strings.Join(updateClauses, ", "),
	)
	_, err := tx.Exec(query, args...)
	return err
}

func applyDelete(tx *sql.Tx, c change.TableChange) error {
	pkCols := table.GetPrimaryKey(c.TableName)
	if len(pkCols) == 0 {
		return fmt.Errorf("no primary key mapping for table %s", c.TableName)
	}

	whereClauses := make([]string, len(pkCols))
	args := make([]interface{}, len(pkCols))
	for i, col := range pkCols {
		v, ok := c.RecordID[col]
		if !ok {
			return fmt.Errorf("delete missing PK column %q in RecordID", col)
		}
		whereClauses[i] = fmt.Sprintf("`%s` = ?", col)
		args[i] = v
	}

	query := fmt.Sprintf("DELETE FROM `%s` WHERE %s", c.TableName, strings.Join(whereClauses, " AND "))
	_, err := tx.Exec(query, args...)
	return err
}
