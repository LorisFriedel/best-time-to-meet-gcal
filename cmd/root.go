package cmd

import (
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
)

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

	// Find optimal meeting times
	optimalSlots := optimizer.FindOptimalMeetingSlots(
		availabilities,
		potentialSlots,
		meetingDuration,
		viper.GetInt("max_slots")*3, // Get more slots initially for filtering
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
		fmt.Println("No suitable meeting times found within the specified constraints.")
		return
	}

	// Output results
	fmt.Printf("\nTop %d meeting times with least conflicts:\n", len(filteredSlots))
	fmt.Println(strings.Repeat("-", 80))

	for i, slot := range filteredSlots {
		fmt.Printf("\n%d. %s - %s\n",
			i+1,
			slot.TimeSlot.Start.Format("Mon, Jan 2, 2006 at 15:04"),
			slot.TimeSlot.End.Format("15:04"),
		)

		if slot.UnavailableCount == 0 {
			fmt.Printf("   ✅ All attendees available!\n")
		} else {
			fmt.Printf("   ⚠️  %d/%d attendees unavailable (%.1f%% conflict)\n",
				slot.UnavailableCount,
				len(availabilities),
				slot.ConflictPercentage,
			)
			fmt.Printf("   Unavailable: %s\n", strings.Join(slot.UnavailableEmails, ", "))
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 80))

	// Group by day for summary
	grouped := optimizer.GroupSlotsByDay(filteredSlots)

	// Sort days chronologically
	var days []string
	for day := range grouped {
		days = append(days, day)
	}
	sort.Strings(days) // sorts in YYYY-MM-DD format naturally

	fmt.Printf("\nSummary by day:\n")
	for _, day := range days {
		slots := grouped[day]
		dayTime, _ := time.Parse("2006-01-02", day)
		fmt.Printf("  %s: %d potential slots\n", dayTime.Format("Mon, Jan 2"), len(slots))
	}
}
