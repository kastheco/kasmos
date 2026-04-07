package taskstore

import (
	"database/sql"
	"sort"
	"strings"
)

// ListDistinctProjectsFromDB returns a sorted, deduplicated list of non-empty
// project names found in the shared SQLite DB by scanning the tasks and signals
// tables. It is a standalone helper (not a Store method) because it operates on
// the raw *sql.DB and carries schema knowledge that lives in this package.
func ListDistinctProjectsFromDB(db *sql.DB) ([]string, error) {
	seen := make(map[string]struct{})
	projects := make([]string, 0)

	collect := func(table string) error {
		// Guard against a DB that hasn't been migrated yet: skip missing tables
		// rather than returning an error.
		var exists int
		if err := db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?`,
			table,
		).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return nil
		}

		rows, err := db.Query(
			`SELECT DISTINCT project FROM ` + table + ` WHERE trim(project) <> '' ORDER BY project`,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var project string
			if err := rows.Scan(&project); err != nil {
				return err
			}
			project = strings.TrimSpace(project)
			if project == "" {
				continue
			}
			if _, ok := seen[project]; ok {
				continue
			}
			seen[project] = struct{}{}
			projects = append(projects, project)
		}
		return rows.Err()
	}

	for _, table := range []string{"tasks", "signals"} {
		if err := collect(table); err != nil {
			return nil, err
		}
	}

	sort.Strings(projects)
	return projects, nil
}
