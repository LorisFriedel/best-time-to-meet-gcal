package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/LorisFriedel/find-best-meeting-time-google/internal/auth"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/calendar"
	"github.com/LorisFriedel/find-best-meeting-time-google/internal/optimizer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	credentialsFile string
	emails          string
	startDate       string
	endDate         string
	duration        int
	startHour       int
	endHour         int
	maxSlots        int
	excludeWeekends bool
	maxConflicts    float64
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

	rootCmd.Flags().StringVarP(&emails, "emails", "e", "", "Comma-separated list of email addresses (required)")
	rootCmd.Flags().StringVarP(&startDate, "start", "s", "", "Start date (YYYY-MM-DD) (required)")
	rootCmd.Flags().StringVarP(&endDate, "end", "E", "", "End date (YYYY-MM-DD) (required)")
	rootCmd.Flags().IntVarP(&duration, "duration", "d", 60, "Meeting duration in minutes")
	rootCmd.Flags().IntVar(&startHour, "start-hour", 9, "Working hours start (24-hour format)")
	rootCmd.Flags().IntVar(&endHour, "end-hour", 17, "Working hours end (24-hour format)")
	rootCmd.Flags().IntVarP(&maxSlots, "max-slots", "m", 10, "Maximum number of slots to display")
	rootCmd.Flags().BoolVarP(&excludeWeekends, "exclude-weekends", "w", true, "Exclude weekends from search")
	rootCmd.Flags().Float64VarP(&maxConflicts, "max-conflicts", "c", 100, "Maximum conflict percentage to display (0-100)")

	rootCmd.MarkFlagRequired("emails")
	rootCmd.MarkFlagRequired("start")
	rootCmd.MarkFlagRequired("end")

	// Bind flags to viper
	viper.BindPFlag("credentials", rootCmd.PersistentFlags().Lookup("credentials"))
	viper.BindPFlag("emails", rootCmd.Flags().Lookup("emails"))
	viper.BindPFlag("start", rootCmd.Flags().Lookup("start"))
	viper.BindPFlag("end", rootCmd.Flags().Lookup("end"))
	viper.BindPFlag("duration", rootCmd.Flags().Lookup("duration"))
	viper.BindPFlag("start_hour", rootCmd.Flags().Lookup("start-hour"))
	viper.BindPFlag("end_hour", rootCmd.Flags().Lookup("end-hour"))
	viper.BindPFlag("max_slots", rootCmd.Flags().Lookup("max-slots"))
	viper.BindPFlag("exclude_weekends", rootCmd.Flags().Lookup("exclude-weekends"))
	viper.BindPFlag("max_conflicts", rootCmd.Flags().Lookup("max-conflicts"))
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
	// Parse inputs
	emailList := strings.Split(viper.GetString("emails"), ",")
	for i, email := range emailList {
		emailList[i] = strings.TrimSpace(email)
	}

	startTime, err := time.Parse("2006-01-02", viper.GetString("start"))
	if err != nil {
		log.Fatalf("Invalid start date: %v", err)
	}

	endTime, err := time.Parse("2006-01-02", viper.GetString("end"))
	if err != nil {
		log.Fatalf("Invalid end date: %v", err)
	}

	meetingDuration := time.Duration(viper.GetInt("duration")) * time.Minute

	fmt.Printf("\nSearching for optimal meeting times...\n")
	fmt.Printf("Attendees: %s\n", strings.Join(emailList, ", "))
	fmt.Printf("Date range: %s to %s\n", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	fmt.Printf("Meeting duration: %d minutes\n", viper.GetInt("duration"))
	fmt.Printf("Working hours: %02d:00 - %02d:00\n", viper.GetInt("start_hour"), viper.GetInt("end_hour"))
	fmt.Printf("Exclude weekends: %v\n\n", viper.GetBool("exclude_weekends"))

	// Initialize Google Calendar service
	service, err := auth.GetCalendarService(viper.GetString("credentials"))
	if err != nil {
		log.Fatalf("Failed to get calendar service: %v", err)
	}

	// Get busy times for all attendees
	availabilities, err := calendar.GetBusyTimes(service, emailList, startTime, endTime.Add(24*time.Hour))
	if err != nil {
		log.Fatalf("Failed to get busy times: %v", err)
	}

	// Get potential meeting slots (working hours)
	potentialSlots := calendar.GetWorkingHours(
		startTime,
		endTime,
		viper.GetInt("start_hour"),
		viper.GetInt("end_hour"),
		viper.GetBool("exclude_weekends"),
	)

	// Find optimal meeting times
	optimalSlots := optimizer.FindOptimalMeetingSlots(
		availabilities,
		potentialSlots,
		meetingDuration,
		viper.GetInt("max_slots")*3, // Get more slots initially for filtering
	)

	// Filter by conflict threshold
	filteredSlots := optimizer.FilterSlotsByThreshold(optimalSlots, viper.GetFloat64("max_conflicts"))

	// Limit to requested number of slots
	if len(filteredSlots) > viper.GetInt("max_slots") {
		filteredSlots = filteredSlots[:viper.GetInt("max_slots")]
	}

	// Display results
	if len(filteredSlots) == 0 {
		fmt.Println("No suitable meeting times found within the specified constraints.")
		return
	}

	fmt.Printf("Top %d meeting times with least conflicts:\n", len(filteredSlots))
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
				len(emailList),
				slot.ConflictPercentage,
			)
			fmt.Printf("   Unavailable: %s\n", strings.Join(slot.UnavailableEmails, ", "))
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 80))

	// Group by day for summary
	grouped := optimizer.GroupSlotsByDay(filteredSlots)
	fmt.Printf("\nSummary by day:\n")
	for day, slots := range grouped {
		dayTime, _ := time.Parse("2006-01-02", day)
		fmt.Printf("  %s: %d potential slots\n", dayTime.Format("Mon, Jan 2"), len(slots))
	}
}
