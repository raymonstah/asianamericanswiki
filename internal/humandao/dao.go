package humandao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/httplog"
	"github.com/segmentio/ksuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrHumanNotFound      = errors.New("human not found")
	ErrHumanAlreadyExists = errors.New("human already exists")
	ErrInvalidOrderBy     = errors.New("orderBy must be one of: created_at, views")
	ErrInvalidGender      = errors.New("invalid gender")
)

type Human struct {
	ID            string   `firestore:"-"`
	Name          string   `firestore:"name"`
	Path          string   `firestore:"urn_path"`
	DOB           string   `firestore:"dob,omitempty"`
	DOD           string   `firestore:"dod,omitempty"`
	Tags          []string `firestore:"tags,omitempty"`
	Ethnicity     []string `firestore:"ethnicity,omitempty"`
	BirthLocation string   `firestore:"birth_location,omitempty"`
	Location      []string `firestore:"location,omitempty"`
	InfluencedBy  []string `firestore:"influenced_by,omitempty"`
	FeaturedImage string   `firestore:"featured_image,omitempty"`
	Draft         bool     `firestore:"draft"`
	AIGenerated   bool     `firestore:"ai_generated,omitempty"`
	Description   string   `firestore:"description,omitempty"`

	CreatedAt time.Time `firestore:"created_at"`
	CreatedBy string    `firestore:"created_by,omitempty"`

	UpdatedAt   time.Time `firestore:"updated_at"`
	PublishedBy string    `firestore:"published_by,omitempty"`
	PublishedAt time.Time `firestore:"published_at,omitempty"`
	Socials     Socials   `firestore:"socials,omitempty"`
	Views       int64     `firestore:"views,omitempty"`
	Gender      Gender    `firestore:"gender,omitempty"`

	// Similar is computed using cmd/compute-similar/main.go
	Similar []string `firestore:"similar,omitempty"`
}

func (h Human) CurrentAge(inputTime ...time.Time) (string, error) {
	now := time.Now()
	if len(inputTime) > 0 {
		now = inputTime[0]
	}
	if h.DOB == "" {
		return "", nil
	}

	born, err := parseDateString(h.DOB)
	if err != nil {
		return "", err
	}

	if h.DOD != "" {
		died, err := parseDateString(h.DOD)
		if err != nil {
			return "", err
		}
		ageInYears, _, _, _, _, _ := diff(died, born)
		return fmt.Sprintf("died at %v y/o", ageInYears), nil
	}

	ageInYears, _, _, _, _, _ := diff(now, born)
	return fmt.Sprintf("%v y/o", ageInYears), nil
}

func parseDateString(date string) (time.Time, error) {
	format := "2006-01-02"
	if len(date) == 4 {
		// only have the year
		format = "2006"
	} else if len(date) == 7 {
		// only have the year and month
		format = "2006-01"
	}

	res, err := time.Parse(format, date)
	if err != nil {
		return time.Time{}, err
	}

	return res, nil
}

func diff(a, b time.Time) (year, month, day, hour, min, sec int) {
	if a.Location() != b.Location() {
		b = b.In(a.Location())
	}
	if a.After(b) {
		a, b = b, a
	}
	y1, M1, d1 := a.Date()
	y2, M2, d2 := b.Date()

	h1, m1, s1 := a.Clock()
	h2, m2, s2 := b.Clock()

	year = int(y2 - y1)
	month = int(M2 - M1)
	day = int(d2 - d1)
	hour = int(h2 - h1)
	min = int(m2 - m1)
	sec = int(s2 - s1)

	// Normalize negative values
	if sec < 0 {
		sec += 60
		min--
	}
	if min < 0 {
		min += 60
		hour--
	}
	if hour < 0 {
		hour += 24
		day--
	}
	if day < 0 {
		// days in month:
		t := time.Date(y1, M1, 32, 0, 0, 0, 0, time.UTC)
		day += 32 - t.Day()
		month--
	}
	if month < 0 {
		month += 12
		year--
	}

	return
}

type Gender string

const (
	GenderMale      Gender = "male"
	GenderFemale    Gender = "female"
	GenderNonBinary Gender = "nonbinary"
)

var ValidGenders = map[Gender]struct{}{
	GenderMale:      {},
	GenderFemale:    {},
	GenderNonBinary: {},
}

type Socials struct {
	IMDB      string `firestore:"imdb,omitempty"`
	Website   string `firestore:"website,omitempty"`
	X         string `firestore:"x,omitempty"`
	YouTube   string `firestore:"youtube,omitempty"`
	Instagram string `firestore:"instagram,omitempty"`
}

type HumanInput struct {
	HumanID string
	Path    string
}

func (d *DAO) Human(ctx context.Context, input HumanInput) (human Human, err error) {
	logger := httplog.LogEntry(ctx)
	var doc *firestore.DocumentSnapshot
	if input.HumanID != "" {
		doc, err = d.client.Collection(d.humanCollection).Doc(input.HumanID).Get(ctx)
	} else if input.Path != "" {
		doc, err = d.client.Collection(d.humanCollection).Where("urn_path", "==", input.Path).
			Documents(ctx).Next()
	}
	if err != nil {
		if status.Code(err) == codes.NotFound || err == iterator.Done {
			logger.Warn().Err(err).Interface("input", input).
				Str("humanID", input.HumanID).
				Str("path", input.Path).
				Msg("human not found")
			identifier := input.HumanID
			if input.Path != "" {
				identifier = input.Path
			}
			return Human{}, fmt.Errorf("%w: %v", ErrHumanNotFound, identifier)
		}
		logger.Err(err).Interface("input", input).Msg("unable to get human")
		return Human{}, fmt.Errorf("unable to get human: %w", err)
	}

	human, err = convertHumanDoc(doc)
	if err != nil {
		return Human{}, fmt.Errorf("unable to convert human: %w", err)
	}

	human.ID = doc.Ref.ID
	return human, nil
}

type HumansByIDInput struct {
	HumanIDs []string
}

func (d *DAO) HumansByID(ctx context.Context, input HumansByIDInput) ([]Human, error) {
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(10)
	var mutex sync.Mutex
	humans := make([]Human, 0, len(input.HumanIDs))

	for _, id := range input.HumanIDs {
		id := id
		group.Go(func() error {
			human, err := d.Human(ctx, HumanInput{HumanID: id})
			if err != nil {
				return err
			}

			mutex.Lock()
			humans = append(humans, human)
			mutex.Unlock()

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	// Preserve ordering
	idToHuman := make(map[string]Human)
	for _, human := range humans {
		idToHuman[human.ID] = human
	}

	orderedHumans := make([]Human, 0, len(humans))
	for _, id := range input.HumanIDs {
		orderedHumans = append(orderedHumans, idToHuman[id])
	}

	return orderedHumans, nil
}

func (d *DAO) UpdateHuman(ctx context.Context, human Human) error {
	human.UpdatedAt = time.Now()
	human.Path = strings.ToLower(strings.ReplaceAll(human.Name, " ", "-"))
	_, err := d.client.Collection(d.humanCollection).
		Doc(human.ID).
		Set(ctx, human)
	if err != nil {
		return fmt.Errorf("unable to update human: %v (%v): %w", human.Name, human.ID, err)
	}

	return nil
}

type AddHumanInput struct {
	HumanID     string
	Name        string
	DOB         string
	DOD         string
	Ethnicity   []string
	Description string
	Location    []string
	Website     string
	Twitter     string
	Instagram   string
	IMDB        string
	Tags        []string
	Draft       bool
	CreatedBy   string
	Gender      Gender
}

func (d *DAO) AddHuman(ctx context.Context, input AddHumanInput) (Human, error) {
	path := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	if input.Name == "" {
		return Human{}, fmt.Errorf("name must be provided")
	}

	_, err := d.Human(ctx, HumanInput{Path: path})
	if err != nil {
		if !errors.Is(err, ErrHumanNotFound) {
			return Human{}, fmt.Errorf("error checking if human (%v) exists: %w", path, err)
		}
	}
	if err == nil {
		return Human{}, ErrHumanAlreadyExists
	}

	_, ok := ValidGenders[input.Gender]
	if !ok {
		return Human{}, ErrInvalidGender
	}

	now := time.Now().In(time.UTC)
	human := Human{
		Name:        input.Name,
		DOB:         input.DOB,
		DOD:         input.DOD,
		Ethnicity:   input.Ethnicity,
		Description: input.Description,
		Location:    input.Location,
		Tags:        input.Tags,
		Draft:       input.Draft,
		CreatedAt:   now,
		CreatedBy:   input.CreatedBy,
		Path:        path,
		UpdatedAt:   now,
		Socials: Socials{
			Website:   input.Website,
			X:         input.Twitter,
			IMDB:      input.IMDB,
			Instagram: input.Instagram,
		},
		Gender: input.Gender,
	}

	if input.HumanID == "" {
		input.HumanID = ksuid.New().String()
	}

	_, err = d.client.Collection(d.humanCollection).Doc(input.HumanID).Create(ctx, human)
	if err != nil {
		return Human{}, fmt.Errorf("unable to create human: %w", err)
	}

	human.ID = input.HumanID
	return human, nil
}

var ErrUnauthorized = errors.New("user is not authorized to perform this operation")

type OrderBy string

var (
	OrderByCreatedAt OrderBy = "created_at"
	OrderByViews     OrderBy = "views"
)

type ListHumansInput struct {
	Limit         int
	Offset        int
	OrderBy       OrderBy
	Direction     firestore.Direction
	IncludeDrafts bool
}

func (d *DAO) ListHumans(ctx context.Context, input ListHumansInput) ([]Human, error) {
	allowedOrderBy := map[OrderBy]struct{}{
		OrderByCreatedAt: {},
		OrderByViews:     {},
	}
	query := d.client.Collection(d.humanCollection).Query

	if !input.IncludeDrafts {
		query = query.Where("draft", "==", false)
	}
	if input.OrderBy == "" {
		query = query.OrderBy(string(OrderByCreatedAt), firestore.Desc)
	} else {
		if _, ok := allowedOrderBy[input.OrderBy]; !ok {
			return nil, ErrInvalidOrderBy
		}
		query = query.OrderBy(string(input.OrderBy), input.Direction)
	}
	docs, err := query.
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type CreatedByInput struct {
	CreatedBy string
	Limit     int
	Offset    int
}

func (d *DAO) CreatedBy(ctx context.Context, input CreatedByInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("created_by", "==", input.CreatedBy).
		OrderBy("created_at", firestore.Desc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type UserDraftsInput struct {
	Limit  int
	Offset int
	UserID string
}

func (d *DAO) UserDrafts(ctx context.Context, input UserDraftsInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("draft", "==", true).
		Where("created_by", "==", input.UserID).
		OrderBy("created_at", firestore.Desc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type DraftsInput struct {
	Limit  int
	Offset int
}

func (d *DAO) Drafts(ctx context.Context, input DraftsInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("draft", "==", true).
		OrderBy("created_at", firestore.Asc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type PublishInput struct {
	HumanID string
	UserID  string
}

func (d *DAO) Publish(ctx context.Context, input PublishInput) error {
	now := time.Now()
	_, err := d.client.Collection(d.humanCollection).
		Doc(input.HumanID).
		Update(ctx, []firestore.Update{
			{Path: "draft", Value: false},
			{Path: "published_by", Value: input.UserID},
			{Path: "published_at", Value: now},
		})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ErrHumanNotFound
		}
		return fmt.Errorf("unable to update human: %v: %w", input.HumanID, err)
	}

	return nil
}

type DeleteInput struct {
	HumanID string
}

func (d *DAO) Delete(ctx context.Context, input DeleteInput) error {
	_, err := d.client.Collection(d.humanCollection).
		Doc(input.HumanID).
		Delete(ctx)
	if err != nil {
		return fmt.Errorf("unable to delete human: %v: %w", input.HumanID, err)
	}

	return nil
}

type ViewInput struct {
	HumanID string
}

func (d *DAO) View(ctx context.Context, input ViewInput) error {
	_, err := d.client.Collection(d.humanCollection).
		Doc(input.HumanID).
		Update(ctx, []firestore.Update{
			{Path: "views", Value: firestore.Increment(1)},
		})
	if err != nil {
		return fmt.Errorf("unable to view human: %v: %w", input.HumanID, err)
	}

	return nil
}
