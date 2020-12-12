package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/wader/gormstore"
)

// Global variables -------------------------------------------
var db *gorm.DB
var sessionStore *gormstore.Store
var passwordRegex *regexp.Regexp
var emailRegex *regexp.Regexp

// ------------------------------------------------------------
type envData struct {
	dbUsername string
	dbPassword string
	secret     []byte
}

//JSONResponse sends a json response to user based on message
func JSONResponse(response interface{}, w http.ResponseWriter) {

	json, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func GetEnvironmentVariables() (env envData, err error) {
	log.Println("Getting environment variables")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	cookieSecret := os.Getenv("COOKIE_SECRET")

	if dbUsername == "" || dbPassword == "" || cookieSecret == "" {
		return env, errors.New("Missing credentials, please check if the envriorement variables are set")
	}

	env = envData{dbUsername, dbPassword, []byte(cookieSecret)}

	return env, nil
}

func main() {
	//Regular expression for passwords to contain at least one capital letter and one number
	passwordRegex = regexp.MustCompile(`([A-Z].*=?)([0-9].*=?)|([0-9].*=?)([A-Z].*=?)`)
	emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	//Get database credentials and cookie store secret from environment variables
	var err error
	envData, err := GetEnvironmentVariables()

	if err != nil {
		log.Println(err)
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	log.Println("Opening database")
	dbString := fmt.Sprint(envData.dbUsername, ":", envData.dbPassword, "@tcp(localhost:3306)/isprojektasktu2020?parseTime=true")
	db, err = gorm.Open("mysql", dbString)

	if err != nil {
		log.Println(err.Error())
	}

	if !db.HasTable(&User{}) {
		db.CreateTable(&User{})
	}
	if !db.HasTable(&Game{}) {
		db.CreateTable(&Game{})
	}
	if !db.HasTable(&Rating{}) {
		db.CreateTable(&Rating{})
	}
	//Creates a table in the database for storing sessions
	//and sets a cleanup time
	sessionStore = gormstore.New(db, []byte(envData.secret))
	quit := make(chan struct{})
	go sessionStore.PeriodicCleanup(time.Minute, quit)

	//Handles the requests and redirects them to functions
	HandleFunctions()
}
