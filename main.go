package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"encoding/json"
	"net/http"
	"strings"

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

		loadEvents(calendarService)
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

	if loadFromFile() {
		mUrl.Hide()
	}

	for {
		select {
		case <-mUrl.ClickedCh:
			mUrl.Hide()
			authCalendar(loadEvents)
		case <-mRefresh.ClickedCh:
			systray.RemoveAllItems()
			onReady()
		case <-mQuit.ClickedCh:
			fmt.Println("Quit2 now...")
			systray.Quit()
		}
	}
}

func loadEvents(service *calendar.Service) {
	// fmt.Printf("Got a calendar\n")
	list, err := service.CalendarList.List().Do()
	if err != nil {
		log.Fatal(err)
	}

	found := false

	for _, entry := range list.Items {
		// text, _ := json.MarshalIndent(entry, "", "  ")
		// fmt.Println("Calendar\n", string(text))

		events, err := service.Events.List(entry.Id).OrderBy("startTime").SingleEvents(true).TimeMin(carbon.Now().StartOfDay().ToRfc3339String()).TimeMax(carbon.Tomorrow().StartOfDay().ToRfc3339String()).Do()
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

			if !found {
				found = true
				title := text
				now := carbon.Now()
				if start.Gt(now) {
					minutes := now.DiffInMinutes(start)
					title += fmt.Sprintf(" in %d min", minutes)
				}
				systray.SetTitle(title)
			}

			text = start.Format("H:i") + "-" + end.Format("H:i") + " " + text
			item := systray.AddMenuItem(text, event.Description)

			go func (event *calendar.Event, item *systray.MenuItem, entry *calendar.CalendarListEntry) {
				for {
					select {
					case <-item.ClickedCh:
						// text, _ := json.MarshalIndent(event, "", "  ")
						// fmt.Println("An Event\n", string(text))
						link := event.HangoutLink
						if link == "" {
							link = event.HtmlLink
						}
						if strings.Contains(link, "?") {
							link += "&authuser=" + entry.Id
						} else {
							link += "?authuser=" + entry.Id
						}
						// fmt.Println("A link", link)
						open.Run(link)
					}
				}
			}(event, item, entry)
		}
	}
}
