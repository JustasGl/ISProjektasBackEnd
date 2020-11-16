package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

//LandingPage comment
func LandingPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello world"))
}

//RefreshToken refreshes authentication token WORK IN PROGRESS not even close to complete
func RefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshSession, _ := sessionStore.Get(r, "refresh-token")

	authSession, _ := sessionStore.Get(r, "auth-token")

	authSession.Values["userID"] = refreshSession.Values["userID"]
	authSession.Options.MaxAge = 60
	authSession.Save(r, w)

}

func HandleFunctions() {
	r := mux.NewRouter()
	r.HandleFunc("/", LandingPage)
	r.HandleFunc("/login", IsLoggedIn).Methods("GET")
	r.HandleFunc("/login", Login).Methods("POST")
	r.HandleFunc("/login", Logout).Methods("DELETE")
	r.HandleFunc("/login", EditPassword).Methods("PATCH")

	r.HandleFunc("/account", RegisterNewAccount).Methods("POST")
	r.HandleFunc("/account", GetAccountInfo).Methods("GET")
	r.HandleFunc("/account", EditAccountInfo).Methods("PATCH")

	r.HandleFunc("/games", GetGames).Methods("GET")

	r.HandleFunc("/games", CreateGame).Methods("POST")
	r.HandleFunc("/games/{id}", EditGame).Methods("PATCH")
	r.HandleFunc("/games/{id}", DeleteGame).Methods("DELETE")

	r.HandleFunc("/games/{id}/users", SellGame).Methods("PATCH")

	r.HandleFunc("/follow/{id}", FollowUser).Methods("POST")
	r.HandleFunc("/follow/{id}", UnfollowUser).Methods("DELETE")

	r.HandleFunc("/followers/{id}", GetFollowers).Methods("GET")
	r.HandleFunc("/followings/{id}", GetFollowings).Methods("GET")
	http.ListenAndServe(":8000", r)
}
