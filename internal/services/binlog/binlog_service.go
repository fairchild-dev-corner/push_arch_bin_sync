package binlog

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	mysqldriver "github.com/go-sql-driver/mysql"

	"push_arch_bin_sync/internal/models/change"
	"push_arch_bin_sync/internal/models/table"
	"push_arch_bin_sync/internal/utils"
)

// BinaryLogMonitor monitors MySQL binary logs for changes to a fixed set of tables.
type BinaryLogMonitor struct {
	host       string
	port       uint16
	user       string
	password   string
	tableNames []string
	cfg        replication.BinlogSyncerConfig
	syncer     *replication.BinlogSyncer
	streamer   *replication.BinlogStreamer
	lastPos    mysql.Position
}

// NewBinaryLogMonitor creates a new binary log monitor. It starts from the
// source's current binlog position (not the oldest retained file), since
// this daemon is a forward-only live sync, not a historical backfill tool.
func NewBinaryLogMonitor(host string, port uint16, user, password string, tables []string) (*BinaryLogMonitor, error) {
	startPos, err := currentMasterPosition(host, port, user, password)

	if err != nil {
		return nil, fmt.Errorf("failed to determine starting binlog position: %w", err)
	}

	cfg := replication.BinlogSyncerConfig{
		ServerID: 100, // Unique ID for this monitor
		Flavor:   "mysql",
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}

	syncer := replication.NewBinlogSyncer(cfg)

	blm := &BinaryLogMonitor{
		host:       host,
		port:       port,
		user:       user,
		password:   password,
		tableNames: tables,
		cfg:        cfg,
		syncer:     syncer,
		lastPos:    startPos,
	}

	log.Printf("✅ Binary log syncer created, starting from %s:%d", startPos.Name, startPos.Pos)
	return blm, nil
}

// currentMasterPosition queries the source for its current binlog file/
// position via a plain SQL connection (separate from the replication
// protocol connection the syncer itself uses).
func currentMasterPosition(host string, port uint16, user, password string) (mysql.Position, error) {
	driverCfg := mysqldriver.NewConfig()
	driverCfg.User = user
	driverCfg.Passwd = password
	driverCfg.Net = "tcp"
	driverCfg.Addr = fmt.Sprintf("%s:%d", host, port)

	db, err := sql.Open("mysql", driverCfg.FormatDSN())
	if err != nil {
		return mysql.Position{}, fmt.Errorf("failed to open source connection: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SHOW MASTER STATUS")
	if err != nil {
		return mysql.Position{}, fmt.Errorf("failed to run SHOW MASTER STATUS: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return mysql.Position{}, fmt.Errorf("failed to read SHOW MASTER STATUS columns: %w", err)
	}
	if !rows.Next() {
		return mysql.Position{}, fmt.Errorf("SHOW MASTER STATUS returned no rows - is binary logging enabled on the source?")
	}

	values := make([]sql.RawBytes, len(cols))
	scanArgs := make([]interface{}, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	if err := rows.Scan(scanArgs...); err != nil {
		return mysql.Position{}, fmt.Errorf("failed to scan SHOW MASTER STATUS row: %w", err)
	}

	file := string(values[0])
	pos, err := strconv.ParseUint(string(values[1]), 10, 32)
	if err != nil {
		return mysql.Position{}, fmt.Errorf("failed to parse binlog position %q: %w", string(values[1]), err)
	}

	return mysql.Position{Name: file, Pos: uint32(pos)}, nil
}

// MonitorBinaryLogs collects binary log changes for up to 5 seconds (or until
// ctx is done, whichever comes first) and returns what it gathered.
func (blm *BinaryLogMonitor) MonitorBinaryLogs(ctx context.Context) ([]change.TableChange, error) {
	if err := blm.ensureStreamer(); err != nil {
		return nil, err
	}

	batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var changes []change.TableChange
	for {
		ev, err := blm.streamer.GetEvent(batchCtx)
		if err != nil {
			if err == context.DeadlineExceeded {
				return changes, nil
			}
			// Any other error means the connection/stream itself is broken
			// (network blip, MySQL restart, etc), not just "no events right
			// now". Tear it down so the *next* call reconnects from
			// blm.lastPos instead of repeatedly calling GetEvent on a dead
			// stream forever.
			blm.resetConnection()
			return changes, err
		}

		// Track our offset within the current binlog file for the next sync.
		blm.lastPos.Pos = ev.Header.LogPos

		changes = append(changes, blm.parseEvent(ev)...)
	}
}

// ensureStreamer (re)establishes the replication connection if needed,
// resuming from blm.lastPos.
func (blm *BinaryLogMonitor) ensureStreamer() error {
	if blm.streamer != nil {
		return nil
	}

	if blm.syncer == nil {
		blm.syncer = replication.NewBinlogSyncer(blm.cfg)
	}

	streamer, err := blm.syncer.StartSync(blm.lastPos)
	if err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}
	blm.streamer = streamer
	return nil
}

// resetConnection tears down the current syncer/streamer after a connection
// failure so ensureStreamer reconnects fresh on the next call.
func (blm *BinaryLogMonitor) resetConnection() {
	if blm.syncer != nil {
		blm.syncer.Close()
	}
	blm.syncer = nil
	blm.streamer = nil
}

// parseEvent parses a binary log event.
func (blm *BinaryLogMonitor) parseEvent(ev *replication.BinlogEvent) []change.TableChange {
	var changes []change.TableChange

	switch ev.Header.EventType {
	case replication.WRITE_ROWS_EVENTv2, replication.WRITE_ROWS_EVENTv1:
		changes = blm.parseWriteEvent(ev, "INSERT")

	case replication.UPDATE_ROWS_EVENTv2, replication.UPDATE_ROWS_EVENTv1:
		changes = blm.parseUpdateEvent(ev, "UPDATE")

	case replication.DELETE_ROWS_EVENTv2, replication.DELETE_ROWS_EVENTv1:
		changes = blm.parseDeleteEvent(ev, "DELETE")

	case replication.ROTATE_EVENT:
		if rotate, ok := ev.Event.(*replication.RotateEvent); ok {
			blm.lastPos.Name = string(rotate.NextLogName)
		}

	case replication.TABLE_MAP_EVENT:
		log.Printf("📋 Table map event received")
	}

	return changes
}

// parseWriteEvent parses INSERT events.
func (blm *BinaryLogMonitor) parseWriteEvent(ev *replication.BinlogEvent, operation string) []change.TableChange {
	var changes []change.TableChange

	event, ok := ev.Event.(*replication.RowsEvent)
	if !ok {
		utils.LogError("❌ Failed to parse write event: unexpected event type")
		return changes
	}

	tableName := string(event.Table.Table)
	dbName := string(event.Table.Schema)

	if !blm.isTableMonitored(tableName) {
		return changes
	}

	log.Printf("INSERT detected in %s.%s", dbName, tableName)

	for _, row := range event.Rows {
		values := blm.rowToMap(event, row)

		c := change.TableChange{
			TableName: tableName,
			Database:  dbName,
			Operation: operation,
			NewValues: values,
			Timestamp: int64(ev.Header.Timestamp),
		}

		c.RecordID = blm.buildRecordID(tableName, values)

		changes = append(changes, c)
	}

	return changes
}

// parseUpdateEvent parses UPDATE events.
func (blm *BinaryLogMonitor) parseUpdateEvent(ev *replication.BinlogEvent, operation string) []change.TableChange {
	var changes []change.TableChange

	event, ok := ev.Event.(*replication.RowsEvent)
	if !ok {
		utils.LogError("Failed to parse update event: unexpected event type")
		return changes
	}

	tableName := string(event.Table.Table)
	dbName := string(event.Table.Schema)

	if !blm.isTableMonitored(tableName) {
		return changes
	}

	log.Printf("UPDATE detected in %s.%s", dbName, tableName)

	// Parse update: pairs of (old_row, new_row)
	for i := 0; i < len(event.Rows); i += 2 {
		if i+1 >= len(event.Rows) {
			break
		}

		oldValues := blm.rowToMap(event, event.Rows[i])
		newValues := blm.rowToMap(event, event.Rows[i+1])

		c := change.TableChange{
			TableName: tableName,
			Database:  dbName,
			Operation: operation,
			OldValues: oldValues,
			NewValues: newValues,
			Timestamp: int64(ev.Header.Timestamp),
		}

		c.RecordID = blm.buildRecordID(tableName, newValues)

		changes = append(changes, c)
	}

	return changes
}

// parseDeleteEvent parses DELETE events.
func (blm *BinaryLogMonitor) parseDeleteEvent(ev *replication.BinlogEvent, operation string) []change.TableChange {
	var changes []change.TableChange

	event, ok := ev.Event.(*replication.RowsEvent)
	if !ok {
		utils.LogError("Failed to parse delete event: unexpected event type")
		return changes
	}

	tableName := string(event.Table.Table)
	dbName := string(event.Table.Schema)

	if !blm.isTableMonitored(tableName) {
		return changes
	}

	log.Printf("DELETE detected in %s.%s", dbName, tableName)

	for _, row := range event.Rows {
		values := blm.rowToMap(event, row)

		c := change.TableChange{
			TableName: tableName,
			Database:  dbName,
			Operation: operation,
			OldValues: values,
			Timestamp: int64(ev.Header.Timestamp),
		}

		c.RecordID = blm.buildRecordID(tableName, values)

		changes = append(changes, c)
	}

	return changes
}

// rowToMap converts a row to a map keyed by column name.
func (blm *BinaryLogMonitor) rowToMap(event *replication.RowsEvent, row []interface{}) map[string]interface{} {
	values := make(map[string]interface{})

	columnNames := event.Table.ColumnNameString()
	for i, v := range row {
		if i < len(columnNames) {
			values[columnNames[i]] = v
		}
	}

	return values
}

// buildRecordID resolves a table's primary-key values out of a decoded row.
// Pass NewValues for insert/update, or the deleted row's values for delete.
func (blm *BinaryLogMonitor) buildRecordID(tableName string, values map[string]interface{}) map[string]interface{} {
	pkCols := table.GetPrimaryKey(tableName)
	if len(pkCols) == 0 {
		utils.LogError("⚠️ no primary key mapping for table %s; skipping RecordID", tableName)
		return nil
	}

	id := make(map[string]interface{}, len(pkCols))
	for _, col := range pkCols {
		v, ok := values[col]
		if !ok {
			utils.LogError("table %s: PK column %q not found in decoded row values", tableName, col)
			continue
		}
		id[col] = v
	}
	return id
}

// isTableMonitored checks if a table is in our monitored set.
func (blm *BinaryLogMonitor) isTableMonitored(tableName string) bool {
	for _, name := range blm.tableNames {
		if name == tableName {
			return true
		}
	}
	return false
}

// Close closes the syncer.
func (blm *BinaryLogMonitor) Close() error {
	if blm.syncer != nil {
		blm.syncer.Close()
	}
	return nil
}
