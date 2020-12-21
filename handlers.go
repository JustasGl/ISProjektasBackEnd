package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func LandingPage(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	w.Write([]byte("Hello world"))
}

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
	r.HandleFunc("/login", IsLoggedIn).Methods("GET", "OPTIONS")
	r.HandleFunc("/login", Login).Methods("POST", "OPTIONS")
	r.HandleFunc("/login", Logout).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/login", EditPassword).Methods("PATCH", "OPTIONS")

	r.HandleFunc("/account", RegisterNewAccount).Methods("POST", "OPTIONS")
	r.HandleFunc("/account", GetAccountInfo).Methods("GET", "OPTIONS")
	r.HandleFunc("/account", EditAccountInfo).Methods("PATCH", "OPTIONS")

	r.HandleFunc("/games", GetGames).Methods("GET", "OPTIONS")

	r.HandleFunc("/games", CreateGame).Methods("POST", "OPTIONS")
	r.HandleFunc("/games/{id}", EditGame).Methods("PATCH", "OPTIONS")
	r.HandleFunc("/games/{id}", DeleteGame).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/games/{id}/users", SellGame).Methods("PATCH", "OPTIONS")

	r.HandleFunc("/follow/{id}", FollowUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/follow/{id}", UnfollowUser).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/followers", GetFollowers).Methods("GET", "OPTIONS")
	r.HandleFunc("/followings", GetFollowings).Methods("GET", "OPTIONS")

	r.HandleFunc("/cart", GetCart).Methods("GET", "OPTIONS")
	r.HandleFunc("/cart/{id}", AddCart).Methods("POST", "OPTIONS")
	r.HandleFunc("/cart/{id}", RemoveCart).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/wishlist", GetWishList).Methods("GET", "OPTIONS")
	r.HandleFunc("/wishlist/{id}", AddWishList).Methods("POST", "OPTIONS")
	r.HandleFunc("/wishlist/{id}", RemoveWishList).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/BoughtList", GetBoughtList).Methods("GET", "OPTIONS")

	r.HandleFunc("/rate/{id}", Rate).Methods("POST", "OPTIONS")
	r.HandleFunc("/rate", GetRatings).Methods("GET", "OPTIONS")
	r.HandleFunc("/rate/{id}", EditRating).Methods("PATCH", "OPTIONS")
	r.HandleFunc("/rate/{id}", DeleteRating).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/accounts", GetAccounts).Methods("GET", "OPTIONS")

	http.ListenAndServe(":8000", r)
}
func setupCorsResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, PATCH, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	(*w).Header().Set("Content-Type", "text/html; charset=utf-8")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	if req.Method == "OPTIONS" {

		(*w).WriteHeader(http.StatusOK)
	}
}
