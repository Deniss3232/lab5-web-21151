package storage

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

type Serie struct {
	ID             int
	Name           string
	CurrentEpisode int
	TotalEpisodes  int
}

func OpenDB(path string) (*sql.DB, error) {
	//  driver 
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS series (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			current_episode INTEGER NOT NULL,
			total_episodes INTEGER NOT NULL
		);
	`)
	return err
}

func ListSeries(db *sql.DB) ([]Serie, error) {
	rows, err := db.Query(`SELECT id, name, current_episode, total_episodes FROM series ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() 

	var out []Serie
	for rows.Next() {
		var s Serie
		if err := rows.Scan(&s.ID, &s.Name, &s.CurrentEpisode, &s.TotalEpisodes); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func InsertSerie(db *sql.DB, name string, currentEp, totalEp int) error {
	_, err := db.Exec(
		`INSERT INTO series (name, current_episode, total_episodes) VALUES (?, ?, ?)`,
		name, currentEp, totalEp,
	)
	return err
}

func IncrementEpisode(db *sql.DB, id int) error {
	res, err := db.Exec(`
		UPDATE series
		SET current_episode = current_episode + 1
		WHERE id = ? AND current_episode < total_episodes
	`, id)
	if err != nil {
		return err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return errors.New("no se pudo incrementar (quizá ya está completa o id inválido)")
	}
	return nil
}

func ValidateSerie(name string, currentEp, totalEp int) error {
	if stringsTrim(name) == "" {
		return fmt.Errorf("el nombre no puede ir vacío")
	}
	if currentEp < 1 {
		return fmt.Errorf("current_episode debe ser >= 1")
	}
	if totalEp < 1 {
		return fmt.Errorf("total_episodes debe ser >= 1")
	}
	if currentEp > totalEp {
		return fmt.Errorf("current_episode no puede ser mayor que total_episodes")
	}
	return nil
}

// helper 
func stringsTrim(s string) string {
	
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}