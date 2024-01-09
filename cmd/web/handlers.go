package main

import (
	"fmt"
	"go-concurrency-web-app/data"
	"html/template"
	"net/http"
	"strconv"
)

func (app *Config) HomePage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home.page.gohtml", nil)
}

func (app *Config) LoginPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "login.page.gohtml", nil)
}

func (app *Config) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.RenewToken(r.Context())

	// parse from post
	err := r.ParseForm()
	if err != nil {
		app.ErrorLog.Println(err)
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	user, err := app.Models.User.GetByEmail(email)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid Email.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// check password
	validPassword, err := user.PasswordMatches(password)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid Password.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !validPassword {
		msg := Message{
			To:      email,
			Subject: "Failed login attempt.",
			Data:    "Invalid login attempt.",
		}
		app.sendEmail(msg)
		app.Session.Put(r.Context(), "error", "Invalid Password.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Session.Put(r.Context(), "userID", user.ID)
	app.Session.Put(r.Context(), "user", user)

	app.Session.Put(r.Context(), "flash", "Successful Login.")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Config) Logout(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.Destroy(r.Context())
	_ = app.Session.RenewToken(r.Context())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *Config) RegisterPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "register.page.gohtml", nil)
}

func (app *Config) PostRegisterPage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.ErrorLog.Println(err)
	}

	// Todo - validate Data

	// create a user
	user := data.User{
		Email:     r.Form.Get("email"),
		LastName:  r.Form.Get("last-name"),
		FirstName: r.Form.Get("first-name"),
		Password:  r.Form.Get("password"),
		Active:    0,
		IsAdmin:   0,
	}

	_, err = user.Insert(user)
	if err != nil {
		app.ErrorLog.Println(err)
		app.Session.Put(r.Context(), "error", "Unable to create user.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// send an activation email
	url := fmt.Sprintf("http://localhost:8080/activate?email=%s", user.Email)
	signedURL := GenerateTokenFromString(url)
	app.InfoLog.Println(signedURL)
	app.sendEmail(Message{
		To:       user.Email,
		Subject:  "Activate your Account!",
		Template: "confirmation-email",
		Data:     template.HTML(signedURL),
	})

	app.Session.Put(r.Context(), "flash", "Conformation email sent. Check your email.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)

}

func (app *Config) ActivateAccount(w http.ResponseWriter, r *http.Request) {
	// validate URL
	url := r.RequestURI
	testURL := fmt.Sprintf("http://localhost:8080%s", url)

	okay := VerifyToken(testURL)

	if !okay {
		app.Session.Put(r.Context(), "error", "Invalid Token")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// activate account
	u, err := app.Models.User.GetByEmail(r.URL.Query().Get("email"))
	if err != nil {
		app.Session.Put(r.Context(), "error", "No user found")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	u.Active = 1

	err = u.Update()

	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to update user")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "flash", "Account activated")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Config) ChooseSubscription(w http.ResponseWriter, r *http.Request) {
	plans, err := app.Models.Plan.GetAll()
	if err != nil {
		app.ErrorLog.Println(err)
		app.Session.Put(r.Context(), "error", "Error getting plans")
		// http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	dataMap := make(map[string]any)
	dataMap["plans"] = plans

	app.render(w, r, "plans.page.gohtml", &TemplateData{
		Data: dataMap,
	})

}

func (app *Config) getInvoice(user data.User, plan *data.Plan) (string, error) {
	return plan.PlanAmountFormatted, nil
}

func (app *Config) SubscribeToPlan(w http.ResponseWriter, r *http.Request) {

	// get id of the plan
	id := r.URL.Query().Get("id")

	planId, _ := strconv.Atoi(id)

	// get the plan from DB
	plan, err := app.Models.Plan.GetOne(planId)
	if err != nil {
		app.ErrorLog.Println(err)
		app.Session.Put(r.Context(), "error", "Unable to find plan")
		http.Redirect(w, r, "/members/plans", http.StatusSeeOther)
		return
	}
	// get the user from session
	user, ok := app.Session.Get(r.Context(), "user").(data.User)
	if !ok {
		app.ErrorLog.Println(err)
		app.Session.Put(r.Context(), "error", "Login first!")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// generate invoice and email it
	app.Wg.Add(1)

	go func() {
		defer app.Wg.Done()

		invoice, err := app.getInvoice(user, plan)
		if err != nil {
			// send this to a channel
			app.ErrorChan <- err
		}

		msg := Message{
			To:       user.Email,
			Subject:  "Your Invoice",
			Data:     invoice,
			Template: "invoice",
		}

		app.sendEmail(msg)
	}()

	// generate a manual

	// send and email with the manual attached

	// subscribe the user to an account

	// redirect
}

func (app *Config) listenForErrors() {
	for {
		select {
		case err := <-app.ErrorChan:
			app.ErrorLog.Println(err)
		case <-app.ErrorChanDone:
			return
		}
	}
}