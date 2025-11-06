package calendar

import (
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
)

// TimeSlot represents a time period
type TimeSlot struct {
	Start time.Time
	End   time.Time
}

// UserAvailability represents a user's busy/free times
type UserAvailability struct {
	Email     string
	BusySlots []TimeSlot
}

// GetBusyTimes fetches busy times for multiple users
func GetBusyTimes(service *calendar.Service, emails []string, startTime, endTime time.Time) ([]UserAvailability, error) {
	// Create freebusy query
	items := make([]*calendar.FreeBusyRequestItem, len(emails))
	for i, email := range emails {
		items[i] = &calendar.FreeBusyRequestItem{
			Id: email,
		}
	}

	freebusyRequest := &calendar.FreeBusyRequest{
		TimeMin:  startTime.Format(time.RFC3339),
		TimeMax:  endTime.Format(time.RFC3339),
		Items:    items,
		TimeZone: "UTC",
	}

	// Execute the query
	freebusyCall := service.Freebusy.Query(freebusyRequest)
	response, err := freebusyCall.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve freebusy: %v", err)
	}

	// Parse results
	var availabilities []UserAvailability
	for email, calendar := range response.Calendars {
		userAvail := UserAvailability{
			Email:     email,
			BusySlots: []TimeSlot{},
		}

		for _, busy := range calendar.Busy {
			start, _ := time.Parse(time.RFC3339, busy.Start)
			end, _ := time.Parse(time.RFC3339, busy.End)
			userAvail.BusySlots = append(userAvail.BusySlots, TimeSlot{
				Start: start,
				End:   end,
			})
		}

		availabilities = append(availabilities, userAvail)
	}

	return availabilities, nil
}

// GetWorkingHours returns working hours for a given date range, excluding lunch time
func GetWorkingHours(startDate, endDate time.Time, startHour, endHour, lunchStartHour, lunchEndHour int, excludeWeekends bool) []TimeSlot {
	var slots []TimeSlot

	current := startDate
	for current.Before(endDate) || current.Equal(endDate) {
		// Skip weekends if requested
		if excludeWeekends && (current.Weekday() == time.Saturday || current.Weekday() == time.Sunday) {
			current = current.AddDate(0, 0, 1)
			continue
		}

		// Create working hours slots for this day, splitting around lunch time
		dayStart := time.Date(current.Year(), current.Month(), current.Day(), startHour, 0, 0, 0, current.Location())
		dayEnd := time.Date(current.Year(), current.Month(), current.Day(), endHour, 0, 0, 0, current.Location())
		lunchStart := time.Date(current.Year(), current.Month(), current.Day(), lunchStartHour, 0, 0, 0, current.Location())
		lunchEnd := time.Date(current.Year(), current.Month(), current.Day(), lunchEndHour, 0, 0, 0, current.Location())

		// Add morning slot (before lunch)
		if dayStart.Before(lunchStart) && lunchStart.Before(dayEnd) {
			// Morning slot exists
			morningEnd := lunchStart
			if dayEnd.Before(lunchStart) {
				morningEnd = dayEnd
			}
			slots = append(slots, TimeSlot{
				Start: dayStart,
				End:   morningEnd,
			})
		}

		// Add afternoon slot (after lunch)
		if lunchEnd.Before(dayEnd) && dayStart.Before(lunchEnd) {
			// Afternoon slot exists
			afternoonStart := lunchEnd
			if dayStart.After(lunchEnd) {
				afternoonStart = dayStart
			}
			slots = append(slots, TimeSlot{
				Start: afternoonStart,
				End:   dayEnd,
			})
		}

		// If lunch time is outside working hours, add the whole working day
		if lunchEnd.Before(dayStart) || lunchStart.After(dayEnd) {
			slots = append(slots, TimeSlot{
				Start: dayStart,
				End:   dayEnd,
			})
		}

		current = current.AddDate(0, 0, 1)
	}

	return slots
}
