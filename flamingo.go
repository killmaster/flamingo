package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/killmaster/RiotGo"
	"github.com/kr/pretty"
	_ "github.com/mattn/go-sqlite3"
	forecast "github.com/mlbright/forecast/v2"
	"github.com/shkh/lastfm-go/lastfm"
	"github.com/thoj/go-ircevent"
	google "golang.org/x/blog/content/context/google"
	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
)

type Configuration struct {
	Nick          string
	User          string
	Server        string
	Password      string
	Channel       string
	ApiKey        string
	ApiSecret     string
	RiotApiKey    string
	WeatherApiKey string
	MapsApiKey    string
}

var (
	nick          string
	user          string
	server        string
	channel       string
	riotUrl       string
	riotApiKey    string
	weatherApiKey string
	mapsApiKey    string
	api           *lastfm.Api
	mapsClient    *maps.Client
	db            *sql.DB
	irccon        *irc.Connection = irc.IRC(nick, user)
	altera        []string
	choppa        []string
	strongLines   []string
	respostas     []string
	legMode       bool
)

func main() {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}

	reload()
	nick = configuration.Nick
	user = configuration.User
	server = configuration.Server
	channel = configuration.Channel
	riotUrl = "https://euw.api.pvp.net/api/lol/euw/"
	riotApiKey = configuration.RiotApiKey
	weatherApiKey = configuration.WeatherApiKey
	mapsApiKey = configuration.MapsApiKey
	legMode = false

	fmt.Println("Server: " + server)
	fmt.Println("Nick: " + nick)
	fmt.Println("User: " + user)
	fmt.Println("channel: " + channel)
	fmt.Println("API Key: " + configuration.ApiKey)
	fmt.Println("API Secret: " + configuration.ApiKey)

	api = lastfm.New(configuration.ApiKey, configuration.ApiSecret)

	mapsClient, _ = maps.NewClient(maps.WithAPIKey(mapsApiKey))

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
		if strings.EqualFold("Rick971", e.Nick) && !legMode {
			reply = "Bom dia Henrique"
		} else if strings.EqualFold("altera", e.Nick) {
			reply = altera[rand.Intn(len(altera))]
		} else if CaseInsensitiveContains(e.Nick, "chop") && !legMode {
			chosen := choppa[rand.Intn(len(choppa))]
			//fmt.Println(chosen)
			reply = fmt.Sprintf(chosen, e.Nick)
		} else if CaseInsensitiveContains(e.Nick, "stron") {
			chosen := strongLines[rand.Intn(len(strongLines))]
			reply = chosen
		} else if strings.EqualFold(nick, e.Nick) && !legMode {
			reply = "Bom dia"
		} else if !legMode {
			reply = "Bom dia " + e.Nick
		}
		go botSay(reply)
	})

	irccon.AddCallback("KICK", func(e *irc.Event) {
		time.Sleep(1000 * time.Millisecond)
		irccon.Join(channel)
	})

	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		if CaseInsensitiveContains(e.Nick, "Flamingo") {
			return
		}
		message := e.Message()
		req := strings.Split(message, " ")
		if req[0] == ".np" && !legMode {
			nowPlaying(req, e)
		} else if req[0] == ".lfmset" {
			lfmSet(req, e)
		} else if req[0] == ".compare" && !legMode {
			lfmCompare(req, e)
		} else if req[0] == ".lolset" {
			lolSet(req, e)
		} else if req[0] == ".sum" {
			lolSummoner(req, e)
		} else if req[0] == ".w" && !legMode {
			weather(req, e)
		} else if req[0] == ".reload" {
			reloadCall(req, e)
		} else if req[0] == ".strong" {
			strongismos(req, e)
		} else if req[0] == ".leg" {
			legModeToggle(req, e)
		} else if CaseInsensitiveContains(message, "flamingo") && !legMode {
			respostasCall(req, e)
		} /* else if req[0] == ".g" {
			googleSearch(message, e)
		}*/
	})

	irccon.Loop()
}

func nowPlaying(req []string, e *irc.Event) {
	var reply string
	var nick string
	if len(req) > 1 {
		nick = req[1]
	} else {
		nick = e.Nick
	}
	rows, err := db.Query("select lastfm from users where nick = '" + nick + "'")
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
				reply = nick + " está agora a ouvir " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
			} else {
				reply = nick + " ouviu " + result.Tracks[0].Artist.Name + " - " + result.Tracks[0].Name
			}
		}
	}
	go botSay(reply)
}

func lfmSet(req []string, e *irc.Event) {
	var reply string
	if len(req) < 2 {
		reply = "E o username ó palhaço?!"
	} else {
		_, err := db.Exec("insert or replace into users(nick,lastfm) values('" + e.Nick + "','" + req[1] + "')")
		if err != nil {
			reply = "Não quero"
		} else {
			reply = "Tá guardado"
		}
	}
	go botSay(reply)
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
	go botSay(reply)

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
	go botSay(reply)
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
	go botSay(reply)
}

func weather(req []string, e *irc.Event) {
	var reply string
	if len(req) > 1 {
		r := &maps.GeocodingRequest{
			Address: req[1],
		}
		resp, _ := mapsClient.Geocode(context.Background(), r)
		//pretty.Println(resp)
		lat := strconv.FormatFloat(resp[0].Geometry.Location.Lat, 'f', -1, 64)
		lng := strconv.FormatFloat(resp[0].Geometry.Location.Lng, 'f', -1, 64)
		f, err := forecast.Get(weatherApiKey, lat, lng, "now", forecast.CA)
		if err != nil {
			log.Fatal(err)
		}
		reply = fmt.Sprintf("Weather in %s: %s, Humidity: %.2f, Temperature: %.2f Celsius, Wind Speed: %.2f", req[1], f.Currently.Summary, f.Currently.Humidity, f.Currently.Temperature, f.Currently.WindSpeed)
		go botSay(reply)
	}
}

func strongismos(req []string, e *irc.Event) {
	var reply string
	indexes := rand.Perm(len(strongLines))
	for i := 0; i < 7; i++ {
		reply += strongLines[indexes[i]]
		reply += " "
	}
	go botSay(reply)
}

func googleSearch(req string, e *irc.Event) {
	query := req[3:]
	pretty.Println(query)
	results, err := google.Search(context.Background(), query)
	pretty.Println(results)
	if err == nil {
		limit := len(results)
		if limit <= 0 {
			irccon.Privmsg(channel, "Não quero")
			return
		}
		for i := 0; i < limit; i++ {
			irccon.Privmsg(channel, results[i].Title)
			irccon.Privmsg(channel, results[i].URL)
		}

	} else {
		irccon.Privmsg(channel, "Hmmmm não me apetece")
	}
}

func respostasCall(req []string, e *irc.Event) {
	var reply string
	chosen := respostas[rand.Intn(len(respostas))]
	reply = fmt.Sprintf(chosen, e.Nick)
	go botSay(reply)
}

func reloadCall(req []string, e *irc.Event) {
	var reply string
	if adminCheck(e.Nick) {
		reload()
		reply = "Done"
	} else {
		reply = "Não mandas em mim. Eu faço o que eu quero."
	}
	go botSay(reply)
}

func legModeToggle(req []string, e *irc.Event) {
	var reply string
	if adminCheck(e.Nick) || strings.EqualFold("leg", e.Nick) {
		legMode = !legMode
		if legMode {
			reply = "leg mode: on"
		} else {
			reply = "leg mode: off"
		}
	}
	go botSay(reply)
}

func adminCheck(nick string) bool {
	if strings.EqualFold("killmaster", nick) {
		return true
	} else {
		return false
	}
}

func CaseInsensitiveContains(s, substr string) bool {
	s, substr = strings.ToUpper(s), strings.ToUpper(substr)
	return strings.Contains(s, substr)
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func reload() {
	fmt.Println("Loading strongismos")
	strongLines, _ = readLines("strongismos.txt")
	fmt.Println("Loading altera's greetings")
	altera, _ = readLines("altera.txt")
	fmt.Println("Loading choppa's greetings")
	choppa, _ = readLines("choppa.txt")
	fmt.Println("Loading respostas")
	respostas, _ = readLines("respostas.txt")

}

/* Use as a goroutine */
func botSay(text string) {
	waitTime := rand.Intn(4)
	time.Sleep(time.Duration(waitTime*1000) * time.Millisecond)
	irccon.Privmsg(channel, text)
}
