package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"locknote/internal/attachments"
	"locknote/internal/sealedfile"
)

func TestSealAndOpenThroughCore(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
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
	doc, err = c.OpenSealedDataWithRecoveryKey(data, "K7MQ2X9P4R3WHT6N")
	if err != nil || doc.Content != "secret" {
		t.Fatalf("open with recovery key = %v, %v", doc, err)
	}
	if _, err := c.OpenSealedDataWithRecoveryKey(data, "W4R8N2QX7K9M3TPZ"); !errors.Is(err, sealedfile.ErrWrongKey) {
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

func TestExportNoteSealed(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
		t.Fatal(err)
	}
	note, err := c.Notes().Create("账号", "user: alice")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "账号.naisho")
	if missing, err := c.ExportNoteSealed(note.ID, out); err != nil || missing != 0 {
		t.Fatal(missing, err)
	}
	doc, err := c.OpenSealedFile(out)
	if err != nil || doc.Title != "账号" || doc.Content != "user: alice" {
		t.Fatalf("exported doc = %+v, %v", doc, err)
	}
}

func TestExportAndImportNoteWithImages(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
		t.Fatal(err)
	}
	imageData := []byte("\x89PNG\r\n\x1a\nimage-bytes")
	img, err := c.Attachments().CreateImage(attachments.CreateImageInput{OriginalName: "截图.png", MIMEType: "image/png", Data: imageData})
	if err != nil {
		t.Fatal(err)
	}
	content := "看图 ![截图](locknote-attachment://" + img.ID + ") 和 ![丢失](locknote-attachment://missing-id)"
	note, err := c.Notes().Create("带图片", content)
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "带图片.naisho")
	missing, err := c.ExportNoteSealed(note.ID, out)
	if err != nil || missing != 1 {
		t.Fatalf("export = %d, %v; want 1 missing image", missing, err)
	}
	doc, err := c.OpenSealedFile(out)
	if err != nil || len(doc.Attachments) != 1 || !bytes.Equal(doc.Attachments[0].Data, imageData) {
		t.Fatalf("opened doc = %+v, %v", doc, err)
	}

	imported, err := c.ImportSealedDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(imported.Content, img.ID) {
		t.Fatal("imported note should reference a new attachment, not the original one")
	}
	refs := attachmentRefs(imported.Content)
	if len(refs) != 2 || refs[1] != "missing-id" {
		t.Fatalf("refs = %v", refs)
	}
	_, data, err := c.Attachments().GetData(refs[0])
	if err != nil || !bytes.Equal(data, imageData) {
		t.Fatalf("imported image = %v, %v", len(data), err)
	}
}
