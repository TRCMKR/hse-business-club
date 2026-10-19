package seed

import (
	"testing"

	"github.com/google/uuid"

	appdomain "github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	listingsdomain "github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	usersdomain "github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

// TestDemoDataIsValid checks the demo data against the context validation rules,
// so a broken seed fails in unit tests rather than at startup.
func TestDemoDataIsValid(t *testing.T) {
	known := map[string]bool{}
	for _, u := range demoUsers {
		if _, err := usersdomain.NormalizeRegister(users.RegisterParams{Username: u.Username, Password: Password, DisplayName: u.DisplayName}); err != nil {
			t.Errorf("user %s: %v", u.Username, err)
		}
		if _, err := usersdomain.NormalizeProfile(users.UpdateProfileParams{
			DisplayName: u.DisplayName, Headline: u.Headline, About: u.About, Skills: u.Skills, Telegram: u.Telegram,
		}); err != nil {
			t.Errorf("profile %s: %v", u.Username, err)
		}
		known[u.Username] = true
	}

	active := 0
	for _, l := range demoListings {
		if !known[l.Author] {
			t.Errorf("listing %q: unknown author %s", l.Title, l.Author)
		}
		if _, err := listingsdomain.NormalizeCreate(listings.CreateParams{
			AuthorID: uuid.New(), Kind: l.Kind, Title: l.Title, Description: l.Description,
			LookingFor: l.LookingFor, Tags: l.Tags, Stage: l.Stage, Compensation: l.Compensation,
		}); err != nil {
			t.Errorf("listing %q: %v", l.Title, err)
		}
		if !l.Closed {
			active++
		}
	}
	if active != 5 {
		t.Errorf("active listings = %d, want 5 (SPEC §6)", active)
	}

	for _, a := range demoApplications {
		if !known[a.Applicant] {
			t.Errorf("application: unknown applicant %s", a.Applicant)
		}
		if a.Listing < 0 || a.Listing >= len(demoListings) {
			t.Fatalf("application of %s: listing index %d out of range", a.Applicant, a.Listing)
		}
		if demoListings[a.Listing].Author == a.Applicant {
			t.Errorf("application of %s to own listing %q", a.Applicant, demoListings[a.Listing].Title)
		}
		if _, err := appdomain.NormalizeMessage(a.Message); err != nil {
			t.Errorf("application of %s: %v", a.Applicant, err)
		}
		switch a.Outcome {
		case outcomePending, outcomeAccepted, outcomeDeclined, outcomeWithdrawn:
		default:
			t.Errorf("application of %s: unknown outcome %q", a.Applicant, a.Outcome)
		}
	}
}
