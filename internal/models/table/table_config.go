package table

// TableConfig describes one production table under binlog monitoring.
type TableConfig struct {
	Name        string
	Description string
	// PrimaryKey lists the table's primary-key column names, in order, as
	// verified against information_schema. Several of these tables use
	// composite keys.
	PrimaryKey []string
}

var AllTables = []TableConfig{
	{"acctstat", "Account status", []string{"AcctStatID"}},
	{"accttype", "Account type", []string{"AccTypeID"}},
	{"amortsched", "Amortization schedule", []string{"BR_CODE", "CLIENTID", "SLC_CODE", "SLT_CODE", "REF_NO", "DUEDATE"}},
	{"ar", "Accounts receivable", []string{"ARBR_CODE", "ARSLC_CODE", "ARSLT_CODE", "ARREF_NO", "ClientIDAR"}},
	{"civilstat", "Civil status", []string{"CivilStatID"}},
	{"client", "Client information", []string{"ClientIDBrCode", "ClientID"}},
	{"clienttype", "Client type", []string{"ClientTypeID"}},
	{"dept", "Department", []string{"DeptID"}},
	{"gender", "Gender", []string{"GenderID"}},
	{"loan", "Loan records", []string{"LoanBR_CODE", "LoanSLC_CODE", "LoanSLT_CODE", "LoanREF_NO", "ClientIDLoan"}},
	{"sc", "Service charge", []string{"SCBR_CODE", "SCSLC_CODE", "SCSLT_CODE", "SCREF_NO", "ClientIDSC"}},
	{"sldtl", "Sub Ledger detail", []string{"SL_BRCODE", "SL_CLIENTID", "SLC_CODE", "SLT_CODE", "REF_NO", "SLE_CODE", "TR_DATE", "SEQNO"}},
	{"sltype", "Sub Ledger type", []string{"SLTypeBR_CODE", "SLTypeSLC_CODE", "SLTypeSLT_CODE"}},
	{"term", "Term", []string{"TermID"}},
	{"wkfcsoa", "Workflow case statement", []string{"BR_CODE", "CTRLNO", "SEQNO"}},
}

var byName map[string]TableConfig

func init() {
	byName = make(map[string]TableConfig, len(AllTables))
	for _, t := range AllTables {
		byName[t.Name] = t
	}
}

// GetTableNames returns just the table names, e.g. for filtering binlog events.
func GetTableNames() []string {
	names := make([]string, len(AllTables))
	for i, t := range AllTables {
		names[i] = t.Name
	}
	return names
}

// GetPrimaryKey returns the ordered primary-key column names for tableName,
// or nil if tableName isn't one of the monitored tables.
func GetPrimaryKey(tableName string) []string {
	t, ok := byName[tableName]
	if !ok {
		return nil
	}
	return t.PrimaryKey
}
