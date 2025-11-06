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
	TimeZone  *time.Location // User's calendar timezone
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

	// Fetch timezone for each user's calendar
	for i := range availabilities {
		tz, err := getCalendarTimeZone(service, availabilities[i].Email)
		if err != nil {
			// If we can't get timezone, assume the default timezone from the query
			// This might happen for external calendars or permission issues
			tz = time.UTC
		}
		availabilities[i].TimeZone = tz
	}

	return availabilities, nil
}

// getCalendarTimeZone fetches the timezone for a specific calendar
func getCalendarTimeZone(service *calendar.Service, email string) (*time.Location, error) {
	// Try to get the calendar settings
	cal, err := service.Calendars.Get(email).Do()
	if err != nil {
		// If we can't access the calendar (e.g., external user), return error
		return nil, err
	}

	// Load the timezone
	loc, err := time.LoadLocation(cal.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %s: %v", cal.TimeZone, err)
	}

	return loc, nil
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

// GetUserWorkingHours returns working hours for a specific user in their timezone
func GetUserWorkingHours(startDate, endDate time.Time, startHour, endHour, lunchStartHour, lunchEndHour int, 
	userTimezone *time.Location, excludeWeekends bool) []TimeSlot {
	var slots []TimeSlot

	// Convert dates to user's timezone
	startInUserTZ := startDate.In(userTimezone)
	endInUserTZ := endDate.In(userTimezone)
	
	current := time.Date(startInUserTZ.Year(), startInUserTZ.Month(), startInUserTZ.Day(), 0, 0, 0, 0, userTimezone)
	endDay := time.Date(endInUserTZ.Year(), endInUserTZ.Month(), endInUserTZ.Day(), 0, 0, 0, 0, userTimezone)
	
	for !current.After(endDay) {
		// Skip weekends if requested
		if excludeWeekends && (current.Weekday() == time.Saturday || current.Weekday() == time.Sunday) {
			current = current.AddDate(0, 0, 1)
			continue
		}

		// Create working hours slots for this day in user's timezone
		dayStart := time.Date(current.Year(), current.Month(), current.Day(), startHour, 0, 0, 0, userTimezone)
		dayEnd := time.Date(current.Year(), current.Month(), current.Day(), endHour, 0, 0, 0, userTimezone)
		lunchStart := time.Date(current.Year(), current.Month(), current.Day(), lunchStartHour, 0, 0, 0, userTimezone)
		lunchEnd := time.Date(current.Year(), current.Month(), current.Day(), lunchEndHour, 0, 0, 0, userTimezone)

		// Convert back to UTC for consistent storage
		dayStartUTC := dayStart.UTC()
		dayEndUTC := dayEnd.UTC()
		lunchStartUTC := lunchStart.UTC()
		lunchEndUTC := lunchEnd.UTC()

		// Add morning slot (before lunch)
		if dayStartUTC.Before(lunchStartUTC) && lunchStartUTC.Before(dayEndUTC) {
			// Morning slot exists
			morningEnd := lunchStartUTC
			if dayEndUTC.Before(lunchStartUTC) {
				morningEnd = dayEndUTC
			}
			slots = append(slots, TimeSlot{
				Start: dayStartUTC,
				End:   morningEnd,
			})
		}

		// Add afternoon slot (after lunch)
		if lunchEndUTC.Before(dayEndUTC) && dayStartUTC.Before(lunchEndUTC) {
			// Afternoon slot exists
			afternoonStart := lunchEndUTC
			if dayStartUTC.After(lunchEndUTC) {
				afternoonStart = dayStartUTC
			}
			slots = append(slots, TimeSlot{
				Start: afternoonStart,
				End:   dayEndUTC,
			})
		}

		// If lunch time is outside working hours, add the whole working day
		if lunchEndUTC.Before(dayStartUTC) || lunchStartUTC.After(dayEndUTC) {
			slots = append(slots, TimeSlot{
				Start: dayStartUTC,
				End:   dayEndUTC,
			})
		}

		current = current.AddDate(0, 0, 1)
	}

	return slots
}
