package core

import (
	"path/filepath"
	"testing"

	"locknote/internal/todos"
)

func TestFullBackupRestoresDatabaseAndKeepsCurrentPassword(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "ABCDEFGHJKMNPQRS"); err != nil {
		t.Fatal(err)
	}
	note, err := c.Notes().Create("备份笔记", "正文")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Todos().Create(todos.CreateTodoInput{Title: "买牛奶", Priority: "high"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tags().Create("工作", "#10b981"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notebooks().Create("项目", "📓"); err != nil {
		t.Fatal(err)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := c.CreateBackup(backupPath); err != nil {
		t.Fatal(err)
	}

	// 备份之后：改密码、删除数据、新增数据
	if err := c.ChangePassword("master-pass", "changed-pass", ""); err != nil {
		t.Fatal(err)
	}
	list, _ := c.Todos().List()
	for _, todo := range list {
		_ = c.Todos().Delete(todo.ID)
	}
	tagList, _ := c.Tags().List()
	for _, tag := range tagList {
		_ = c.Tags().Delete(tag.ID)
	}
	if _, err := c.Notes().Create("备份后新建", "x"); err != nil {
		t.Fatal(err)
	}

	if err := c.RestoreBackup(backupPath); err != nil {
		t.Fatal(err)
	}

	restoredTodos, err := c.Todos().List()
	if err != nil || len(restoredTodos) != 1 || restoredTodos[0].Title != "买牛奶" {
		t.Fatalf("todos after restore = %+v, %v", restoredTodos, err)
	}
	restoredTags, err := c.Tags().List()
	if err != nil || len(restoredTags) != 1 || restoredTags[0].Name != "工作" {
		t.Fatalf("tags after restore = %+v, %v", restoredTags, err)
	}
	restoredBooks, err := c.Notebooks().List()
	if err != nil || len(restoredBooks) != 1 || restoredBooks[0].Name != "项目" {
		t.Fatalf("notebooks after restore = %+v, %v", restoredBooks, err)
	}
	notesList, err := c.Notes().List()
	if err != nil || len(notesList) != 1 || notesList[0].ID != note.ID {
		t.Fatalf("notes after restore = %+v, %v", notesList, err)
	}

	// 主密码保持为恢复前的当前密码
	c.Lock()
	if ok, _ := c.Unlock("master-pass"); ok {
		t.Fatal("old password from the backup must not be restored")
	}
	if ok, err := c.Unlock("changed-pass"); !ok || err != nil {
		t.Fatalf("current password should still unlock: %v, %v", ok, err)
	}
}
