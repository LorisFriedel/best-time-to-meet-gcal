package optimizer

import (
	"sort"
	"time"

	"github.com/LorisFriedel/find-best-meeting-time-google/internal/calendar"
)

// MeetingSlot represents a potential meeting slot with conflict information
type MeetingSlot struct {
	TimeSlot           calendar.TimeSlot
	UnavailableCount   int
	UnavailableEmails  []string
	AvailableEmails    []string
	ConflictPercentage float64
}

// FindOptimalMeetingSlots finds the best meeting times based on availability
func FindOptimalMeetingSlots(
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
