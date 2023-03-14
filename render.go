package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
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

func renderCalendarIcon(state State) {
	height := 60
	width := 100

	topMargin := 10
	bottomMargin := 10

	m := image.NewRGBA(image.Rect(0, 0, width, height))

	sod := carbon.Now().SubHours(1)
	eod := carbon.Now().AddHours(3)
	// sod := carbon.Now().StartOfDay().SetHour(8)
	// eod := carbon.Now().StartOfDay().SetHour(12 + 8)

	blocksToday := (int(sod.DiffInMinutes(eod)) / 15)
	pixelsPerBlock := width / blocksToday

	calHeight := (height - topMargin - bottomMargin) / len(state.calendars)

	tickColor := color.RGBA{100,100,100,255}
	halfColor := color.RGBA{50,50,50,255}
	tick := carbon.Now().StartOfDay()
	for i := 0; i < 24; i++ {
		at := tick.SetHour(i).SetMinute(0)
		x := int(sod.DiffInMinutes(at)) / 15 * pixelsPerBlock
		draw.Draw(m, image.Rect( x, 0, x + 2, topMargin / 2), &image.Uniform{tickColor}, image.ZP, draw.Src)
		at = tick.SetHour(i).SetMinute(30)
		x = int(sod.DiffInMinutes(at)) / 15 * pixelsPerBlock
		draw.Draw(m, image.Rect( x, 0, x + 2, topMargin / 2), &image.Uniform{halfColor}, image.ZP, draw.Src)
	}

	for _, event := range state.events {
		start := -int(event.start.DiffInMinutes(sod)) / 15 * pixelsPerBlock
		end := -int(event.end.DiffInMinutes(sod)) / 15 * pixelsPerBlock

		top := topMargin + event.calIdx * calHeight
		bottom := top + calHeight

		draw.Draw(m, image.Rect( start, top, end, bottom), &image.Uniform{event.color}, image.ZP, draw.Src)
	}

	now := -int(carbon.Now().DiffInMinutes(sod)) / 15 * pixelsPerBlock
	draw.Draw(m, image.Rect( now, topMargin, now + 1, height - bottomMargin), &image.Uniform{color.RGBA{255,255,255,255}}, image.ZP, draw.Src)

	buf := new(bytes.Buffer)
	_ = png.Encode(buf, m)
	image_bytes := buf.Bytes()
	systray.SetIconWithSize(image_bytes, width / 2, height / 2)
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
	renderCalendarIcon(state)

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
		// if event.end.Lt(now.SubMinutes(30)) {
		// 	if event.menuItem != nil {
		// 		event.menuItem.Remove()
		// 		event.menuItem = nil
		// 	}
		// 	continue
		// }

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
