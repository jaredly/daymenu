package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/color"
	"bytes"
	"image/png"

	"github.com/getlantern/systray"
	"github.com/golang-module/carbon/v2"
)

func renderSquare(color color.Color) []byte {
	m := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(m, image.Rect(4, 4, 12, 12), &image.Uniform{color}, image.ZP, draw.Src)

	buf := new(bytes.Buffer)
	_ = png.Encode(buf, m)
	return buf.Bytes()
}

func renderCalendar() {
	m := image.NewRGBA(image.Rect(0, 0, 112, 80))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(m, m.Bounds(), &image.Uniform{white}, image.ZP, draw.Src)
	for i := 0; i < 80; i+=4 {
		blue := color.RGBA{0, 0, 255, 255}
		draw.Draw(m, image.Rect(0,i,112,i + 1), &image.Uniform{blue}, image.ZP, draw.Src)
		green := color.RGBA{0, 255, 0, 255}
		draw.Draw(m, image.Rect(0,i + 2,112, i + 3), &image.Uniform{green}, image.ZP, draw.Src)
	}

	buf := new(bytes.Buffer)
	_ = png.Encode(buf, m)
	image_bytes := buf.Bytes()
	systray.SetIconWithSize(image_bytes, 112, 40)
}


func findNext(state State) *EventAndTimes {
	now := carbon.Now()
	var next *EventAndTimes
	for _, event := range state.events {
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

func setTitle(state State) {
	next := findNext(state)
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

func renderEvents(state State) {
	now := carbon.Now()
	for _, cal := range state.calendars {
		if cal.menuItem != nil {
			cal.menuItem.SetTitle(cal.title)
		} else {
			cal.menuItem = systray.AddMenuItem(cal.title, cal.id)
			cal.menuItem.SetIcon(cal.icon)
		}
	}
	systray.AddSeparator()

	for _, event := range state.events {
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
			event.menuItem.SetIcon(event.icon)
			go handleClick(event.event, event.menuItem, event.calId)
		}

		if now.Gt(event.end) {
			event.menuItem.Disable()
		}
	}
}
