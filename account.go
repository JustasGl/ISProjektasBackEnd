package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/pbkdf2"
)

type User struct {
	ID          uint       `gorm:"primary_key"`
	Name        string     `gorm:"size:20"`
	LastName    string     `gorm:"size:40"`
	Email       string     `gorm:"size:50;not null"`
	Password    string     `json:"-" gorm:"not null"`
	Username    string     `gorm:"size:30"`
	DateOfBirth string     `gorm:"size:10"`
	Address     string     `gorm:"size:50"`
	PostalCode  string     `gorm:"size:10"`
	Role        int        `gorm:"not null"`
	CreatedAt   time.Time  `json:"CreatedAt"`
	UpdatedAt   time.Time  `json:"-"`
	DeletedAt   *time.Time `json:"-"`
	Following   int        `json:"FollowingAmount"`
	Followers   int        `json:"FollowersAmount"`

	Salt       string  `json:"-" gorm:"size:64;not null"`
	Cart       []Game  `gorm:"many2many:Cart;"`
	Follow     []User  `gorm:"many2many:Follow;association_jointable_foreignkey:follower_id"`
	BoughtList []*Game `gorm:"many2many:games_bought;"`

	WishList []Game `gorm:"many2many:WishList;"`
}

//RegisterPageHandler decodes user sent in data, verifies that
//it is formatted correctly, and tries to create an account in
//the database
func RegisterNewAccount(w http.ResponseWriter, r *http.Request) {
	//Creates a struct used to store data decoded from the body
	user := struct {
		Name           string `json: "Name"`
		LastName       string `json: "LastName"`
		Email          string `json: "Email"`
		Password       string `json: "Password"`
		RepeatPassword string `json: "RepeatPassword"`
		Username       string `json: "Username"`
		DateOfBirth    string `json: "DateOfBirth"`
		Address        string `json: "Address"`
		PostalCode     string `json: "PostalCode"`
		Role           int    `json: "Role"`
	}{"", "", "", "", "", "", "", "", "", 0}

	err := json.NewDecoder(r.Body).Decode(&user)

	res, err := PerformUserDataChecks(user.Email, user.Password, user.RepeatPassword)

	w.WriteHeader(res)

	if err != nil {
		JSONResponse(struct{}{}, w)
		return
	}

	salt := GenerateSalt()
	hashedPassword := GenerateSecurePassword(user.Password, salt)

	newUser := User{
		Name:        user.Name,
		LastName:    user.LastName,
		Email:       user.Email,
		Password:    hashedPassword,
		Username:    user.Username,
		DateOfBirth: user.DateOfBirth,
		Address:     user.Address,
		PostalCode:  user.PostalCode,
		Role:        user.Role,
		Salt:        salt,
	}
	db.Debug().Create(&newUser)
	db.Save(&newUser)

	JSONResponse(struct{}{}, w)
	return
}

func Login(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	//Creates a struct used to store data decoded from the body
	userRequestData := struct {
		Email    string `json: "Email"`
		Password string `json: "Password"`
	}{"", ""}

	json.NewDecoder(r.Body).Decode(&userRequestData)

	var userDatabaseData User

	// Finds user by email in database, if no user, then returns "bad request"
	if db.Find(&userDatabaseData, "Email = ?", userRequestData.Email).RecordNotFound() {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	hashedPassword := GenerateSecurePassword(userRequestData.Password, userDatabaseData.Salt)
	//checks if salted hashed password from database matches the sent in salted hashed password
	if hashedPassword != userDatabaseData.Password {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	session = CreateAccessToken(userDatabaseData, session)
	session.Save(r, w)

	w.WriteHeader(http.StatusAccepted)
	JSONResponse(struct{}{}, w)
	return
}

func GetAccountInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	keys := r.URL.Query()
	id := keys.Get("id")

	var user User

	if id != "" {
		db.First(&user, id)
	} else if session.Values["userID"] != nil {
		db.First(&user, session.Values["userID"].(uint))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	JSONResponse(user, w)
	w.WriteHeader(http.StatusOK)
	return
}

func EditPassword(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	passwordData := struct {
		Password          string `json: "password"`
		NewPassword       string `json: "newPassword"`
		NewPasswordRepeat string `json: "newPasswordRepeat"`
	}{"", "", ""}

	json.NewDecoder(r.Body).Decode(&passwordData)

	var user User
	// Finds user by id in database, if no user, then returns "bad request"
	if db.Find(&user, "id = ?", session.Values["userID"]).RecordNotFound() {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	hashedPassword := GenerateSecurePassword(passwordData.Password, user.Salt)
	//checks if sent in password matches the database stored password
	if hashedPassword != user.Password {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//checks newPassword and newPasswordRepeat are the same
	err := ComparePasswords(passwordData.NewPassword, passwordData.NewPasswordRepeat)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Hashes new password and puts it in user
	newPassword := GenerateSecurePassword(passwordData.NewPassword, user.Salt)
	db.Model(&user).Updates(User{Password: newPassword})

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

func EditAccountInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	var user User
	tx := db.Where("id = ?", session.Values["userID"])

	var updatedUser User
	json.NewDecoder(r.Body).Decode(&updatedUser)

	if updatedUser.Username != "" {
		tx.Model(&user).Updates(User{Username: updatedUser.Username})
	}
	if updatedUser.Name != "" {
		tx.Model(&user).Updates(User{Name: updatedUser.Name})
	}
	if updatedUser.LastName != "" {
		tx.Model(&user).Updates(User{LastName: updatedUser.LastName})
	}
	if updatedUser.Email != "" {
		tx.Model(&user).Updates(User{Email: updatedUser.Email})
	}
	if updatedUser.DateOfBirth != "" {
		tx.Model(&user).Updates(User{DateOfBirth: updatedUser.DateOfBirth})
	}
	if updatedUser.Address != "" {
		tx.Model(&user).Updates(User{Address: updatedUser.Address})
	}
	if updatedUser.PostalCode != "" {
		tx.Model(&user).Updates(User{PostalCode: updatedUser.PostalCode})
	}
	if updatedUser.Role != -1 {
		tx.Model(&user).Updates(User{Role: updatedUser.Role})
	}
	tx.First(&user)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

//GenerateSalt creates a pseudorandom salt used in password salting
func GenerateSalt() string {
	salt, _ := uuid.NewV4()

	return hex.EncodeToString(salt.Bytes())
}

//GenerateSecurePassword generates a password using PBKDF2 standard
func GenerateSecurePassword(password string, salt string) string {
	hashedPassword := pbkdf2.Key([]byte(password), []byte(salt), 4096, 32, sha1.New)

	return hex.EncodeToString(hashedPassword)
}

//CheckNameAvailability checks if a username is available
func CheckEmailAvailability(email string) error {
	var user User

	//if no record of the email is found, returns an error
	if !db.Find(&user, "email = ?", email).RecordNotFound() {
		return errors.New("Email exists")
	}

	return nil
}

//CreateNewAccount creates an account if the sent data
//is correctly formatted
func PerformUserDataChecks(email string, password string, repeatedPassword string) (httpStatus int, err error) {
	if emailRegex.MatchString(email) != true {
		return http.StatusNotAcceptable, errors.New("Bad email format")
	}

	err = CheckEmailAvailability(email)
	if err != nil {
		return http.StatusNotAcceptable, err
	}

	err = ComparePasswords(password, repeatedPassword)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return http.StatusOK, nil
}

//ComparePasswords checks that, while registering a new account,
//the password matches the repeated password, is atleast 8 characters long and
//contains at least one number and one capital letter
func ComparePasswords(passwordOne string, passwordTwo string) error {
	if passwordOne != passwordTwo {
		return errors.New("Passwords do not match")
	}

	if len(passwordOne) < 8 {
		return errors.New("Passwords too short")
	}

	if passwordRegex.MatchString(passwordOne) != true {
		return errors.New("Passwords needs to contain at least one number and one capital letter")
	}

	return nil
}

func CreateAccessToken(user User, session *sessions.Session) *sessions.Session {
	//Access-token values
	session.Values["userID"] = user.ID
	session.Options.MaxAge = 60 * 60 * 24
	session.Options.HttpOnly = true
	return session
}

func IsLoggedIn(w http.ResponseWriter, r *http.Request) {
	session, err := sessionStore.Get(r, "Access-token")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}

func Logout(w http.ResponseWriter, r *http.Request) {
	sessionAccess, err := sessionStore.Get(r, "Access-token")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	sessionRefresh, err := sessionStore.Get(r, "Refresh-token")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	sessionAccess.Options.MaxAge = -1
	sessionRefresh.Options.MaxAge = -1

	sessionAccess.Save(r, w)
	sessionRefresh.Save(r, w)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func FollowUser(w http.ResponseWriter, r *http.Request) {
	//Get user id from auth token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Gets id from /follow/{id}
	params := mux.Vars(r)
	usrID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Get users
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var selectedUser User
	db.First(&selectedUser, usrID)
	fmt.Printf("User  %d wants to follow user %d \n", session.Values["userID"].(uint), usrID)

	//Check if both users exist
	if selectedUser.ID == 0 || user.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Check if user is not tryng to follow himself
	if user.ID == selectedUser.ID {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	//Check if current user doesint already follow the selected user
	var CheckUser User
	database := db.Raw("Select user_id as ID from Follow where user_id = ? and follower_id = ?", selectedUser.ID, user.ID).Scan(&CheckUser)

	if database.RowsAffected != 0 {
		w.WriteHeader(http.StatusAlreadyReported)
		JSONResponse(struct{}{}, w)
		return
	}

	//Add user to followings
	db.Model(&selectedUser).Association("Follow").Append(&user)
	db.Model(&selectedUser).Updates(User{Followers: selectedUser.Followers + 1})
	db.Model(&user).Updates(User{Following: user.Following + 1})

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func UnfollowUser(w http.ResponseWriter, r *http.Request) {
	//Get user id from auth token
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Gets id from /follow/{id}
	params := mux.Vars(r)
	usrID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Get users from provided IDs
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var selectedUser User
	db.First(&selectedUser, usrID)
	//db.Preload("Follow").First(&selectedUser, "id = ?", userID)

	//Check if both users exist
	if selectedUser.ID == 0 || user.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Check if user is not himself
	if user.ID == selectedUser.ID {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	var CheckUser User
	database := db.Raw("Select user_id as ID from Follow where user_id = ? and follower_id = ?", selectedUser.ID, user.ID).Scan(&CheckUser)

	if database.RowsAffected == 0 {
		w.WriteHeader(http.StatusAlreadyReported)
		JSONResponse(struct{}{}, w)
		return
	}
	// Delete user from follow

	db.Exec("UPDATE Users SET Followers=? WHERE ID = ?", selectedUser.Followers-1, selectedUser.ID)
	db.Model(&user).Updates(User{Following: user.Following - 1})
	db.Model(&selectedUser).Association("Follow").Delete(&user)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func GetFollowers(w http.ResponseWriter, r *http.Request) {
	// Gets filtering keys from url. e.x ?location=kaunas&creatorId=1
	var users []User

	params := mux.Vars(r)
	usrID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	fmt.Printf("get %d user followers", usrID)
	// Preloads user and creator tables for use in game response

	// If a certain tag is not null, it is used to filter games
	if usrID != 0 {
		db.Raw("select * from users where id IN ((Select follower_id from Follow where user_id = ?))", usrID).Scan(&users)
	}
	// Finds users based on given parameters

	// If no users exist, return Bad request
	if len(users) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(users, w)
	return
}
func GetFollowings(w http.ResponseWriter, r *http.Request) {
	var users []User

	params := mux.Vars(r)
	usrID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	fmt.Printf("get %d user followings", usrID)
	// Preloads user and creator tables for use in game response

	// If a certain tag is not null, it is used to filter games
	if usrID != 0 {
		db.Raw("select * from users where id IN ((Select user_id from Follow where follower_id = ?))", usrID).Scan(&users)
	}

	// If no games exist, return Bad request
	if len(users) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(users, w)
	return
}
func AddCart(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Gets id from /follow/{id}
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Get users
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var game Game
	db.Raw("Select * from games where ID = ?", gameID).Scan(&game)

	//Check if both users exist
	if game.ID == 0 || user.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	db.Model(&user).Association("Cart").Append(&game)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func GetCart(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	var games []Game
	db.Raw("SELECT * from games WHERE ID IN (select game_id from cart WHERE user_id = ?)", session.Values["userID"]).Scan(&games)

	if len(games) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	w.WriteHeader(http.StatusOK)
	JSONResponse(games, w)
	return
}

func RemoveCart(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	var user User
	db.First(&user, session.Values["userID"].(uint))

	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	var game Game

	db.Raw("SELECT * from users where ID = (select user_id from Cart where game_id = ?)", gameID).Scan(&game)

	if game.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	db.Model(&user).Association("Cart").Delete(&game)
	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func AddWishList(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	//Gets id from /follow/{id}
	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	//Get users
	var user User
	db.First(&user, session.Values["userID"].(uint))

	var game Game
	db.Raw("Select * from games where ID = ?", gameID).Scan(&game)

	//Check if both users exist
	if game.ID == 0 || user.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	db.Model(&user).Association("WishList").Append(&game)

	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func GetWishList(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	var games []Game
	db.Raw("SELECT * from games WHERE ID IN (select game_id from WishList WHERE user_id = ?)", session.Values["userID"]).Scan(&games)

	if len(games) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	w.WriteHeader(http.StatusOK)
	JSONResponse(games, w)
	return
}

func RemoveWishList(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}

	var user User
	db.First(&user, session.Values["userID"].(uint))

	params := mux.Vars(r)
	gameID, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	var game Game

	db.Raw("SELECT * from users where ID = (select user_id from WishList where game_id = ?)", gameID).Scan(&game)

	if game.ID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	db.Model(&user).Association("WishList").Delete(&game)
	w.WriteHeader(http.StatusOK)
	JSONResponse(struct{}{}, w)
	return
}
func GetBoughtList(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "Access-token")

	if session.Values["userID"] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		JSONResponse(struct{}{}, w)
		return
	}
	var games []Game
	db.Raw("SELECT * from games WHERE ID IN (select game_id from games_bought WHERE user_id = ?)", session.Values["userID"]).Scan(&games)

	if len(games) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}
	w.WriteHeader(http.StatusOK)
	JSONResponse(games, w)
	return
}
func GetAccounts(w http.ResponseWriter, r *http.Request) {
	// Gets filtering keys from url. e.x ?location=kaunas&creatorId=1
	keys := r.URL.Query()
	FromFollowers := keys.Get("FromFollowers")
	FromFollowings := keys.Get("FromFollowings")
	ToFollowers := keys.Get("ToFollowers")
	ToFollowings := keys.Get("ToFollowings")
	var Users []User

	// Preloads user and creator tables for use in game response
	tx := db

	// If a certain tag is not null, it is used to filter games
	if FromFollowers != "" {
		tx = tx.Where("Followers > ?", FromFollowers)
	}
	if FromFollowings != "" {
		tx = tx.Where("Following > ?", FromFollowings)
	}
	if ToFollowers != "" {
		tx = tx.Where("Followers < ?", ToFollowers)
	}
	if ToFollowings != "" {
		tx = tx.Where("Following < ?", ToFollowings)
	}
	// Finds games based on given parameters
	tx.Find(&Users)

	// If no games exist, return Bad request
	if len(Users) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		JSONResponse(struct{}{}, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	JSONResponse(Users, w)
	return
}
