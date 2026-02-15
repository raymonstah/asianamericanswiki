package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
)

type HTMLResponseXAIAdmin struct {
	Base
	Humans []humandao.Human
}

func (s *ServerHTML) HandlerXAIAdmin(w http.ResponseWriter, r *http.Request) error {
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

	response := HTMLResponseXAIAdmin{
		Base:   getBase(s, admin),
		Humans: humans,
	}
	if err := s.template.ExecuteTemplate(w, "xai-admin.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-admin template")
	}

	return nil
}

type HTMLResponseXAIHuman struct {
	Base
	Human          humandao.Human
	ExistingImages []string
}

func (s *ServerHTML) HandlerXAIHuman(w http.ResponseWriter, r *http.Request) error {
	token, err := s.parseToken(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	id := chi.URLParam(r, "id")
	var human humandao.Human
	
	s.lock.Lock()
	for _, h := range s.humans {
		if h.ID == id || h.Path == id {
			human = h
			break
		}
	}
	s.lock.Unlock()
	
	if human.ID == "" {
		return NewNotFoundError(fmt.Errorf("human not found"))
	}

	// Scan for existing local images
	var existingImages []string
	localDir := filepath.Join("tmp", "xai_generations", human.ID)
	files, err := os.ReadDir(localDir)
	if err == nil {
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".webp") {
				existingImages = append(existingImages, "/xai-generations/"+human.ID+"/"+file.Name())
			}
		}
	}
	// Sort to show newest first if they have timestamps in names
	sort.Slice(existingImages, func(i, j int) bool {
		return existingImages[i] > existingImages[j]
	})

	response := HTMLResponseXAIHuman{
		Base:           getBase(s, admin),
		Human:          human,
		ExistingImages: existingImages,
	}
	if err := s.template.ExecuteTemplate(w, "xai-human.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-human template")
	}

	return nil
}

func (s *ServerHTML) HandlerXAIGenerate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(err)
	}

	humanID := r.FormValue("human_id")
	prompt := r.FormValue("prompt")
	numImagesStr := r.FormValue("num_images")
	numImages := 1
	if _, err := fmt.Sscanf(numImagesStr, "%d", &numImages); err != nil {
		s.logger.Warn().Err(err).Str("num_images", numImagesStr).Msg("invalid num_images value, defaulting to 1")
	}

	// Fetch human to get source images
	var human humandao.Human
	
	s.lock.Lock()
	for _, h := range s.humans {
		if h.ID == humanID {
			human = h
			break
		}
	}
	s.lock.Unlock()

	baseImage := human.Images.Featured
	if baseImage != "" && (strings.Contains(baseImage, "127.0.0.1") || strings.HasPrefix(baseImage, "http://")) {
		s.logger.Info().Str("url", baseImage).Msg("local source image detected, converting to base64")
		// Parse the emulator URL to get the object path
		// URL format: http://127.0.0.1:9199/asianamericanswiki-images/<humanID>/original.webp
		prefix := fmt.Sprintf("%s/%s/", s.storageURL, api.ImagesStorageBucket)
		objectPath := strings.TrimPrefix(baseImage, prefix)

		obj := s.storageClient.Bucket(api.ImagesStorageBucket).Object(objectPath)
		reader, err := obj.NewReader(ctx)
		if err != nil {
			s.logger.Error().Err(err).Str("path", objectPath).Msg("failed to create reader for local storage object")
		} else {
			defer func() {
				_ = reader.Close()
			}()
			data, err := io.ReadAll(reader)
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to read local storage object")
			} else {
				base64Data := base64.StdEncoding.EncodeToString(data)
				mimeType := http.DetectContentType(data)
				baseImage = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
				s.logger.Info().Msg("successfully converted local image to base64 for xAI")
			}
		}
	}

	imageURLs, err := s.xaiClient.GenerateImage(ctx, xai.GenerateImageInput{
		Prompt: prompt,
		N:      numImages,
		Image:  baseImage,
	})
	if err != nil {
		if strings.Contains(err.Error(), "(status 429)") {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`<div class="col-span-full p-4 bg-amber-50 border border-amber-200 rounded-lg text-amber-800">
                <p class="font-bold">xAI is currently overloaded</p>
                <p class="text-sm">The model is experiencing high demand. Please wait a few minutes and try again.</p>
            </div>`))
			return nil
		}
		return NewInternalServerError(err)
	}

	// Save images locally
	localDir := filepath.Join("tmp", "xai_generations", humanID)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return NewInternalServerError(err)
	}

	var localPaths []string
	for i, url := range imageURLs {
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		filename := fmt.Sprintf("%d_%d.webp", time.Now().Unix(), i)
		localPath := filepath.Join(localDir, filename)
		out, err := os.Create(localPath)
		if err != nil {
			continue
		}
		defer func() {
			_ = out.Close()
		}()
		_, _ = io.Copy(out, resp.Body)
		localPaths = append(localPaths, "/xai-generations/"+humanID+"/"+filename)
	}

	var data = struct {
		Images  []string
		HumanID string
	}{
		Images:  localPaths,
		HumanID: humanID,
	}

	if err := s.template.ExecuteTemplate(w, "xai-images-partial.html", data); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-images-partial template")
	}

	return nil
}

func (s *ServerHTML) HandlerXAIUpload(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(err)
	}

	humanID := r.FormValue("human_id")
	imagePath := r.FormValue("image_path") // e.g. /xai-generations/human_id/filename.webp

	// Convert local path back to filesystem path
	fsPath := filepath.Join("tmp", "xai_generations", strings.TrimPrefix(imagePath, "/xai-generations/"))

	raw, err := os.ReadFile(fsPath)
	if err != nil {
		return NewInternalServerError(err)
	}

	// Update human record
	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{HumanID: humanID})
	if err != nil {
		return NewInternalServerError(err)
	}

	human.AIGenerated = true
	if _, err := s.uploader.UploadHumanImages(ctx, human, raw); err != nil {
		return NewInternalServerError(err)
	}

	if err := s.updateIndex(human); err != nil {
		s.logger.Error().Err(err).Str("id", human.ID).Msg("unable to update index")
	}

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully updated human with AI image")
	w.Header().Add("HX-Redirect", fmt.Sprintf("/humans/%s", human.Path))
	return nil
}
