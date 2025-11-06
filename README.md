# Best Time to Meet - Google Calendar Tool

A Go command-line tool that analyzes multiple Google Calendars to find optimal meeting times with the least conflicts. Perfect for scheduling large meetings across teams with many attendees.

## Features

- **Google Calendar Integration**: Directly fetches availability from Google Calendar
- **Smart Conflict Analysis**: Identifies time slots with the least number of unavailable attendees
- **Customizable Working Hours**: Set preferred meeting hours and exclude weekends
- **Lunch Time Exclusion**: Automatically avoids scheduling during lunch hours
- **Timezone Support**: Configure timezone for accurate local time handling
- **Flexible Duration**: Support for meetings of any duration
- **Batch Analysis**: Check availability for multiple days at once
- **Conflict Threshold**: Filter results by maximum acceptable conflict percentage

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
   - Add scope: `https://www.googleapis.com/auth/calendar.readonly`
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
```

### All Options

```bash
./best-time-to-meet \
  --emails "email1@company.com,email2@company.com" \  # Required: attendee emails
  --start "2024-01-15" \                              # Required: start date (YYYY-MM-DD)
  --end "2024-01-19" \                                # Required: end date (YYYY-MM-DD)
  --duration 60 \                                     # Meeting duration in minutes (default: 60)
  --start-hour 9 \                                    # Working hours start (default: 9)
  --end-hour 17 \                                     # Working hours end (default: 17)
  --lunch-start-hour 12 \                             # Lunch break start (default: 12)
  --lunch-end-hour 13 \                               # Lunch break end (default: 13)
  --timezone "America/New_York" \                     # IANA timezone (default: local timezone)
  --max-slots 10 \                                    # Max results to show (default: 10)
  --exclude-weekends \                                # Skip weekends (default: true)
  --max-conflicts 30 \                                # Max conflict % to show (default: 100)
  --credentials "credentials.json"                    # Path to Google credentials
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

Top 10 meeting times with least conflicts:
--------------------------------------------------------------------------------

1. Mon, Jan 15, 2024 at 14:00 - 15:00
   ✅ All attendees available!

2. Tue, Jan 16, 2024 at 10:00 - 11:00
   ⚠️  1/3 attendees unavailable (33.3% conflict)
   Unavailable: bob@company.com

3. Wed, Jan 17, 2024 at 15:30 - 16:30
   ⚠️  1/3 attendees unavailable (33.3% conflict)
   Unavailable: charlie@company.com

--------------------------------------------------------------------------------

Summary by day:
  Mon, Jan 15: 3 potential slots
  Tue, Jan 16: 4 potential slots
  Wed, Jan 17: 3 potential slots
```

## Tips for Best Results

1. **Time Zones**: All times are displayed in your local timezone. The tool handles timezone conversions automatically.

2. **Large Groups**: For large groups, consider using `--max-conflicts` to find slots where most (but not all) can attend:
   ```bash
   ./best-time-to-meet --emails "..." --max-conflicts 20  # Show slots with ≤20% conflicts
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
