# Best Time to Meet - Google Calendar Tool

A Go command-line tool that analyzes multiple Google Calendars to find optimal meeting times with the least conflicts. Perfect for scheduling large meetings across teams with many attendees.

## Features

- **Google Calendar Integration**: Directly fetches availability from Google Calendar
- **Smart Conflict Analysis**: Identifies time slots with the least number of unavailable attendees
- **Mailing List Support**: Automatically resolves Google Groups/mailing lists to individual members
- **Customizable Working Hours**: Set preferred meeting hours and exclude weekends
- **Lunch Time Exclusion**: Automatically avoids scheduling during lunch hours
- **Timezone-Aware Scheduling**: Automatically detects each attendee's timezone and counts being outside working hours as a conflict
- **Flexible Duration**: Support for meetings of any duration  
- **Batch Analysis**: Check availability for multiple days at once
- **Conflict Threshold**: Filter results by maximum acceptable conflict percentage
- **Conflict Types**: Distinguishes between calendar conflicts (busy times) and working hours conflicts

## Prerequisites

1. Go 1.21 or higher
2. Google Cloud Console account
3. Google Calendar API enabled
4. OAuth 2.0 credentials

## Installation

### 1. Clone and Build

```bash
git clone https://github.com/LorisFriedel/find-best-meeting-time-google.git
cd find-best-meeting-time-google
go mod download
go build -o best-time-to-meet
```

### 2. Set Up Google Calendar API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Calendar API:
   - Go to "APIs & Services" > "Enable APIs and Services"
   - Search for "Google Calendar API"
   - Click "Enable"

### 3. Create OAuth 2.0 Credentials

1. In Google Cloud Console, go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "OAuth client ID"
3. If prompted, configure the OAuth consent screen:
   - Choose "Internal" for organization use or "External" for general use
   - Fill in the required fields
   - Add scopes:
     - `https://www.googleapis.com/auth/calendar.readonly`
     - `https://www.googleapis.com/auth/admin.directory.group.member.readonly` (for mailing list support)
     - `https://www.googleapis.com/auth/admin.directory.group.readonly` (for mailing list support)
4. For Application type, choose "Desktop app"
5. Download the credentials JSON file
6. Rename it to `credentials.json` and place it in the project root

### 4. First-Time Authentication

On first run, the tool will:
1. Display a URL in the terminal
2. Ask you to visit the URL and authorize the application
3. Provide an authorization code to paste back into the terminal
4. Save the token for future use

## Usage

### Basic Command

```bash
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com,charlie@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60

# Or using mailing lists
./best-time-to-meet \
  --mailing-lists "engineering@company.com,product@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60

# Or combining both individual emails and mailing lists
./best-time-to-meet \
  --emails "alice@company.com" \
  --mailing-lists "engineering@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60
```

### All Options

```bash
./best-time-to-meet \
  --emails "email1@company.com,email2@company.com" \  # Individual attendee emails
  --mailing-lists "team@company.com,dept@company.com" \ # Google Groups/mailing lists
  --start "2024-01-15" \                              # Required: start date (YYYY-MM-DD)
  --end "2024-01-19" \                                # Required: end date (YYYY-MM-DD)
  --duration 60 \                                     # Meeting duration in minutes (default: 60)
  --start-hour 9 \                                    # Working hours start (default: 9)
  --end-hour 17 \                                     # Working hours end (default: 17)
  --lunch-start-hour 12 \                             # Lunch break start (default: 12)
  --lunch-end-hour 13 \                               # Lunch break end (default: 13)
  --timezone "America/New_York" \                     # Reference timezone for search (default: local timezone)
  --max-slots 10 \                                    # Max results to show (default: 10)
  --exclude-weekends \                                # Skip weekends (default: true)
  --max-conflicts 30 \                                # Max conflict % to show (default: 100)
  --credentials "credentials.json" \                  # Path to Google credentials
  --debug                                             # Enable debug logging
```

### Using Configuration File

Create a `config.yaml` file (see `config.yaml.example`):

```yaml
credentials: "credentials.json"
timezone: "America/New_York"  # Optional: IANA timezone
duration: 60
start_hour: 9
end_hour: 17
lunch_start_hour: 12
lunch_end_hour: 13
exclude_weekends: true
max_slots: 10
max_conflicts: 30
```

Then run with fewer command-line arguments:

```bash
./best-time-to-meet \
  --config config.yaml \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19"
```

## Logging and Debugging

The tool uses structured logging with automatic pretty-printing when running in a terminal:
- Normal mode: Shows only important information with colored output
- Debug mode (`--debug`): Shows detailed information including API calls and calendar access

When output is piped or redirected, JSON logging is used automatically for easy parsing.

### Terminal Detection Issues

If the tool doesn't detect your terminal correctly and shows JSON output instead of pretty printing, you can force pretty output by setting the `FORCE_PRETTY` environment variable:

```bash
FORCE_PRETTY=1 ./best-time-to-meet --emails "user@example.com" --start "2024-01-15" --end "2024-01-19"
```

## Output Example

```
Searching for optimal meeting times...
Attendees: alice@company.com, bob@company.com, charlie@company.com
Date range: 2024-01-15 to 2024-01-19
Meeting duration: 60 minutes
Working hours: 09:00 - 17:00
Lunch break: 12:00 - 13:00
Timezone: America/New_York
Exclude weekends: true

================================================================================
🌍 TIMEZONE INFORMATION
================================================================================

Attendees by timezone:
  • America/New_York: 5 attendee(s)
    alice@company.com, bob@company.com, charlie@company.com, ... and 2 more
  • Europe/London: 3 attendee(s)
    david@company.com, emma@company.com, frank@company.com
  • Asia/Tokyo: 2 attendee(s)
    george@company.com, helen@company.com

Working hours: 9:00 - 17:00 (in each attendee's local time)

================================================================================
📅 BEST MEETING TIME OPTIONS
================================================================================

🏆 PERFECT SLOTS (All attendees available):
   Found 3 perfect slot(s)

   ⭐ Mon, Jan 15 at 14:00 - 15:00
   ⭐ Mon, Jan 15 at 15:00 - 16:00
   ⭐ Wed, Jan 17 at 09:30 - 10:30

✅ GOOD OPTIONS (1-25% conflicts):
   Found 5 slot(s) with minimal conflicts
   Best: Tue, Jan 16 at 10:00 - 11:00 (25% conflict)

--------------------------------------------------------------------------------

📊 AVAILABILITY SUMMARY BY DAY:

📆 Mon, Jan 15
   Total slots: 4 | Perfect slots: 2 | Best conflict: 0% | Avg: 25%
   Time range: 09:00 - 16:00

📆 Tue, Jan 16
   Total slots: 3 | Perfect slots: 0 | Best conflict: 25% | Avg: 33%
   Time range: 10:00 - 16:30

📆 Wed, Jan 17
   Total slots: 3 | Perfect slots: 0 | Best conflict: 10% | Avg: 28%
   Time range: 09:30 - 16:00

--------------------------------------------------------------------------------

📋 DETAILED TIME SLOTS (Top 10):
--------------------------------------------------------------------------------

1. Mon, Jan 15, 2024 at 14:00 - 15:00 ✅ Perfect - All attendees available!

2. Mon, Jan 15, 2024 at 15:00 - 16:00 ✅ Perfect - All attendees available!

3. Wed, Jan 17, 2024 at 09:30 - 10:30 🟡 10% conflict
   ⏰ Outside working hours (1): george@company.com

4. Tue, Jan 16, 2024 at 10:00 - 11:00 🟡 25% conflict
   📅 Calendar conflicts (1): bob@company.com

5. Wed, Jan 17, 2024 at 15:30 - 16:30 ⚠️ 40% conflict
   📅 Calendar conflicts (1): charlie@company.com
   ⏰ Outside working hours (1): helen@company.com

================================================================================
💡 RECOMMENDATION:
   Book: Monday, January 15 at 14:00 - 15:00
   This slot has perfect attendance with all attendees available!
   (This matches the best option shown above)
================================================================================
```

## Timezone-Aware Scheduling

The tool automatically detects each attendee's calendar timezone and ensures meetings are scheduled within everyone's working hours:

1. **Automatic Detection**: Fetches timezone from each Google Calendar
2. **Working Hours Enforcement**: Being outside working hours (e.g., 9 AM - 5 PM in their local timezone) is counted as a conflict
3. **Conflict Types**: The tool clearly distinguishes between:
   - 📅 Calendar conflicts: When someone has another meeting
   - ⏰ Working hours conflicts: When the time falls outside someone's working hours
4. **Unified Conflict Percentage**: Both types of conflicts contribute to the overall conflict percentage
5. **Smart Prioritization**: When conflict percentages are equal, slots with fewer working hours violations are preferred

This ensures meetings are scheduled at times that respect everyone's working hours across different time zones, treating timezone incompatibility as seriously as calendar conflicts.

## Mailing List Support

The tool can automatically resolve Google Groups (mailing lists) to individual members. This is especially useful for scheduling team meetings without having to manually list every team member.

### Requirements for Mailing Lists

1. **Google Workspace**: Mailing list resolution requires access to Google Workspace Admin Directory API
2. **Permissions**: Your Google account needs permission to read group members:
   - For Google Workspace domains: Request "Groups Reader" role from your admin
   - OAuth consent screen must include the directory scopes listed above
3. **Token Update**: If you previously authenticated without directory scopes, delete `token.json` and re-authenticate

### Mailing List Examples

```bash
# Schedule a meeting for the entire engineering team
./best-time-to-meet \
  --mailing-lists "engineering@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60

# Mix individual attendees with groups
./best-time-to-meet \
  --emails "ceo@company.com,external@partner.com" \
  --mailing-lists "leadership@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19"
```

### Notes on Mailing Lists

- The tool automatically handles nested groups (groups within groups)
- Duplicate members are automatically removed
- If a mailing list can't be resolved (e.g., external or no permissions), it's treated as an individual email
- Large mailing lists may take a few seconds to resolve

## Tips for Best Results

1. **Time Zones**: The tool automatically detects each attendee's calendar timezone and considers it when finding optimal meeting times. Meeting slots are scored based on how well they fit into everyone's local working hours. Times are displayed in the timezone specified by the `--timezone` parameter (or your local timezone if not specified).

2. **Large Groups**: For large groups, consider using `--max-conflicts` to find slots where most (but not all) can attend:
   ```bash
   ./best-time-to-meet --mailing-lists "all-hands@company.com" --max-conflicts 20  # Show slots with ≤20% conflicts
   ```

3. **Recurring Meetings**: To find recurring meeting times, run the tool for different weeks and look for patterns.

4. **Performance**: The tool fetches calendar data in batches. For many attendees or long date ranges, the initial query may take a few seconds.

## Troubleshooting

### "Unable to read client secret file"
- Ensure `credentials.json` exists in the project root
- Check the file has proper read permissions

### "Unable to retrieve token from web"
- Make sure you're copying the entire authorization code
- Check that the OAuth consent screen is properly configured
- Verify the Calendar API is enabled in your Google Cloud project

### No available slots found
- Try expanding the date range
- Reduce the meeting duration
- Increase `--max-conflicts` to allow some conflicts
- Check that working hours overlap with actual availability

### Token expired
- Delete `token.json` and re-authenticate
- The tool will automatically prompt for re-authentication

### Mailing list issues
- **"insufficient permissions to read group members"**: Your account needs "Groups Reader" role in Google Workspace Admin
- **"Could not get members for [group]"**: The group might be external or you lack permissions
- **Groups not resolving**: Delete `token.json` and re-authenticate to get the new directory scopes

## Privacy & Security

- **Read-Only Access**: The tool only reads calendar free/busy information
- **Local Token Storage**: Authentication tokens are stored locally in `token.json`
- **No Calendar Details**: The tool cannot see event titles, descriptions, or attendee lists
- **Minimal Permissions**: Only requests `calendar.readonly` scope

## Development

### Project Structure

```
.
├── cmd/                    # CLI command definitions
│   └── root.go            # Main command and flags
├── internal/              # Internal packages
│   ├── auth/             # Google OAuth authentication
│   ├── calendar/         # Calendar API interactions
│   └── optimizer/        # Meeting time optimization logic
├── config.yaml.example    # Sample configuration
├── main.go               # Entry point
└── README.md             # This file
```

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# macOS
GOOS=darwin GOARCH=amd64 go build -o best-time-to-meet-mac

# Linux
GOOS=linux GOARCH=amd64 go build -o best-time-to-meet-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o best-time-to-meet.exe
```

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

## License

This project is licensed under the MIT License.
