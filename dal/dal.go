package dal

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var MAX_NUM_FUNC = 100

type DalConfig struct {
	// data source
	DBHost   string
	Username string
	Password string

	// db
	DBName string

	// tables
	UsersTable      string
	FunctionsTable  string
	ExecutionsTable string
}

func (c *DalConfig) getDataSourceName() string {
	return fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", c.Username, c.Password, c.DBHost, c.DBName)
}

type MySQL struct {
	*sql.DB

	DBName string

	UsersTable      string
	FunctionsTable  string
	ExecutionsTable string
}

func NewMySQL(config *DalConfig) (*MySQL, error) {
	db, err := sql.Open("mysql", config.getDataSourceName())
	if err != nil {
		return nil, err
	}

	// This prevents broken pipe caused by idle connection
	db.SetMaxIdleConns(0)

	// Create the users table if not already existed
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s ( 
		u_id INT NOT NULL AUTO_INCREMENT, 
		name VARCHAR(255) NOT NULL, 
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP, 
		PRIMARY KEY (u_id),
		UNIQUE(name)
	)`, config.UsersTable))

	if err != nil {
		return nil, err
	}

	// Create the functions table if not already existed
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s ( 
		f_id INT NOT NULL AUTO_INCREMENT, 
		u_id INT NOT NULL,
		name VARCHAR(255) NOT NULL,
		content TEXT, 
		updated TIMESTAMP, 
		PRIMARY KEY (f_id), 
		FOREIGN KEY (u_id) REFERENCES %s(u_id)
	)`, config.FunctionsTable, config.UsersTable))

	if err != nil {
		return nil, err
	}

	// Create the executions table if not already existed
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		e_id INT NOT NULL AUTO_INCREMENT, 
		f_id INT NOT NULL,
		params TEXT,
		status VARCHAR(255) NOT NULL,
		uuid VARCHAR(255) NOT NULL,
		log TEXT, 
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP, 
		PRIMARY KEY (e_id), 
		FOREIGN KEY (f_id) REFERENCES %s(f_id)
	)`, config.ExecutionsTable, config.FunctionsTable))

	if err != nil {
		return nil, err
	}

	return &MySQL{
		db,
		config.DBName,
		config.UsersTable,
		config.FunctionsTable,
		config.ExecutionsTable,
	}, nil
}

// List all functions created by a user
func (dal *MySQL) ListFunctionsOfUser(username string, userId int64) ([]*Function, error) {
	log.Println("Listing functions for user", username)

	uid := userId

	if uid < 0 && username == "" {
		return nil, errors.New("Either userName or userId should be valid")
	}

	if uid < 0 {
		err := dal.QueryRow(fmt.Sprintf("SELECT u_id FROM %s WHERE name = ?", dal.UsersTable), username).Scan(&uid)
		if err != nil {
			return nil, err
		}
	}

	stmt, err := dal.Prepare(fmt.Sprintf(
		"SELECT f_id, name, content, updated FROM %s WHERE u_id = ?",
		dal.FunctionsTable))
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	funcList := make([]*Function, 0, MAX_NUM_FUNC)
	for rows.Next() {
		function := &Function{
			ID:      -1,
			UserID:  uid,
			Name:    "",
			Content: "",
			Updated: time.Time{},
		}

		err := rows.Scan(&function.ID, &function.Name, &function.Content, &function.Updated)
		if err != nil {
			return funcList, err
		}

		funcList = append(funcList, function)
	}

	if err = rows.Err(); err != nil {
		return funcList, err
	}

	return funcList, nil
}

// PutUserIfNotExists inserts user into DB if the user
// is not already inserted. The caller is responsible for
// making sure `userName` is not empty.
func (dal *MySQL) PutUserIfNotExisted(groupName, userName string) (int64, int64, error) {
	log.Println("Adding user", userName, "to DB...")

	stmt, err := dal.Prepare(fmt.Sprintf(
		"INSERT IGNORE INTO %s (name) VALUES (?)",
		dal.UsersTable))

	if err != nil {
		return -1, -1, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(userName)
	if err != nil {
		return -1, -1, err
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		return -1, -1, err
	}

	rowCnt, err := res.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	return lastId, rowCnt, nil
}

//
// When both `userName` and `userId` are not empty, the function check
// userId first.
func (dal *MySQL) PutFunction(userName, funcName, funcContent string, userId int64) (int64, int64, error) {
	var res sql.Result
	var fid int
	uid := userId

	if uid < 0 && userName == "" {
		return -1, -1, errors.New("Either userName or userId should be valid")
	}

	if uid < 0 {
		err := dal.QueryRow(fmt.Sprintf("SELECT u_id FROM %s WHERE name = ?", dal.UsersTable), userName).Scan(&uid)
		if err != nil {
			return -1, -1, err
		}
	}

	// Check if the function exists
	err := dal.QueryRow(fmt.Sprintf("SELECT f_id FROM %s WHERE name = ? AND u_id = ?", dal.FunctionsTable), funcName, uid).Scan(&fid)
	// Not exist, insert a new one
	if err == sql.ErrNoRows {
		log.Println("Inserting function", funcName, "into DB...")

		stmt, err := dal.Prepare(fmt.Sprintf(
			"INSERT INTO %s (u_id, name, content) VALUES (?, ?, ?)",
			dal.FunctionsTable))

		if err != nil {
			return -1, -1, err
		}
		defer stmt.Close()

		res, err = stmt.Exec(uid, funcName, funcContent)
		if err != nil {
			return -1, -1, err
		}

		log.Println("Inserted!")

	} else if err != nil {
		return -1, -1, err
		// Already exist, update the function
	} else {
		log.Println("Updating function", funcName, "in DB...")
		stmt, err := dal.Prepare(fmt.Sprintf(
			"UPDATE %s SET content = ? WHERE f_id = ?",
			dal.FunctionsTable))
		if err != nil {
			return -1, -1, err
		}
		defer stmt.Close()
		res, err = stmt.Exec(funcContent, fid)
		if err != nil {
			return -1, -1, err
		}
		log.Println("Updated!")
	}
	lastId, err := res.LastInsertId()
	if err != nil {
		return -1, -1, err
	}

	rowCnt, err := res.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	return lastId, rowCnt, nil

}

func (dal *MySQL) GetFunction(userName, funcName string) (*Function, error) {
	log.Println("Retriving function", funcName, "for user", userName)

	var function Function
	err := dal.QueryRow(fmt.Sprintf(
		"SELECT f.f_id, f.u_id, f.name, content, updated FROM %s f INNER JOIN %s u ON f.u_id=u.u_id WHERE f.name = ? AND u.name = ?",
		dal.FunctionsTable, dal.UsersTable), funcName, userName).Scan(
		&function.ID, &function.UserID, &function.Name, &function.Content, &function.Updated)
	if err != nil {
		return nil, err
	}

	return &function, nil

}

func (dal *MySQL) DeleteFunction(userName, funcName string) error {
	var uid int64

	log.Println("Deleting function", funcName, "for user", userName)

	err := dal.QueryRow(fmt.Sprintf("SELECT u_id FROM %s WHERE name = ?", dal.UsersTable), userName).Scan(&uid)
	if err != nil {
		return err
	}
	stmt, err := dal.Prepare(fmt.Sprintf(
		"DELETE FROM %s WHERE name = ? AND u_id = ?",
		dal.FunctionsTable))

	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(funcName, uid)
	if err != nil {
		return err
	}
	return nil
}

func (dal *MySQL) PutExecution(functionID int64, params, status, uuid, log string, timestamp time.Time) (int64, int64, error) {
	stmt, err := dal.Prepare(fmt.Sprintf(
		"INSERT INTO %s (f_id, params, status, uuid, log, created) VALUES (?, ?, ?, ?, ?, ?)",
		dal.ExecutionsTable))

	if err != nil {
		return -1, -1, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(functionID, params, status, uuid, log, timestamp)
	if err != nil {
		return -1, -1, err
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		return -1, -1, err
	}

	rowCnt, err := res.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	return lastId, rowCnt, nil
}

// Be careful with this function, it drops your entire database.
// Only used for test purpose.
func (dal *MySQL) ClearDatabase() error {
	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.ExecutionsTable)); err != nil {
		return err
	}

	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.FunctionsTable)); err != nil {
		return err
	}

	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.UsersTable)); err != nil {
		return err
	}

	return nil
}
