package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"github.com/golang-module/carbon/v2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"image/color"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
char* getResourcePath(char* path) {
	NSString *settingsPath = [[NSString stringWithUTF8String:path] stringByExpandingTildeInPath];
	return [settingsPath UTF8String];
}
*/
import "C"


type EventAndTimes struct {
	event *calendar.Event
	start carbon.Carbon
	end carbon.Carbon
	menuItem *systray.MenuItem
	color color.Color
	calId string
	icon []byte
}

type Events []*EventAndTimes

type CalItem struct {
	id string
	title string
	menuItem *systray.MenuItem
	color color.Color
	icon []byte
	// color string
}

type State struct {
	calendars []*CalItem
	events Events
}

func (s Events) Len() int      { return len(s) }
func (s Events) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s Events) Less(i, j int) bool { return s[i].start.Lt(s[j].start) }

func parseHexColor(s string) (c color.RGBA, err error) {
    c.A = 0xff
    switch len(s) {
    case 7:
        _, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
    case 4:
        _, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
        // Double the hex digits:
        c.R *= 17
        c.G *= 17
        c.B *= 17
    default:
        err = fmt.Errorf("invalid length, must be 7 or 4")

    }
    return
}

func loadEvents(service *calendar.Service) State {
	list, err := service.CalendarList.List().Do()
	if err != nil {
		fmt.Println("hi")
		log.Fatal(err)
	}

	state := State{}

	for _, entry := range list.Items {
		if entry.Hidden || !entry.Selected {
			continue
		}

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

		color, err := parseHexColor(entry.BackgroundColor)
		icon := renderSquare(color)
		if err != nil {
			log.Fatal(err)
		}

		cal := CalItem{entry.Id, entry.Summary, nil, color, icon}
		state.calendars = append(state.calendars, &cal)

		for _, event := range events.Items {
			if event.Start.DateTime != "" && event.End.DateTime != "" {
				start := carbon.Parse(event.Start.DateTime)
				end   := carbon.Parse(event.End.DateTime)

				going := true
				for _, att := range event.Attendees {
					if att.Self {
						going = att.ResponseStatus != "declined"
					}
				}
				if !going {
					continue
				}

				state.events = append(state.events, &EventAndTimes{event, start, end, nil, color, entry.Id, icon})
			}
		}
	}

	sort.Sort(state.events)

	return state
}

func getConfig() *oauth2.Config {
	config, err := google.ConfigFromJSON(configBytes, 
		"https://www.googleapis.com/auth/calendar.readonly")
	config.RedirectURL = "http://localhost:5221"
	if err != nil {
		fmt.Println("hon")
		log.Fatal(err)
	}
	return config
}

func tokenFile() string {
	return C.GoString(C.getResourcePath(C.CString("~/.menunder.token")))
}

func authCalendar(cb func(*calendar.Service, map[string]bool), opened map[string]bool) {
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
		fmt.Println("hot")
			log.Fatal(err)
		}
		saveToken(token)
		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, token)))
		if err != nil {
		fmt.Println("nnhote")
			log.Fatal(err)
		}

		cb(calendarService, opened)

		fmt.Printf("Ok here we go\n")

		server.Close()
	})

	// listen to port
	server.Addr = "localhost:5221"
	server.ListenAndServe()

}

func saveToken(token *oauth2.Token) {
	text, err := json.Marshal(token)
	if err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile(tokenFile(), text, 0644)
}
