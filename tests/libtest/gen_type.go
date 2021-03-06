// +build ignore

package main

import (
	"bytes"
	"flag"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/SAP/go-ase/libase/flagslice"
)

var sTemplate = `package libtest

import (
{{ range $import, $_ := .Imports }}
	"{{ $import }}"
{{ end }}
)

// DoTest{{.ASEType}} tests the handling of the {{.ASEType}}.
func DoTest{{.ASEType}}(t *testing.T) {
	TestForEachDB("Test{{.ASEType}}", t, test{{.ASEType}})
	// {{ if .Null }}
	// TestForEachDB("TestNull{{.ASEType}}", t, testNull{{.ASEType}})
	// {{ end }}
}

func test{{.ASEType}}(t *testing.T, db *sql.DB, tableName string) {
	pass := make([]interface{}, len(samples{{.ASEType}}))
	mySamples := make([]{{.GoType}}, len(samples{{.ASEType}}))

	for i, sample := range samples{{.ASEType}} {
		{{ if .Convert }}
		// Convert sample with passed function before proceeding
		mySample, err := {{.Convert}}(sample)
		if err != nil {
			t.Errorf("Failed to convert sample %v: %v", sample, err)
			return
		}
		{{ else }}
		mySample := sample
		{{ end }}


		pass[i] = mySample
		mySamples[i] = mySample
	}

	rows, err := SetupTableInsert(db, tableName, "{{if .ColumnDef}}{{.ColumnDef}}{{else}}{{.ASETypeLower}}{{end}}", pass...)
	if err != nil {
		t.Errorf("Error preparing table: %v", err)
		return
	}
	defer rows.Close()

	i := 0
	var recv {{.GoType}}
	for rows.Next() {
		err = rows.Scan(&recv)
		if err != nil {
			t.Errorf("Scan failed on %dth scan: %v", i, err)
			continue
		}

		{{ if .Compare }}
		if {{.Compare}}(recv, mySamples[i]) {
		{{ else }}
		if recv != mySamples[i] {
		{{ end }}
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

{{ if .Null }}
func testNull{{.ASEType}}(t *testing.T, db *sql.DB, tableName string) {
	{{ if .Convert }}
	sample, err := {{.Convert}}(samples{{.ASEType}}[0])
	if err != nil {
		t.Errorf("Failed to convert sample %v: %v", samples{{.ASEType}}[0], err)
		return
	}
	{{ else }}
	sample := samples{{.ASEType}}[0]
	{{ end }}

	rows, err := SetupTableInsert(db, tableName,
		"{{if .ColumnDef}}{{.ColumnDef}} null{{else}}{{.ASETypeLower}} null{{end}}",
		sample, nil,
	)

	if err != nil {
		t.Errorf("Error preparing table: %v", err)
		return
	}
	defer rows.Close()

	for _, shouldBeNull := range []bool{false, true} {
		b := rows.Next()
		if b != true {
			t.Errorf("No rows to read")
			return
		}

		var recv {{.Null}}
		err = rows.Scan(&recv)
		if err != nil {
			t.Errorf("Failed to scan row value into {{.Null}}: %v", err)
			return
		}

		if shouldBeNull {
			// Test null
			if recv.Valid {
				t.Errorf("Scanned value is valid but shouldn't be")
				return
			}
		} else {
			// Test value
			if !recv.Valid {
				t.Errorf("Scanned value is not valid but should be")
				continue
			}

			val, err := recv.Value()
			if err != nil {
				t.Errorf("Failed to retrieve value from {{.Null}}: %#v", val)
				continue
			}

			if val == nil {
				t.Errorf("Returned value should be %v but is nil", samples{{.ASEType}}[0])
				continue
			}

			{{ if .Compare }}
			if {{.Compare}}(val.({{.GoType}}), sample) {
			{{ else }}
			if val.({{.GoType}}) != sample {
			{{ end }}
				t.Errorf("Entered value and retrieved value is not equal: %v != %v",
					val, samples{{.ASEType}}[0])
			}
		}
	}
}
{{ end }}
`

type data struct {
	ASEType, ASETypeLower string
	GoType                string
	Imports               map[string]bool
	ColumnDef             string
	Compare               string
	Null                  string
	Convert               string
}

func main() {
	args := os.Args[1:]

	if len(args) < 2 {
		log.Printf("Expected exactly two arguments - ASE type and Go type, got: %#v", args)
		os.Exit(1)
	}

	flagset := flag.NewFlagSet("", flag.ContinueOnError)

	fColumnDef := flagset.String("columndef", "", "Column definition")
	fCompare := flagset.String("compare", "", "Function name to use to compare received and expected values")
	fNull := flagset.String("null", "", "Null type to test against")
	fConvert := flagset.String("convert", "", "Function to convert samples before used in testing")

	fImports := &flagslice.FlagStringSlice{}
	flagset.Var(fImports, "import", "Additional packages to import")

	err := flagset.Parse(args[2:])
	if err != nil {
		log.Printf("Failed to parse parameters: %v", err)
		os.Exit(1)
	}

	d := data{
		ASEType:      args[0],
		ASETypeLower: strings.ToLower(args[0]),
		GoType:       "",
		Imports: map[string]bool{
			"database/sql": true,
			"testing":      true,
		},
		ColumnDef: *fColumnDef,
		Compare:   "",
		Null:      "",
		Convert:   "",
	}

	for _, imp := range fImports.Slice() {
		d.Imports[imp] = true
	}

	goTypeImport, goType := splitType(args[1])
	if len(goTypeImport) > 0 {
		d.Imports[goTypeImport] = true
	}
	d.GoType = goType

	// -null "github.com/SAP/go-ase/libase/types.NullTime"
	// Split up to type "types.NullTime" and import "github.com/SAP/go-ase/libase/types"
	if len(*fNull) > 0 {
		nullTypeImport, nullType := splitType(*fNull)
		if len(nullTypeImport) > 0 {
			d.Imports[nullTypeImport] = true
		}
		d.Null = nullType
	}

	if len(*fConvert) > 0 {
		convertImport, convertFunc := splitType(*fConvert)
		if len(convertImport) > 0 {
			d.Imports[convertImport] = true
		}
		d.Convert = convertFunc
	}

	if len(*fCompare) > 0 {
		compareImport, compareFunc := splitType(*fCompare)
		if len(compareImport) > 0 {
			d.Imports[compareImport] = true
		}
		d.Compare = compareFunc
	}

	tmpl, err := template.New("").Parse(sTemplate)
	if err != nil {
		log.Printf("Failed to parse template: %v", err)
		os.Exit(1)
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, d)
	if err != nil {
		log.Printf("Failed to execute template with data '%v': %v", d, err)
		os.Exit(1)
	}

	formattedBuf, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Formatting code failed: %v", err)
		os.Exit(1)
	}

	outfileName := "type_" + d.ASETypeLower + ".go"
	err = ioutil.WriteFile(outfileName, formattedBuf, 0644)
	if err != nil {
		log.Printf("Failed to write executed template to '%s': %v", outfileName, err)
		os.Exit(1)
	}
}

// splitType separates a fully qualified go type into the type reference
// ($package.$name) and the import path ($domain.$tld/$package_path).
//
// Example:
//	splitType("github.com/SAP/go-ase/libase/types.NullTime")
//	-> "github.com/SAP/go-ase/libase/types", "types.NullTime"
//	splitType("sql.NullInt64")
//	-> "sql", "sql.NullInt64"
//	splitType("github.com/SAP/go-ase/libase/*types.Decimal")
//	-> "github.com/SAP/go-ase/libase/types", "*types.Decimal"
func splitType(input string) (string, string) {
	if !strings.Contains(input, ".") {
		// Return immediately on e.g. []byte
		return "", input
	}

	inputS := strings.Split(input, "/")

	// goType -> "types.NullTime"
	goType := inputS[len(inputS)-1]

	// imp -> "types"
	imp := strings.Split(goType, ".")[0]
	if strings.Contains(input, "/") {
		// imp -> github.com/SAP/go-ase/libase/types
		imp = strings.Join(
			[]string{
				strings.Join(inputS[0:len(inputS)-1], "/"),
				imp,
			},
			"/",
		)
	}

	// Remove asterisk from pointer types, e.g.
	// github.com/SAP/go-ase/libase/*types.Decimal
	imp = strings.Replace(imp, "*", "", 1)

	return imp, goType
}
