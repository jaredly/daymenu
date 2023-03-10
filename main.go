package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/golang-module/carbon/v2"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

//go:embed client-config.json
var configBytes []byte

func main() {
	// fmt.Println("Main ok")

	systray.Run(func() {
		onReady()
	}, func() {
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

		ctx := context.Background()
		conf := getConfig()

		tokenSource := conf.TokenSource(ctx, &token)
		_ = oauth2.NewClient(ctx, tokenSource)
		savedToken, err := tokenSource.Token()
		if err != nil {
			return false
		}
		saveToken(savedToken)

		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Fatal(err)
		}

		runCalendar(calendarService, opened)
		return true
	}
	return false
}

/*
ok, so we have 40 vertical pixels
WOW ok 80 vertical pixels, if I really want to push it.

a single vertical line can indicate where we are
we can do an 8am to 10pm or something?

that's ... 14 hours. with 15-minute increments, that's 56 pixels
where 1 pixel is 15 minutes.
It'd be nice to have 2 pixels per 15 minutes. So I can do 112 pixels wide, that's
not terrible.

OR should we just do "the next 4 hours"?
*/

func onReady() {
	systray.SetTitle("Loading...")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mRefresh := systray.AddMenuItem("Refresh", "Refresh the things")
	mUrl := systray.AddMenuItem("Authorize with Google Calendar", "my home")
	mUrl.Hide()

	systray.AddSeparator()

	opened := make(map[string]bool)

	go func() {
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

func openIfNeeded(state State, opened map[string]bool) {
	now := carbon.Now()
	for _, event := range state.events {
		if !opened[event.event.Id] && event.event.HangoutLink != "" {
			if event.start.Lte(now.AddMinutes(1)) && event.end.Gte(now) {
				opened[event.event.Id] = true
				openEvent(event.event, event.calId)
			}
		}
	}
}

func runCalendar(service *calendar.Service, opened map[string]bool) {
	state := loadEvents(service)
	setTitle(state)
	renderEvents(state)

	start := carbon.Now()

	openIfNeeded(state, opened)

	for {
		select {
		case <-time.After(time.Second * 30):
			openIfNeeded(state, opened)
			setTitle(state)
			renderEvents(state)
			if carbon.Now().Gt(start.AddMinutes(15)) {
				systray.RemoveAllItems()
				mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
				go func() {
					<-mQuit.ClickedCh
					os.Exit(0)
				}()
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
