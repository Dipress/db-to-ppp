package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"time"

	"github.com/dipress/db-to-ppp/internal/updater"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func main() {
	var (
		login    = flag.String("login", "login", "enter login for mikrotik")
		password = flag.String("password", "password", "enter password for mikrotik")
		address  = flag.String("address", "host:port", "enter host and port")
		dsn      = flag.String("dsn", "username:password@tcp(127.0.0.1:3306)/databasename",
			"mysql connection string")
	)
	flag.Parse()

	context := context.TODO()
	client, err := conn(*login, *password, *address)

	if err != nil {
		log.Fatalf("open ssh client: %s", err)
	}
	defer client.Close()

	db, err := openDB(*dsn)
	if err != nil {
		log.Fatalf("mysql db connection open: %s", err)
	}
	defer db.Close()

	upd := updater.New(client, db)
	if err := upd.Update(context, 12); err != nil {
		log.Fatalf("upload failed: %s", err)
	}
}

// OpenDB return sql instance
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)

	if err != nil {
		return nil, errors.Wrap(err, "open sql connection failed: %v\n")
	}

	if err := db.Ping(); err != nil {
		return nil, errors.Wrap(err, "mysql ping failure: %v\n")
	}

	return db, nil
}

// conn return ssh client instance
func conn(login, password, address string) (*ssh.Client, error) {
	config := ssh.ClientConfig{
		User: login,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", address, &config)

	if err != nil {
		return nil, errors.Wrap(err, "Dial error")
	}

	return client, nil
}
