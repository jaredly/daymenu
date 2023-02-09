package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"github.com/golang-module/carbon/v2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func getConfig() *oauth2.Config {
	// Your credentials should be obtained from the Google
	// Developer Console (https://console.developers.google.com).
	bytes, err := ioutil.ReadFile("client-config.json")
	if err != nil {
		log.Fatal(err)
	}
	config, err := google.ConfigFromJSON(bytes, 
		"https://www.googleapis.com/auth/calendar.readonly")
	config.RedirectURL = "http://localhost:5221"
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func authCalendar(cb func(*calendar.Service)) {
	ctx := context.Background()
	// Redirect user to Google's consent page to ask for permission
	// for the scopes specified above.
	conf := getConfig()
	open.Run(conf.AuthCodeURL("state"))

	var server http.Server
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		fmt.Printf("Welcome to new server! %s\n", code)

		// Handle the exchange code to initiate a transport.
		token, err := conf.Exchange(oauth2.NoContext, code)
		if err != nil {
			log.Fatal(err)
		}
		text, err := json.Marshal(token)
		if err != nil {
			log.Fatal(err)
		}
		ioutil.WriteFile("token.txt", text, 0644)
		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, token)))
		if err != nil {
			log.Fatal(err)
		}

		cb(calendarService)

		fmt.Printf("Ok here we go\n")

		server.Close()
	})

	// listen to port
	server.Addr = "localhost:5221"
	server.ListenAndServe()

}

func main() {
	fmt.Println("Main ok")

	systray.Run(func () {
		onReady()
	}, func () {
		// idk
	})
}

func loadFromFile() bool {
	text, err := ioutil.ReadFile("token.txt")
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

		runCalendar(calendarService)
		return true
	}
	return false
}

func onReady() {
	systray.SetTitle("Menunder")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mRefresh := systray.AddMenuItem("Refresh", "Refresh the things")
	mUrl := systray.AddMenuItem("Authorize with Google Calendar", "my home")

	systray.AddSeparator()

	fmt.Println("Ok loading")
	if loadFromFile() {
	fmt.Println("Ok loaded")
		mUrl.Hide()
	}

	for {
		select {
		case <-mUrl.ClickedCh:
			mUrl.Hide()
			authCalendar(runCalendar)
		case <-mRefresh.ClickedCh:
			systray.RemoveAllItems()
			onReady()
		case <-mQuit.ClickedCh:
			fmt.Println("Quit2 now...")
			systray.Quit()
		}
	}
}

type EventAndTimes struct {
	event *calendar.Event
	start carbon.Carbon
	end carbon.Carbon
}

type Calendar struct {
	id string
	events []EventAndTimes
}

func findNext(events []Calendar) *EventAndTimes {
	var next *EventAndTimes
	for _, cal := range events {
		for _, event := range cal.events {
			if next == nil || next.start.Gt(event.start) {
				next = &event
			}
		}
	}
	return next
}

func renderEvents(cals []Calendar) {
	next := findNext(cals)
	if next == nil {
		systray.SetTitle("No next event")
	} else {
		text := next.event.Summary
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		systray.SetTitle(text)

		fmt.Println("Event " + next.event.Summary)
	}
	for _, cal := range cals {
		systray.AddMenuItem(cal.id, "").Disable()

		for _, event := range cal.events {
			text := event.event.Summary
			if event.event.HangoutLink != "" {
				text = "📹️ " + text
			}

			text = event.start.Format("h:ia") + "-" + event.end.Format("h:ia") + " " + text
			item := systray.AddMenuItem(text, event.event.Description)

			go handleClick(event.event, item, cal.id)
		}
	}
}

func runCalendar(service *calendar.Service) {
	cals := loadEvents(service)
	renderEvents(cals)
}

func loadEvents(service *calendar.Service) []Calendar {
	list, err := service.CalendarList.List().Do()
	if err != nil {
		log.Fatal(err)
	}

	cals := []Calendar{}

	for _, entry := range list.Items {
		events, err := service.Events.List(entry.Id).
			OrderBy("startTime").SingleEvents(true).
			TimeMin(carbon.Now().StartOfDay().ToRfc3339String()).
			TimeMax(carbon.Tomorrow().StartOfDay().ToRfc3339String()).Do()
		if err != nil {
			log.Fatal(err)
		}
		if len(events.Items) == 0 {
			continue
		}

		eventsAndTimes := []EventAndTimes{}

		for _, event := range events.Items {
			if event.Start.DateTime != "" && event.End.DateTime != "" {
				start := carbon.Parse(event.Start.DateTime)
				end   := carbon.Parse(event.End.DateTime)
				if end.Lt(carbon.Now().SubMinutes(30)) {
					continue
				}

				eventsAndTimes = append(eventsAndTimes, EventAndTimes{event, start, end})
			}
		}
		cals = append(cals, Calendar{id: entry.Id, events: eventsAndTimes})
	}

	return cals
}

func loadEvents_(service *calendar.Service) {
	list, err := service.CalendarList.List().Do()
	if err != nil {
		log.Fatal(err)
	}

	var found *calendar.Event

	for _, entry := range list.Items {
		events, err := service.Events.List(entry.Id).
			OrderBy("startTime").SingleEvents(true).
			TimeMin(carbon.Now().StartOfDay().ToRfc3339String()).
			TimeMax(carbon.Tomorrow().StartOfDay().ToRfc3339String()).Do()
		if err != nil {
			log.Fatal(err)
		}
		if len(events.Items) == 0 {
			continue
		}

		systray.AddMenuItem(entry.Id, "").Disable()

		for _, event := range events.Items {
			start := carbon.Parse(event.Start.DateTime)
			end   := carbon.Parse(event.End.DateTime)

			if end.Lt(carbon.Now().SubMinutes(30)) {
				continue
			}

			text := event.Summary
			if event.HangoutLink != "" {
				text = "📹️ " + text
			}

			if found == nil {
				found = event
				title := text
				now := carbon.Now()
				if start.Gt(now) {
					minutes := now.DiffInMinutes(start)
					title += fmt.Sprintf(" in %d min", minutes)
				}
				systray.SetTitle(title)
			}

			text = start.Format("h:i") + "-" + end.Format("h:i") + " " + text
			item := systray.AddMenuItem(text, event.Description)

			go handleClick(event, item, entry.Id)
		}
	}
	if found == nil {
		return
	}

	for {
		<-time.After(time.Second * 30)

		start := carbon.Parse(found.Start.DateTime)
		end   := carbon.Parse(found.End.DateTime)

		if end.Lt(carbon.Now().SubMinutes(30)) {
			return
		}

		text := found.Summary
		if found.HangoutLink != "" {
			text = "📹️ " + text
		}

		title := text
		now := carbon.Now()
		if start.Gt(now) {
			minutes := now.DiffInMinutes(start)
			title += fmt.Sprintf(" in %d min", minutes)
		}
		systray.SetTitle(title)
	}
}

func handleClick(event *calendar.Event, item *systray.MenuItem, id string) {
	for {
		select {
		case <-item.ClickedCh:
			text, _ := json.MarshalIndent(event, "", "  ")
			fmt.Println(string(text))
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
	}
}
