package directory

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	directory "google.golang.org/api/admin/directory/v1"
)

// ResolutionResult represents the result of resolving a single email/group
type ResolutionResult struct {
	OriginalEmail string
	ResolvedTo    []string
	IsGroup       bool
	Error         error
	ErrorType     string // "external_domain", "permission_denied", "not_found", etc.
}

// ResolutionSummary contains details about the mailing list resolution process
type ResolutionSummary struct {
	Results           []ResolutionResult
	TotalEmails       int
	ResolvedGroups    int
	UnresolvedGroups  int
	ExternalGroups    int
	IndividualEmails  int
}

// ResolveMemberEmails takes a list of email addresses (which may include group/mailing list addresses)
// and returns a list of individual member email addresses
func ResolveMemberEmails(service *directory.Service, emails []string) ([]string, error) {
	memberEmails := make(map[string]bool) // Use map to avoid duplicates

	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}

		// Check if this is a group email by trying to get its members
		members, err := getGroupMembers(service, email)
		if err != nil {
			// If we can't get members, assume it's an individual email
			log.Debug().Err(err).Str("email", email).Msg("Could not get members (might be an individual email)")
			memberEmails[email] = true
			continue
		}

		// If we successfully got members, add them instead of the group
		if len(members) > 0 {
			log.Info().Str("group", email).Int("member_count", len(members)).Msg("Resolved group")
			for _, member := range members {
				memberEmails[member] = true
			}
		} else {
			// Empty group or individual email
			memberEmails[email] = true
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(memberEmails))
	for email := range memberEmails {
		result = append(result, email)
	}

	return result, nil
}

// ResolveMemberEmailsDetailed provides detailed information about the resolution process
func ResolveMemberEmailsDetailed(service *directory.Service, emails []string) ([]string, *ResolutionSummary) {
	memberEmails := make(map[string]bool)
	summary := &ResolutionSummary{
		Results: make([]ResolutionResult, 0),
	}

	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}

		result := ResolutionResult{
			OriginalEmail: email,
			ResolvedTo:    []string{},
		}

		// Try to get group members
		members, err := getGroupMembers(service, email)
		if err != nil {
			// Analyze the error to determine the type
			errorType := categorizeError(err)
			result.Error = err
			result.ErrorType = errorType
			result.IsGroup = false
			
			// Treat as individual email
			memberEmails[email] = true
			result.ResolvedTo = []string{email}
			summary.IndividualEmails++
			
			if errorType == "external_domain" || errorType == "not_found" {
				summary.ExternalGroups++
				summary.UnresolvedGroups++
			}
		} else if len(members) > 0 {
			// Successfully resolved group
			result.IsGroup = true
			result.ResolvedTo = members
			for _, member := range members {
				memberEmails[member] = true
			}
			summary.ResolvedGroups++
		} else {
			// Empty result - treat as individual email
			result.IsGroup = false
			result.ResolvedTo = []string{email}
			memberEmails[email] = true
			summary.IndividualEmails++
		}

		summary.Results = append(summary.Results, result)
		summary.TotalEmails++
	}

	// Convert map to slice
	allEmails := make([]string, 0, len(memberEmails))
	for email := range memberEmails {
		allEmails = append(allEmails, email)
	}

	return allEmails, summary
}

// categorizeError determines the type of error from the API response
func categorizeError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	
	// Check for common error patterns
	if strings.Contains(errStr, "404") || strings.Contains(errStr, "notFound") || strings.Contains(errStr, "Resource Not Found") {
		return "not_found"
	}
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") || strings.Contains(errStr, "Permission denied") {
		return "permission_denied"
	}
	if strings.Contains(errStr, "400") || strings.Contains(errStr, "Bad Request") {
		return "bad_request"
	}
	if strings.Contains(errStr, "Domain not found") || strings.Contains(errStr, "domain") {
		return "external_domain"
	}
	
	// If it's a 404, it could be external domain or non-existent group
	// Groups from external domains typically return 404
	if strings.Contains(errStr, "404") {
		return "external_domain"
	}

	return "unknown"
}

// getGroupMembers retrieves all member email addresses for a given group
func getGroupMembers(service *directory.Service, groupEmail string) ([]string, error) {
	var members []string
	pageToken := ""

	for {
		call := service.Members.List(groupEmail).MaxResults(200)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, err
		}

		for _, member := range resp.Members {
			// Only add USER type members (not groups within groups)
			if member.Type == "USER" {
				members = append(members, member.Email)
			} else if member.Type == "GROUP" {
				// Recursively get members of nested groups
				nestedMembers, err := getGroupMembers(service, member.Email)
				if err != nil {
					log.Warn().Err(err).Str("nested_group", member.Email).Msg("Could not get members of nested group")
					continue
				}
				members = append(members, nestedMembers...)
			}
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return members, nil
}

// CheckGroupAccess verifies if the service account has access to read group members
func CheckGroupAccess(service *directory.Service) error {
	// Try to list members of a non-existent group to check if we have the right permissions
	// This will return a different error for permission issues vs not found
	_, err := service.Members.List("test-non-existent-group@example.com").MaxResults(1).Do()
	if err != nil {
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			return fmt.Errorf("insufficient permissions to read group members. Make sure the service account has 'Groups Reader' role in Google Workspace Admin")
		}
		// Other errors (like 404) are fine - it means we have permission but the group doesn't exist
	}
	return nil
}
