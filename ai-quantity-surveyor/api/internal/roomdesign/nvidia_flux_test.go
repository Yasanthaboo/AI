package roomdesign

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNvidiaFluxGeneratorSendsTextOnlyAndReturnsTwoImages(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(png)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/flux" || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected NIM image request: %s %s", request.Method, request.URL)
		}
		var body struct {
			Prompt string  `json:"prompt"`
			Width  int     `json:"width"`
			Height int     `json:"height"`
			Steps  int     `json:"steps"`
			Seed   *int64  `json:"seed"`
			Mode   string  `json:"mode"`
			Scale  float64 `json:"cfg_scale"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body.Prompt, "4 m long, 3 m wide") || strings.Contains(body.Prompt, "floor-plan-bytes") || body.Width != 1024 || body.Height != 1024 || body.Steps != 28 {
			t.Errorf("unexpected FLUX payload: %+v", body)
		}
		if body.Seed == nil || *body.Seed != 0 || body.Mode != "base" || body.Scale != 3.5 {
			t.Errorf("payload does not match the hosted FLUX contract: %+v", body)
		}
		calls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"image":"` + encoded + `"}`))
	}))
	defer server.Close()
	generator, err := NewNvidiaFluxGenerator("test-key", "flux-model-fixture", server.URL+"/flux")
	if err != nil {
		t.Fatal(err)
	}
	images, err := generator.Generate(context.Background(), Prompt{RoomName: "Kitchen", Length: "4 m", Width: "3 m", WallHeight: "2.7 m", Style: "Contemporary", Palette: "Warm neutrals"}, 2)
	if err != nil || len(images) != 2 || calls.Load() != 2 || string(images[0].PNG) != string(png) || images[0].Model != "flux-model-fixture" {
		t.Fatalf("generated images = %d, calls=%d, err=%v", len(images), calls.Load(), err)
	}
}

func TestNvidiaFluxArtifactsAreValidatedAndConvertedToPNG(t *testing.T) {
	var content bytes.Buffer
	if err := jpeg.Encode(&content, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"SUCCESS", "CONTENT_FILTERED", "ERROR"} {
		t.Run(reason, func(t *testing.T) {
			response := httptest.NewRecorder()
			_ = json.NewEncoder(response).Encode(map[string]any{"artifacts": []map[string]any{{"base64": base64.StdEncoding.EncodeToString(content.Bytes()), "finishReason": reason, "seed": 42}}})
			converted, err := readNvidiaFluxImage(response.Result())
			if reason != "SUCCESS" {
				if err == nil {
					t.Fatal("unsuccessful artifact accepted")
				}
				return
			}
			if err != nil || !validPNG(converted) {
				t.Fatalf("JPEG artifact was not converted to PNG: %v", err)
			}
			configuration, format, err := image.DecodeConfig(bytes.NewReader(converted))
			if err != nil || format != "png" || configuration.Width != 2 || configuration.Height != 2 {
				t.Fatalf("invalid converted image: %+v %s %v", configuration, format, err)
			}
		})
	}
}

func TestNvidiaFluxGeneratorRejectsInvalidEndpointAndRedirect(t *testing.T) {
	if _, err := NewNvidiaFluxGenerator("test-key", "model", "http://remote.example/flux"); err == nil {
		t.Fatal("insecure endpoint accepted")
	}
	t.Run("redirect is not followed", func(t *testing.T) {
		var targetCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/target" {
				targetCalls.Add(1)
				return
			}
			http.Redirect(writer, request, "/target", http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		generator, err := NewNvidiaFluxGenerator("test-key", "model", server.URL+"/flux")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := generator.Generate(context.Background(), Prompt{}, 2); err == nil || targetCalls.Load() != 0 {
			t.Fatalf("redirect followed or error missing: calls=%d err=%v", targetCalls.Load(), err)
		}
	})
}
