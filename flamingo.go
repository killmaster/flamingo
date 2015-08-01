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
	nick      string
	user      string
	server    string
	password  string
	channel   string
	apiKey    string
	apiSecret string
}

func main() {
	file, err := os.Open("conf.json")
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		log.Fatal(err)
	}

	nick := configuration.nick
	user := configuration.user
	server := configuration.server
	channel := configuration.channel

	api := lastfm.New(configuration.apiKey, configuration.apiSecret)
	db, err := sql.Open("sqlite3", "./flamingo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	irccon := irc.IRC(nick, user)
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
		irccon.Privmsg("NickServ", "identify "+configuration.password)
		fmt.Println("Identified")
		irccon.Join(channel)
	})

	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		req := strings.Split(e.Message(), " ")
		if req[0] == "-np" {
			var reply string
			if len(req) > 1 {
				result, err := api.User.GetRecentTracks(lastfm.P{"user": req[1]})
				if err != nil {
					reply = "Não quero"
				} else if result.Tracks != nil {
					if result.Tracks[0].NowPlaying == "true" {
						reply = req[1] + " está agora a ouvir " +
							result.Tracks[0].Artist.Name +
							" - " + result.Tracks[0].Name
					} else {
						reply = req[1] + " ouviu " + result.Tracks[0].Artist.Name +
							" - " + result.Tracks[0].Name
					}
				}
				reply = "Não quero fdp, deslarga-me"
			} else {
				rows, err := db.Query("select lastfm from users where nick = '" + e.Nick + "'")
				var lfm string
				rows.Next()
				rows.Scan(&lfm)
				if err != nil {
					log.Fatal(err)
				}
				if lfm == "" {
					reply = "Não sei qual é o teu username"
				} else {
					result, err := api.User.GetRecentTracks(lastfm.P{"user": lfm})
					if err != nil {
						reply = "Não quero"
					} else {
						if result.Tracks[0].NowPlaying == "true" {
							reply = e.Nick + " está agora a ouvir " +
								result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
						} else {
							reply = e.Nick + " ouviu " +
								result.Tracks[0].Artist.Name +
								" - " + result.Tracks[0].Name
						}
					}
				}
			}
			irccon.Privmsg(channel, reply)
		}
	})

	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		req := strings.Split(e.Message(), " ")
		if req[0] == "-lfmset" {
			var reply string
			if len(req) < 2 {
				reply = "E o username?!"
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
	})

	irccon.AddCallback("JOIN", func(e *irc.Event) {
		var reply string
		if strings.EqualFold("Rick971", e.Nick) {
			reply = "Baza daqui ilheu!"
		} else {
			reply = "Bom dia " + e.Nick
		}
		irccon.Privmsg(channel, reply)
	})

	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		req := strings.Split(e.Message(), " ")
		var user1 string
		var user2 string
		var reply string
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
			score, _ := strconv.Atoi(strconv.ParseFloat(result.Result.Score, 64) * 100)
			reply := user1 + " tem " + strconv.Itoa(score) + "% de compatibilidade com " + user2 + "."
		} else if len(req) > 1 {
			rows, err := db.Query("select lastfm from users where nick = '" + e.Nick + "'")
			if err != nil {
				log.Fatal(err)
			}
			rows.Next()
			rows.Scan(&user1)
			rows, err := db.Query("select lastfm from users where nick = '" + req[1] + "'")
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
		} else {
			reply = "Deslarga-me!"
		}
		irccon.Privmsg(channel, reply)
	})

	irccon.AddCallback("KICK", func(e *irc.Event) {
		time.Sleep(1000 * time.Millisecond)
		irccon.Join(channel)
	})

	irccon.Loop()
}
