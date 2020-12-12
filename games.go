package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type Game struct {
	ID          uint       `json: "-" gorm:"primary_key"`
	Name        string     `json: "Name"`
	Description string     `json: "description"`
	Price       float64    `json: "Price"`
	Category    string     `json: "Category"`
	CreatedAt   time.Time  `json: "CreatedAt"`
	UpdatedAt   time.Time  `json: "-"`
	DeletedAt   *time.Time `json: "-"`
	Sells       int        `json:"Sells"`
	CreatorID   uint
	Creator     User `gorm:"foreignkey:CreatorID"`

	BoughtList []User `gorm:"many2many:games_bought;"`
}

func CreateGame(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	// Get the user that is creating the game
	var user User
	db.First(&user, session.Values["userID"].(uint))
	if user.Role != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	var newGame Game
	// Get game data from json body
	err := json.NewDecoder(r.Body).Decode(&newGame)
	newGame.Creator = user
	newGame.Sells = 0
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Create an association between creator_id and a users id
	db.Model(&newGame).AddForeignKey("creator_id", "users(id)", "RESTRICT", "RESTRICT")
	// Create game
	if db.Create(&newGame).Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusCreated)
	JSONResponse(struct{}{}, w)
	return
}

func SellGame(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	fmt.Printf("%s\n", "SellGame Called")
	//Get user id from auth token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Gets id from /games/{id}/users
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Get user and game from provided IDs
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var selectedGame Game
	db.Preload("BoughtList").First(&selectedGame, "id = ?", gameID)

	//Check if game and user exist
	if selectedGame.ID == 0 || user.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Check if user is not the creator
	if user.ID == selectedGame.CreatorID {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Add game to Bought List
	db.Model(&selectedGame).Association("BoughtList").Append(&user)
	db.Model(&selectedGame).Updates(Game{Sells: selectedGame.Sells + 1})

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

func GetGames(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	// Gets filtering keys from url. e.x ?category=shooter&priceFrom=20&priceTo=40
	keys := r.URL.Query()
	category := keys.Get("category")
	creatorID := keys.Get("creatorID")
	PriceFrom := keys.Get("priceFrom")
	PriceTo := keys.Get("priceTo")
	var games []Game

	// Preloads user and creator tables for use in game response
	tx := db.Preload("Creator").Preload("ratings")

	// If a certain tag is not null, it is used to filter games
	if category != "" {
		tx = tx.Where("Category = ?", category)
	}
	if creatorID != "" {
		tx = tx.Where("creator_id = ?", creatorID)
	}
	if PriceFrom != "" && PriceTo != "" {
		tx = tx.Where("Price > ? and Price < ?", PriceFrom, PriceTo)
	} else if PriceFrom != "" {
		tx = tx.Where("Price > ?", PriceFrom)
	} else if PriceTo != "" {
		tx = tx.Where("Price < ?", PriceFrom)
	}
	// Finds games based on given parameters
	tx.Find(&games)

	// If no games exist, return Bad request
	if len(games) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(games, w)
	return
}

func DeleteGame(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	//Loads creator id from authentication token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	userID := session.Values["userID"].(uint)

	//Gets id from /games/{id}
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Loads game with joined users preloaded
	var game Game
	db.Where("id = ?", gameID).First(&game)

	//checks if the user that is trying to delete game is its creator
	if game.CreatorID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Deletes the record from database
	if db.Unscoped().Delete(&game).RecordNotFound() {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Deletes associations (users that bought the game)
	db.Model(&game).Association("Users").Delete(&game.BoughtList)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

func EditGame(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	//Loads creator id from authentication token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	userID := session.Values["userID"].(uint)

	//Gets id from /games/{id}
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Loads game with joined users preloaded
	var game Game
	tx := db.Where("ID = ?", gameID).First(&game)
	db.First(&game, gameID)
	//checks if the user that is trying to edit game is its creator
	if game.CreatorID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	var updatedGame Game
	json.NewDecoder(r.Body).Decode(&updatedGame)

	if updatedGame.Description != "" {
		tx.Model(&game).Updates(Game{Description: updatedGame.Description})
	}
	if updatedGame.Category != "" {
		tx.Model(&game).Updates(Game{Category: updatedGame.Category})
	}
	if updatedGame.Price != 0 {
		tx.Model(&game).Updates(Game{Price: updatedGame.Price})
	}
	if updatedGame.Name != "" {
		tx.Model(&game).Updates(Game{Name: updatedGame.Name})
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
