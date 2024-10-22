package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"

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

func (s SQLStorage) CreateFranchise(admin_account_id string, franchise domain.Franchise) error {
	// deconstruct franchise object  and insert into franchise table
	// Construct query string
	// TODO: Right now the flow is shit af
	// Basically, I'm not sure how we want to do the business logic on this
	// Like do we want an account to be able to create multiple franchises???
	// Anyways, the current apporach to make sure that an admin user can be created securely
	// If a post /user comes in to create a user, it will create the franchise id,
	// then we query the franchise entry, check the admin_account_id, and verify that the account_ID in the entry matches the one in the JWT.
	// I know its shit but I can't think of something better rn

	query := fmt.Sprintf(
		"INSERT INTO franchise(franchise_name, headquarters_address, phone_number, admin_account_id) VALUES('%s', '%s', '%s', '%s')",
		franchise.FranchiseName, franchise.HeadquartersAddress, franchise.PhoneNumber, admin_account_id,
	)

	log.Println("Executing query:", query)

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func (s SQLStorage) GetFranchiseIDFromAccountId(accountID string) (int, error) {
	var franchiseID int
	query := "SELECT franchise_id FROM franchise WHERE admin_account_id = $1"
	err := s.db.QueryRow(query, accountID).Scan(&franchiseID)
	if err != nil {
		return 0, err
	}
	return franchiseID, nil
}

func (s SQLStorage) CreateFranchiseUser(accountID string, franchiseID int, name string) error {
	// Setting all to role = 3 for now
	query := fmt.Sprintf(
		"INSERT INTO app_user(account_id, franchise_id, role, name) VALUES ('%s', '%d', '%d', '%s')",
		accountID, franchiseID, 3, name,
	)
	log.Println("Executing query:", query)

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func (s SQLStorage) CreateFranchiseeUser(accountId string, franchiseId int, franchiseeId int, name string) error {
	// Setting all to role = 1 for now
	// Create Franchisee in Database
	query := fmt.Sprintf(
		"INSERT INTO app_user(account_id, franchise_id, franchisee_id, role, name) VALUES ('%s', '%d', '%d', '%d', '%s')",
		accountId, franchiseId, franchiseeId, 1, name,
	)
	log.Println("Executing query:", query)

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func (s SQLStorage) GetUserClaims(accountID string) (int, int, int, int, error) {
	var userID int
	var franchise_id int
	var franchisee_id sql.NullInt32
	var role int
	query := "SELECT user_id, franchise_id, franchisee_id, role FROM app_user WHERE account_id = $1"
	err := s.db.QueryRow(query, accountID).Scan(&userID, &franchise_id, &franchisee_id, &role)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if !franchisee_id.Valid {
		return userID, franchise_id, 0, role, nil
	} else {
		return userID, franchise_id, int(franchisee_id.Int32), role, nil
	}
}

func (s SQLStorage) CreateFranchisee(franchiseId int, franchiseeName string, headquartersAddress string, phone string) (int, error) {
	// Create Franchisee in Database
	// and return the franchisee_id
	var franchise_id int
	query := fmt.Sprintf(
		"INSERT INTO franchisee(franchise_id, franchisee_name, headquarters_address, phone_number) VALUES ('%d', '%s', '%s', '%s') RETURNING franchisee_id",
		franchiseId, franchiseeName, headquartersAddress, phone,
	)
	log.Println("Executing query:", query)
	err := s.db.QueryRow(query).Scan(&franchise_id)
	if err != nil {
		return 0, err
	}
	return franchise_id, nil
}

func (s SQLStorage) CreateProduct(franchiseId int, name string, description string, price float64) (int, error) {
	// Create Product in Database and get the product_id
	var product_id int
	query := fmt.Sprintf(
		"INSERT INTO product(franchise_id, product_name, description, price, product_status) VALUES ('%d', '%s', '%s', '%f', '%d') RETURNING product_id",
		franchiseId, name, description, price, 0,
	)
	log.Println("Executing query:", query)

	err := s.db.QueryRow(query).Scan(&product_id)
	if err != nil {
		return 0, err
	}
	return product_id, nil
}

func (s SQLStorage) ListProducts(franchiseId int, pageSize int, pageToken string, orderBy string) ([]domain.Product, string, error) {
	var nextToken string
	var products []domain.Product
	var rows *sql.Rows
	var err error

	// query with pagination
	if pageToken == "" {
		dbQuery := fmt.Sprintf("SELECT * FROM product WHERE franchise_id = $1 ORDER BY %s LIMIT $2", orderBy)
		rows, err = s.db.Query(dbQuery,
			franchiseId, pageSize)
	} else {
		dbQuery := fmt.Sprintf("SELECT * FROM product WHERE franchise_id = $1 AND product_id > $2 ORDER BY %s LIMIT $3", orderBy)
		rows, err = s.db.Query(dbQuery,
			franchiseId, pageToken, pageSize)
	}

	if err != nil {
		return nil, "", err
	}

	for rows.Next() {
		var product domain.Product
		err = rows.Scan(&product.ID, &product.FranchiseID, &product.ProductName, &product.Description, &product.Price, &product.ProductStatus, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		products = append(products, product)
	}

	if len(products) != 0 {
		nextToken = fmt.Sprintf("%d", products[len(products)-1].ID)
	} else {
		nextToken = pageToken
	}

	return products, nextToken, nil
}

func (s SQLStorage) SearchProducts(franchiseId int, query string, pageSize int, pageToken string, orderBy string) ([]domain.Product, string, error) {
	// For now we use ILIKE but in the future implement tsvector
	var nextToken string
	var products []domain.Product
	var rows *sql.Rows
	var err error
	// mutate query to add percentage signs
	query = "%" + query + "%"
	if pageToken == "" {
		dbQuery := fmt.Sprintf("SELECT * FROM product WHERE franchise_id = $1 AND (product_name ILIKE $2 OR description ILIKE $2) ORDER BY %s LIMIT $3", orderBy)
		rows, err = s.db.Query(dbQuery,
			franchiseId, query, pageSize)
	} else {
		dbQuery := fmt.Sprintf("SELECT * FROM product WHERE franchise_id = $1 AND (product_name ILIKE $2 OR description ILIKE $2) AND product_id > $3 ORDER BY %s LIMIT $4", orderBy)
		rows, err = s.db.Query(dbQuery,
			franchiseId, query, pageToken, pageSize)
	}

	if err != nil {
		return nil, "", err
	}

	for rows.Next() {
		var product domain.Product
		err = rows.Scan(&product.ID, &product.FranchiseID, &product.ProductName, &product.Description, &product.Price, &product.ProductStatus, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		products = append(products, product)
	}

	if len(products) != 0 {
		nextToken = fmt.Sprintf("%d", products[len(products)-1].ID)
	} else {
		nextToken = pageToken
	}

	return products, nextToken, nil
}

// TODO: fix type signature somehow
// Probably add struct for Order and OrderRequest in the domain file?
func (s SQLStorage) CreateOrder(appUserId int, franchiseId int, franchiseeId int, products domain.Products) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	var total float64
	var order_id int
	total = 0.00

	// Create Order in Database and get the order_id
	err = tx.QueryRow("INSERT INTO orders(franchise_id, franchisee_id, created_by) VALUES ($1, $2, $3) RETURNING order_id", franchiseId, franchiseeId, appUserId).Scan(&order_id)
	if err != nil {
		return 0, err
	}

	// Create Order Product in join table
	for _, product := range products {
		// Get price of product
		var price float64
		err = tx.QueryRow("SELECT price FROM product WHERE product_id = $1", product.ProductID).Scan(&price)
		if err != nil {
			return 0, err
		}

		// Insert into order_product table
		_, err = tx.Exec("INSERT INTO order_product(order_id, product_id, quantity, price) VALUES ($1, $2, $3, $4)", order_id, product.ProductID, product.Quantity, price)
		if err != nil {
			return 0, err
		}

		// Add to total
		total += price * float64(product.Quantity)
	}

	// Insert total into order table
	_, err = tx.Exec("UPDATE orders SET total = $1 WHERE order_id = $2", total, order_id)
	if err != nil {
		return 0, err
	}
	return order_id, nil
}

func (s SQLStorage) GetQuickbooksAuth(franchiseID int) (string, string, string, bool, error) {
	var realmId, authTokenValue, jwtTokenValue sql.NullString
	var expTime sql.NullTime
	query := "SELECT quickbooks_id, quickbooks_auth_token, quickbooks_refresh_token, quickbooks_refresh_token_expires FROM franchise WHERE franchise_id = $1"
	err := s.db.QueryRow(query, franchiseID).Scan(&realmId, &authTokenValue, &jwtTokenValue, &expTime)
	if err != nil {
		return "", "", "", false, err
	}

	var authToken, jwtToken string
	var exp time.Time
	if authTokenValue.Valid {
		authToken = authTokenValue.String
	}
	if jwtTokenValue.Valid {
		jwtToken = jwtTokenValue.String
	}
	if expTime.Valid {
		exp = expTime.Time
	}

	var isExpired bool
	if !expTime.Valid || exp.Before(time.Now()) {
		isExpired = true
	}

	return realmId.String, authToken, jwtToken, isExpired, nil
}

func (s SQLStorage) SetQuickbooksRefresh(franchiseId int, refreshToken string) error {
	refreshTokenExpires := time.Now().Add(time.Hour)
	_, err := s.db.Exec("UPDATE franchise SET quickbooks_refresh_token = $1, quickbooks_refresh_token_expires = $2 WHERE franchise_id = $3", refreshToken, refreshTokenExpires, franchiseId)
	if err != nil {
		return err
	}
	return nil
}

func (s SQLStorage) SetQuickbooksAuth(franchiseId int, realmId string, authToken string, jwtToken string) error {
	_, err := s.db.Exec("UPDATE franchise SET quickbooks_id = $1, quickbooks_auth_token = $2, quickbooks_refresh_token = $3, quickbooks_refresh_token_expires = $4 WHERE franchise_id = $5", realmId, authToken, jwtToken, time.Now().Add(time.Hour), franchiseId)
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
