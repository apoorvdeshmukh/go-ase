package libtest

import (
	"database/sql"

	"testing"
)

// DoTestBinary tests the handling of the Binary.
func DoTestBinary(t *testing.T) {
	TestForEachDB("TestBinary", t, testBinary)
	//
}

func testBinary(t *testing.T, db *sql.DB, tableName string) {
	pass := make([]interface{}, len(samplesBinary))
	mySamples := make([][]byte, len(samplesBinary))

	for i, sample := range samplesBinary {

		mySample := sample

		pass[i] = mySample
		mySamples[i] = mySample
	}

	rows, err := SetupTableInsert(db, tableName, "binary(13)", pass...)
	if err != nil {
		t.Errorf("Error preparing table: %v", err)
		return
	}
	defer rows.Close()

	i := 0
	var recv []byte
	for rows.Next() {
		err = rows.Scan(&recv)
		if err != nil {
			t.Errorf("Scan failed on %dth scan: %v", i, err)
			continue
		}

		if compareBinary(recv, mySamples[i]) {

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
