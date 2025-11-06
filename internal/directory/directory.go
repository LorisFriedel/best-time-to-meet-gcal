package directory

import (
	"fmt"
	"log"
	"strings"

	directory "google.golang.org/api/admin/directory/v1"
)

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
			log.Printf("Could not get members for %s (might be an individual email): %v", email, err)
			memberEmails[email] = true
			continue
		}
		
		// If we successfully got members, add them instead of the group
		if len(members) > 0 {
			log.Printf("Resolved group %s to %d members", email, len(members))
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
					log.Printf("Warning: Could not get members of nested group %s: %v", member.Email, err)
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
