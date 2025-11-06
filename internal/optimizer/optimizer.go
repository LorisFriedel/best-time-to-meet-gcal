package optimizer

import (
	"sort"
	"time"

	"github.com/LorisFriedel/find-best-meeting-time-google/internal/calendar"
)

// MeetingSlot represents a potential meeting slot with conflict information
type MeetingSlot struct {
	TimeSlot            calendar.TimeSlot
	UnavailableCount    int
	UnavailableEmails   []string
	AvailableEmails     []string
	ConflictPercentage  float64
	TimeZoneScore       float64         // Score indicating how well the time works across timezones (0-100, higher is better)
	OutsideWorkingHours map[string]bool // Email -> true if outside their working hours
}

// FindOptimalMeetingSlots finds the best meeting times based on availability (legacy version)
func findOptimalMeetingSlotsLegacy(
	availabilities []calendar.UserAvailability,
	potentialSlots []calendar.TimeSlot,
	meetingDuration time.Duration,
	maxSlots int,
) []MeetingSlot {
	var meetingSlots []MeetingSlot

	// For each potential time slot
	for _, slot := range potentialSlots {
		// Generate meeting slots of the requested duration within this slot
		currentStart := slot.Start
		for currentStart.Add(meetingDuration).Before(slot.End) || currentStart.Add(meetingDuration).Equal(slot.End) {
			meetingEnd := currentStart.Add(meetingDuration)

			// Check conflicts for this meeting slot
			unavailable := []string{}
			available := []string{}

			for _, userAvail := range availabilities {
				hasConflict := false
				for _, busySlot := range userAvail.BusySlots {
					// Check if the meeting overlaps with this busy slot
					if overlaps(currentStart, meetingEnd, busySlot.Start, busySlot.End) {
						hasConflict = true
						break
					}
				}

				if hasConflict {
					unavailable = append(unavailable, userAvail.Email)
				} else {
					available = append(available, userAvail.Email)
				}
			}

			totalUsers := len(availabilities)
			conflictPercentage := 0.0
			if totalUsers > 0 {
				conflictPercentage = float64(len(unavailable)) / float64(totalUsers) * 100
			}

			meetingSlots = append(meetingSlots, MeetingSlot{
				TimeSlot: calendar.TimeSlot{
					Start: currentStart,
					End:   meetingEnd,
				},
				UnavailableCount:   len(unavailable),
				UnavailableEmails:  unavailable,
				AvailableEmails:    available,
				ConflictPercentage: conflictPercentage,
			})

			// Move to next slot (30-minute increments)
			currentStart = currentStart.Add(30 * time.Minute)
		}
	}

	// Sort by number of conflicts (ascending) to get the best slots first
	sort.Slice(meetingSlots, func(i, j int) bool {
		// Primary sort: fewer conflicts first
		if meetingSlots[i].UnavailableCount != meetingSlots[j].UnavailableCount {
			return meetingSlots[i].UnavailableCount < meetingSlots[j].UnavailableCount
		}
		// Secondary sort: earlier time first
		return meetingSlots[i].TimeSlot.Start.Before(meetingSlots[j].TimeSlot.Start)
	})

	// Return only the top N slots
	if len(meetingSlots) > maxSlots {
		return meetingSlots[:maxSlots]
	}
	return meetingSlots
}

// FindOptimalMeetingSlots finds the best meeting times considering timezones
func FindOptimalMeetingSlots(
	availabilities []calendar.UserAvailability,
	potentialSlots []calendar.TimeSlot,
	meetingDuration time.Duration,
	maxSlots int,
	workingHours WorkingHoursConfig,
) []MeetingSlot {
	var meetingSlots []MeetingSlot

	// For each potential time slot
	for _, slot := range potentialSlots {
		// Generate meeting slots of the requested duration within this slot
		currentStart := slot.Start
		for currentStart.Add(meetingDuration).Before(slot.End) || currentStart.Add(meetingDuration).Equal(slot.End) {
			meetingEnd := currentStart.Add(meetingDuration)

			// Check conflicts and timezone compatibility for this meeting slot
			unavailable := []string{}
			available := []string{}
			outsideWorkingHours := make(map[string]bool)
			workingHoursCount := 0

			for _, userAvail := range availabilities {
				hasConflict := false
				for _, busySlot := range userAvail.BusySlots {
					// Check if the meeting overlaps with this busy slot
					if overlaps(currentStart, meetingEnd, busySlot.Start, busySlot.End) {
						hasConflict = true
						break
					}
				}

				if hasConflict {
					unavailable = append(unavailable, userAvail.Email)
				} else {
					available = append(available, userAvail.Email)

					// Check if this time is within user's working hours
					if userAvail.TimeZone != nil {
						if isWithinWorkingHours(currentStart, meetingEnd, userAvail.TimeZone, workingHours) {
							workingHoursCount++
						} else {
							outsideWorkingHours[userAvail.Email] = true
						}
					} else {
						// If no timezone info, assume it's within working hours
						workingHoursCount++
					}
				}
			}

			totalUsers := len(availabilities)
			conflictPercentage := 0.0
			if totalUsers > 0 {
				conflictPercentage = float64(len(unavailable)) / float64(totalUsers) * 100
			}

			// Calculate timezone score (percentage of available users for whom this is within working hours)
			timezoneScore := 100.0
			if len(available) > 0 {
				timezoneScore = float64(workingHoursCount) / float64(len(available)) * 100
			}

			meetingSlots = append(meetingSlots, MeetingSlot{
				TimeSlot: calendar.TimeSlot{
					Start: currentStart,
					End:   meetingEnd,
				},
				UnavailableCount:    len(unavailable),
				UnavailableEmails:   unavailable,
				AvailableEmails:     available,
				ConflictPercentage:  conflictPercentage,
				TimeZoneScore:       timezoneScore,
				OutsideWorkingHours: outsideWorkingHours,
			})

			// Move to next slot (30-minute increments)
			currentStart = currentStart.Add(30 * time.Minute)
		}
	}

	// Sort by combined score (conflicts + timezone compatibility)
	sort.Slice(meetingSlots, func(i, j int) bool {
		// Calculate combined score (lower is better)
		// Weight: 70% for conflicts, 30% for timezone compatibility
		scoreI := meetingSlots[i].ConflictPercentage*0.7 + (100-meetingSlots[i].TimeZoneScore)*0.3
		scoreJ := meetingSlots[j].ConflictPercentage*0.7 + (100-meetingSlots[j].TimeZoneScore)*0.3

		if scoreI != scoreJ {
			return scoreI < scoreJ
		}
		// If scores are equal, prefer earlier times
		return meetingSlots[i].TimeSlot.Start.Before(meetingSlots[j].TimeSlot.Start)
	})

	// Return only the top N slots
	if len(meetingSlots) > maxSlots {
		return meetingSlots[:maxSlots]
	}
	return meetingSlots
}

// WorkingHoursConfig holds working hours configuration
type WorkingHoursConfig struct {
	StartHour       int
	EndHour         int
	LunchStartHour  int
	LunchEndHour    int
	ExcludeWeekends bool
}

// isWithinWorkingHours checks if a time slot is within working hours for a specific timezone
func isWithinWorkingHours(start, end time.Time, userTZ *time.Location, config WorkingHoursConfig) bool {
	// Convert to user's timezone
	startInUserTZ := start.In(userTZ)
	endInUserTZ := end.In(userTZ)

	// Check if it spans multiple days
	if startInUserTZ.Day() != endInUserTZ.Day() {
		return false // Meeting spans days, not ideal
	}

	// Check weekend
	if config.ExcludeWeekends && (startInUserTZ.Weekday() == time.Saturday || startInUserTZ.Weekday() == time.Sunday) {
		return false
	}

	// Check working hours
	startHour := startInUserTZ.Hour()
	startMinute := startInUserTZ.Minute()
	endHour := endInUserTZ.Hour()
	endMinute := endInUserTZ.Minute()

	// Convert to minutes for easier comparison
	startTotalMinutes := startHour*60 + startMinute
	endTotalMinutes := endHour*60 + endMinute
	workStartMinutes := config.StartHour * 60
	workEndMinutes := config.EndHour * 60
	lunchStartMinutes := config.LunchStartHour * 60
	lunchEndMinutes := config.LunchEndHour * 60

	// Check if it's within working hours
	if startTotalMinutes < workStartMinutes || endTotalMinutes > workEndMinutes {
		return false
	}

	// Check if it overlaps with lunch
	if startTotalMinutes < lunchEndMinutes && endTotalMinutes > lunchStartMinutes {
		return false
	}

	return true
}

// overlaps checks if two time ranges overlap
func overlaps(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && end1.After(start2)
}

// FilterSlotsByThreshold filters slots to only include those below a conflict threshold
func FilterSlotsByThreshold(slots []MeetingSlot, maxConflictPercentage float64) []MeetingSlot {
	var filtered []MeetingSlot
	for _, slot := range slots {
		if slot.ConflictPercentage <= maxConflictPercentage {
			filtered = append(filtered, slot)
		}
	}
	return filtered
}

// GroupSlotsByDay groups meeting slots by day for easier viewing
func GroupSlotsByDay(slots []MeetingSlot) map[string][]MeetingSlot {
	grouped := make(map[string][]MeetingSlot)

	for _, slot := range slots {
		day := slot.TimeSlot.Start.Format("2006-01-02")
		grouped[day] = append(grouped[day], slot)
	}

	return grouped
}

// GroupSlotsByConflictLevel groups slots by conflict percentage ranges
func GroupSlotsByConflictLevel(slots []MeetingSlot) map[string][]MeetingSlot {
	groups := map[string][]MeetingSlot{
		"no-conflicts":   {},
		"low-conflicts":  {}, // 1-25%
		"med-conflicts":  {}, // 26-50%
		"high-conflicts": {}, // 51-75%
		"very-high":      {}, // 76-100%
	}

	for _, slot := range slots {
		switch {
		case slot.ConflictPercentage == 0:
			groups["no-conflicts"] = append(groups["no-conflicts"], slot)
		case slot.ConflictPercentage <= 25:
			groups["low-conflicts"] = append(groups["low-conflicts"], slot)
		case slot.ConflictPercentage <= 50:
			groups["med-conflicts"] = append(groups["med-conflicts"], slot)
		case slot.ConflictPercentage <= 75:
			groups["high-conflicts"] = append(groups["high-conflicts"], slot)
		default:
			groups["very-high"] = append(groups["very-high"], slot)
		}
	}

	return groups
}

// GetDaySummaryStats calculates statistics for slots on a given day
func GetDaySummaryStats(slots []MeetingSlot) (bestConflict float64, avgConflict float64, noConflictCount int, bestTimezoneScore float64) {
	if len(slots) == 0 {
		return 100, 100, 0, 0
	}

	bestConflict = 100.0
	totalConflict := 0.0
	bestTimezoneScore = 0.0

	for _, slot := range slots {
		if slot.ConflictPercentage < bestConflict {
			bestConflict = slot.ConflictPercentage
		}
		if slot.ConflictPercentage == 0 {
			noConflictCount++
		}
		if slot.TimeZoneScore > bestTimezoneScore {
			bestTimezoneScore = slot.TimeZoneScore
		}
		totalConflict += slot.ConflictPercentage
	}

	avgConflict = totalConflict / float64(len(slots))
	return
}
