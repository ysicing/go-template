package model

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWebAuthnCredential_SQLiteBinaryColumnsUseBlob(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm sqlite dialector: %v", err)
	}

	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&WebAuthnCredential{}); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	binaryFields := []string{"CredentialID", "PublicKey", "AAGUID"}
	for _, fieldName := range binaryFields {
		field := stmt.Schema.LookUpField(fieldName)
		if field == nil {
			t.Fatalf("field %s not found in schema", fieldName)
		}

		dataType := strings.ToLower(db.Dialector.DataTypeOf(field))
		if dataType != "blob" {
			t.Fatalf("expected %s to map to sqlite blob, got %q", fieldName, dataType)
		}
	}
}
