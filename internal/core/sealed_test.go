package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"locknote/internal/sealedfile"
)

func TestSealAndOpenThroughCore(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "ABCDEFGHJKMNPQRS"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := c.SealTextFile(src, true)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := c.OpenSealedFile(result.OutputPath)
	if err != nil || doc.Content != "secret" {
		t.Fatalf("open file = %v, %v", doc, err)
	}
	data, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = c.OpenSealedData(data)
	if err != nil || doc.Content != "secret" {
		t.Fatalf("open data = %v, %v", doc, err)
	}
	doc, err = c.OpenSealedDataWithRecoveryKey(data, "ABCDEFGHJKMNPQRS")
	if err != nil || doc.Content != "secret" {
		t.Fatalf("open with recovery key = %v, %v", doc, err)
	}
	if _, err := c.OpenSealedDataWithRecoveryKey(data, "23456789ABCDEFGH"); !errors.Is(err, sealedfile.ErrWrongKey) {
		t.Fatalf("wrong recovery key err = %v", err)
	}
	note, err := c.ImportSealedDocument(doc)
	if err != nil || note.Content != "secret" {
		t.Fatalf("import = %v, %v", note, err)
	}

	c.Lock()
	if _, err := c.OpenSealedData(data); err == nil {
		t.Fatal("opening while locked should fail")
	}
}
