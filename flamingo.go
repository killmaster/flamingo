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

	"github.com/killmaster/RiotGo"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shkh/lastfm-go/lastfm"
	"github.com/thoj/go-ircevent"
)

type Configuration struct {
	Nick       string
	User       string
	Server     string
	Password   string
	Channel    string
	ApiKey     string
	ApiSecret  string
	RiotApiKey string
}

var (
	nick       string
	user       string
	server     string
	channel    string
	riotUrl    string
	riotApiKey string
	api        *lastfm.Api
	db         *sql.DB
	irccon     *irc.Connection = irc.IRC(nick, user)
)

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
	riotUrl = "https://euw.api.pvp.net/api/lol/euw/"
	riotApiKey = configuration.RiotApiKey

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

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users(nick TEXT PRIMARY KEY NOT NULL, lastfm TEXT NOT NULL)")
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("Last.fm table created successfully")
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS league(nick TEXT PRIMARY KEY NOT NULL, summoner TEXT NOT NULL, id TEXT NOT NULL)")
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("League of Legends table created successfully")
	}

	riot.SetAPIKey(riotApiKey)

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
		} else if req[0] == "-lolset" {
			lolSet(req, e)
		} else if req[0] == "-sum" {
			lolSummoner(req, e)
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

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func lolSet(req []string, e *irc.Event) {
	var reply string
	if len(req) < 2 {
		reply = "E o summoner name ó palhaço?!"
	} else {
		summoners, err := riot.SummonerByName(riot.EUW, req[1])
		if err != nil {
			reply = "Não quero fdp, deslarga-me crlh"
		} else {
			_, err := db.Exec("insert into league(nick,summoner,id) values('" + e.Nick + "','" + req[1] + "','" + strconv.FormatInt(summoners[req[1]].ID, 10) + "')")
			if err != nil {
				reply = "Não quero"
			} else {
				reply = "Tá guardado"
			}
		}
	}
	irccon.Privmsg(channel, reply)
}

func lolSummoner(req []string, e *irc.Event) {
	var reply string
	var id int64
	if len(req) > 1 {
		summoners, err := riot.SummonerByName(riot.EUW, req[1])
		if err != nil {
			reply = "Não quero fdp, deslarga-me crlh"
		}
		id = summoners[req[1]].ID
		leagues, err := riot.LeagueEntry(riot.EUW, id)
		if err != nil {
			reply = summoners[req[1]].Name + " | Level " + strconv.FormatInt(summoners[req[1]].SummonerLevel, 10)
		} else {
			reply = summoners[req[1]].Name + " | Level " + strconv.FormatInt(summoners[req[1]].SummonerLevel, 10) + " | " + leagues[id][0].Tier + " " + leagues[id][0].Entries[0].Division
		}
	} else {
		rows, err := db.Query("select summoner from league where nick = '" + e.Nick + "'")
		var summoner string
		rows.Next()
		rows.Scan(&summoner)
		rows, err = db.Query("select id from league where nick ='" + e.Nick + "'")
		var summonerID string
		rows.Next()
		rows.Scan(&summonerID)
		if err != nil {
			log.Fatal(err)
		}
		if summoner == "" {
			reply = "Não sei qual é o teu summoner name fdp"
		} else {
			summoners, err := riot.SummonerByName(riot.EUW, summoner)
			if err != nil {
				reply = "Não quero fdp, deslarga-me crlh"
			}
			id, _ := strconv.ParseInt(summonerID, 10, 64)
			leagues, err := riot.LeagueEntry(riot.EUW, id)
			if err != nil {
				reply = summoners[summoner].Name + " | Level " + strconv.FormatInt(summoners[summoner].SummonerLevel, 10)
			} else {
				reply = summoners[summoner].Name + " | Level " + strconv.FormatInt(summoners[summoner].SummonerLevel, 10) + " | " + leagues[id][0].Tier + " " + leagues[id][0].Entries[0].Division
			}
		}
	}
	irccon.Privmsg(channel, reply)
}
