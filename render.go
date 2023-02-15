package main

import (
	_ "embed"
	"fmt"

	"github.com/getlantern/systray"
	"github.com/golang-module/carbon/v2"
)


func findNext(events Events) *EventAndTimes {
	now := carbon.Now()
	var next *EventAndTimes
	for _, event := range events {
		if event.end.Lt(now) {
			continue
		}
		if next == nil {
			next = event
		} else if next.start.Gt(event.start) {
			next = event
		} else if next.start.Eq(event.start) && next.event.HangoutLink == "" && event.event.HangoutLink != "" {
			next = event
		}
	}
	return next
}

func setTitle(events Events) {
	next := findNext(events)
	if next == nil {
		systray.SetTitle("No next event")
	} else {
		text := next.event.Summary
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		mins := carbon.Now().DiffInMinutes(next.start)
		if mins > 30 {
			text += fmt.Sprintf(" at %s", next.start.Format("h:ia"))
		} else if mins > -5 {
			text += fmt.Sprintf(" in %d min", mins)
		} else {
			text += " now"
		}
		systray.SetTitle(text)
	}
}

func renderEvents(events Events) {
	now := carbon.Now()
	for _, event := range events {
		if event.end.Lt(now.SubMinutes(30)) {
			if event.menuItem != nil {
				event.menuItem.Remove()
				event.menuItem = nil
			}
			continue
		}

		text := event.event.Summary
		if event.event.HangoutLink != "" {
			text = "📹️ " + text
		}

		text = event.start.Format("h:ia") + "-" + event.end.Format("h:ia") + " " + text
		if now.Gte(event.start) && now.Lte(event.end) {
			text = "* " + text
		}

		if event.menuItem != nil {
			event.menuItem.SetTitle(text)
		} else {
			event.menuItem = systray.AddMenuItem(text, event.event.Description)
			go handleClick(event.event, event.menuItem, event.calId)
		}

		if now.Gt(event.end) {
			event.menuItem.Disable()
		}
	}
}
