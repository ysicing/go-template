package model

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWebAuthnCredential_PostgresBinaryColumnsUseBytea(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=postgres sslmode=disable",
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm postgres dialector: %v", err)
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
		if dataType != "bytea" {
			t.Fatalf("expected %s to map to postgres bytea, got %q", fieldName, dataType)
		}
	}
}
