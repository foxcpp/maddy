package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <DB file path>\n", os.Args[0])
		return
	}

	fmt.Println("Seriously, take a DB backup now. I will give you 5 seconds to abort.")
	time.Sleep(5 * time.Second)

	/* In go-imap-sql used by maddy 0.2:
	            id BIGSERIAL NOT NULL PRIMARY KEY AUTOINCREMENT,
				username VARCHAR(255) NOT NULL UNIQUE,
				msgsizelimit INTEGER DEFAULT NULL,
				password VARCHAR(255) DEFAULT NULL,
				password_salt VARCHAR(255) DEFAULT NULL,
	            inboxId BIGINT DEFAULT 0
	*/
	/* In go-imap-sql used by maddy 0.3:
				id BIGSERIAL NOT NULL PRIMARY KEY AUTOINCREMENT,
				username VARCHAR(255) NOT NULL UNIQUE,
				msgsizelimit INTEGER DEFAULT NULL,
	            inboxId BIGINT DEFAULT 0
	*/

	db, err := sql.Open("sqlite3", os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Println(err)
		return
	}
	db.SetMaxOpenConns(1)

	fmt.Println("Disabling foreign keys...")
	db.Exec("PRAGMA foreign_keys=OFF")

	fmt.Println("Taking exclusive DB lock...")
	db.Exec("PRAGMA locking_mode=EXCLUSIVE")
	tx, err := db.Begin()
	if err != nil {
		fmt.Println("Tx begin:", err)
		return
	}
	defer tx.Rollback()

	fmt.Println("Creating new users table...")
	_, err = tx.Exec(`
        CREATE TABLE __new_users (
            id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(255) NOT NULL UNIQUE,
			msgsizelimit INTEGER DEFAULT NULL,
            inboxId BIGINT DEFAULT 0
        )`)
	if err != nil {
		fmt.Println("Create new table:", err)
		return
	}

	fmt.Println("Moving data from old table...")
	_, err = tx.Exec(`
        INSERT INTO __new_users
        SELECT id, username, msgsizelimit, inboxId
        FROM users`)
	if err != nil {
		fmt.Println("Data move:", err)
		return
	}

	fmt.Println("Removing old table...")
	_, err = tx.Exec(`DROP TABLE users`)
	if err != nil {
		fmt.Println("Table drop:", err)
		return
	}

	fmt.Println("Renaming new table to the normal name...")
	_, err = tx.Exec(`ALTER TABLE __new_users RENAME TO users`)
	if err != nil {
		fmt.Println("Table rename:", err)
		return
	}

	fmt.Println("Completing transaction...")
	if err := tx.Commit(); err != nil {
		fmt.Println("Tx commit:", err)
		return
	}

	fmt.Println("Done! Now go back and run maddyctl to readd passwords to DB.")
}
