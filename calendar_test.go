// Copyright 2026 calblink contributors.
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

package main

import (
	"testing"

	"google.golang.org/api/calendar/v3"
)

func TestFilterCandidateEventsSkipsPseudoEventsAndDedupesByICalUID(t *testing.T) {
	events := []*calendar.Event{
		{
			Summary:   "Working Location",
			EventType: "workingLocation",
			Start:     &calendar.EventDateTime{Date: "2026-03-23"},
		},
		{
			Summary:   "OOO",
			EventType: "outOfOffice",
			Start:     &calendar.EventDateTime{DateTime: "2026-03-23T10:00:00Z"},
			End:       &calendar.EventDateTime{DateTime: "2026-03-23T11:00:00Z"},
		},
		{
			Summary:   "Team Sync",
			EventType: "default",
			ICalUID:   "shared-event@example.com",
			Id:        "calendar-a-event",
			Start:     &calendar.EventDateTime{DateTime: "2026-03-23T12:00:00Z"},
			End:       &calendar.EventDateTime{DateTime: "2026-03-23T12:30:00Z"},
		},
		{
			Summary:   "Team Sync Duplicate",
			EventType: "default",
			ICalUID:   "shared-event@example.com",
			Id:        "calendar-b-event",
			Start:     &calendar.EventDateTime{DateTime: "2026-03-23T12:00:00Z"},
			End:       &calendar.EventDateTime{DateTime: "2026-03-23T12:30:00Z"},
		},
	}

	filtered := filterCandidateEvents(events)
	if len(filtered) != 1 {
		t.Fatalf("expected one displayable deduped event, got %d", len(filtered))
	}
	if filtered[0].Summary != "Team Sync" {
		t.Fatalf("expected first real meeting to remain, got %q", filtered[0].Summary)
	}
}

func TestResolveCalendarRefsSupportsNamesAndDetectsAmbiguity(t *testing.T) {
	entries := []*calendar.CalendarListEntry{
		{Id: "primary@example.com", Summary: "Primary", Primary: true},
		{Id: "work@example.com", Summary: "Work"},
		{Id: "work-2@example.com", Summary: "Work"},
	}

	refs, err := resolveCalendarRefs([]string{"primary"}, entries)
	if err != nil {
		t.Fatalf("resolveCalendarRefs returned unexpected error: %v", err)
	}
	if len(refs) != 1 || refs[0] != "primary@example.com" {
		t.Fatalf("expected primary calendar id, got %#v", refs)
	}

	if _, err := resolveCalendarRefs([]string{"Work"}, entries); err == nil {
		t.Fatal("expected ambiguous calendar name to return an error")
	}
}

func TestNextEventWorkingLocationWildcardMatchesNamedOffice(t *testing.T) {
	userPrefs := getDefaultPrefs()
	userPrefs.WorkingLocations = []WorkSite{{SiteType: WorkSiteOffice}}
	userPrefs.MultiEvent = true

	events := []*calendar.Event{
		{
			Summary:   "Office Meeting",
			EventType: "default",
			Start:     &calendar.EventDateTime{DateTime: "2026-03-23T12:00:00Z"},
			End:       &calendar.EventDateTime{DateTime: "2026-03-23T13:00:00Z"},
		},
	}

	next := nextEvent(events, []WorkSite{{SiteType: WorkSiteOffice, Name: "HQ"}}, userPrefs)
	if len(next) != 1 {
		t.Fatalf("expected office wildcard to match named office location, got %d events", len(next))
	}
}
