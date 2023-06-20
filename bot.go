package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	openai "github.com/meinside/openai-go"
	tele "gopkg.in/telebot.v3"
)

const (
	intervalSeconds = 1

	cmdStart  = "/start"
	cmdSearch = "/search "
	msgStart  = "This bot will search games on steam"
)

// config struct for loading a configuration file
type config struct {
	// telegram bot api
	TelegramBotToken string `json:"telegram_bot_token"`

	// openai api
	OpenAIAPIKey         string `json:"openai_api_key"`
	OpenAIOrganizationID string `json:"openai_org_id"`

	// other configurations
	AllowedTelegramUsers []string `json:"allowed_telegram_users"`
	Verbose              bool     `json:"verbose,omitempty"`
	Model                string   `json:"openai_model"`
}

var ErrAppNotFound = errors.New("Steam Store game not found")
var timer *time.Ticker
var lastCheck time.Time

// launch bot with given parameters
func (s Server) run() {
	pref := tele.Settings{
		Token:  s.conf.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}
	s.bot = b

	log.Println("Bot is running")

	b.Handle("/start", func(c tele.Context) error {
		return c.Send(msgStart, "text", &tele.SendOptions{
			ReplyTo: c.Message(),
		})
	})

	b.Handle("/search", func(c tele.Context) error {
		return s.searchGames(c, true, nil)
	})
	b.Handle("/s", func(c tele.Context) error {
		return s.searchGames(c, true, nil)
	})
	b.Handle("/sa", func(c tele.Context) error {
		return s.searchGames(c, false, nil)
	})

	b.Handle("/wb", func(c tele.Context) error {
		response, err := s.getWorldBossInfo(c)
		if err != nil {
			log.Println(err)
			return c.Send(err.Error(), "text", &tele.SendOptions{ReplyTo: c.Message()})
		}
		return c.Send(response, "text", &tele.SendOptions{ReplyTo: c.Message()})
	})
	b.Handle("/wbtrack", func(c tele.Context) error {
		query := c.Message().Payload
		if query == "on" {
			if timer != nil {
				return c.Send("Tracker is already started", "text", &tele.SendOptions{ReplyTo: c.Message()})
			}

			err := s.startWorldBossTracker(c)
			if err != nil {
				log.Println(err)
			}
			lastCheck = time.Now()

			return err
		} else if query == "off" {
			if timer != nil {
				timer.Stop()
				return c.Send("Stopped tracker", "text", &tele.SendOptions{ReplyTo: c.Message()})
			}
			lastCheck = time.Time{}

			return c.Send("Tracker is not started", "text", &tele.SendOptions{ReplyTo: c.Message()})
		}

		return c.Send(fmt.Sprintf("Last check on %s", lastCheck.Format("15:04")), "text", &tele.SendOptions{ReplyTo: c.Message()})
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		if c.Message().IsReply() {
			return s.searchGames(c, true, &c.Message().Text)
		}
		return nil
	})

	b.Handle(tele.OnQuery, func(c tele.Context) error {
		query := c.Query().Text

		if len(query) < 3 {
			return nil
		}

		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Println(string(debug.Stack()), err)
				}
			}()

			games, err := searchSteamStore(query)
			if err != nil {
				log.Println(err)

				return
			}

			var game *SteamGame

			if len(games) > 1 && s.isAllowed(c.Sender().Username) {
				game = s.summarize(query, games)
			} else if len(games) >= 1 {
				game = games[0]
			} else {
				return
			}

			text := fmt.Sprintf("Price: ¥%d (Discount: %d%%)\nRelease date: %s\nMetacritic score: %d", int(game.Price.Final/100), game.Price.DiscountPercent, game.ReleaseDate.Date, game.Metacritic.Score)

			result := &tele.ArticleResult{
				URL:         fmt.Sprintf("https://store.steampowered.com/app/%d/", game.AppID),
				Title:       game.Name,
				Text:        text,
				Description: game.ShortDescription,
				ThumbURL:    game.HeaderImage,
			}

			results := make(tele.Results, 1)
			results[0] = result
			// needed to set a unique string ID for each result
			results[0].SetResultID(strconv.Itoa(game.AppID))

			c.Answer(&tele.QueryResponse{
				Results:   results,
				CacheTime: 100,
			})

		}()

		return nil
	})

	b.Start()

}

func (s Server) summarize(query string, games []*SteamGame) *SteamGame {
	prompt := "From those games:\n"
	for _, game := range games {
		prompt += fmt.Sprintf("Title: %s, App ID: %d\nRelease date: %s\n", game.Name, game.AppID, game.ReleaseDate.Date)
	}
	prompt += "\nWhich app ID is more relevant for the search “" + query + "”? Choose most recent game. Reply only with App ID please."
	if s.conf.Verbose {
		log.Printf("[verbose] Prompt: %s", prompt)
	}
	appID := s.answer(prompt, 31337)
	if len(appID) != 0 {
		if game, err := getSteamGame(appID); err == nil {
			return game
		}
	}

	return nil
}

func (s Server) searchGames(c tele.Context, useGPT bool, reply *string) error {
	c.Notify(tele.Typing)
	query := c.Message().Payload
	if reply != nil && len(*reply) > 3 {
		if s.conf.Verbose {
			log.Println("[verbose] Reply:", *reply)
		}
		query = *reply
	}
	if query == "/search" || query == "/sa" || len(query) < 3 {
		return c.Send("Please provide a longer query", "text", &tele.SendOptions{
			ReplyTo:     c.Message(),
			ReplyMarkup: &tele.ReplyMarkup{ForceReply: true},
		})

	}
	if len(query) > 30 {
		return c.Send("Title is too long", "text", &tele.SendOptions{ReplyTo: c.Message()})
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println(string(debug.Stack()), err)
			}
		}()

		c.Notify(tele.Typing)

		games, err := searchSteamStore(query)
		if err != nil {
			log.Println(err)

			c.Send(err.Error(), "text", &tele.SendOptions{ReplyTo: c.Message()})
			return
		}

		if useGPT && len(games) > 1 && s.isAllowed(c.Sender().Username) {
			game := s.summarize(query, games)
			if game != nil {
				s.sendGame(c, game)
				return
			}
		}

		for _, game := range games {
			s.sendGame(c, game)
		}
	}()

	return nil
}

func (s Server) sendGame(c tele.Context, game *SteamGame) {
	genres := []string{}
	for _, genre := range game.Genres {
		genres = append(genres, genre.Description)
	}
	genresString := strings.Join(genres, ", ")

	categories := []string{}
	for _, category := range game.Categories {
		categories = append(categories, category.Description)
	}
	categoriesString := strings.Join(categories, ", ")

	msg := fmt.Sprintf("[%s](https://store.steampowered.com/app/%d/)\n*Price:* ¥%d (Discount: %d%%)\n*Release date:* %s\n*Genres*: %s\n*Categories*: %s", game.Name, game.AppID, int(game.Price.Final/100), game.Price.DiscountPercent, game.ReleaseDate.Date, genresString, categoriesString)

	c.Send(msg, "text", &tele.SendOptions{
		ReplyTo:   c.Message(),
		ParseMode: tele.ModeMarkdown,
	})
}

func getSteamGame(id string) (*SteamGame, error) {
	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&CC=JP", id)
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	var response SteamDetails

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	if data, ok := response[id]; ok {
		if data.Success == false {
			return nil, errors.New(fmt.Sprintf("Steam Store search not successful %s", id))
		}

		return data.Data, nil
	}

	return nil, ErrAppNotFound
}

func searchSteamStore(query string) ([]*SteamGame, error) {
	log.Printf("Searching for %s\n", query)
	url := fmt.Sprintf("https://steamcommunity.com/actions/SearchApps/%s", query)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var appList SearchResult

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&appList); err != nil {
		return nil, err
	}

	var games []*SteamGame
	for _, app := range appList {

		game, err := getSteamGame(app.AppID)
		if err != nil {
			log.Println(err)
			continue
		}

		games = append(games, game)
		if len(games) > 5 {
			log.Println("Too many results returned")
			return games, nil
		}
	}

	if len(games) == 0 {
		return nil, ErrAppNotFound
	}

	return games, nil
}

// generate an answer to given message and send it to the chat
func (s Server) answer(message string, userID int64) string {
	msg := openai.NewChatUserMessage(message)

	history := []openai.ChatMessage{}
	history = append(history, msg)

	response, err := s.ai.CreateChatCompletion(s.conf.Model, history, openai.ChatCompletionOptions{}.SetUser(userAgent(userID)))
	if err != nil {
		log.Printf("failed to create chat completion: %s", err)
		return ""
	}
	if s.conf.Verbose {
		log.Printf("[verbose] %s ===> %+v", message, response.Choices)
	}

	// bot.SendChatAction(chatID, tg.ChatActionTyping, nil)

	var answer string
	if len(response.Choices) > 0 {
		answer = response.Choices[0].Message.Content
	} else {
		answer = "No response from API."
	}

	if s.conf.Verbose {
		log.Printf("[verbose] sending answer: '%s'", answer)
	}

	return answer
}

// checks if given update is allowed or not
func (s Server) isAllowed(username string) bool {
	if _, exists := s.users[username]; exists {
		return true
	}

	return false
}

// generate a user-agent value
func userAgent(userID int64) string {
	return fmt.Sprintf("telegram-steam-bot:%d", userID)
}

func getWorldBossEventInfo() (*WorldBoss, error) {
	r, err := http.NewRequest("GET", "https://diablo4.life/api/trackers/worldBoss/list", nil)
	r.Header.Set("Referer", "https://diablo4.life/trackers/world-bosses")
	r.Header.Set("Authority", "diablo4.life")
	r.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"113\", \"Not-A.Brand\";v=\"24\"")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response WorldBoss

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// get world boss info
func (s Server) getWorldBossInfo(c tele.Context) (string, error) {
	log.Println("Getting world boss info")

	response, err := getWorldBossEventInfo()
	if err != nil {
		return "", err
	}

	if response.Event != nil && response.Event.Name != nil {
		eventTime := time.UnixMilli(*response.Event.Time)
		return fmt.Sprintf(
			"%s appears in %s at %s", *response.Event.Name, *response.Event.Location, eventTime.Format("15:04 JST")), nil
	}

	if len(*response.LastEvent.Name) == 0 {
		err := fmt.Errorf("World boss info not found")
		return "", err
	}

	eventTime := time.UnixMilli(*response.LastEvent.Time)

	return fmt.Sprintf("Last event: %s, at %s. \nPossible spawn times: %s, %s, %s, %s.",
		*response.LastEvent.Name,
		eventTime.Format("15:04 JST"),
		eventTime.Add(time.Minute*324).Format("15:04 JST"),
		eventTime.Add(time.Minute*354).Format("15:04 JST"),
		eventTime.Add(time.Minute*444).Format("15:04 JST"),
		eventTime.Add(time.Minute*474).Format("15:04 JST")), nil
}

// start world boss event tracker
func (s Server) startWorldBossTracker(c tele.Context) error {
	response, err := getWorldBossEventInfo()
	if err != nil {
		return err
	}

	if response.Event != nil && response.Event.Name != nil {
		eventTime := time.UnixMilli(*response.Event.Time)
		msg := fmt.Sprintf("%s appears in %s at %s", *response.Event.Name, *response.Event.Location, eventTime.Format("15:04 JST"))

		return c.Send(msg, "text", &tele.SendOptions{ReplyTo: c.Message()})
	}

	timer = time.NewTicker(time.Minute * 10)
	go func() {
		for {
			select {
			case <-timer.C:
				lastCheck = time.Now()
				response, err := getWorldBossEventInfo()
				if err != nil {
					log.Println(err)
					continue
				}
				if response.Event != nil && response.Event.Name != nil {
					eventTime := time.UnixMilli(*response.Event.Time)
					msg := fmt.Sprintf("%s appears in %s at %s", *response.Event.Name, *response.Event.Location, eventTime.Format("15:04 JST"))
					c.Send(msg, "text", &tele.SendOptions{ReplyTo: c.Message()})
				}
			}
		}
	}()

	return c.Send("Started tracking", "text", &tele.SendOptions{ReplyTo: c.Message()})
}
