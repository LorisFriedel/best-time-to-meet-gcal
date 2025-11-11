package directory

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	directory "google.golang.org/api/admin/directory/v1"
)

// normalizeEmail standardizes email casing for comparisons
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GroupResolutionError conveys additional context when resolving nested groups
type GroupResolutionError struct {
	Email     string
	Err       error
	ErrorType string
}

func (e *GroupResolutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.ErrorType != "" {
		return fmt.Sprintf("group resolution error (%s): %s", e.ErrorType, e.Err)
	}
	return fmt.Sprintf("group resolution error: %s", e.Err)
}

// Unwrap allows errors.Is / errors.As to inspect underlying error
func (e *GroupResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ResolutionResult represents the result of resolving a single email/group
type ResolutionResult struct {
	OriginalEmail      string
	ResolvedTo         []string
	IsGroup            bool
	Error              error
	ErrorType          string           // "external_domain", "permission_denied", "not_found", etc.
	NestedGroups       []string         // Groups found within this group
	ResolutionDepth    int              // How deep the nesting went
	CircularRef        bool             // True if circular reference was detected
	PartialFailure     bool             // True if some nested groups failed to resolve
	FailedNestedGroups map[string]error // Track which nested groups failed
	CircularGroups     []string         // List of groups involved in circular references
}

// ResolutionSummary contains details about the mailing list resolution process
type ResolutionSummary struct {
	Results           []ResolutionResult
	TotalEmails       int
	ResolvedGroups    int
	UnresolvedGroups  int
	ExternalGroups    int
	IndividualEmails  int
	MaxDepthReached   int // Maximum nesting depth encountered
	CircularRefsFound int // Number of circular references detected
	NestedGroupsTotal int // Total number of nested groups found
}

// groupResolutionContext tracks state during recursive group resolution
type groupResolutionContext struct {
	visitedGroups     map[string]bool   // Track visited groups (normalized) to detect circular references
	memberEmails      map[string]string // normalized -> original email for deduplication
	nestedGroups      map[string]string // normalized -> original nested group email
	maxDepth          int               // Maximum depth reached
	currentPath       []string          // Current path for circular reference detection (original casing)
	failedGroups      map[string]error  // Track groups that failed to resolve
	circularRefs      map[string]string // normalized -> original group involved in circular references
	hasPartialFailure bool              // True if any nested group failed
}

// ResolveMemberEmails takes a list of email addresses (which may include group/mailing list addresses)
// and returns a list of individual member email addresses
func ResolveMemberEmails(service *directory.Service, emails []string) ([]string, error) {
	memberEmails := make(map[string]string) // Use map to avoid duplicates

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
			memberEmails[normalizeEmail(email)] = email
			continue
		}

		// If we successfully got members, add them instead of the group
		if len(members) > 0 {
			log.Info().Str("group", email).Int("member_count", len(members)).Msg("Resolved group")
			for _, member := range members {
				if member == "" {
					continue
				}
				memberEmails[normalizeEmail(member)] = member
			}
		} else {
			// Empty group or individual email
			memberEmails[normalizeEmail(email)] = email
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(memberEmails))
	for _, original := range memberEmails {
		result = append(result, original)
	}

	return result, nil
}

// ResolveMemberEmailsDetailed provides detailed information about the resolution process
func ResolveMemberEmailsDetailed(service *directory.Service, emails []string) ([]string, *ResolutionSummary) {
	memberEmails := make(map[string]string)
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
			memberEmails[normalizeEmail(email)] = email
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
			nestedGroups := make([]string, 0, len(ctx.nestedGroups))
			for _, nested := range ctx.nestedGroups {
				nestedGroups = append(nestedGroups, nested)
			}
			result.NestedGroups = nestedGroups
			result.ResolutionDepth = ctx.maxDepth
			result.PartialFailure = ctx.hasPartialFailure
			result.FailedNestedGroups = ctx.failedGroups
			circularGroups := make([]string, 0, len(ctx.circularRefs))
			for _, circ := range ctx.circularRefs {
				circularGroups = append(circularGroups, circ)
			}
			result.CircularGroups = circularGroups

			// Check for circular references
			for _, circRef := range result.CircularGroups {
				if strings.EqualFold(circRef, email) {
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
				if member == "" {
					continue
				}
				memberEmails[normalizeEmail(member)] = member
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
			memberEmails[normalizeEmail(email)] = email
			summary.IndividualEmails++
		}

		summary.Results = append(summary.Results, result)
		summary.TotalEmails++
	}

	// Convert map to slice
	allEmails := make([]string, 0, len(memberEmails))
	for _, original := range memberEmails {
		allEmails = append(allEmails, original)
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
		memberEmails:      make(map[string]string),
		nestedGroups:      make(map[string]string),
		maxDepth:          0,
		currentPath:       []string{},
		failedGroups:      make(map[string]error),
		circularRefs:      make(map[string]string),
		hasPartialFailure: false,
	}

	// Start recursive resolution
	err := getGroupMembersRecursive(service, groupEmail, ctx, 0)
	if err != nil {
		return nil, err
	}

	// Convert member emails to slice
	members := make([]string, 0, len(ctx.memberEmails))
	for _, original := range ctx.memberEmails {
		members = append(members, original)
	}

	return members, nil
}

// getGroupMembersRecursive recursively retrieves group members with circular reference detection
func getGroupMembersRecursive(service *directory.Service, groupEmail string, ctx *groupResolutionContext, depth int) error {
	normalizedGroup := normalizeEmail(groupEmail)

	// Update max depth
	if depth > ctx.maxDepth {
		ctx.maxDepth = depth
	}

	// Check for circular reference
	for _, pathEmail := range ctx.currentPath {
		if normalizeEmail(pathEmail) == normalizedGroup {
			log.Warn().
				Str("group", groupEmail).
				Strs("path", ctx.currentPath).
				Msg("Circular reference detected in nested groups")
			ctx.circularRefs[normalizedGroup] = groupEmail
			return nil // Don't error out, just skip this group
		}
	}

	// Check if we've already processed this group
	if ctx.visitedGroups[normalizedGroup] {
		log.Debug().
			Str("group", groupEmail).
			Int("depth", depth).
			Msg("Group already processed, skipping")
		return nil
	}

	// Mark as visited and add to current path
	ctx.visitedGroups[normalizedGroup] = true
	ctx.currentPath = append(ctx.currentPath, groupEmail)
	defer func() {
		// Remove from current path when done
		ctx.currentPath = ctx.currentPath[:len(ctx.currentPath)-1]
	}()

	// Fetch group members
	pageToken := ""
	for {
		call := service.Members.
			List(groupEmail).
			MaxResults(200).
			IncludeDerivedMembership(true)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return &GroupResolutionError{
				Email:     groupEmail,
				Err:       err,
				ErrorType: categorizeError(err),
			}
		}

		for _, member := range resp.Members {
			if member == nil {
				continue
			}

			memberEmail := strings.TrimSpace(member.Email)
			if memberEmail == "" {
				continue
			}

			normalizedMember := normalizeEmail(memberEmail)
			memberType := strings.ToUpper(strings.TrimSpace(member.Type))

			// Direct user members are added immediately
			if memberType == "USER" {
				ctx.memberEmails[normalizedMember] = memberEmail
				continue
			}

			// Determine if we should attempt to resolve this member as a nested group
			shouldAttemptGroup := memberType == "GROUP" || memberType == "CUSTOMER" || memberType == ""

			if shouldAttemptGroup {
				ctx.nestedGroups[normalizedMember] = memberEmail

				err := getGroupMembersRecursive(service, memberEmail, ctx, depth+1)
				if err != nil {
					var groupErr *GroupResolutionError
					if errors.As(err, &groupErr) {
						switch groupErr.ErrorType {
						case "not_found", "bad_request":
							// Treat as an individual email (likely not a group)
							delete(ctx.nestedGroups, normalizedMember)
							ctx.memberEmails[normalizedMember] = memberEmail
							log.Debug().
								Str("email", memberEmail).
								Str("parent_group", groupEmail).
								Msg("Nested entry is not a resolvable group; treating as individual")
							continue
						}
					}

					log.Warn().
						Err(err).
						Str("nested_group", memberEmail).
						Str("parent_group", groupEmail).
						Int("depth", depth+1).
						Msg("Could not resolve nested group, continuing with other members")
					ctx.failedGroups[memberEmail] = err
					ctx.hasPartialFailure = true
					continue
				}

				continue
			}

			// Fallback: treat as individual email
			ctx.memberEmails[normalizedMember] = memberEmail
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
		memberEmails:      make(map[string]string),
		nestedGroups:      make(map[string]string),
		maxDepth:          0,
		currentPath:       []string{},
		failedGroups:      make(map[string]error),
		circularRefs:      make(map[string]string),
		hasPartialFailure: false,
	}

	// Start recursive resolution
	err := getGroupMembersRecursive(service, groupEmail, ctx, 0)
	if err != nil {
		return nil, ctx, err
	}

	// Convert member emails to slice
	members := make([]string, 0, len(ctx.memberEmails))
	for _, original := range ctx.memberEmails {
		members = append(members, original)
	}

	return members, ctx, nil
}

// CheckGroupAccess performs a lightweight permission probe for each mailing list domain.
// It returns a warning error if the service account appears to lack the required scope.
func CheckGroupAccess(service *directory.Service, groupEmails []string) error {
	domainSet := make(map[string]struct{})
	for _, email := range groupEmails {
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			continue
		}
		domain := strings.ToLower(strings.TrimSpace(parts[1]))
		if domain != "" {
			domainSet[domain] = struct{}{}
		}
	}

	if len(domainSet) == 0 {
		return nil
	}

	for domain := range domainSet {
		testGroup := fmt.Sprintf("btm-access-check-nonexistent@%s", domain)
		_, err := service.Members.List(testGroup).MaxResults(1).Do()
		if err != nil {
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
				return fmt.Errorf("insufficient permissions to read group members in domain %s. Make sure the service account has 'Groups Reader' role in Google Workspace Admin", domain)
			}
			// 404 is expected for non-existent group within accessible domain
		}
	}

	return nil
}
