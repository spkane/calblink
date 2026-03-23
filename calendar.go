// Copyright 2024 Google Inc.
// Modifications Copyright 2026 calblink contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file manages retrieving and filtering events from Google Calendar.

package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

// Event handling methods
func eventHasAcceptableResponse(item *calendar.Event, responseState ResponseState) bool {
	for _, attendee := range item.Attendees {
		if attendee.Self {
			return responseState.CheckStatus(attendee.ResponseStatus)
		}
	}
	debugLog("No self attendee found for %v\n", item)
	debugLog("Attendees: %v\n", item.Attendees)
	return true
}

func eventExcludedByPrefs(item string, userPrefs *UserPrefs) bool {
	if userPrefs.Excludes[item] {
		return true
	}
	for _, prefix := range userPrefs.ExcludePrefixes {
		if strings.HasPrefix(item, prefix) {
			debugLog("Skipping event '%v' due to prefix match '%v'\n", item, prefix)
			return true
		}
	}
	return false
}

func nextEvent(items []*calendar.Event, locations []WorkSite, userPrefs *UserPrefs) []*calendar.Event {
	var events []*calendar.Event

	if len(userPrefs.WorkingLocations) > 0 {
		match := false
		for _, prefLocation := range userPrefs.WorkingLocations {
			for _, location := range locations {
				if prefLocation.Matches(location) {
					debugLog("Found matching location: %v\n", prefLocation)
					match = true
					break
				}
			}
			if match {
				break
			}
		}

		if !match {
			debugLog("Skipping all events due to no matching locations in %v\n", locations)
			return events
		}
	}

	for _, i := range items {
		if !eventExcludedByPrefs(i.Summary, userPrefs) &&
			eventHasAcceptableResponse(i, userPrefs.ResponseState) {
			events = append(events, i)
			if len(events) == 2 || (len(events) == 1 && !userPrefs.MultiEvent) {
				break
			}
		}
	}
	debugLog("nextEvent returning %d events\n", len(events))
	return events
}

func blinkStateForDelta(delta float64) CalendarState {
	blinkState := Black
	switch {
	case delta < -1:
		blinkState = Blue
	case delta < 0:
		blinkState = BlueFlash
	case delta < 2:
		blinkState = FastRedFlash
	case delta < 5:
		blinkState = RedFlash
	case delta < 10:
		blinkState = Red
	case delta < 30:
		blinkState = Yellow
	case delta < 60:
		blinkState = Green
	}
	return blinkState
}

func blinkStateForEvent(next []*calendar.Event, priority int) CalendarState {
	blinkState := Black
	for i, event := range next {
		startTime, err := eventStartTime(event)
		if err != nil {
			errorLog("%v\n", err)
			break
		}

		delta := -time.Since(startTime).Minutes()
		if i == 0 {
			blinkState = blinkStateForDelta(delta)
		} else {
			secondary := blinkStateForDelta(delta)
			if secondary != Black {
				blinkState = CombineStates(blinkState, secondary)
			}
			if (priority == 1 && blinkState.primaryFlash == 0 && blinkState.secondaryFlash > 0) ||
				(priority == 2 && blinkState.primaryFlash > 0 && blinkState.secondaryFlash == 0) {
				debugLog("Swapping")
				blinkState = SwapState(blinkState)
			}
		}
		debugLog("Event %v, time %v, delta %v, state %v\n", event.Summary, startTime, delta, blinkState.Name)
	}
	return blinkState
}

func eventStartTime(event *calendar.Event) (time.Time, error) {
	if event.Start == nil {
		return time.Time{}, fmt.Errorf("event %q missing start time", event.Summary)
	}
	if event.Start.DateTime != "" {
		return time.Parse(time.RFC3339, event.Start.DateTime)
	}
	if event.Start.Date != "" {
		return time.Parse("2006-01-02", event.Start.Date)
	}
	return time.Time{}, fmt.Errorf("event %q missing parseable start time", event.Summary)
}

func eventEndTime(event *calendar.Event) (time.Time, error) {
	if event.End == nil {
		return time.Time{}, fmt.Errorf("event %q missing end time", event.Summary)
	}
	if event.End.DateTime != "" {
		return time.Parse(time.RFC3339, event.End.DateTime)
	}
	if event.End.Date != "" {
		return time.Parse("2006-01-02", event.End.Date)
	}
	return time.Time{}, fmt.Errorf("event %q missing parseable end time", event.Summary)
}

func isDisplayableEvent(event *calendar.Event) bool {
	return event != nil &&
		event.EventType != "workingLocation" &&
		event.EventType != "outOfOffice" &&
		event.Start != nil &&
		event.Start.DateTime != ""
}

func eventDedupKey(event *calendar.Event) string {
	if event.ICalUID != "" {
		return fmt.Sprintf("ical:%s|%s|%s", event.ICalUID, event.Start.DateTime, event.End.DateTime)
	}
	return fmt.Sprintf("id:%s|%s|%s", event.Id, event.Start.DateTime, event.End.DateTime)
}

func filterCandidateEvents(events []*calendar.Event) []*calendar.Event {
	filtered := make([]*calendar.Event, 0, len(events))
	seen := make(map[string]bool, len(events))
	for _, event := range events {
		if !isDisplayableEvent(event) {
			if event != nil {
				debugLog("Skipping non-displayable event %q of type %q\n", event.Summary, event.EventType)
			}
			continue
		}
		key := eventDedupKey(event)
		if seen[key] {
			debugLog("Skipping duplicate event with key %v\n", key)
			continue
		}
		filtered = append(filtered, event)
		seen[key] = true
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		t1, err1 := eventStartTime(filtered[i])
		t2, err2 := eventStartTime(filtered[j])
		if err1 != nil {
			log.Fatalf("Found bad time after times should have been filtered out: %v\n", err1)
		}
		if err2 != nil {
			log.Fatalf("Found bad time after times should have been filtered out: %v\n", err2)
		}
		return t1.Before(t2)
	})
	return filtered
}

func fetchEvents(now time.Time, srv *calendar.Service, userPrefs *UserPrefs) ([]*calendar.Event, error) {
	start := now.Format(time.RFC3339)
	endTime := now.Add(2 * time.Hour)
	end := endTime.Format(time.RFC3339)
	var allEvents []*calendar.Event
	locations := make([]WorkSite, 0)
	for _, calendarID := range userPrefs.Calendars {
		var locationCreated time.Time
		var location WorkSite
		skip := false
		events, err := srv.Events.List(calendarID).ShowDeleted(false).
			SingleEvents(true).TimeMin(start).TimeMax(end).OrderBy("startTime").
			EventTypes("default", "focusTime", "outOfOffice", "workingLocation").Do()
		if err != nil {
			return nil, err
		}
	eventLoop:
		for _, event := range events.Items {
			switch event.EventType {
			case "workingLocation":
				// The calendar event can return three or more working locations:
				// 1. The recurring one for the given day of the week
				// 2. The override for that particular day
				// 3. Any time overrides that are currently set for specific hours of the day.
				//
				// The logic here isn't complicated enough to manage matching events to
				// a location, so instead, gather the latest all-date event and all
				// time overrides. Any event that matches one of those will have an
				// acceptable location.
				isAllDay := event.Start == nil || event.Start.DateTime == ""
				thisCreated, err := time.Parse(time.RFC3339, event.Created)
				if err != nil || (thisCreated.Before(locationCreated) && isAllDay) {
					debugLog("Skipping location event %v because it's before the current one\n", event.Summary)
					continue
				}
				locationProperties := event.WorkingLocationProperties
				locationType := makeWorkSiteType(locationProperties.Type)
				locationString := ""
				switch locationType {
				case WorkSiteOffice:
					locationString = locationProperties.OfficeLocation.Label
				case WorkSiteCustom:
					locationString = locationProperties.CustomLocation.Label
				}
				if isAllDay {
					location = WorkSite{SiteType: locationType, Name: locationString}
					locationCreated = thisCreated
				} else {
					debugLog("Location override detected: calendar %v, location %v\n", calendarID, location)
					locations = append(locations, WorkSite{SiteType: locationType, Name: locationString})
				}
			case "outOfOffice":
				// If the calendar is fully OOO for the active window, skip all events from it.
				eventStart, err1 := eventStartTime(event)
				eventEnd, err2 := eventEndTime(event)
				if err1 != nil || err2 != nil {
					debugLog("Skipping OOO event %v because of time parse errors: %v, %v\n", event.Summary, err1, err2)
					continue
				}
				if !eventStart.After(now) && !eventEnd.Before(endTime) {
					debugLog("Skipping calendar %v due to OOO\n", calendarID)
					skip = true
					break eventLoop
				}
			}
		}
		if !skip {
			if !locationCreated.IsZero() {
				debugLog("Adding final location %v\n", location)
				locations = append(locations, location)
			}
			allEvents = append(allEvents, events.Items...)
		}
	}
	return nextEvent(filterCandidateEvents(allEvents), locations, userPrefs), nil
}

func listAvailableCalendars(srv *calendar.Service) ([]*calendar.CalendarListEntry, error) {
	var entries []*calendar.CalendarListEntry
	pageToken := ""
	for {
		call := srv.CalendarList.List()
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		result, err := call.Do()
		if err != nil {
			return nil, err
		}
		entries = append(entries, result.Items...)
		if result.NextPageToken == "" {
			return entries, nil
		}
		pageToken = result.NextPageToken
	}
}

func resolveConfiguredCalendars(srv *calendar.Service, userPrefs *UserPrefs) error {
	entries, err := listAvailableCalendars(srv)
	if err != nil {
		return err
	}
	ids, err := resolveCalendarRefs(userPrefs.Calendars, entries)
	if err != nil {
		return err
	}
	userPrefs.Calendars = ids
	return nil
}

func resolveCalendarRefs(refs []string, entries []*calendar.CalendarListEntry) ([]string, error) {
	if len(refs) == 0 {
		return []string{"primary"}, nil
	}
	resolved := make([]string, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		id, err := resolveCalendarRef(ref, entries)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			resolved = append(resolved, id)
			seen[id] = true
		}
	}
	return resolved, nil
}

func resolveCalendarRef(ref string, entries []*calendar.CalendarListEntry) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("calendar reference cannot be empty")
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Id, ref) {
			return entry.Id, nil
		}
	}

	if strings.EqualFold(ref, "primary") {
		for _, entry := range entries {
			if entry.Primary {
				return entry.Id, nil
			}
		}
		return "primary", nil
	}

	var matches []*calendar.CalendarListEntry
	for _, entry := range entries {
		for _, candidate := range []string{entry.SummaryOverride, entry.Summary} {
			if candidate != "" && strings.EqualFold(candidate, ref) {
				matches = append(matches, entry)
				break
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("calendar %q not found in Google Calendar list", ref)
	case 1:
		return matches[0].Id, nil
	default:
		options := make([]string, 0, len(matches))
		for _, match := range matches {
			options = append(options, fmt.Sprintf("%s (%s)", match.Summary, match.Id))
		}
		return "", fmt.Errorf("calendar %q is ambiguous: %s", ref, strings.Join(options, ", "))
	}
}
