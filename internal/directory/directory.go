package directory

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	directory "google.golang.org/api/admin/directory/v1"
)

// ResolutionResult represents the result of resolving a single email/group
type ResolutionResult struct {
	OriginalEmail    string
	ResolvedTo       []string
	IsGroup          bool
	Error            error
	ErrorType        string            // "external_domain", "permission_denied", "not_found", etc.
	NestedGroups     []string          // Groups found within this group
	ResolutionDepth  int               // How deep the nesting went
	CircularRef      bool              // True if circular reference was detected
	PartialFailure   bool              // True if some nested groups failed to resolve
	FailedNestedGroups map[string]error // Track which nested groups failed
}

// ResolutionSummary contains details about the mailing list resolution process
type ResolutionSummary struct {
	Results           []ResolutionResult
	TotalEmails       int
	ResolvedGroups    int
	UnresolvedGroups  int
	ExternalGroups    int
	IndividualEmails  int
	MaxDepthReached   int    // Maximum nesting depth encountered
	CircularRefsFound int    // Number of circular references detected
	NestedGroupsTotal int    // Total number of nested groups found
}

// groupResolutionContext tracks state during recursive group resolution
type groupResolutionContext struct {
	visitedGroups     map[string]bool   // Track visited groups to detect circular references
	memberEmails      map[string]bool   // Deduplicated member emails
	nestedGroups      []string          // All nested groups encountered
	maxDepth          int               // Maximum depth reached
	currentPath       []string          // Current path for circular reference detection
	failedGroups      map[string]error  // Track groups that failed to resolve
	circularRefs      []string          // Groups involved in circular references
	hasPartialFailure bool              // True if any nested group failed
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
		Results:           make([]ResolutionResult, 0),
		MaxDepthReached:   0,
		CircularRefsFound: 0,
		NestedGroupsTotal: 0,
	}

	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}

		result := ResolutionResult{
			OriginalEmail:      email,
			ResolvedTo:         []string{},
			FailedNestedGroups: make(map[string]error),
		}

		// Try to get group members with full details
		members, ctx, err := getGroupMembersWithDetails(service, email)
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
			// Successfully resolved group (possibly with partial failures)
			result.IsGroup = true
			result.ResolvedTo = members
			result.NestedGroups = ctx.nestedGroups
			result.ResolutionDepth = ctx.maxDepth
			result.PartialFailure = ctx.hasPartialFailure
			result.FailedNestedGroups = ctx.failedGroups
			
			// Check for circular references
			for _, circRef := range ctx.circularRefs {
				if circRef == email {
					result.CircularRef = true
					break
				}
			}
			
			// Update summary stats
			summary.ResolvedGroups++
			summary.NestedGroupsTotal += len(ctx.nestedGroups)
			if ctx.maxDepth > summary.MaxDepthReached {
				summary.MaxDepthReached = ctx.maxDepth
			}
			summary.CircularRefsFound += len(ctx.circularRefs)
			
			// Add members to overall set
			for _, member := range members {
				memberEmails[member] = true
			}
			
			// Log detailed information if there were issues
			if result.PartialFailure || result.CircularRef {
				log.Info().
					Str("group", email).
					Int("members_found", len(members)).
					Int("nested_groups", len(ctx.nestedGroups)).
					Int("failed_groups", len(ctx.failedGroups)).
					Int("max_depth", ctx.maxDepth).
					Bool("circular_ref", result.CircularRef).
					Msg("Group resolved with issues")
			}
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
	// Create a new context for this resolution
	ctx := &groupResolutionContext{
		visitedGroups:     make(map[string]bool),
		memberEmails:      make(map[string]bool),
		nestedGroups:      []string{},
		maxDepth:          0,
		currentPath:       []string{},
		failedGroups:      make(map[string]error),
		circularRefs:      []string{},
		hasPartialFailure: false,
	}

	// Start recursive resolution
	err := getGroupMembersRecursive(service, groupEmail, ctx, 0)
	if err != nil {
		return nil, err
	}

	// Convert member emails to slice
	members := make([]string, 0, len(ctx.memberEmails))
	for email := range ctx.memberEmails {
		members = append(members, email)
	}

	return members, nil
}

// getGroupMembersRecursive recursively retrieves group members with circular reference detection
func getGroupMembersRecursive(service *directory.Service, groupEmail string, ctx *groupResolutionContext, depth int) error {
	// Update max depth
	if depth > ctx.maxDepth {
		ctx.maxDepth = depth
	}

	// Check for circular reference
	for _, pathEmail := range ctx.currentPath {
		if pathEmail == groupEmail {
				log.Warn().
					Str("group", groupEmail).
					Strs("path", ctx.currentPath).
					Msg("Circular reference detected in nested groups")
			ctx.circularRefs = append(ctx.circularRefs, groupEmail)
			return nil // Don't error out, just skip this group
		}
	}

	// Check if we've already processed this group
	if ctx.visitedGroups[groupEmail] {
		log.Debug().
			Str("group", groupEmail).
			Int("depth", depth).
			Msg("Group already processed, skipping")
		return nil
	}

	// Mark as visited and add to current path
	ctx.visitedGroups[groupEmail] = true
	ctx.currentPath = append(ctx.currentPath, groupEmail)
	defer func() {
		// Remove from current path when done
		ctx.currentPath = ctx.currentPath[:len(ctx.currentPath)-1]
	}()

	// Fetch group members
	pageToken := ""
	for {
		call := service.Members.List(groupEmail).MaxResults(200)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return fmt.Errorf("failed to list members for group %s: %w", groupEmail, err)
		}

		for _, member := range resp.Members {
			if member.Type == "USER" {
				// Add user email to the set
				ctx.memberEmails[member.Email] = true
			} else if member.Type == "GROUP" {
				// Track nested group
				ctx.nestedGroups = append(ctx.nestedGroups, member.Email)

				// Recursively process nested group
				err := getGroupMembersRecursive(service, member.Email, ctx, depth+1)
				if err != nil {
					log.Warn().
						Err(err).
						Str("nested_group", member.Email).
						Str("parent_group", groupEmail).
						Int("depth", depth+1).
						Msg("Could not resolve nested group, continuing with other members")
					ctx.failedGroups[member.Email] = err
					ctx.hasPartialFailure = true
					// Continue processing other members even if one nested group fails
				}
			}
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return nil
}

// getGroupMembersWithDetails retrieves group members with full resolution details
func getGroupMembersWithDetails(service *directory.Service, groupEmail string) ([]string, *groupResolutionContext, error) {
	// Create a new context for this resolution
	ctx := &groupResolutionContext{
		visitedGroups:     make(map[string]bool),
		memberEmails:      make(map[string]bool),
		nestedGroups:      []string{},
		maxDepth:          0,
		currentPath:       []string{},
		failedGroups:      make(map[string]error),
		circularRefs:      []string{},
		hasPartialFailure: false,
	}

	// Start recursive resolution
	err := getGroupMembersRecursive(service, groupEmail, ctx, 0)
	if err != nil {
		return nil, ctx, err
	}

	// Convert member emails to slice
	members := make([]string, 0, len(ctx.memberEmails))
	for email := range ctx.memberEmails {
		members = append(members, email)
	}

	return members, ctx, nil
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
