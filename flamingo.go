package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shkh/lastfm-go/lastfm"
	"github.com/thoj/go-ircevent"
)

type Configuration struct {
	Nick      string
	User      string
	Server    string
	Password  string
	Channel   string
	ApiKey    string
	ApiSecret string
}

var nick string
var user string
var server string
var channel string
var api *lastfm.Api
var db *sql.DB
var irccon *irc.Connection = irc.IRC(nick, user)

func main() {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}

	nick = configuration.Nick
	user = configuration.User
	server = configuration.Server
	channel = configuration.Channel

	fmt.Println("Server: " + server)
	fmt.Println("Nick: " + nick)
	fmt.Println("User: " + user)
	fmt.Println("channel: " + channel)
	fmt.Println("API Key: " + configuration.ApiKey)
	fmt.Println("API Secret: " + configuration.ApiKey)

	api = lastfm.New(configuration.ApiKey, configuration.ApiSecret)

	db, err = sql.Open("sqlite3", "./flamingo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	irccon = irc.IRC(nick, user)
	//irccon.Debug = true
	//irccon.VerboseCallbackHandler = true
	err = irccon.Connect(server)
	if err != nil {
		log.Fatal(err.Error())
		fmt.Println("Não me consegui ligar ao server")
	}

	irccon.AddCallback("001", func(e *irc.Event) {
		time.Sleep(5000 * time.Millisecond)
		fmt.Println("Identifying")
		irccon.Privmsg("NickServ", "identify "+configuration.Password)
		fmt.Println("Identified")
		irccon.Join(channel)
	})

	irccon.AddCallback("JOIN", func(e *irc.Event) {
		var reply string
		if strings.EqualFold("Rick971", e.Nick) {
			reply = "Bom dia Henrique"
		} else {
			reply = "Bom dia " + e.Nick
		}
		irccon.Privmsg(channel, reply)
	})

	irccon.AddCallback("KICK", func(e *irc.Event) {
		time.Sleep(1000 * time.Millisecond)
		irccon.Join(channel)
	})

	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		req := strings.Split(e.Message(), " ")
		if req[0] == "-np" {
			nowPlaying(req, e)
		} else if req[0] == "-lfmset" {
			lfmSet(req, e)
		} else if req[0] == "-compare" {
			lfmCompare(req, e)
		}
	})

	irccon.Loop()
}

func nowPlaying(req []string, e *irc.Event) {
	var reply string
	if len(req) > 1 {
		result, err := api.User.GetRecentTracks(lastfm.P{"user": req[1]})
		if err != nil {
			reply = "Não quero"
		} else if result.Tracks != nil {
			if result.Tracks[0].NowPlaying == "true" {
				reply = req[1] + " está agora a ouvir " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
			} else {
				reply = req[1] + " ouviu " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
			}
		}
		reply = "Não quero fdp, deslarga-me crlh"
	} else {
		rows, err := db.Query("select lastfm from users where nick = '" + e.Nick + "'")
		var lfm string
		rows.Next()
		rows.Scan(&lfm)
		if err != nil {
			log.Fatal(err)
		}
		if lfm == "" {
			reply = "Não sei qual é o teu username fdp"
		} else {
			result, err := api.User.GetRecentTracks(lastfm.P{"user": lfm})
			if err != nil {
				reply = "Não quero"
			} else {
				if result.Tracks[0].NowPlaying == "true" {
					reply = e.Nick + " está agora a ouvir " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
				} else {
					reply = e.Nick + " ouviu " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
				}
			}
		}
	}
	irccon.Privmsg(channel, reply)
}

func lfmSet(req []string, e *irc.Event) {
	var reply string
	if len(req) < 2 {
		reply = "E o username ó palhaço?!"
	} else {
		_, err := db.Exec("insert into users(nick,lastfm) values('" + e.Nick + "','" + req[1] + "')")
		if err != nil {
			reply = "Não quero"
		} else {
			reply = "Tá guardado"
		}
	}
	irccon.Privmsg(channel, reply)
}

func lfmCompare(req []string, e *irc.Event) {
	var user1 string
	var user2 string
	reply := "Não mandas em mim"
	if len(req) > 2 {
		rows, err := db.Query("select lastfm from users where nick = '" + req[1] + "'")
		if err != nil {
			log.Fatal(err)
		}
		rows.Next()
		rows.Scan(&user1)
		rows, err = db.Query("select lastfm from users where nick = '" + req[2] + "'")
		if err != nil {
			log.Fatal(err)
		}
		rows.Next()
		rows.Scan(&user2)

		result, err := api.Tasteometer.Compare(lastfm.P{"type1": "user",
			"type2":  "user",
			"value1": user1,
			"value2": user2})
		if err != nil {
			reply = "Não quero"
		}
		percent, _ := strconv.ParseFloat(result.Result.Score, 64)
		score := int(percent * 100)
		reply = req[1] + " tem " + strconv.Itoa(score) + "% de compatibilidade com " + req[2] + "."
	} else if len(req) > 1 {
		rows, err := db.Query("select lastfm from users where nick = '" + e.Nick + "'")
		if err != nil {
			log.Fatal(err)
		}
		rows.Next()
		rows.Scan(&user1)
		rows, err = db.Query("select lastfm from users where nick = '" + req[1] + "'")
		if err != nil {
			log.Fatal(err)
		}
		rows.Next()
		rows.Scan(&user2)
		result, err := api.Tasteometer.Compare(lastfm.P{"type1": "user",
			"type2":  "user",
			"value1": user1,
			"value2": user2})
		if err != nil {
			reply = "Não quero"
		}
		percent, _ := strconv.ParseFloat(result.Result.Score, 64)
		score := int(percent * 100)
		reply = e.Nick + " tem " + strconv.Itoa(score) + "% de compatibilidade com " + req[1] + "."
	} else {
		reply = "Deslarga-me!"
	}
	irccon.Privmsg(channel, reply)

}
