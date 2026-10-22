package roomdesign

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIGeneratorSendsRoomTextAndReturnsTwoImages(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/images/generations" || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL)
		}
		var input struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			N      int    `json:"n"`
			Size   string `json:"size"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Model != "test-image-model" || input.N != 2 || input.Size != "1024x1024" || !strings.Contains(input.Prompt, "4 m long, 3 m wide") || !strings.Contains(input.Prompt, "Scandinavian") || strings.Contains(input.Prompt, "floor-plan-bytes") {
			t.Errorf("unexpected image request: %+v", input)
		}
		encoded := base64.StdEncoding.EncodeToString(png)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"b64_json":"` + encoded + `"},{"b64_json":"` + encoded + `"}]}`))
	}))
	defer server.Close()

	generator := newOpenAIGenerator("test-key", "test-image-model", server.URL+"/v1/images/generations", server.Client())
	images, err := generator.Generate(context.Background(), Prompt{RoomName: "Kitchen", Length: "4 m", Width: "3 m", WallHeight: "2.7 m", Style: "Scandinavian", Palette: "Warm neutrals"}, 2)
	if err != nil || len(images) != 2 || string(images[0].PNG) != string(png) || images[0].Model != "test-image-model" {
		t.Fatalf("generated images = %d, %v", len(images), err)
	}
}

func TestOpenAIGeneratorRejectsRedirectAndUnexpectedImageCount(t *testing.T) {
	t.Run("redirect is not followed", func(t *testing.T) {
		followed := false
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/target" {
				followed = true
			}
			http.Redirect(writer, request, "/target", http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		generator := NewOpenAIGenerator("test-key", "test-image-model")
		generator.endpoint = server.URL
		_, err := generator.Generate(context.Background(), Prompt{}, 2)
		if err == nil || followed {
			t.Fatalf("redirect followed=%v, err=%v", followed, err)
		}
	})

	t.Run("wrong image count", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(`{"data":[{"b64_json":"aGk="}]}`))
		}))
		defer server.Close()
		generator := newOpenAIGenerator("test-key", "test-image-model", server.URL, server.Client())
		if _, err := generator.Generate(context.Background(), Prompt{}, 2); err == nil {
			t.Fatal("incomplete image set was accepted")
		}
	})
}
