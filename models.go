package main

import (
	"github.com/meinside/openai-go"
	"github.com/recoilme/pudge"
	tele "gopkg.in/telebot.v3"
)

type SteamGame struct {
	Type                string `json:"type"`
	Name                string `json:"name"`
	AppID               int    `json:"steam_appid"`
	IsFree              bool   `json:"is_free"`
	DetailedDescription string `json:"detailed_description"`
	AboutTheGame        string `json:"about_the_game"`
	ShortDescription    string `json:"short_description"`
	HeaderImage         string `json:"header_image"`
	Website             string `json:"website"`
	Price               struct {
		Currency         string `json:"currency"`
		Initial          int    `json:"initial"`
		Final            int    `json:"final"`
		DiscountPercent  int    `json:"discount_percent"`
		InitialFormatted string `json:"initial_formatted"`
		FinalFormatted   string `json:"final_formatted"`
	} `json:"price_overview"`
	Platforms struct {
		Windows bool `json:"windows"`
		Mac     bool `json:"mac"`
		Linux   bool `json:"linux"`
	} `json:"platforms"`
	Metacritic struct {
		Score int `json:"score"`
	} `json:"metacritic"`
	Categories []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"categories"`
	Genres []struct {
		Description string `json:"description"`
	} `json:"genres"`
	Screenshots []struct {
		ID            int    `json:"id"`
		PathThumbnail string `json:"path_thumbnail"`
		PathFull      string `json:"path_full"`
	} `json:"screenshots"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
}

type SearchResult []struct {
	AppID string `json:"appid"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Logo  string `json:"logo"`
}

type SteamDetails map[string]struct {
	Data    *SteamGame `json:"data"`
	Success bool       `json:"success"`
}

type Server struct {
	conf  config
	users map[string]bool
	ai    *openai.Client
	bot   *tele.Bot
	db    *pudge.Db
}

type Event struct {
	ID       int64
	Name     string `json:"name,omitempty"`
	Time     int64  `json:"time,omitempty"`
	Location string `json:"location,omitempty"`
}

type WorldBoss struct {
	NextSpawn string  `json:"nextSpawn"`
	NextDiff  float64 `json:"nextDiff"`
	TimeLeft  float64 `json:"timeLeft"`
	Event     *Event  `json:"previousEvent"`
}

type User struct {
	TelegramID int64
	Subscribed bool
}
