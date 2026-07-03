package db

import (
	"database/sql"
	"testing"
	"time"
)

// --- birthdayUrgency (pure, deep module) ---

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestBirthdayUrgency_7Days(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-06-08", date(2025, time.June, 1))
	if !ok || d != 7 || urg != "upcoming" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 7/upcoming/true", d, urg, ok)
	}
}

func TestBirthdayUrgency_3Days(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-06-04", date(2025, time.June, 1))
	if !ok || d != 3 || urg != "soon" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 3/soon/true", d, urg, ok)
	}
}

func TestBirthdayUrgency_Today(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-06-01", date(2025, time.June, 1))
	if !ok || d != 0 || urg != "today" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 0/today/true", d, urg, ok)
	}
}

func TestBirthdayUrgency_Tomorrow(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-06-02", date(2025, time.June, 1))
	if !ok || d != 1 || urg != "soon" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 1/soon/true", d, urg, ok)
	}
}

// A birthday that just passed folds to next year and drops out of the near window.
func TestBirthdayUrgency_Yesterday(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-05-31", date(2025, time.June, 1))
	if !ok {
		t.Fatalf("ok=false, want true")
	}
	if urg != "" {
		t.Fatalf("urgency=%q, want empty (expired/out of window)", urg)
	}
	if d <= 7 {
		t.Fatalf("daysUntil=%d, want a far-future wrap (>7)", d)
	}
}

// Feb 29 birthday in a non-leap year folds to Feb 28.
func TestBirthdayUrgency_LeapYear(t *testing.T) {
	d, urg, ok := birthdayUrgency("2000-02-29", date(2025, time.February, 27))
	if !ok || d != 1 || urg != "soon" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 1/soon/true (Feb29->Feb28 in 2025)", d, urg, ok)
	}
}

// January birthday checked in late December wraps into next year.
func TestBirthdayUrgency_YearWrap(t *testing.T) {
	d, urg, ok := birthdayUrgency("1990-01-02", date(2025, time.December, 28))
	if !ok || d != 5 || urg != "upcoming" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 5/upcoming/true", d, urg, ok)
	}
}

// Across a spring-forward boundary a local day is 23h; civil-day math must still
// count it as one day. (America/Boise springs forward 2025-03-09.)
func TestBirthdayUrgency_DSTBoundary(t *testing.T) {
	loc, err := time.LoadLocation("America/Boise")
	if err != nil {
		t.Skipf("tz data unavailable: %v", err)
	}
	today := time.Date(2025, time.March, 8, 0, 0, 0, 0, loc)
	d, urg, ok := birthdayUrgency("1990-03-09", today)
	if !ok || d != 1 || urg != "soon" {
		t.Fatalf("got days=%d urgency=%q ok=%v; want 1/soon/true across DST", d, urg, ok)
	}
}

// --- CRUD ---

func TestInsertAndGetPerson(t *testing.T) {
	db := openTestDB(t)

	p := Person{
		Name:         "Jason",
		Surface:      "life",
		Relationship: sql.NullString{String: "friend", Valid: true},
		Birthday:     sql.NullString{String: "1988-03-04", Valid: true},
	}
	if err := db.InsertPerson(p); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	people, err := db.ListPeople(nil, nil)
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("got %d people, want 1", len(people))
	}

	got, err := db.GetPerson(people[0].ID)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}
	if got == nil || got.Name != "Jason" || got.Relationship.String != "friend" {
		t.Fatalf("GetPerson returned %+v", got)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("timestamps not set: %+v", got)
	}
}

func TestGetPerson_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetPerson("missing")
	if err != nil {
		t.Fatalf("GetPerson err: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestUpdatePerson(t *testing.T) {
	db := openTestDB(t)
	p := Person{ID: "p1", Name: "Mom", Surface: "life"}
	if err := db.InsertPerson(p); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	if err := db.UpdatePerson("p1", map[string]any{
		"notes":                "allergic to peanuts",
		"contact_cadence_days": 30,
	}); err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}

	got, _ := db.GetPerson("p1")
	if got.Notes.String != "allergic to peanuts" {
		t.Fatalf("notes=%q, want 'allergic to peanuts'", got.Notes.String)
	}
	if !got.ContactCadenceDays.Valid || got.ContactCadenceDays.Int64 != 30 {
		t.Fatalf("cadence=%+v, want 30", got.ContactCadenceDays)
	}

	if err := db.UpdatePerson("p1", map[string]any{"bogus": 1}); err == nil {
		t.Fatalf("expected error updating non-whitelisted column")
	}
	if err := db.UpdatePerson("missing", map[string]any{"notes": "x"}); err != sql.ErrNoRows {
		t.Fatalf("update missing: got %v, want sql.ErrNoRows", err)
	}
}

func TestSuppressPerson(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "p1", Name: "Ghost", Surface: "life"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	if err := db.SuppressPerson("p1"); err != nil {
		t.Fatalf("SuppressPerson: %v", err)
	}

	people, _ := db.ListPeople(nil, nil)
	if len(people) != 0 {
		t.Fatalf("suppressed person still listed: %d", len(people))
	}
	if err := db.SuppressPerson("missing"); err != sql.ErrNoRows {
		t.Fatalf("suppress missing: got %v, want sql.ErrNoRows", err)
	}
}

func TestListPeopleBySurface(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertPerson(Person{Name: "Auggie", Surface: "life"})
	_ = db.InsertPerson(Person{Name: "Client", Surface: "business", Relationship: sql.NullString{String: "client", Valid: true}})

	life := "life"
	got, err := db.ListPeople(&life, nil)
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Auggie" {
		t.Fatalf("surface=life returned %+v", got)
	}

	client := "client"
	got, _ = db.ListPeople(nil, &client)
	if len(got) != 1 || got[0].Name != "Client" {
		t.Fatalf("relationship=client returned %+v", got)
	}
}

func TestInsertInteraction_UpdatesLastInteractionAt(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "p1", Name: "Jason", Surface: "life"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	err := db.InsertInteraction(Interaction{
		PersonID: "p1",
		Date:     "2026-07-01",
		Summary:  "coffee",
		FollowUp: sql.NullString{String: "send book", Valid: true},
	})
	if err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}

	got, _ := db.GetPerson("p1")
	if got.LastInteractionAt.String != "2026-07-01" {
		t.Fatalf("last_interaction_at=%q, want 2026-07-01", got.LastInteractionAt.String)
	}

	list, err := db.ListInteractions("p1")
	if err != nil {
		t.Fatalf("ListInteractions: %v", err)
	}
	if len(list) != 1 || list[0].Summary != "coffee" {
		t.Fatalf("ListInteractions returned %+v", list)
	}

	// Bad person_id must roll back (FK enforced).
	if err := db.InsertInteraction(Interaction{PersonID: "nope", Date: "2026-07-01", Summary: "x"}); err == nil {
		t.Fatalf("expected FK error for unknown person_id")
	}
}

// A backdated interaction must not pull last_interaction_at backward.
func TestInsertInteraction_DoesNotRegressLastInteraction(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertPerson(Person{ID: "p1", Name: "Jason", Surface: "life"})

	if err := db.InsertInteraction(Interaction{PersonID: "p1", Date: "2026-07-01", Summary: "recent"}); err != nil {
		t.Fatalf("InsertInteraction recent: %v", err)
	}
	if err := db.InsertInteraction(Interaction{PersonID: "p1", Date: "2020-01-01", Summary: "backdated"}); err != nil {
		t.Fatalf("InsertInteraction backdated: %v", err)
	}

	got, _ := db.GetPerson("p1")
	if got.LastInteractionAt.String != "2026-07-01" {
		t.Fatalf("last_interaction_at=%q, want it to stay at 2026-07-01", got.LastInteractionAt.String)
	}
}

func TestFollowUpsDue(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertPerson(Person{ID: "p1", Name: "Jason", Surface: "life"})

	// due yesterday, and far future
	_ = db.InsertInteraction(Interaction{PersonID: "p1", Date: "2020-01-01", Summary: "old",
		FollowUpDate: sql.NullString{String: "2000-01-01", Valid: true}})
	_ = db.InsertInteraction(Interaction{PersonID: "p1", Date: "2020-01-02", Summary: "future",
		FollowUpDate: sql.NullString{String: "2999-01-01", Valid: true}})

	due, err := db.FollowUpsDue()
	if err != nil {
		t.Fatalf("FollowUpsDue: %v", err)
	}
	if len(due) != 1 || due[0].Summary != "old" {
		t.Fatalf("FollowUpsDue returned %+v", due)
	}
}

func TestUpcomingBirthdays_IncludesKids(t *testing.T) {
	db := openTestDB(t)

	now := time.Now().In(localLocation)
	// Person birthday 2 days out; kid birthday 5 days out. Use MM-DD near today.
	self := now.AddDate(0, 0, 2)
	kid := now.AddDate(0, 0, 5)

	kids, _ := MarshalKids([]Kid{{Name: "Marcus", Birthday: "2014-" + kid.Format("01-02")}})
	err := db.InsertPerson(Person{
		ID:       "p1",
		Name:     "Dad",
		Surface:  "life",
		Birthday: sql.NullString{String: "1980-" + self.Format("01-02"), Valid: true},
		Kids:     kids,
	})
	if err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	hits, err := db.UpcomingBirthdays(7)
	if err != nil {
		t.Fatalf("UpcomingBirthdays: %v", err)
	}

	var sawSelf, sawKid bool
	for _, h := range hits {
		if h.Kind == "self" && h.Name == "Dad" {
			sawSelf = true
		}
		if h.Kind == "kid" && h.Name == "Marcus" {
			sawKid = true
		}
		if h.Urgency == "" {
			t.Fatalf("hit within window has empty urgency: %+v", h)
		}
	}
	if !sawSelf || !sawKid {
		t.Fatalf("expected self+kid birthdays; got %+v", hits)
	}
	// Sorted ascending by days.
	for i := 1; i < len(hits); i++ {
		if hits[i-1].DaysUntil > hits[i].DaysUntil {
			t.Fatalf("hits not sorted by DaysUntil: %+v", hits)
		}
	}
}

func TestPeopleSlipping(t *testing.T) {
	db := openTestDB(t)

	// Slipping: last interaction long ago, default cadence 90.
	_ = db.InsertPerson(Person{ID: "slip", Name: "Neglected", Surface: "life",
		LastInteractionAt: sql.NullString{String: "2000-01-01", Valid: true}})
	// Fresh: interacted today.
	_ = db.InsertPerson(Person{ID: "fresh", Name: "Recent", Surface: "life",
		LastInteractionAt: sql.NullString{String: today(), Valid: true}})
	// Never interacted: excluded (NULL last_interaction_at).
	_ = db.InsertPerson(Person{ID: "never", Name: "New", Surface: "life"})

	slipping, err := db.PeopleSlipping()
	if err != nil {
		t.Fatalf("PeopleSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "slip" {
		t.Fatalf("PeopleSlipping returned %+v", slipping)
	}
}
