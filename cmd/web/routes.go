package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *Config) routes() http.Handler {
	// Create a router
	mux := chi.NewRouter()

	// set up Middleware
	mux.Use(middleware.Recoverer)
	mux.Use(app.SessionLoad)

	// define application routes
	mux.Get("/", app.HomePage)

	mux.Get("/login", app.LoginPage)
	mux.Post("/login", app.PostLoginPage)
	mux.Get("/register", app.RegisterPage)
	mux.Post("/register", app.PostRegisterPage)
	mux.Get("/logout", app.Logout)
	mux.Get("/activate", app.ActivateAccount)

	mux.Mount("/members", app.authRoute())

	// mux.Get("/test-email", func(w http.ResponseWriter, r *http.Request) {
	// 	m := Mail{
	// 		Domain:      "localhost",
	// 		Host:        "localhost",
	// 		Port:        1025,
	// 		Encryption:  "none",
	// 		FromAddress: "info@mycompany.com",
	// 		FromName:    "Info",
	// 		ErrorChan:   make(chan error),
	// 	}

	// 	msg := Message{
	// 		To:      "me@hear.com",
	// 		Subject: "Test Email",
	// 		Data:    "Hello World!",
	// 	}

	// 	m.sendMail(msg, make(chan error))
	// })
	return mux
}

func (app *Config) authRoute() http.Handler {
	mux := chi.NewRouter()
	mux.Use(app.Auth)

	mux.Get("/plans", app.ChooseSubscription)
	mux.Get("/subscribe", app.SubscribeToPlan)

	return mux
}
