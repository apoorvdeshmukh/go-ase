package libtest

import (
	"database/sql"

	"testing"

	"time"
)

// DoTestDateTime tests the handling of the DateTime.
func DoTestDateTime(t *testing.T) {
	TestForEachDB("TestDateTime", t, testDateTime)
	//
}

func testDateTime(t *testing.T, db *sql.DB, tableName string) {
	pass := make([]interface{}, len(samplesDateTime))
	mySamples := make([]time.Time, len(samplesDateTime))

	for i, sample := range samplesDateTime {

		mySample := sample

		pass[i] = mySample
		mySamples[i] = mySample
	}

	rows, err := SetupTableInsert(db, tableName, "datetime", pass...)
	if err != nil {
		t.Errorf("Error preparing table: %v", err)
		return
	}
	defer rows.Close()

	i := 0
	var recv time.Time
	for rows.Next() {
		err = rows.Scan(&recv)
		if err != nil {
			t.Errorf("Scan failed on %dth scan: %v", i, err)
			continue
		}

		if recv != mySamples[i] {

			t.Errorf("Received value does not match passed parameter")
			t.Errorf("Expected: %v", mySamples[i])
			t.Errorf("Received: %v", recv)
		}

		i++
	}

	if err := rows.Err(); err != nil {
		t.Errorf("Error preparing rows: %v", err)
	}
}
