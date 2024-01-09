package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"go-concurrency-web-app/data"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

const PORT = "8080"
const DSN = "host=localhost port=5432 user=postgres password=password dbname=concurrency sslmode=disable timezone=UTC connect_timeout=5"
const REDIS = "127.0.0.1:6379"

func main() {
	// connect to the DB
	db := initDB()

	session := initSession()

	// create loggers
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	// create sessions

	// create some channels

	// wait groups
	wg := sync.WaitGroup{}

	// set up app config
	app := Config{
		Session:  session,
		DB:       db,
		Wg:       &wg,
		InfoLog:  infoLog,
		ErrorLog: errorLog,
		Models:   data.New(db),
		ErrorChan: make(chan error),
		ErrorChanDone: make(chan bool),
	}

	// setup mail
	app.Mailer = app.CreateMail()
	go app.listenForMail()

	// listen for signals
	go app.listenForShutDown()

	// listen for errors
	go app.listenForErrors()

	// listen for web connections
	app.serve()
}

func (app *Config) serve() {
	// start http server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", PORT),
		Handler: app.routes(),
	}

	app.InfoLog.Println("Starting web server...")

	err := srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func initDB() *sql.DB {
	conn := connectToDB()

	if conn == nil {
		log.Panic("Can't connect to Database")
	}

	return conn
}

func connectToDB() *sql.DB {
	count := 0
	dsn := os.Getenv("DSN")
	if os.Getenv("DSN") == "" {
		dsn = DSN
	}

	for {
		connection, err := openDB(dsn)
		if err != nil {
			log.Println("Postgres not yet read...")
		} else {
			log.Println("Connected to DB.")
			return connection
		}

		if count > 10 {
			return nil
		}

		log.Println("Backing off for 1sec")
		time.Sleep(1 * time.Second)
		count += 1
		continue
	}
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)

	if err != nil {
		return nil, err
	} 

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initSession() *scs.SessionManager {
	gob.Register(data.User{})
	// Set up session
	session := scs.New()
	session.Store = redisstore.New(initRedis())
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true

	return session
}

func initRedis() *redis.Pool {

	r := os.Getenv("REDIS")
	if r == "" {
		r = REDIS
	}
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", r)
		},
	}

	return redisPool
}

func (app *Config) listenForShutDown() {
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *Config) shutdown() {
	// perform any cleanup tasks
	app.InfoLog.Println("Would run clean up tasks...")

	// Block untill wg is empty
	app.Wg.Wait()
	app.Mailer.DoneChan <- true
	app.ErrorChanDone <- true

	app.InfoLog.Println("Closing channels...")
	app.InfoLog.Println("Shutting down app...")

	close(app.Mailer.DoneChan)
	close(app.Mailer.ErrorChan)
	close(app.Mailer.MailerChan)
	close(app.ErrorChanDone)
	close(app.ErrorChan)
}

func (app *Config) CreateMail() Mail {
	errorChan := make(chan error)
	mailerDoneChan := make(chan bool)
	mailerChan := make(chan Message, 100)

	m := Mail{
		Domain:      "localhost",
		Host:        "localhost",
		Port:        1025,
		Encryption:  "none",
		FromAddress: "info@mycompany.com",
		WaitGroup:   app.Wg,
		FromName:    "Info",
		ErrorChan:   errorChan,
		DoneChan:    mailerDoneChan,
		MailerChan:  mailerChan,
	}

	return m
}
