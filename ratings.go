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

type Rating struct {
	ID           uint       `json: "-" gorm:"primary_key"`
	Score        float64    `json: "Score"`
	Comment      string     `json: "Comment"`
	CreatedAt    time.Time  `json: "-"`
	UpdatedAt    time.Time  `json: "-"`
	DeletedAt    *time.Time `json: "-"`
	CreatorID    uint
	Creator      User `gorm:"foreignkey:CreatorID"`
	CreatorEmail string
	GameID       uint
	Game         Game `gorm:"foreignkey:GameID"`
	GameName     string
}

func Rate(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	// Get the user that is creating the rating
	var user User
	db.First(&user, session.Values["userID"].(uint))
	fmt.Println("User ID is ", user.ID)
	//Gets game id from /Rate/{id}
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	var game Game
	db.Raw("Select * from games where id = ?", gameID).Scan(&game)
	fmt.Println("game ID is ", game.ID)

	Rate := struct {
		Comment string  `json: "Comment"`
		Score   float64 `json: "Score"`
	}{"", 0}
	err = json.NewDecoder(r.Body).Decode(&Rate)

	if err != nil {
		JSONResponse(struct{}{}, w)
		return
	}
	newRating := Rating{
		Comment:      Rate.Comment,
		Score:        Rate.Score,
		CreatorID:    user.ID,
		Creator:      user,
		CreatorEmail: user.Email,
		GameID:       game.ID,
		GameName:     game.Name,
	}
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	CheckUserID := int(user.ID)
	ChecKGameID := int(game.ID)
	if CheckUserID == 0 || ChecKGameID == 0 {
		fmt.Println(" Game Id is ", game.ID, " User id is ", user.ID)
		w.WriteHeader(http.StatusNotFound)
		JSONResponse(struct{}{}, w)
		return
	}

	if db.Create(&newRating).Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusCreated)
	JSONResponse(&newRating, w)
	return
}

func GetRatings(w http.ResponseWriter, r *http.Request) {
	setupCorsResponse(&w, r)
	// Gets filtering keys from url. e.x ?comment=Puikus&creatorID=1&GameID=1
	keys := r.URL.Query()
	id := keys.Get("ID")
	creatorID := keys.Get("CreatorID")
	comment := keys.Get("Comment")
	GameID := keys.Get("GameID")
	RatingFrom := keys.Get("From")
	RatingTo := keys.Get("To")
	var ratings []Rating

	// Preloads user and creator tables for use in rating response
	tx := db.Preload("Game").Preload("Creator")

	// If a certain tag is not null, it is used to filter ratings
	if comment != "" {
		tx = tx.Where("Comment like ?", comment)
	}
	if creatorID != "" {
		tx = tx.Where("creator_id = ?", creatorID)
	}
	if id != "" {
		tx = tx.Where("ID = ?", id)
	}
	if GameID != "" {
		tx = tx.Where("game_id = ?", GameID)
	}
	if RatingTo != "" {
		tx = tx.Where("Score < ?", RatingTo)
	}
	if RatingFrom != "" {
		tx = tx.Where("Score > ?", RatingFrom)
	}
	// Finds ratings based on given parameters
	tx.Find(&ratings)

	// If no ratings exist, return Bad request
	if len(ratings) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(ratings, w)
	return
}

func DeleteRating(w http.ResponseWriter, r *http.Request) {
	//Loads creator id from authentication token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	userID := session.Values["userID"].(uint)

	//Gets id from /ratings/{id}
	params := mux.Vars(r)
	ratingID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Loads rating with joined users preloaded
	var rating Rating
	db.Where("id = ?", ratingID).First(&rating)

	//checks if the user that is trying to delete rating is its creator
	if rating.CreatorID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Deletes the record from database
	if db.Unscoped().Delete(&rating).RecordNotFound() {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Deletes associations (users that bought the rating)
	//db.Model(&rating).Association("Users").Delete(&rating.BoughtList)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

func EditRating(w http.ResponseWriter, r *http.Request) {
	//Loads creator id from authentication token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	userID := session.Values["userID"].(uint)

	//Gets id from /rate/{id}
	params := mux.Vars(r)
	ratingID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Loads rating with joined users preloaded
	var rating Rating
	tx := db.Where("id = ?", ratingID).First(&rating)

	//checks if the user that is trying to delete rating is its creator
	if rating.CreatorID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	var updatedRating Rating
	json.NewDecoder(r.Body).Decode(&updatedRating)

	if updatedRating.Score != 0 {
		tx.Model(&rating).Updates(Rating{Score: updatedRating.Score})
	}
	if updatedRating.Comment != "" {
		tx.Model(&rating).Updates(Rating{Comment: updatedRating.Comment})
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
