package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/breadcrumbs"
)

// MySQLRepository keeps every breadcrumb read and write inside one DB transaction;
// the service uses the generic keyed executor so nested-set shifts, inserts,
// deletes, and ancestor checks are atomic.
type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

var _ breadcrumbs.Repository = (*MySQLRepository)(nil)

func (repository *MySQLRepository) Transaction(ctx context.Context, fn func(context.Context, breadcrumbs.Querier) error) error {
	if repository == nil || repository.db == nil {
		return breadcrumbs.ErrServiceNotReady
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin breadcrumb transaction: %w", err)
	}
	adapter := &transactionQuerier{tx: tx}
	if err := fn(ctx, adapter); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit breadcrumb transaction: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) List(ctx context.Context, listQuery breadcrumbs.ListQuery) (breadcrumbs.Page, error) {
	if repository == nil || repository.db == nil {
		return breadcrumbs.Page{}, breadcrumbs.ErrServiceNotReady
	}
	var total int64
	if err := repository.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cch_breadcrumblinks").Scan(&total); err != nil {
		return breadcrumbs.Page{}, fmt.Errorf("count breadcrumbs: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, "SELECT Page_Title, Page_URL, Lft, Rgt FROM cch_breadcrumblinks ORDER BY Lft ASC LIMIT ? OFFSET ?", listQuery.Limit, listQuery.Offset)
	if err != nil {
		return breadcrumbs.Page{}, fmt.Errorf("list breadcrumbs: %w", err)
	}
	defer rows.Close()
	var data []breadcrumbs.Breadcrumb
	for rows.Next() {
		var node breadcrumbs.Breadcrumb
		if err := rows.Scan(&node.PageTitle, &node.PageURL, &node.Left, &node.Right); err != nil {
			return breadcrumbs.Page{}, fmt.Errorf("scan breadcrumb: %w", err)
		}
		data = append(data, node)
	}
	if err := rows.Err(); err != nil {
		return breadcrumbs.Page{}, fmt.Errorf("iterate breadcrumbs: %w", err)
	}
	return breadcrumbs.Page{Data: data, Total: total, Limit: listQuery.Limit, Offset: listQuery.Offset}, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, pageTitle string) (breadcrumbs.Breadcrumb, error) {
	if repository == nil || repository.db == nil {
		return breadcrumbs.Breadcrumb{}, breadcrumbs.ErrServiceNotReady
	}
	var node breadcrumbs.Breadcrumb
	err := repository.db.QueryRowContext(ctx, "SELECT Page_Title, Page_URL, Lft, Rgt FROM cch_breadcrumblinks WHERE Page_Title = ? LIMIT 1", pageTitle).
		Scan(&node.PageTitle, &node.PageURL, &node.Left, &node.Right)
	if errors.Is(err, sql.ErrNoRows) {
		return breadcrumbs.Breadcrumb{}, breadcrumbs.ErrNotFound
	}
	if err != nil {
		return breadcrumbs.Breadcrumb{}, fmt.Errorf("get breadcrumb: %w", err)
	}
	return node, nil
}

func (repository *MySQLRepository) Ancestors(ctx context.Context, pageTitle string) ([]breadcrumbs.Ancestor, error) {
	if repository == nil || repository.db == nil {
		return nil, breadcrumbs.ErrServiceNotReady
	}
	rows, err := repository.db.QueryContext(ctx, `
SELECT C.Page_Title, C.Page_URL
FROM cch_breadcrumblinks AS B
JOIN cch_breadcrumblinks AS C ON B.Lft BETWEEN C.Lft AND C.Rgt
WHERE B.Page_Title = ?
ORDER BY C.Lft ASC`, pageTitle)
	if err != nil {
		return nil, fmt.Errorf("ancestors breadcrumbs: %w", err)
	}
	defer rows.Close()
	var ancestors []breadcrumbs.Ancestor
	for rows.Next() {
		var ancestor breadcrumbs.Ancestor
		if err := rows.Scan(&ancestor.PageTitle, &ancestor.PageURL); err != nil {
			return nil, fmt.Errorf("scan ancestor: %w", err)
		}
		ancestors = append(ancestors, ancestor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ancestors: %w", err)
	}
	return ancestors, nil
}

type transactionQuerier struct {
	tx *sql.Tx
}

func (query *transactionQuerier) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := query.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cch_breadcrumblinks").Scan(&total); err != nil {
		return 0, fmt.Errorf("count breadcrumbs: %w", err)
	}
	return total, nil
}

func (query *transactionQuerier) Exists(ctx context.Context, pageTitle string) (bool, error) {
	var exists bool
	if err := query.tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM cch_breadcrumblinks WHERE Page_Title = ?)", pageTitle).Scan(&exists); err != nil {
		return false, fmt.Errorf("exists breadcrumb: %w", err)
	}
	return exists, nil
}

func (query *transactionQuerier) Get(ctx context.Context, pageTitle string) (breadcrumbs.Breadcrumb, error) {
	var node breadcrumbs.Breadcrumb
	err := query.tx.QueryRowContext(ctx, "SELECT Page_Title, Page_URL, Lft, Rgt FROM cch_breadcrumblinks WHERE Page_Title = ? LIMIT 1", pageTitle).
		Scan(&node.PageTitle, &node.PageURL, &node.Left, &node.Right)
	if errors.Is(err, sql.ErrNoRows) {
		return breadcrumbs.Breadcrumb{}, breadcrumbs.ErrNotFound
	}
	if err != nil {
		return breadcrumbs.Breadcrumb{}, fmt.Errorf("get breadcrumb: %w", err)
	}
	return node, nil
}

func (query *transactionQuerier) Add(ctx context.Context, pageTitle, pageURL, _ string, left, right int) error {
	if left == 1 && right == 2 {
		// First node becomes the root; there is nothing to shift.
		if _, err := query.tx.ExecContext(ctx, "INSERT INTO cch_breadcrumblinks (Page_Title, Page_URL, Lft, Rgt) VALUES (?, ?, ?, ?)", pageTitle, pageURL, left, right); err != nil {
			return fmt.Errorf("insert breadcrumb root: %w", err)
		}
		return nil
	}
	// Insert a child immediately after the parent's right edge. Rows at or after
	// `left` move right by 2 for their left value; only rows strictly after `left`
	// move right by 2 for their right value, which widens the parent's range.
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft + 2 WHERE Lft >= ?", left); err != nil {
		return fmt.Errorf("shift breadcrumb lefts: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Rgt = Rgt + 2 WHERE Rgt > ?", left); err != nil {
		return fmt.Errorf("shift breadcrumb rights: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "INSERT INTO cch_breadcrumblinks (Page_Title, Page_URL, Lft, Rgt) VALUES (?, ?, ?, ?)", pageTitle, pageURL, left, right); err != nil {
		return fmt.Errorf("insert breadcrumb: %w", err)
	}
	return nil
}

func (query *transactionQuerier) Move(ctx context.Context, currentRoot, newParent string) error {
	subtree, err := query.Get(ctx, currentRoot)
	if err != nil {
		return err
	}
	parent, err := query.Get(ctx, newParent)
	if err != nil {
		return err
	}
	distance := subtree.Right - subtree.Left + 1
	targetLft := parent.Right
	if subtree.Left >= targetLft {
		// Moving the subtree right: first detach by deleting the subtree range,
		// close the gap, then shift the range after targetLft right by distance.
		if _, err := query.tx.ExecContext(ctx, "DELETE FROM cch_breadcrumblinks WHERE Page_Title = ?", currentRoot); err != nil {
			return fmt.Errorf("detach breadcrumb subtree: %w", err)
		}
		if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft - ?, Rgt = Rgt - ? WHERE Lft >= ?", distance, distance, subtree.Left); err != nil {
			return fmt.Errorf("close breadcrumb gap: %w", err)
		}
		if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft + ?, Rgt = Rgt + ? WHERE Lft >= ?", distance, distance, targetLft); err != nil {
			return fmt.Errorf("shift breadcrumb tail: %w", err)
		}
		if _, err := query.tx.ExecContext(ctx, "INSERT INTO cch_breadcrumblinks (Page_Title, Page_URL, Lft, Rgt) SELECT Page_Title, Page_URL, Lft + ?, Rgt + ? FROM cch_breadcrumblinks WHERE Page_Title = ?", distance, distance, currentRoot); err != nil {
			return fmt.Errorf("reinsert breadcrumb subtree: %w", err)
		}
		return nil
	}
	// Moving the subtree left: first detach, close the gap, then shift the range
	// after the target left right by distance.
	if _, err := query.tx.ExecContext(ctx, "DELETE FROM cch_breadcrumblinks WHERE Page_Title = ?", currentRoot); err != nil {
		return fmt.Errorf("detach breadcrumb subtree: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft - ?, Rgt = Rgt - ? WHERE Lft >= ?", distance, distance, subtree.Left); err != nil {
		return fmt.Errorf("close breadcrumb gap: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft + ?, Rgt = Rgt + ? WHERE Lft >= ?", distance, distance, targetLft); err != nil {
		return fmt.Errorf("shift breadcrumb tail: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft + ?, Rgt = Rgt + ? WHERE Page_Title = ?", distance, distance, currentRoot); err != nil {
		return fmt.Errorf("reposition breadcrumb subtree: %w", err)
	}
	return nil
}

func (query *transactionQuerier) Delete(ctx context.Context, pageTitle string) error {
	node, err := query.Get(ctx, pageTitle)
	if err != nil {
		return err
	}
	distance := node.Right - node.Left + 1
	if _, err := query.tx.ExecContext(ctx, "DELETE FROM cch_breadcrumblinks WHERE Lft BETWEEN ? AND ?", node.Left, node.Right); err != nil {
		return fmt.Errorf("delete breadcrumb subtree: %w", err)
	}
	if _, err := query.tx.ExecContext(ctx, "UPDATE cch_breadcrumblinks SET Lft = Lft - ?, Rgt = Rgt - ? WHERE Lft > ?", distance, distance, node.Right); err != nil {
		return fmt.Errorf("close breadcrumb gap: %w", err)
	}
	return nil
}
