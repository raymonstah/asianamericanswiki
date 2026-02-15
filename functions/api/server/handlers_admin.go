package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/raymonstah/asianamericanswiki/internal/ethnicity"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
)

type HTMLResponseLogin struct {
	Base
	FirebaseConfig FirebaseConfig
}

func (s *ServerHTML) HandlerLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		idToken, err := parseBearerToken(r)
		if err != nil {
			return err
		}
		// Set session expiration to 5 days.
		expiresIn := time.Hour * 24 * 5

		// Create the session cookie. This will also verify the ID token in the process.
		// The session cookie will have the same claims as the ID token.
		// To only allow session cookie setting on recent sign-in, auth_time in ID token
		// can be checked to ensure user was recently signed in before creating a session cookie.
		cookie, err := s.authClient.SessionCookie(r.Context(), idToken, expiresIn)
		if err != nil {
			return NewUnauthorizedError(fmt.Errorf("unable to create session token: %w", err))
		}

		// Set cookie policy for session cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    cookie,
			MaxAge:   int(expiresIn.Seconds()),
			HttpOnly: true,
			Secure:   true,
		})

		// Get the original path the user was trying to access.
		referer := r.Header.Get("Referer")
		fmt.Println("referer", referer)
		if referer == "" {
			referer = "/admin"
		}

		w.Header().Add("HX-Redirect", referer)
		http.Redirect(w, r, referer, http.StatusFound)
		return nil
	}

	response := HTMLResponseLogin{
		Base:           getBase(s, false),
		FirebaseConfig: s.firebaseConfig,
	}
	if err := s.template.ExecuteTemplate(w, "login.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute login template")
	}

	return nil
}

type HTMLResponseAdmin struct {
	Base
	AdminName       string
	Drafts          []humandao.Human
	HumanFormFields HumanFormFields
	Human           humandao.Human
}

// HumanFormFields holds helper data to populate the form to add a new human.
type HumanFormFields struct {
	Source      string
	Ethnicities []ethnicity.Ethnicity
	Tags        []string
}

func (s *ServerHTML) HandlerAdmin(w http.ResponseWriter, r *http.Request) error {
	token, err := s.parseToken(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	humans, err := s.humanDAO.ListHumans(r.Context(), humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	var drafts []humandao.Human
	for _, human := range humans {
		if human.Draft {
			drafts = append(drafts, human)
		}
	}

	response := HTMLResponseAdmin{
		Base:      getBase(s, false),
		AdminName: token.Claims["name"].(string),
		HumanFormFields: HumanFormFields{
			Ethnicities: ethnicity.All,
			Tags:        getTags(humans),
		},
		Drafts: drafts,
	}
	if err := s.template.ExecuteTemplate(w, "admin.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute admin template")
	}

	return nil
}

// HandlerGenerate takes in the form, and populates it based on the data in the 'source' field.
func (s *ServerHTML) HandlerGenerate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := s.logger
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(fmt.Errorf("invalid form received: %w", err))
	}

	source := r.FormValue("source")

	addHumanRequest, err := s.xaiClient.FromText(ctx, xai.FromTextInput{Data: source})
	if err != nil {
		return NewInternalServerError(err)
	}
	logger.Info().Str("source", source).Any("addHumanRequest", addHumanRequest).Msg("generated response from xAI")

	human := humandao.Human{
		Name:        addHumanRequest.Name,
		Gender:      humandao.Gender(addHumanRequest.Gender),
		Ethnicity:   addHumanRequest.Ethnicity,
		DOB:         addHumanRequest.DOB,
		DOD:         addHumanRequest.DOD,
		Description: addHumanRequest.Description,
	}

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	response := HTMLResponseAdmin{
		HumanFormFields: HumanFormFields{
			Source:      source,
			Ethnicities: ethnicity.All,
			Tags:        getTags(humans),
		},
		Human: human,
	}
	if err := s.template.ExecuteTemplate(w, "new-human-form.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute admin template")
	}

	return nil
}
