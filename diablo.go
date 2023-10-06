package main

import (
	"encoding/json"
	"fmt"
	tele "gopkg.in/telebot.v3"
	"log"
	"os"
	"os/exec"
	"time"
)

//func (s Server) startWBTimer() {
//	defer func() {
//		recover()
//	}()
//
//	_, err := getWorldBossEventInfo()
//	if err != nil {
//		log.Println(err)
//	} else {
//		//if wb != nil {
//		//	_ = pudge.Set("events", wb.Time, wb)
//		//}
//	}
//	for range time.NewTicker(3600 * time.Second).C {
//		_, err = getWorldBossEventInfo()
//		if err != nil {
//			log.Println(err)
//		}
//		//if wb.Event != nil {
//		//	_ = pudge.Set("events", wb.Event.Time, wb.Event)
//		//}
//	}
//}

//func getWorldBossEventInfo() (*Event, error) {
//	r, err := http.NewRequest("GET", "https://api.worldstone.io/world-bosses/", nil)
//	r.Header.Set("Referer", "https://diablo4.life/trackers/world-bosses")
//	r.Header.Set("Authority", "api.worldstone.io")
//	r.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"113\", \"Not-A.Brand\";v=\"24\"")
//
//	resp, err := http.DefaultClient.Do(r)
//	if err != nil {
//		return nil, err
//	}
//	defer resp.Body.Close()
//	var response Event
//
//	decoder := json.NewDecoder(resp.Body)
//	if err := decoder.Decode(&response); err != nil {
//		return nil, err
//	}
//
//	return &response, nil
//}

func (s Server) getWorldBossEventInfo() (*Event, error) {
	cmd := exec.Command(s.conf.Python, s.conf.Bosspy, "--single")
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty output")
	}
	var response Event
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// get world boss info
func (s Server) getWorldBossInfo() (string, error) {
	log.Println("Getting world boss info")

	response, err := s.getWorldBossEventInfo()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Next spawn: %s, at %s.",
		response.Name,
		time.Now().Add(time.Duration(response.Time)*time.Minute).Format("15:04 JST"),
	), nil
}

// start world boss event tracker
func (s Server) startWorldBossTracker(c tele.Context) error {
	info, err := s.getWorldBossEventInfo()
	if err != nil {
		return err
	}
	next := time.Now().Add(time.Duration(info.Time) * time.Minute)
	log.Println(next.Format("15:04 JST"))
	// get difference between now and next event
	nextEvent := time.Duration(info.Time) * time.Minute

	// check if next event is more than 15 minutes away, if so, subtract 15 minutes
	if nextEvent > time.Minute*15 {
		nextEvent -= time.Minute * 15
	}
	log.Println("Set timer to next event in", nextEvent)
	timer = time.NewTicker(nextEvent)

	//timer = time.NewTicker(time.Minute * 10)
	go func() {
		select {
		case t := <-timer.C:
			lastCheck = t
			response, err := s.getWorldBossInfo()
			if err != nil {
				log.Println(err)
				timer.Stop()
				timer = nil
				return
			}

			_ = c.Send(response)
			timer.Stop()
			// restart the timer in 30 minutes
			go s.restartTimer(c)
		}
	}()

	return c.Send("Started tracking, next event in " + nextEvent.String())
}

func (s Server) restartTimer(c tele.Context) {
	r := time.NewTicker(time.Minute * 30)
	log.Println("Restarting timer")
	select {
	case <-r.C:
		log.Println("Time to restart the timer")
		_ = s.startWorldBossTracker(c)
		r.Stop()
		return
	}
}
