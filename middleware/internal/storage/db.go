package storage

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Vertisphere/backend-service/internal/domain"
	_ "github.com/lib/pq"
)

// SQLStorage is a wrapper for database operations
type SQLStorage struct {
	db *sql.DB
}

// Init kicks off the database connector
func (s *SQLStorage) Init(user, password, host, name string) error {
	fmt.Printf("Initializing database with user: %s, host: %s\n", user, host)
	deebz, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, password, host, name))
	if err != nil {
		return err
	}
	// Assign the database connection to s.db
	s.db = deebz
	return nil
}

// Close ends the database connection
func (s *SQLStorage) Close() error {
	return s.db.Close()
}

func (s SQLStorage) CreateFranchise(franchise domain.Franchise) error {
	// deconstruct franchise object  and insert into franchise table
	// Construct query string
	query := fmt.Sprintf(
		"INSERT INTO franchise(franchise_name, headquarters_address, phone_number) VALUES('%s', '%s', '%s')",
		franchise.FranchiseName, franchise.HeadquartersAddress, franchise.PhoneNumber,
	)
	log.Println("Executing query:", query)

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

// func (s SQLStorage) ReadUser(id string) (bool, bool, error) {
// 	//get is_franchiser and is_admin from users table where id = id
// 	results, err := s.db.Query("SELECT is_franchiser, is_admin FROM users WHERE id =?", id)
// 	if err != nil {
// 		return false, false, err
// 	}
// 	var is_franchiser, is_admin bool
// 	results.Scan(&is_franchiser, &is_admin)
// 	return is_franchiser, is_admin, nil
// }

// func (s SQLStorage) CreateFranchise(franchise domain.Franchise) error {
// 	// deconstruct franchise object  and insert into franchise table
// 	// For fields in franchise that are not null, insert into franchise table
// 	// For fields in franchise that are null, insert null into franchise table
// 	_, err := s.db.Exec(
// 		"INSERT INTO franchise... "
// 	)
// 	_, err := s.db.Exec(
// 		"INSERT INTO franchise(?, ? ,) VALUES(?, ?, ?)",
// 		franchise.ID, franchise., franchise.OwnerID,
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// // List returns a list of all todos
// func (s SQLStorage) List() (Todos, error) {
// 	ts := Todos{}
// 	results, err := s.db.Query("SELECT * FROM todo ORDER BY updated DESC")
// 	if err != nil {
// 		return ts, err
// 	}

// 	for results.Next() {
// 		t, err := resultToTodo(results)
// 		if err != nil {
// 			return ts, err
// 		}

// 		ts = append(ts, t)
// 	}
// 	return ts, nil
// }

// // Create records a new todo in the database.
// func (s SQLStorage) Create(t Todo) (Todo, error) {
// 	sql := `
// 		INSERT INTO todo(title, updated)
// 		VALUES(?,NOW())
// 	`

// 	if t.Complete {
// 		sql = `
// 		INSERT INTO todo(title, updated, completed)
// 		VALUES(?,NOW(),NOW())
// 	`
// 	}

// 	op, err := s.db.Prepare(sql)
// 	if err != nil {
// 		return t, err
// 	}

// 	results, err := op.Exec(t.Title)

// 	id, err := results.LastInsertId()
// 	if err != nil {
// 		return t, err
// 	}

// 	t.ID = int(id)

// 	return t, nil
// }

// func resultToTodo(results *sql.Rows) (Todo, error) {
// 	t := Todo{}
// 	if err := results.Scan(&t.ID, &t.Title, &t.Updated, &t.completedNull); err != nil {
// 		return t, err
// 	}

// 	if t.completedNull.Valid {
// 		t.Completed = t.completedNull.Time
// 		t.Complete = true
// 	}

// 	return t, nil
// }

// // Read returns a single todo from the database
// func (s SQLStorage) Read(id string) (Todo, error) {
// 	t := Todo{}
// 	results, err := s.db.Query("SELECT * FROM todo WHERE id =?", id)
// 	if err != nil {
// 		return t, err
// 	}

// 	results.Next()
// 	t, err = resultToTodo(results)
// 	if err != nil {
// 		return t, err
// 	}

// 	return t, nil
// }

// // Update changes one todo in the database.
// func (s SQLStorage) Update(t Todo) error {
// 	orig, err := s.Read(strconv.Itoa(t.ID))
// 	if err != nil {
// 		return err
// 	}

// 	sql := `
// 		UPDATE todo
// 		SET title = ?, updated = NOW()
// 		WHERE id = ?
// 	`

// 	if t.Complete && !orig.Complete {
// 		sql = `
// 		UPDATE todo
// 		SET title = ?, updated = NOW(), completed = NOW()
// 		WHERE id = ?
// 	`
// 	}

// 	if orig.Complete && !t.Complete {
// 		sql = `
// 		UPDATE todo
// 		SET title = ?, updated = NOW(), completed = NULL
// 		WHERE id = ?
// 	`
// 	}

// 	op, err := s.db.Prepare(sql)
// 	if err != nil {
// 		return err
// 	}

// 	_, err = op.Exec(t.Title, t.ID)

// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Delete removes one todo from the database.
// func (s SQLStorage) Delete(id string) error {
// 	op, err := s.db.Prepare("DELETE FROM todo WHERE id =?")
// 	if err != nil {
// 		return err
// 	}

// 	if _, err = op.Exec(id); err != nil {
// 		return err
// 	}

// 	return nil
// }
