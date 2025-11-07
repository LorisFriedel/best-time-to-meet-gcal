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
- **JSON Output**: Export results in JSON format for easy integration with other tools and automation

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

### 2. Set Up Google APIs

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable required APIs:
   - Go to "APIs & Services" > "Enable APIs and Services"
   - Search and enable these APIs:
     - **Google Calendar API** (for calendar access)
     - **Admin SDK API** (for mailing list/group support)

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

### 5. Additional Setup for Mailing List Support

**⚠️ IMPORTANT: Mailing list support requires ALL of the following to be configured correctly. Missing any one will cause it to fail.**

#### Required Setup Checklist

You **MUST** complete all of these steps for mailing lists to work:

1. ✅ **Enable Admin SDK API** (in addition to Calendar API)
   - Go to Google Cloud Console → APIs & Services → Library
   - Search for "Admin SDK API" and click "Enable"
   - This is different from just enabling Calendar API - both must be enabled

2. ✅ **Add Directory Scopes to OAuth Consent Screen**
   - Go to Google Cloud Console → APIs & Services → OAuth consent screen → Edit
   - In the Scopes section, add these scopes:
     - `https://www.googleapis.com/auth/admin.directory.group.member.readonly`
     - `https://www.googleapis.com/auth/admin.directory.group.readonly`
   - Save the changes

3. ✅ **Get Google Workspace Admin Permissions** (REQUIRED - OAuth scopes alone are not enough)
   - You need the "Groups Reader" role in Google Workspace Admin
   - Ask your Google Workspace administrator to grant you this role
   - Or have them create a custom role with "Groups → Read" permission
   - **Personal Gmail accounts cannot read Google Groups membership**

4. ✅ **Re-authenticate After Changes**
   - Delete your existing token: `rm token.json`
   - Run the tool again - it will prompt for re-authentication with new scopes

#### Important Limitations

- **200+ Member Groups**: Google Calendar API cannot process groups with more than 200 members in a single request. The tool automatically handles this by batching requests, but you still need proper permissions to read the group members first.
- **External Groups**: Groups from external domains (e.g., `group@external-company.com`) cannot be resolved - they must be from your Google Workspace domain.
- **Workspace Required**: Personal Gmail accounts cannot access Google Groups membership - you need a Google Workspace account with proper admin permissions.

#### Common Setup Issues

If you get "insufficient permissions to read group members", check:

- ❌ **Admin SDK API not enabled** - Most common mistake: only Calendar API is enabled
- ❌ **OAuth scopes missing** - Directory scopes not added to OAuth consent screen
- ❌ **No Workspace permissions** - Account doesn't have Groups Reader role
- ❌ **Token not refreshed** - Need to delete `token.json` and re-authenticate after changes
- ❌ **External group** - Trying to access a group from another organization

#### Verify Your Setup

After completing all steps above, verify:

1. **OAuth Scopes**: Google Cloud Console → APIs & Services → OAuth consent screen → Scopes
   - Must include: `admin.directory.group.member.readonly` and `admin.directory.group.readonly`

2. **Enabled APIs**: Google Cloud Console → APIs & Services → Library
   - Both "Google Calendar API" and "Admin SDK API" must show as "Enabled"

3. **Workspace Permissions**: Google Admin Console → Account → Admin roles
   - Your account should have "Groups Reader" role or equivalent

4. **Re-authenticate**: After any changes
   ```bash
   rm token.json
   ./best-time-to-meet --mailing-lists "your-group@company.com" --start "2024-01-15" --end "2024-01-19"
   ```

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
  --debug \                                           # Enable debug logging
  --json                                              # Output results in JSON format
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
batch_size: 50  # Number of calendars per API request (for large groups)
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

## JSON Output

The tool supports JSON output for easy integration with other software, automated workflows, or custom visualization tools. Simply add the `--json` flag to get structured data instead of the human-readable format.

### Basic Usage

```bash
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60 \
  --json
```

### JSON Output Structure

The JSON output includes all the information from the human-readable format in a structured format:

```json
{
  "metadata": {
    "search_start_date": "2024-01-15",
    "search_end_date": "2024-01-19",
    "meeting_duration_minutes": 60,
    "total_attendees": 3,
    "accessible_calendars": 3,
    "working_hours": "9:00 - 17:00",
    "lunch_hours": "12:00 - 13:00",
    "exclude_weekends": true,
    "max_conflicts_percentage": 100,
    "timezone": "America/New_York"
  },
  "summary": {
    "total_slots_found": 10,
    "perfect_slots": 3,
    "low_conflict_slots": 5,
    "medium_conflict_slots": 2
  },
  "timezone_info": {
    "attendees_by_timezone": {
      "America/New_York": ["alice@company.com", "bob@company.com"],
      "Europe/London": ["charlie@company.com"]
    },
    "working_hours_note": "9:00 - 17:00 (in each attendee's local time)"
  },
  "best_options": {
    "perfect_slots": [
      {
        "start_time": "2024-01-15T14:00:00-05:00",
        "end_time": "2024-01-15T15:00:00-05:00",
        "conflict_percentage": 0,
        "conflict_count": 0
      }
    ],
    "good_options": [
      {
        "start_time": "2024-01-16T10:00:00-05:00",
        "end_time": "2024-01-16T11:00:00-05:00",
        "conflict_percentage": 25,
        "conflict_count": 1
      }
    ]
  },
  "daily_summary": [
    {
      "date": "2024-01-15",
      "total_slots": 4,
      "perfect_slots": 2,
      "best_conflict_percentage": 0,
      "average_conflict_percentage": 25,
      "earliest_slot_time": "09:00",
      "latest_slot_time": "16:00"
    }
  ],
  "detailed_slots": [
    {
      "start_time": "2024-01-15T14:00:00-05:00",
      "end_time": "2024-01-15T15:00:00-05:00",
      "conflict_percentage": 0,
      "unavailable_count": 0,
      "unavailable_emails": [],
      "available_emails": ["alice@company.com", "bob@company.com", "charlie@company.com"],
      "timezone_score": 100,
      "conflicts_by_type": {
        "calendar": [],
        "working_hours": []
      }
    }
  ],
  "recommendation": {
    "start_time": "2024-01-15T14:00:00-05:00",
    "end_time": "2024-01-15T15:00:00-05:00",
    "conflict_percentage": 0,
    "unavailable_count": 0,
    "calendar_conflicts": 0,
    "working_hours_conflicts": 0,
    "reason": "Perfect slot with all attendees available"
  }
}
```

### JSON Fields Description

- **metadata**: Search parameters and configuration used
- **summary**: High-level statistics about available slots
- **timezone_info**: Breakdown of attendees by timezone
- **best_options**: Top meeting slots categorized by quality
  - `perfect_slots`: No conflicts (up to 5 slots)
  - `good_options`: Low conflicts, 1-25% (up to 5 slots)
- **daily_summary**: Statistics grouped by day
- **detailed_slots**: Complete list of all found slots with full details
- **recommendation**: The single best recommended slot with reasoning

### Integration Examples

#### Process with jq
```bash
# Get just the recommended meeting time
./best-time-to-meet --emails "..." --start "..." --end "..." --json | jq -r '.recommendation.start_time'

# List all perfect slots
./best-time-to-meet --emails "..." --start "..." --end "..." --json | jq -r '.best_options.perfect_slots[] | "\(.start_time) - \(.end_time)"'

# Count slots by day
./best-time-to-meet --emails "..." --start "..." --end "..." --json | jq '.daily_summary[] | "\(.date): \(.total_slots) slots"'
```

#### Python Integration
```python
import subprocess
import json
import datetime

# Run the tool and parse JSON output
result = subprocess.run([
    './best-time-to-meet',
    '--emails', 'alice@company.com,bob@company.com',
    '--start', '2024-01-15',
    '--end', '2024-01-19',
    '--json'
], capture_output=True, text=True)

data = json.loads(result.stdout)

# Process the recommendation
if data['recommendation']:
    start_time = datetime.datetime.fromisoformat(data['recommendation']['start_time'])
    print(f"Best meeting time: {start_time}")
    print(f"Conflicts: {data['recommendation']['conflict_percentage']}%")
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

# Handle very large groups (e.g., 300+ members) with custom batch size
./best-time-to-meet \
  --mailing-lists "all-staff@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 30 \
  --batch-size 100  # Process 100 calendars per API request
```

### Notes on Mailing Lists

- The tool automatically handles nested groups (groups within groups)
- Duplicate members are automatically removed
- **Large groups (200+ members)**: Automatically processed in batches to work around Google Calendar API limitations
- **External mailing lists**: Groups from external domains (not your Google Workspace) cannot be resolved and have no calendar data
- When a mailing list can't be resolved, you'll receive a detailed error message with suggestions
- Use `--batch-size` to adjust the number of calendars processed per API request (default: 50)

### Requirements for Mailing Lists

**⚠️ ALL of the following are REQUIRED - missing any one will cause mailing lists to fail:**

1. **Google Workspace Account**: Personal Gmail accounts cannot read Google Groups membership
2. **Admin SDK API Enabled**: Must be enabled in Google Cloud Console (in addition to Calendar API)
3. **OAuth Directory Scopes**: OAuth consent screen must include:
   - `admin.directory.group.member.readonly`
   - `admin.directory.group.readonly`
4. **Google Workspace Admin Permissions**: Your account must have "Groups Reader" role (OAuth scopes alone are NOT sufficient)
5. **Fresh Authentication**: Delete `token.json` and re-authenticate after adding scopes/permissions

**Note on Large Groups**: Groups with 200+ members are automatically handled via batching, but you still need all the above permissions to read the group members first.

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

#### "insufficient permissions to read group members"

**This error means ONE or MORE of the required setup steps are missing. ALL must be completed:**

1. ✅ **Admin SDK API Enabled** (in Google Cloud Console):
   - Go to APIs & Services → Library
   - Search "Admin SDK API" and verify it shows "Enabled"
   - **Most common mistake**: Only Calendar API is enabled, Admin SDK API is missing

2. ✅ **OAuth Directory Scopes Added** (in Google Cloud Console):
   - Go to APIs & Services → OAuth consent screen → Edit → Scopes
   - Must include BOTH:
     - `admin.directory.group.member.readonly`
     - `admin.directory.group.readonly`
   - Save changes

3. ✅ **Google Workspace Admin Permissions** (REQUIRED - OAuth scopes alone are NOT enough):
   - Go to Google Admin Console → Account → Admin roles
   - Your account MUST have "Groups Reader" role
   - **This is the most commonly missed requirement** - OAuth scopes grant app permissions, but you still need Workspace admin role to actually read groups
   - If you don't have admin access, ask your IT administrator to grant you this role

4. ✅ **Re-authenticate After Changes**:
   ```bash
   rm token.json
   ./best-time-to-meet --mailing-lists "your-group@company.com" --start "2024-01-15" --end "2024-01-19"
   ```

5. ✅ **Check API Quotas**:
   - In Google Cloud Console → APIs & Services → Quotas
   - Look for Admin SDK API quotas
   - Ensure you haven't exceeded any limits

**If you cannot get Workspace admin permissions:**
- Export group members manually from Google Groups UI
- Use `--emails` flag with the exported email addresses instead

#### "Could not resolve mailing list - may be external domain"
This means the mailing list is from an external organization (e.g., a partner company) or doesn't exist in your Google Workspace domain. 

**Solutions:**
- Request individual member email addresses from the group owner
- Use the `--emails` flag instead with comma-separated individual addresses
- For mixed internal/external: Use both `--mailing-lists` for internal groups and `--emails` for external individuals

#### No calendar data available
If you get "No calendar data could be retrieved for any attendees":

1. **Missing permissions**: You may not have completed all required setup steps (see "insufficient permissions" section above)
2. **External groups**: Group email addresses don't have calendars. You need individual member emails.
3. **Large groups (200+ members)**: Google Calendar API has a hard limit - it cannot process groups with more than 200 members in a single request. The tool automatically handles this by batching requests, BUT you must first have proper permissions to read the group members via Directory API.
4. **Calendar sharing**: Calendars must be shared with you (at minimum "free/busy" visibility).
5. **Wrong domain**: Verify the email addresses are from domains you have access to.

#### Groups not resolving
Delete `token.json` and re-authenticate to get the new directory scopes.

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
