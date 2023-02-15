package main

import (
	_ "embed"
	"context"
	"fmt"
	"os"
	"io/ioutil"
	"log"
	"encoding/json"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"github.com/golang-module/carbon/v2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

//go:embed client-config.json
var configBytes []byte

func main() {
	// fmt.Println("Main ok")

	systray.Run(func () {
		onReady()
	}, func () {
		// idk
	})
}

func loadFromFile(opened map[string]bool) bool {
	text, err := ioutil.ReadFile(tokenFile())
	if err == nil {
		var token oauth2.Token
		err := json.Unmarshal(text, &token)
		if err != nil {
			log.Fatal(err)
		}

		conf := getConfig()
		ctx := context.Background()
		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, &token)))
		if err != nil {
			log.Fatal(err)
		}

		runCalendar(calendarService, opened)
		return true
	}
	return false
}

func onReady() {
	systray.SetTitle("Loading...")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mRefresh := systray.AddMenuItem("Refresh", "Refresh the things")
	mUrl := systray.AddMenuItem("Authorize with Google Calendar", "my home")
	mUrl.Hide()

	systray.AddSeparator()

	opened := make(map[string]bool)

	go func () {
		for {
			select {
			case <-mUrl.ClickedCh:
				mUrl.Hide()
				authCalendar(runCalendar, opened)
			case <-mRefresh.ClickedCh:
				systray.RemoveAllItems()
				onReady()
			case <-mQuit.ClickedCh:
				fmt.Println("Quitting?")
				os.Exit(0)
			}
		}
	}()

	if !loadFromFile(opened) {
		mUrl.Show()
		systray.SetTitle("Please authorize?")
	}

}

func openIfNeeded(events Events, opened map[string]bool) {
	now := carbon.Now()
	for _, event := range events {
		if !opened[event.event.Id] && event.event.HangoutLink != "" {
			if event.start.Lte(now.AddMinutes(1)) && event.end.Gte(now) {
				opened[event.event.Id] = true
				openEvent(event.event, event.calId)
			}
		}
	}
}

func runCalendar(service *calendar.Service, opened map[string]bool) {
	events := loadEvents(service)
	setTitle(events)
	renderEvents(events)

	start := carbon.Now()

	openIfNeeded(events, opened)

	for {
		select {
		case <-time.After(time.Second * 30):
			openIfNeeded(events, opened)
			setTitle(events)
			renderEvents(events)
			if carbon.Now().Gt(start.AddMinutes(15)) {
				// systray.RemoveAllItems()
				// mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
				// go func () {
				// 	<-mQuit.ClickedCh
				// 	os.Exit(0)
				// }()
				runCalendar(service, opened)
				return
			}
		}
	}
}

func openEvent(event *calendar.Event, id string) {
	link := event.HangoutLink
	if link == "" {
		link = event.HtmlLink
	}
	if strings.Contains(link, "?") {
		link += "&authuser=" + id
	} else {
		link += "?authuser=" + id
	}
	open.Run(link)
}

func handleClick(event *calendar.Event, item *systray.MenuItem, id string) {
	for {
		select {
		case <-item.ClickedCh:
			text, _ := json.MarshalIndent(event, "", "  ")
			fmt.Println(string(text))
			openEvent(event, id)
		}
	}
}
