package change

// TableChange represents a single row-level change captured from the
// MySQL binary log.
type TableChange struct {
	TableName string                 `json:"table_name"`
	Operation string                 `json:"operation"` // INSERT, UPDATE, DELETE
	RecordID  map[string]interface{} `json:"record_id"`
	OldValues map[string]interface{} `json:"old_values,omitempty"`
	NewValues map[string]interface{} `json:"new_values"`
	Timestamp int64                  `json:"timestamp"`
	Database  string                 `json:"database"`
}
