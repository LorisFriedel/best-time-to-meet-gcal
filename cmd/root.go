package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/LorisFriedel/find-best-meeting-time-google/internal/auth"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/calendar"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/directory"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/logger"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/optimizer"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	credentialsFile string
	emails          string
	mailingLists    string
	startDate       string
	endDate         string
	duration        int
	startHour       int
	endHour         int
	lunchStartHour  int
	lunchEndHour    int
	timezone        string
	maxSlots        int
	excludeWeekends bool
	maxConflicts    float64
	debug           bool
	jsonOutput      bool
)

// JSONOutput represents the complete output in JSON format
type JSONOutput struct {
	Metadata       OutputMetadata      `json:"metadata"`
	Summary        Summary             `json:"summary"`
	TimezoneInfo   TimezoneInfo        `json:"timezone_info"`
	BestOptions    BestOptions         `json:"best_options"`
	DailySummary   []DailySummary      `json:"daily_summary"`
	DetailedSlots  []DetailedTimeSlot  `json:"detailed_slots"`
	Recommendation *RecommendationSlot `json:"recommendation"`
}

// OutputMetadata contains metadata about the search
type OutputMetadata struct {
	SearchStartDate     string  `json:"search_start_date"`
	SearchEndDate       string  `json:"search_end_date"`
	MeetingDuration     int     `json:"meeting_duration_minutes"`
	TotalAttendees      int     `json:"total_attendees"`
	AccessibleCalendars int     `json:"accessible_calendars"`
	WorkingHours        string  `json:"working_hours"`
	LunchHours          string  `json:"lunch_hours"`
	ExcludeWeekends     bool    `json:"exclude_weekends"`
	MaxConflicts        float64 `json:"max_conflicts_percentage"`
	Timezone            string  `json:"timezone"`
}

// Summary contains high-level statistics
type Summary struct {
	TotalSlotsFound     int `json:"total_slots_found"`
	PerfectSlots        int `json:"perfect_slots"`
	LowConflictSlots    int `json:"low_conflict_slots"`
	MediumConflictSlots int `json:"medium_conflict_slots"`
}

// TimezoneInfo contains timezone information for attendees
type TimezoneInfo struct {
	AttendeesByTimezone map[string][]string `json:"attendees_by_timezone"`
	WorkingHoursNote    string              `json:"working_hours_note"`
}

// BestOptions contains categorized best meeting options
type BestOptions struct {
	PerfectSlots []TimeSlotSummary `json:"perfect_slots"`
	GoodOptions  []TimeSlotSummary `json:"good_options"`
}

// TimeSlotSummary is a simplified view of a time slot
type TimeSlotSummary struct {
	StartTime          string  `json:"start_time"`
	EndTime            string  `json:"end_time"`
	ConflictPercentage float64 `json:"conflict_percentage"`
	ConflictCount      int     `json:"conflict_count"`
}

// DailySummary contains summary statistics for a day
type DailySummary struct {
	Date             string  `json:"date"`
	TotalSlots       int     `json:"total_slots"`
	PerfectSlots     int     `json:"perfect_slots"`
	BestConflict     float64 `json:"best_conflict_percentage"`
	AverageConflict  float64 `json:"average_conflict_percentage"`
	EarliestSlotTime string  `json:"earliest_slot_time"`
	LatestSlotTime   string  `json:"latest_slot_time"`
}

// DetailedTimeSlot contains detailed information about a time slot
type DetailedTimeSlot struct {
	StartTime          string              `json:"start_time"`
	EndTime            string              `json:"end_time"`
	ConflictPercentage float64             `json:"conflict_percentage"`
	UnavailableCount   int                 `json:"unavailable_count"`
	UnavailableEmails  []string            `json:"unavailable_emails"`
	AvailableEmails    []string            `json:"available_emails"`
	TimeZoneScore      float64             `json:"timezone_score"`
	ConflictsByType    map[string][]string `json:"conflicts_by_type"`
}

// RecommendationSlot contains the recommended meeting slot
type RecommendationSlot struct {
	StartTime             string  `json:"start_time"`
	EndTime               string  `json:"end_time"`
	ConflictPercentage    float64 `json:"conflict_percentage"`
	UnavailableCount      int     `json:"unavailable_count"`
	CalendarConflicts     int     `json:"calendar_conflicts"`
	WorkingHoursConflicts int     `json:"working_hours_conflicts"`
	Reason                string  `json:"reason"`
}

var rootCmd = &cobra.Command{
	Use:   "best-time-to-meet",
	Short: "Find optimal meeting times across multiple Google calendars",
	Long: `A tool that analyzes multiple Google calendars to find the best meeting times
with the least number of conflicts. It uses the Google Calendar API to check
availability and suggests optimal time slots.`,
	Run: runFindMeetingTime,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().StringVar(&credentialsFile, "credentials", "credentials.json", "Google API credentials file")

	rootCmd.Flags().StringVarP(&emails, "emails", "e", "", "Comma-separated list of individual email addresses")
	rootCmd.Flags().StringVarP(&mailingLists, "mailing-lists", "l", "", "Comma-separated list of mailing list/group email addresses")
	rootCmd.Flags().StringVarP(&startDate, "start", "s", "", "Start date (YYYY-MM-DD) (required)")
	rootCmd.Flags().StringVarP(&endDate, "end", "E", "", "End date (YYYY-MM-DD) (required)")
	rootCmd.Flags().IntVarP(&duration, "duration", "d", 60, "Meeting duration in minutes")
	rootCmd.Flags().IntVar(&startHour, "start-hour", 9, "Working hours start (24-hour format)")
	rootCmd.Flags().IntVar(&endHour, "end-hour", 17, "Working hours end (24-hour format)")
	rootCmd.Flags().IntVar(&lunchStartHour, "lunch-start-hour", 12, "Lunch break start (24-hour format)")
	rootCmd.Flags().IntVar(&lunchEndHour, "lunch-end-hour", 13, "Lunch break end (24-hour format)")
	rootCmd.Flags().StringVar(&timezone, "timezone", "", "IANA timezone (e.g. 'America/New_York'). If empty, uses local timezone")
	rootCmd.Flags().IntVarP(&maxSlots, "max-slots", "m", 10, "Maximum number of slots to display")
	rootCmd.Flags().BoolVarP(&excludeWeekends, "exclude-weekends", "w", true, "Exclude weekends from search")
	rootCmd.Flags().Float64VarP(&maxConflicts, "max-conflicts", "c", 100, "Maximum conflict percentage to display (0-100)")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	// At least one of emails or mailing-lists is required
	rootCmd.MarkFlagRequired("start")
	rootCmd.MarkFlagRequired("end")

	// Bind flags to viper
	viper.BindPFlag("credentials", rootCmd.PersistentFlags().Lookup("credentials"))
	viper.BindPFlag("emails", rootCmd.Flags().Lookup("emails"))
	viper.BindPFlag("mailing_lists", rootCmd.Flags().Lookup("mailing-lists"))
	viper.BindPFlag("start", rootCmd.Flags().Lookup("start"))
	viper.BindPFlag("end", rootCmd.Flags().Lookup("end"))
	viper.BindPFlag("duration", rootCmd.Flags().Lookup("duration"))
	viper.BindPFlag("start_hour", rootCmd.Flags().Lookup("start-hour"))
	viper.BindPFlag("end_hour", rootCmd.Flags().Lookup("end-hour"))
	viper.BindPFlag("lunch_start_hour", rootCmd.Flags().Lookup("lunch-start-hour"))
	viper.BindPFlag("lunch_end_hour", rootCmd.Flags().Lookup("lunch-end-hour"))
	viper.BindPFlag("timezone", rootCmd.Flags().Lookup("timezone"))
	viper.BindPFlag("max_slots", rootCmd.Flags().Lookup("max-slots"))
	viper.BindPFlag("exclude_weekends", rootCmd.Flags().Lookup("exclude-weekends"))
	viper.BindPFlag("max_conflicts", rootCmd.Flags().Lookup("max-conflicts"))
	viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	viper.BindPFlag("json_output", rootCmd.Flags().Lookup("json"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func runFindMeetingTime(cmd *cobra.Command, args []string) {
	// Initialize logger
	logger.Init(viper.GetBool("debug"))

	// Parse inputs
	emailsStr := viper.GetString("emails")
	mailingListsStr := viper.GetString("mailing_lists")

	// Check that at least one of emails or mailing-lists is provided
	if emailsStr == "" && mailingListsStr == "" {
		log.Fatal().Msg("At least one of --emails or --mailing-lists must be provided")
	}

	var allEmails []string

	// Parse individual emails
	if emailsStr != "" {
		emails := strings.Split(emailsStr, ",")
		for _, email := range emails {
			email = strings.TrimSpace(email)
			if email != "" {
				allEmails = append(allEmails, email)
			}
		}
	}

	// Parse and resolve mailing lists
	if mailingListsStr != "" {
		mailingLists := strings.Split(mailingListsStr, ",")
		var mailingListsClean []string
		for _, ml := range mailingLists {
			ml = strings.TrimSpace(ml)
			if ml != "" {
				mailingListsClean = append(mailingListsClean, ml)
			}
		}

		if len(mailingListsClean) > 0 {
			// Get Directory service
			directoryService, err := auth.GetDirectoryService(viper.GetString("credentials"))
			if err != nil {
				log.Warn().Err(err).Msg("Could not get Directory service for mailing list resolution")
				log.Warn().Msg("Treating mailing lists as individual emails")
				allEmails = append(allEmails, mailingListsClean...)
			} else {
				// Check if we have proper access
				if err := directory.CheckGroupAccess(directoryService); err != nil {
					log.Warn().Err(err).Msg("Group access check failed")
					log.Warn().Msg("Treating mailing lists as individual emails")
					allEmails = append(allEmails, mailingListsClean...)
				} else {
					// Resolve mailing list members
					log.Info().Msg("Resolving mailing lists...")
					resolvedEmails, err := directory.ResolveMemberEmails(directoryService, mailingListsClean)
					if err != nil {
						log.Warn().Err(err).Msg("Error resolving mailing lists")
						log.Warn().Msg("Treating mailing lists as individual emails")
						allEmails = append(allEmails, mailingListsClean...)
					} else {
						allEmails = append(allEmails, resolvedEmails...)
					}
				}
			}
		}
	}

	// Remove duplicates
	emailMap := make(map[string]bool)
	var emailList []string
	for _, email := range allEmails {
		if !emailMap[email] {
			emailMap[email] = true
			emailList = append(emailList, email)
		}
	}

	if len(emailList) == 0 {
		log.Fatal().Msg("No valid email addresses found")
	}

	// Handle timezone
	var loc *time.Location
	tzName := viper.GetString("timezone")
	if tzName == "" {
		loc = time.Local
	} else {
		var err error
		loc, err = time.LoadLocation(tzName)
		if err != nil {
			log.Fatal().Err(err).Str("timezone", tzName).Msg("Invalid timezone")
		}
	}

	// Parse dates in the specified timezone
	startTime, err := time.ParseInLocation("2006-01-02", viper.GetString("start"), loc)
	if err != nil {
		log.Fatal().Err(err).Str("date", viper.GetString("start")).Msg("Invalid start date")
	}

	endTime, err := time.ParseInLocation("2006-01-02", viper.GetString("end"), loc)
	if err != nil {
		log.Fatal().Err(err).Str("date", viper.GetString("end")).Msg("Invalid end date")
	}

	meetingDuration := time.Duration(viper.GetInt("duration")) * time.Minute

	log.Info().Msg("Searching for optimal meeting times...")
	log.Info().
		Strs("attendees", emailList).
		Str("start_date", startTime.Format("2006-01-02")).
		Str("end_date", endTime.Format("2006-01-02")).
		Int("duration_minutes", viper.GetInt("duration")).
		Int("start_hour", viper.GetInt("start_hour")).
		Int("end_hour", viper.GetInt("end_hour")).
		Int("lunch_start_hour", viper.GetInt("lunch_start_hour")).
		Int("lunch_end_hour", viper.GetInt("lunch_end_hour")).
		Str("timezone", loc.String()).
		Bool("exclude_weekends", viper.GetBool("exclude_weekends")).
		Msg("Search parameters")

	// Initialize Google Calendar service
	service, err := auth.GetCalendarService(viper.GetString("credentials"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get calendar service")
	}

	// Get busy times for all attendees
	availabilities, err := calendar.GetBusyTimes(service, emailList, startTime, endTime.Add(24*time.Hour))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get busy times")
	}

	log.Debug().
		Int("requested_attendees", len(emailList)).
		Int("available_calendars", len(availabilities)).
		Msg("Calendar access summary")

	for _, avail := range availabilities {
		log.Debug().Str("email", avail.Email).Msg("Got calendar data")
	}

	// If we couldn't get calendar data for all attendees, show a warning
	if len(availabilities) < len(emailList) {
		log.Warn().
			Int("accessible_calendars", len(availabilities)).
			Int("requested_attendees", len(emailList)).
			Msg("Could not access all requested calendars. Results are based only on accessible calendars.")
	}

	// Get potential meeting slots (working hours)
	potentialSlots := calendar.GetWorkingHours(
		startTime,
		endTime,
		viper.GetInt("start_hour"),
		viper.GetInt("end_hour"),
		viper.GetInt("lunch_start_hour"),
		viper.GetInt("lunch_end_hour"),
		viper.GetBool("exclude_weekends"),
	)

	// Create working hours config
	workingHoursConfig := optimizer.WorkingHoursConfig{
		StartHour:       viper.GetInt("start_hour"),
		EndHour:         viper.GetInt("end_hour"),
		LunchStartHour:  viper.GetInt("lunch_start_hour"),
		LunchEndHour:    viper.GetInt("lunch_end_hour"),
		ExcludeWeekends: viper.GetBool("exclude_weekends"),
	}

	// Find optimal meeting times
	optimalSlots := optimizer.FindOptimalMeetingSlots(
		availabilities,
		potentialSlots,
		meetingDuration,
		viper.GetInt("max_slots")*3, // Get more slots initially for filtering
		workingHoursConfig,
	)

	log.Debug().
		Int("total_slots", len(optimalSlots)).
		Msg("Found optimal slots")

	if len(optimalSlots) > 0 {
		log.Debug().
			Float64("first_slot_conflict_pct", optimalSlots[0].ConflictPercentage).
			Float64("last_slot_conflict_pct", optimalSlots[len(optimalSlots)-1].ConflictPercentage).
			Msg("Conflict percentage range")
	}

	// Filter by conflict threshold
	filteredSlots := optimizer.FilterSlotsByThreshold(optimalSlots, viper.GetFloat64("max_conflicts"))

	log.Debug().
		Float64("max_conflicts_threshold", viper.GetFloat64("max_conflicts")).
		Int("filtered_slots", len(filteredSlots)).
		Msg("Filtered by conflict threshold")

	// Limit to requested number of slots
	if len(filteredSlots) > viper.GetInt("max_slots") {
		filteredSlots = filteredSlots[:viper.GetInt("max_slots")]
	}

	// Sort the final results chronologically for calendar-style display
	sort.Slice(filteredSlots, func(i, j int) bool {
		return filteredSlots[i].TimeSlot.Start.Before(filteredSlots[j].TimeSlot.Start)
	})

	// Display results
	if len(filteredSlots) == 0 {
		if viper.GetBool("json_output") {
			output := JSONOutput{
				Metadata: OutputMetadata{
					SearchStartDate:     startTime.Format("2006-01-02"),
					SearchEndDate:       endTime.Format("2006-01-02"),
					MeetingDuration:     viper.GetInt("duration"),
					TotalAttendees:      len(emailList),
					AccessibleCalendars: len(availabilities),
					WorkingHours:        fmt.Sprintf("%d:00 - %d:00", viper.GetInt("start_hour"), viper.GetInt("end_hour")),
					LunchHours:          fmt.Sprintf("%d:00 - %d:00", viper.GetInt("lunch_start_hour"), viper.GetInt("lunch_end_hour")),
					ExcludeWeekends:     viper.GetBool("exclude_weekends"),
					MaxConflicts:        viper.GetFloat64("max_conflicts"),
					Timezone:            loc.String(),
				},
				Summary: Summary{
					TotalSlotsFound: 0,
				},
			}
			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to marshal JSON output")
			}
			fmt.Println(string(jsonData))
		} else {
			fmt.Println("No suitable meeting times found within the specified constraints.")
		}
		return
	}

	// Check for JSON output mode
	if viper.GetBool("json_output") {
		outputJSON(availabilities, filteredSlots, optimalSlots, emailList, startTime, endTime, loc)
		return
	}

	// === TIMEZONE SUMMARY ===
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🌍 TIMEZONE INFORMATION")
	fmt.Println(strings.Repeat("=", 80))

	// Collect timezone information
	tzMap := make(map[string][]string) // timezone -> list of emails
	for _, avail := range availabilities {
		tzName := "Unknown"
		if avail.TimeZone != nil {
			tzName = avail.TimeZone.String()
		}
		tzMap[tzName] = append(tzMap[tzName], avail.Email)
	}

	fmt.Printf("\nAttendees by timezone:\n")
	for tz, emails := range tzMap {
		fmt.Printf("  • %s: %d attendee(s)\n", tz, len(emails))
		for _, email := range emails {
			fmt.Printf("    - %s\n", email)
		}
	}
	fmt.Printf("\nWorking hours: %d:00 - %d:00 (in each attendee's local time)\n",
		viper.GetInt("start_hour"), viper.GetInt("end_hour"))

	// === BEST OPTIONS SUMMARY ===
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📅 BEST MEETING TIME OPTIONS")
	fmt.Println(strings.Repeat("=", 80))

	// Group slots by conflict level
	conflictGroups := optimizer.GroupSlotsByConflictLevel(filteredSlots)

	// Show best options first
	if len(conflictGroups["no-conflicts"]) > 0 {
		fmt.Printf("\n🏆 PERFECT SLOTS (All attendees available):\n")
		fmt.Printf("   Found %d perfect slot(s)\n\n", len(conflictGroups["no-conflicts"]))

		// Show up to 3 best no-conflict slots
		maxShow := 3
		if len(conflictGroups["no-conflicts"]) < maxShow {
			maxShow = len(conflictGroups["no-conflicts"])
		}

		for i := 0; i < maxShow; i++ {
			slot := conflictGroups["no-conflicts"][i]
			fmt.Printf("   ⭐ %s - %s\n",
				slot.TimeSlot.Start.Format("Mon, Jan 2 at 15:04"),
				slot.TimeSlot.End.Format("15:04"),
			)
		}
		if len(conflictGroups["no-conflicts"]) > 3 {
			fmt.Printf("   ... and %d more perfect slots\n", len(conflictGroups["no-conflicts"])-3)
		}
	}

	// Show slots with minimal conflicts
	if len(conflictGroups["low-conflicts"]) > 0 {
		fmt.Printf("\n✅ GOOD OPTIONS (1-25%% conflicts):\n")
		fmt.Printf("   Found %d slot(s) with minimal conflicts\n", len(conflictGroups["low-conflicts"]))

		// Show best one from this group
		bestLowConflict := conflictGroups["low-conflicts"][0]
		for _, slot := range conflictGroups["low-conflicts"] {
			if slot.ConflictPercentage < bestLowConflict.ConflictPercentage {
				bestLowConflict = slot
			}
		}
		fmt.Printf("   Best: %s - %s (%.0f%% conflict)\n",
			bestLowConflict.TimeSlot.Start.Format("Mon, Jan 2 at 15:04"),
			bestLowConflict.TimeSlot.End.Format("15:04"),
			bestLowConflict.ConflictPercentage,
		)
	}

	// === SUMMARY BY DAY ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Printf("\n📊 AVAILABILITY SUMMARY BY DAY:\n\n")

	// Group by day for summary
	grouped := optimizer.GroupSlotsByDay(filteredSlots)

	// Sort days chronologically
	var days []string
	for day := range grouped {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, day := range days {
		slots := grouped[day]
		dayTime, _ := time.Parse("2006-01-02", day)

		bestConflict, avgConflict, perfectCount, _ := optimizer.GetDaySummaryStats(slots)

		dayName := dayTime.Format("Mon, Jan 2")
		fmt.Printf("📆 %s\n", dayName)
		fmt.Printf("   Total slots: %d | Perfect slots: %d | Best conflict: %.0f%% | Avg: %.0f%%\n",
			len(slots), perfectCount, bestConflict, avgConflict)

		// Show time ranges for this day
		if len(slots) > 0 {
			// Find earliest and latest slots
			earliest := slots[0].TimeSlot.Start
			latest := slots[0].TimeSlot.End
			for _, s := range slots {
				if s.TimeSlot.Start.Before(earliest) {
					earliest = s.TimeSlot.Start
				}
				if s.TimeSlot.End.After(latest) {
					latest = s.TimeSlot.End
				}
			}
			fmt.Printf("   Time range: %s - %s\n",
				earliest.Format("15:04"),
				latest.Format("15:04"))
		}
		fmt.Println()
	}

	// === DETAILED TIME SLOTS ===
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("\n📋 DETAILED TIME SLOTS (Top %d):\n", len(filteredSlots))
	fmt.Println(strings.Repeat("-", 80))

	for i, slot := range filteredSlots {
		fmt.Printf("\n%d. %s - %s",
			i+1,
			slot.TimeSlot.Start.Format("Mon, Jan 2, 2006 at 15:04"),
			slot.TimeSlot.End.Format("15:04"),
		)

		if slot.UnavailableCount == 0 {
			fmt.Printf(" ✅ Perfect - All attendees available!\n")
		} else {
			conflictIcon := "⚠️"
			if slot.ConflictPercentage > 50 {
				conflictIcon = "❌"
			} else if slot.ConflictPercentage <= 25 {
				conflictIcon = "🟡"
			}

			fmt.Printf(" %s %.0f%% conflict\n",
				conflictIcon,
				slot.ConflictPercentage,
			)

			// Show conflicts by type
			if len(slot.ConflictsByType["calendar"]) > 0 {
				fmt.Printf("   📅 Calendar conflicts (%d): %s\n",
					len(slot.ConflictsByType["calendar"]),
					strings.Join(slot.ConflictsByType["calendar"], ", "))
			}
			if len(slot.ConflictsByType["working_hours"]) > 0 {
				fmt.Printf("   ⏰ Outside working hours (%d): %s\n",
					len(slot.ConflictsByType["working_hours"]),
					strings.Join(slot.ConflictsByType["working_hours"], ", "))
			}
		}
	}

	// === QUICK RECOMMENDATION ===
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("💡 RECOMMENDATION:")

	// Find the actual best slot across all filtered slots
	var bestSlot optimizer.MeetingSlot
	if len(filteredSlots) > 0 {
		bestSlot = filteredSlots[0]
		for _, slot := range filteredSlots {
			// Since working hours violations are now counted as conflicts,
			// we can simply use conflict percentage as the primary criterion
			if slot.ConflictPercentage < bestSlot.ConflictPercentage {
				bestSlot = slot
			} else if slot.ConflictPercentage == bestSlot.ConflictPercentage {
				// If conflicts are equal, prefer fewer working hours violations
				currentWorkingHoursConflicts := len(slot.ConflictsByType["working_hours"])
				bestWorkingHoursConflicts := len(bestSlot.ConflictsByType["working_hours"])

				if currentWorkingHoursConflicts < bestWorkingHoursConflicts {
					bestSlot = slot
				} else if currentWorkingHoursConflicts == bestWorkingHoursConflicts {
					// If still equal, prefer earlier time
					if slot.TimeSlot.Start.Before(bestSlot.TimeSlot.Start) {
						bestSlot = slot
					}
				}
			}
		}

		fmt.Printf("   Book: %s - %s\n",
			bestSlot.TimeSlot.Start.Format("Monday, January 2 at 15:04"),
			bestSlot.TimeSlot.End.Format("15:04"),
		)

		if bestSlot.UnavailableCount == 0 {
			fmt.Println("   This slot has perfect attendance with all attendees available!")
		} else {
			fmt.Printf("   Only %.0f%% conflict rate (%d/%d unavailable)\n",
				bestSlot.ConflictPercentage,
				bestSlot.UnavailableCount,
				len(availabilities),
			)

			// Show breakdown of conflicts
			if len(bestSlot.ConflictsByType["calendar"]) > 0 {
				fmt.Printf("   - Calendar conflicts: %d attendee(s)\n", len(bestSlot.ConflictsByType["calendar"]))
			}
			if len(bestSlot.ConflictsByType["working_hours"]) > 0 {
				fmt.Printf("   - Outside working hours: %d attendee(s)\n", len(bestSlot.ConflictsByType["working_hours"]))
			}

			// If this matches what we showed in the GOOD OPTIONS section, mention it
			if len(conflictGroups["low-conflicts"]) > 0 {
				bestLowConflict := conflictGroups["low-conflicts"][0]
				for _, slot := range conflictGroups["low-conflicts"] {
					if slot.ConflictPercentage < bestLowConflict.ConflictPercentage {
						bestLowConflict = slot
					}
				}
				if bestSlot.TimeSlot.Start.Equal(bestLowConflict.TimeSlot.Start) &&
					bestSlot.ConflictPercentage == bestLowConflict.ConflictPercentage {
					fmt.Println("   (This matches the best option shown above)")
				}
			}
		}
	}
	fmt.Println(strings.Repeat("=", 80))
}

// outputJSON outputs the results in JSON format
func outputJSON(availabilities []calendar.UserAvailability, filteredSlots []optimizer.MeetingSlot,
	allSlots []optimizer.MeetingSlot, emailList []string, startTime, endTime time.Time, loc *time.Location) {

	output := JSONOutput{
		Metadata: OutputMetadata{
			SearchStartDate:     startTime.Format("2006-01-02"),
			SearchEndDate:       endTime.Format("2006-01-02"),
			MeetingDuration:     viper.GetInt("duration"),
			TotalAttendees:      len(emailList),
			AccessibleCalendars: len(availabilities),
			WorkingHours:        fmt.Sprintf("%d:00 - %d:00", viper.GetInt("start_hour"), viper.GetInt("end_hour")),
			LunchHours:          fmt.Sprintf("%d:00 - %d:00", viper.GetInt("lunch_start_hour"), viper.GetInt("lunch_end_hour")),
			ExcludeWeekends:     viper.GetBool("exclude_weekends"),
			MaxConflicts:        viper.GetFloat64("max_conflicts"),
			Timezone:            loc.String(),
		},
	}

	// Prepare timezone info
	tzMap := make(map[string][]string)
	for _, avail := range availabilities {
		tzName := "Unknown"
		if avail.TimeZone != nil {
			tzName = avail.TimeZone.String()
		}
		tzMap[tzName] = append(tzMap[tzName], avail.Email)
	}

	output.TimezoneInfo = TimezoneInfo{
		AttendeesByTimezone: tzMap,
		WorkingHoursNote: fmt.Sprintf("%d:00 - %d:00 (in each attendee's local time)",
			viper.GetInt("start_hour"), viper.GetInt("end_hour")),
	}

	// Group slots by conflict level
	conflictGroups := optimizer.GroupSlotsByConflictLevel(filteredSlots)

	// Prepare summary
	output.Summary = Summary{
		TotalSlotsFound:     len(filteredSlots),
		PerfectSlots:        len(conflictGroups["no-conflicts"]),
		LowConflictSlots:    len(conflictGroups["low-conflicts"]),
		MediumConflictSlots: len(conflictGroups["med-conflicts"]),
	}

	// Prepare best options
	output.BestOptions = BestOptions{
		PerfectSlots: []TimeSlotSummary{},
		GoodOptions:  []TimeSlotSummary{},
	}

	// Add perfect slots (up to 5)
	maxPerfect := 5
	if len(conflictGroups["no-conflicts"]) < maxPerfect {
		maxPerfect = len(conflictGroups["no-conflicts"])
	}
	for i := 0; i < maxPerfect; i++ {
		slot := conflictGroups["no-conflicts"][i]
		output.BestOptions.PerfectSlots = append(output.BestOptions.PerfectSlots, TimeSlotSummary{
			StartTime:          slot.TimeSlot.Start.Format("2006-01-02T15:04:05Z07:00"),
			EndTime:            slot.TimeSlot.End.Format("2006-01-02T15:04:05Z07:00"),
			ConflictPercentage: slot.ConflictPercentage,
			ConflictCount:      slot.UnavailableCount,
		})
	}

	// Add good options (up to 5)
	maxGood := 5
	if len(conflictGroups["low-conflicts"]) < maxGood {
		maxGood = len(conflictGroups["low-conflicts"])
	}
	for i := 0; i < maxGood; i++ {
		slot := conflictGroups["low-conflicts"][i]
		output.BestOptions.GoodOptions = append(output.BestOptions.GoodOptions, TimeSlotSummary{
			StartTime:          slot.TimeSlot.Start.Format("2006-01-02T15:04:05Z07:00"),
			EndTime:            slot.TimeSlot.End.Format("2006-01-02T15:04:05Z07:00"),
			ConflictPercentage: slot.ConflictPercentage,
			ConflictCount:      slot.UnavailableCount,
		})
	}

	// Prepare daily summary
	grouped := optimizer.GroupSlotsByDay(filteredSlots)
	var days []string
	for day := range grouped {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, day := range days {
		slots := grouped[day]
		bestConflict, avgConflict, perfectCount, _ := optimizer.GetDaySummaryStats(slots)

		// Find earliest and latest slots
		earliest := slots[0].TimeSlot.Start
		latest := slots[0].TimeSlot.End
		for _, s := range slots {
			if s.TimeSlot.Start.Before(earliest) {
				earliest = s.TimeSlot.Start
			}
			if s.TimeSlot.End.After(latest) {
				latest = s.TimeSlot.End
			}
		}

		output.DailySummary = append(output.DailySummary, DailySummary{
			Date:             day,
			TotalSlots:       len(slots),
			PerfectSlots:     perfectCount,
			BestConflict:     bestConflict,
			AverageConflict:  avgConflict,
			EarliestSlotTime: earliest.Format("15:04"),
			LatestSlotTime:   latest.Format("15:04"),
		})
	}

	// Prepare detailed slots
	for _, slot := range filteredSlots {
		output.DetailedSlots = append(output.DetailedSlots, DetailedTimeSlot{
			StartTime:          slot.TimeSlot.Start.Format("2006-01-02T15:04:05Z07:00"),
			EndTime:            slot.TimeSlot.End.Format("2006-01-02T15:04:05Z07:00"),
			ConflictPercentage: slot.ConflictPercentage,
			UnavailableCount:   slot.UnavailableCount,
			UnavailableEmails:  slot.UnavailableEmails,
			AvailableEmails:    slot.AvailableEmails,
			TimeZoneScore:      slot.TimeZoneScore,
			ConflictsByType:    slot.ConflictsByType,
		})
	}

	// Find the best recommendation
	if len(filteredSlots) > 0 {
		bestSlot := filteredSlots[0]
		for _, slot := range filteredSlots {
			if slot.ConflictPercentage < bestSlot.ConflictPercentage {
				bestSlot = slot
			} else if slot.ConflictPercentage == bestSlot.ConflictPercentage {
				currentWorkingHoursConflicts := len(slot.ConflictsByType["working_hours"])
				bestWorkingHoursConflicts := len(bestSlot.ConflictsByType["working_hours"])

				if currentWorkingHoursConflicts < bestWorkingHoursConflicts {
					bestSlot = slot
				} else if currentWorkingHoursConflicts == bestWorkingHoursConflicts {
					if slot.TimeSlot.Start.Before(bestSlot.TimeSlot.Start) {
						bestSlot = slot
					}
				}
			}
		}

		reason := "Best overall slot with lowest conflicts"
		if bestSlot.UnavailableCount == 0 {
			reason = "Perfect slot with all attendees available"
		} else if bestSlot.ConflictPercentage <= 25 {
			reason = "Good slot with minimal conflicts"
		}

		output.Recommendation = &RecommendationSlot{
			StartTime:             bestSlot.TimeSlot.Start.Format("2006-01-02T15:04:05Z07:00"),
			EndTime:               bestSlot.TimeSlot.End.Format("2006-01-02T15:04:05Z07:00"),
			ConflictPercentage:    bestSlot.ConflictPercentage,
			UnavailableCount:      bestSlot.UnavailableCount,
			CalendarConflicts:     len(bestSlot.ConflictsByType["calendar"]),
			WorkingHoursConflicts: len(bestSlot.ConflictsByType["working_hours"]),
			Reason:                reason,
		}
	}

	// Marshal and output JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal JSON output")
	}
	fmt.Println(string(jsonData))
}
