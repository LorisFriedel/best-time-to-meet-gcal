package calendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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
	Holidays  []Holiday
}

// Holiday represents an observed public holiday window for a user.
type Holiday struct {
	Name     string
	Region   string
	TimeSlot TimeSlot
}

// CalendarAccessResult represents the result of checking calendar access for an email
type CalendarAccessResult struct {
	Email       string
	HasAccess   bool
	Error       error
	ErrorReason string // "no_calendar", "permission_denied", "external", etc.
}

// Default batch size for Calendar API requests
const DefaultBatchSize = 50

// GetBusyTimes fetches busy times for multiple users, automatically batching if needed
func GetBusyTimes(service *calendar.Service, emails []string, startTime, endTime time.Time) ([]UserAvailability, error) {
	return GetBusyTimesWithBatching(service, emails, startTime, endTime, DefaultBatchSize)
}

// GetBusyTimesWithBatching fetches busy times for multiple users with configurable batch size
func GetBusyTimesWithBatching(service *calendar.Service, emails []string, startTime, endTime time.Time, batchSize int) ([]UserAvailability, error) {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	// If we have few enough emails, process in a single batch
	if len(emails) <= batchSize {
		return getBusyTimesBatch(service, emails, startTime, endTime)
	}

	// Process in batches
	log.Info().
		Int("total_emails", len(emails)).
		Int("batch_size", batchSize).
		Int("num_batches", (len(emails)+batchSize-1)/batchSize).
		Msg("Processing calendars in batches")

	var allAvailabilities []UserAvailability
	emailMap := make(map[string]bool) // Track which emails we've already processed

	for i := 0; i < len(emails); i += batchSize {
		end := i + batchSize
		if end > len(emails) {
			end = len(emails)
		}

		batch := emails[i:end]
		batchNum := (i / batchSize) + 1
		totalBatches := (len(emails) + batchSize - 1) / batchSize

		log.Debug().
			Int("batch_num", batchNum).
			Int("total_batches", totalBatches).
			Int("batch_size", len(batch)).
			Msg("Processing batch")

		// Get availability for this batch
		batchAvailabilities, err := getBusyTimesBatch(service, batch, startTime, endTime)
		if err != nil {
			// Log the error but continue with other batches
			log.Warn().
				Err(err).
				Int("batch_num", batchNum).
				Int("batch_size", len(batch)).
				Msg("Failed to get calendar data for batch")
			// Continue processing other batches rather than failing entirely
			continue
		}

		// Add unique results (avoid duplicates if an email appears in multiple batches)
		for _, avail := range batchAvailabilities {
			if !emailMap[avail.Email] {
				emailMap[avail.Email] = true
				allAvailabilities = append(allAvailabilities, avail)
			}
		}

		log.Debug().
			Int("batch_num", batchNum).
			Int("calendars_retrieved", len(batchAvailabilities)).
			Msg("Batch completed")
	}

	log.Info().
		Int("total_calendars_retrieved", len(allAvailabilities)).
		Int("total_requested", len(emails)).
		Msg("Batch processing completed")

	return allAvailabilities, nil
}

// getBusyTimesBatch fetches busy times for a single batch of users
func getBusyTimesBatch(service *calendar.Service, emails []string, startTime, endTime time.Time) ([]UserAvailability, error) {
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

// ValidateCalendarAccess checks which emails have accessible calendars
func ValidateCalendarAccess(service *calendar.Service, emails []string) []CalendarAccessResult {
	results := make([]CalendarAccessResult, 0, len(emails))

	for _, email := range emails {
		result := CalendarAccessResult{
			Email:     email,
			HasAccess: false,
		}

		// Try to get the calendar to check access
		_, err := service.Calendars.Get(email).Do()
		if err != nil {
			result.Error = err
			result.ErrorReason = categorizeCalendarError(err)
			log.Debug().Err(err).Str("email", email).Str("reason", result.ErrorReason).Msg("Calendar access check failed")
		} else {
			result.HasAccess = true
		}

		results = append(results, result)
	}

	return results
}

// GetMissingCalendars identifies which requested emails don't have calendar data in the response
func GetMissingCalendars(requestedEmails []string, availabilities []UserAvailability) []string {
	// Create a map of emails that returned data
	returnedEmails := make(map[string]bool)
	for _, avail := range availabilities {
		returnedEmails[avail.Email] = true
	}

	// Find which emails are missing
	var missing []string
	for _, email := range requestedEmails {
		if !returnedEmails[email] {
			missing = append(missing, email)
		}
	}

	return missing
}

// categorizeCalendarError determines the type of calendar access error
func categorizeCalendarError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	if errStr == "" {
		return "unknown"
	}

	// Check for common error patterns
	if strings.Contains(errStr, "404") || strings.Contains(errStr, "notFound") || strings.Contains(errStr, "Not Found") {
		return "no_calendar"
	}
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") || strings.Contains(errStr, "Permission denied") {
		return "permission_denied"
	}
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "Unauthorized") {
		return "unauthorized"
	}

	return "unknown"
}
