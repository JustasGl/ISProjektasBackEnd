package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type Rating struct {
	ID          uint       `json: "-" gorm:"primary_key"`
	Score       int        `json: "Score"`
	Comment     string     `json: "Comment"`
	CreatedAt   time.Time  `json: "-"`
	UpdatedAt   time.Time  `json: "-"`
	DeletedAt   *time.Time `json: "-"`
	CreatorID   uint
	Creator     User `gorm:"foreignkey:CreatorID"`
	CreatorName string
	//Rating	   []Rating `gorm:"many2many:games_raited;"`
}

func CreateRating(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	// Get the user that is creating the rating
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var newRating Rating
	// Get rating data from json body
	err := json.NewDecoder(r.Body).Decode(&newRating)
	newRating.Creator = user
	newRating.CreatorName = user.Username

	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Create an association between creator_id and a users id
	db.Model(&newRating).AddForeignKey("creator_id", "users(id)", "RESTRICT", "RESTRICT")
	// Create rating
	if db.Create(&newRating).Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusCreated)
	JSONResponse(struct{}{}, w)
	return
}

func GetRatings(w http.ResponseWriter, r *http.Request) {
	// Gets filtering keys from url. e.x ?location=kaunas&creatorId=1
	keys := r.URL.Query()
	id := keys.Get("ID")
	creatorID := keys.Get("CreatorID")
	comment := keys.Get("Comment")

	var ratings []Rating

	// Preloads user and creator tables for use in rating response
	tx := db.Preload("Users").Preload("Creator")

	// If a certain tag is not null, it is used to filter ratings
	if comment != "" {
		tx = tx.Where("Comment = ?", comment)
	}
	if creatorID != "" {
		tx = tx.Where("creator_id = ?", creatorID)
	}
	if id != "" {
		tx = tx.Where("ID = ?", id)
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

func DeleteRaiting(w http.ResponseWriter, r *http.Request) {
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
	db.Preload("Users").Where("id = ?", ratingID).First(&rating)

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

func EditRaiting(w http.ResponseWriter, r *http.Request) {
	//Loads creator id from authentication token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	userID := session.Values["userID"].(uint)

	//Gets id from /events/{id}
	params := mux.Vars(r)
	ratingID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Loads rating with joined users preloaded
	var rating Rating
	tx := db.Preload("Users").Where("id = ?", ratingID).First(&rating)

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

	// //Edits the record in database
	// if tx.Model(&rating).Updates(Rating{Description: updatedRating.Description}).RowsAffected == 0 {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	JSONResponse(struct{}{}, w)
	// 	return
	// }

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
