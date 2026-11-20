package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// restorableTables 是从备份快照恢复的表，按外键依赖排序（被引用的表在前）。
// master_password 和 settings 不恢复：保留当前的主密码、PIN 和偏好设置。
var restorableTables = []string{
	"notebooks",
	"tags",
	"smart_views",
	"todo_projects",
	"notes",
	"note_tags",
	"note_history",
	"attachments",
	"note_attachments",
	"todos",
	"todo_subtasks",
	"todo_note_links",
}

// SnapshotTo 把当前数据库生成一份一致的快照文件（VACUUM INTO）。目标文件不能已存在。
func (d *DB) SnapshotTo(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("snapshot target already exists: %s", path)
	}
	_, err := d.db.Exec(`VACUUM INTO ?`, path)
	return err
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// RestoreFromSnapshot 用备份快照替换各数据表的内容（主密码和设置除外）。
// 只复制两边都有的列；快照里没有的表保持不变。整个过程在一个事务里完成。
func (d *DB) RestoreFromSnapshot(snapshotPath string) (err error) {
	if _, statErr := os.Stat(snapshotPath); statErr != nil {
		return statErr
	}
	ctx := context.Background()
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS bak`, snapshotPath); err != nil {
		return err
	}
	defer func() {
		if _, detachErr := conn.ExecContext(ctx, `DETACH DATABASE bak`); detachErr != nil && err == nil {
			err = detachErr
		}
	}()

	columnsOf := func(schema, table string) ([]string, error) {
		rows, err := conn.QueryContext(ctx, `SELECT name FROM pragma_table_info(?, ?)`, table, schema)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			cols = append(cols, name)
		}
		return cols, rows.Err()
	}

	type plan struct {
		table string
		cols  []string
	}
	var plans []plan
	for _, table := range restorableTables {
		backupCols, err := columnsOf("bak", table)
		if err != nil {
			return err
		}
		if len(backupCols) == 0 {
			continue
		}
		mainCols, err := columnsOf("main", table)
		if err != nil {
			return err
		}
		inBackup := make(map[string]bool, len(backupCols))
		for _, c := range backupCols {
			inBackup[c] = true
		}
		var common []string
		for _, c := range mainCols {
			if inBackup[c] {
				common = append(common, quoteIdent(c))
			}
		}
		if len(common) == 0 {
			return errors.New("backup table has no compatible columns: " + table)
		}
		plans = append(plans, plan{table: table, cols: common})
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return err
	}
	// 先按依赖逆序清空，再按顺序写入
	for i := len(plans) - 1; i >= 0; i-- {
		if _, err = tx.ExecContext(ctx, `DELETE FROM main.`+quoteIdent(plans[i].table)); err != nil {
			return err
		}
	}
	for _, p := range plans {
		cols := strings.Join(p.cols, ", ")
		stmt := fmt.Sprintf(`INSERT INTO main.%s (%s) SELECT %s FROM bak.%s`, quoteIdent(p.table), cols, cols, quoteIdent(p.table))
		if _, err = tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}
