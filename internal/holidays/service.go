package holidays

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/LorisFriedel/find-best-meeting-time-google/internal/calendar"
	"github.com/rs/zerolog/log"
)

const (
	nagerDateBaseURL     = "https://date.nager.at/api/v3"
	restCountriesBaseURL = "https://restcountries.com/v3.1"
)

// Service enriches user availability with public holiday information.
type Service struct {
	client            *http.Client
	regionOverrides   map[string]string
	tzRegionCache     map[string]string
	holidayCache      map[string]map[int][]publicHoliday
	mutex             sync.Mutex
	requestTimeout    time.Duration
	restCountriesPath string
	nagerDatePath     string
}

type publicHoliday struct {
	Date      string `json:"date"`
	LocalName string `json:"localName"`
	Name      string `json:"name"`
}

type restCountry struct {
	CCA2 string `json:"cca2"`
	// Some timezones are used by multiple territories (e.g., "Asia/Kolkata" -> IN)
	// We only need the ISO code, but Capture the common name for logging if necessary.
	Name struct {
		Common string `json:"common"`
	} `json:"name"`
}

// NewService creates a holiday enrichment service.
// The overrides map should use lowercase email addresses mapped to ISO-3166-1 alpha-2 country codes.
func NewService(client *http.Client, overrides map[string]string) *Service {
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	normalized := make(map[string]string)
	for email, code := range overrides {
		if email == "" || code == "" {
			continue
		}
		normalized[strings.ToLower(email)] = strings.ToUpper(code)
	}

	return &Service{
		client:            client,
		regionOverrides:   normalized,
		tzRegionCache:     make(map[string]string),
		holidayCache:      make(map[string]map[int][]publicHoliday),
		requestTimeout:    10 * time.Second,
		restCountriesPath: restCountriesBaseURL,
		nagerDatePath:     nagerDateBaseURL,
	}
}

// Augment adds public holiday information to each attendee availability.
func (s *Service) Augment(ctx context.Context, availabilities []calendar.UserAvailability, start, end time.Time) error {
	if len(availabilities) == 0 {
		return nil
	}

	var errs []error
	searchStartUTC := start.UTC()
	// Include the whole end date by extending to the next day boundary.
	searchEndUTC := end.Add(24 * time.Hour).UTC()

	for i := range availabilities {
		user := &availabilities[i]
		if user.TimeZone == nil {
			log.Debug().
				Str("email", user.Email).
				Msg("Skipping holiday lookup - timezone unknown")
			continue
		}

		regionCode, err := s.lookupRegion(ctx, user.Email, user.TimeZone)
		if err != nil {
			errs = append(errs, fmt.Errorf("determine holiday region for %s: %w", user.Email, err))
			continue
		}
		if regionCode == "" {
			log.Debug().
				Str("email", user.Email).
				Str("timezone", user.TimeZone.String()).
				Msg("Could not infer region for holiday lookup")
			continue
		}

		startYear := searchStartUTC.In(user.TimeZone).Year()
		endYear := searchEndUTC.In(user.TimeZone).Year()

		added := make(map[string]bool)
		for year := startYear; year <= endYear; year++ {
			holidays, yearErr := s.getHolidaysForYear(ctx, regionCode, year)
			if yearErr != nil {
				errs = append(errs, fmt.Errorf("fetch holidays for %s in %d: %w", regionCode, year, yearErr))
				continue
			}

			for _, h := range holidays {
				holidayStartLocal, parseErr := time.ParseInLocation("2006-01-02", h.Date, user.TimeZone)
				if parseErr != nil {
					errs = append(errs, fmt.Errorf("parse holiday date %s for %s: %w", h.Date, user.Email, parseErr))
					continue
				}

				holidayEndLocal := holidayStartLocal.Add(24 * time.Hour)
				holidayStartUTC := holidayStartLocal.UTC()
				holidayEndUTC := holidayEndLocal.UTC()

				if !holidayEndUTC.After(searchStartUTC) || !holidayStartUTC.Before(searchEndUTC) {
					continue
				}

				dateKey := holidayStartLocal.Format("2006-01-02")
				if added[dateKey] {
					continue
				}

				name := strings.TrimSpace(h.LocalName)
				if name == "" {
					name = h.Name
				}

				user.Holidays = append(user.Holidays, calendar.Holiday{
					Name:   name,
					Region: regionCode,
					TimeSlot: calendar.TimeSlot{
						Start: holidayStartUTC,
						End:   holidayEndUTC,
					},
				})
				added[dateKey] = true
			}
		}

		if len(user.Holidays) > 0 {
			log.Debug().
				Str("email", user.Email).
				Int("holiday_count", len(user.Holidays)).
				Str("region", regionCode).
				Msg("Added bank holidays to attendee availability")
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Service) lookupRegion(ctx context.Context, email string, loc *time.Location) (string, error) {
	emailKey := strings.ToLower(email)
	if code, ok := s.regionOverrides[emailKey]; ok {
		return strings.ToUpper(code), nil
	}

	timezone := loc.String()

	s.mutex.Lock()
	if code, ok := s.tzRegionCache[timezone]; ok {
		s.mutex.Unlock()
		return code, nil
	}
	s.mutex.Unlock()

	if code, ok := mapRegionForTimezone(timezone); ok {
		s.mutex.Lock()
		s.tzRegionCache[timezone] = code
		s.mutex.Unlock()
		return code, nil
	}

	code, err := s.fetchRegionForTimezone(ctx, timezone)
	if err != nil {
		return "", err
	}
	if code == "" {
		return "", nil
	}

	s.mutex.Lock()
	s.tzRegionCache[timezone] = code
	s.mutex.Unlock()

	return code, nil
}

func (s *Service) fetchRegionForTimezone(ctx context.Context, timezone string) (string, error) {
	if timezone == "" {
		return "", nil
	}

	ctx, cancel := s.ensureTimeout(ctx)
	defer cancel()

	urlPath := fmt.Sprintf("%s/timezone/%s", s.restCountriesPath, url.PathEscape(timezone))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "best-time-to-meet-gcal/holidays")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("restcountries response %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var countries []restCountry
	if err := json.NewDecoder(resp.Body).Decode(&countries); err != nil {
		return "", err
	}

	if len(countries) == 0 {
		return "", nil
	}

	for _, country := range countries {
		if country.CCA2 != "" {
			return strings.ToUpper(country.CCA2), nil
		}
	}

	return "", nil
}

func (s *Service) getHolidaysForYear(ctx context.Context, region string, year int) ([]publicHoliday, error) {
	if region == "" || year <= 0 {
		return nil, fmt.Errorf("invalid region/year %s/%d", region, year)
	}

	region = strings.ToUpper(region)

	s.mutex.Lock()
	if cached, ok := s.holidayCache[region]; ok {
		if holidays, okYear := cached[year]; okYear {
			s.mutex.Unlock()
			return holidays, nil
		}
	} else {
		s.holidayCache[region] = make(map[int][]publicHoliday)
	}
	s.mutex.Unlock()

	ctx, cancel := s.ensureTimeout(ctx)
	defer cancel()

	urlPath := fmt.Sprintf("%s/PublicHolidays/%d/%s", s.nagerDatePath, year, url.PathEscape(region))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "best-time-to-meet-gcal/holidays")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Warn().
			Str("region", region).
			Int("year", year).
			Msg("No bank holiday data available for region")
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("nager.date response %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var holidays []publicHoliday
	if err := json.NewDecoder(resp.Body).Decode(&holidays); err != nil {
		return nil, err
	}

	s.mutex.Lock()
	s.holidayCache[region][year] = holidays
	s.mutex.Unlock()

	return holidays, nil
}

func (s *Service) ensureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.requestTimeout)
}

func mapRegionForTimezone(timezone string) (string, bool) {
	if timezone == "" {
		return "", false
	}

	codes, ok := timezoneToRegions[timezone]
	if !ok || len(codes) == 0 {
		return "", false
	}

	return codes[0], true
}
