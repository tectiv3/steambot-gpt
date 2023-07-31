package main

import (
	"encoding/json"
	"fmt"
	"github.com/recoilme/pudge"
	tele "gopkg.in/telebot.v3"
	"log"
	"net/http"
	"time"
)

func (s Server) startWBTimer() {
	defer func() {
		recover()
	}()

	wb, err := getWorldBossEventInfo()
	if err != nil {
		log.Println(err)
	} else {
		if wb.Event != nil {
			_ = pudge.Set("events", wb.Event.Time, wb.Event)
		}
	}
	for range time.NewTicker(3600 * time.Second).C {
		wb, err = getWorldBossEventInfo()
		if err != nil {
			log.Println(err)
		}
		if wb.Event != nil {
			_ = pudge.Set("events", wb.Event.Time, wb.Event)
		}
	}
}

func getWorldBossEventInfo() (*WorldBoss, error) {
	r, err := http.NewRequest("GET", "https://diablo4.life/api/trackers/worldBoss/upcomming", nil)
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
func (s Server) getWorldBossInfo() (string, error) {
	log.Println("Getting world boss info")

	response, err := getWorldBossEventInfo()
	if err != nil {
		return "", err
	}

	next := time.UnixMilli(response.Event.Time + int64(response.NextDiff))

	return fmt.Sprintf("Last event: %s in %s at %s. \nNext spawn: %s, at %s.",
		response.Event.Name,
		response.Event.Location,
		time.UnixMilli(response.Event.Time).Format("15:04 JST"),
		response.NextSpawn,
		next.Format("15:04 JST")), nil
}

// start world boss event tracker
func (s Server) startWorldBossTracker(c tele.Context) error {
	info, err := getWorldBossEventInfo()
	if err != nil {
		return err
	}
	next := time.UnixMilli(info.Event.Time + int64(info.NextDiff))
	log.Println(next.Format("15:04 JST"))
	// get difference between now and next event
	nextEvent := next.Sub(time.Now())

	// check if next event is more than 15 minutes away, if so, subtract 15 minutes
	if nextEvent > time.Minute*15 {
		nextEvent -= time.Minute * 15
	}
	log.Println("Set timer to next event in", nextEvent)
	timer = time.NewTicker(nextEvent)

	//timer = time.NewTicker(time.Minute * 10)
	go func() {
		for {
			select {
			case t := <-timer.C:
				lastCheck = t
				response, err := s.getWorldBossInfo()
				if err != nil {
					log.Println(err)
					continue
				}

				_ = c.Send(response, "text", &tele.SendOptions{ReplyTo: c.Message()})
				// restart the timer in 30 minutes
				go s.restartTimer(c)
			}
		}
	}()

	return c.Send("Started tracking, next event in "+nextEvent.String(),
		"text",
		&tele.SendOptions{ReplyTo: c.Message()})
}

func (s Server) restartTimer(c tele.Context) {
	r := time.NewTicker(time.Minute * 30)
	log.Println("Restarting timer")
	for {
		select {
		case <-r.C:
			log.Println("Time to restart the timer")
			_ = s.startWorldBossTracker(c)
		}
	}
}
