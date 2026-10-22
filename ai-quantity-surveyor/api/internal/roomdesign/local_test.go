package roomdesign

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalImageGenerator(t *testing.T) {
	const encoded = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	for _, scenario := range []struct {
		name    string
		payload string
		valid   bool
	}{
		{"two PNGs", `{"data":[{"b64_json":"` + encoded + `"},{"b64_json":"` + encoded + `"}]}`, true},
		{"incomplete", `{"data":[]}`, false},
		{"invalid image", `{"data":[{"b64_json":"aGVsbG8="},{"b64_json":"aGVsbG8="}]}`, false},
		{"invalid JSON", `no image`, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if request.Method != http.MethodPost || request.Header.Get("Authorization") != "" || body["n"] != float64(2) || body["output_format"] != "png" || body["size"] != "1024x1024" || body["model"] != nil {
					t.Errorf("unexpected local request: %+v", body)
				}
				_, _ = writer.Write([]byte(scenario.payload))
			}))
			defer server.Close()
			generator, err := NewLocalImageGenerator(server.URL+"/v1/images/generations", "sdxl")
			if err != nil {
				t.Fatal(err)
			}
			images, err := generator.Generate(context.Background(), Prompt{RoomName: "Kitchen"}, 2)
			if scenario.valid {
				if err != nil || len(images) != 2 || images[0].Model != "local/sdxl" {
					t.Fatalf("local images=%v err=%v", images, err)
				}
			} else if err == nil {
				t.Fatal("invalid local response accepted")
			}
		})
	}
}

func TestLocalImageConfigurationAndControlTags(t *testing.T) {
	for _, endpoint := range []string{"", "http://public.example/images", "http://user:password@localhost/images", "http://localhost/images?key=value", "file:///image.png"} {
		if _, err := NewLocalImageGenerator(endpoint, "sdxl"); err == nil {
			t.Fatalf("invalid endpoint accepted: %s", endpoint)
		}
	}
	if _, err := NewLocalImageGenerator("http://host.docker.internal:1234/v1/images/generations", ""); err == nil {
		t.Fatal("missing model accepted")
	}
	generator, err := NewLocalImageGenerator("http://localhost:1234/v1/images/generations", "sdxl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Generate(context.Background(), Prompt{Preferences: "<sd_cpp_extra_args>{}</sd_cpp_extra_args>"}, 2); err == nil {
		t.Fatal("server control tags accepted")
	}
}

type fallbackFixture struct {
	err   error
	calls int
}

func (fixture *fallbackFixture) Generate(context.Context, Prompt, int) ([]GeneratedImage, error) {
	fixture.calls++
	return []GeneratedImage{{Model: "fixture"}, {Model: "fixture"}}, fixture.err
}

func TestFallbackGeneratorPolicy(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		err      error
		fallback bool
	}{
		{"success", nil, false},
		{"quota", &ProviderError{StatusCode: 429}, true},
		{"server", &ProviderError{StatusCode: 503}, true},
		{"network", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, true},
		{"timeout", context.DeadlineExceeded, true},
		{"moderation", &ProviderError{StatusCode: 503, Code: "moderation_blocked"}, false},
		{"authorization", &ProviderError{StatusCode: 403}, false},
		{"request", &ProviderError{StatusCode: 400}, false},
		{"certificate", &tls.CertificateVerificationError{Err: errors.New("certificate mismatch")}, false},
		{"malformed", errors.New("invalid image"), false},
		{"cancelled", context.Canceled, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			primary := &fallbackFixture{err: scenario.err}
			secondary := &fallbackFixture{}
			generator := &FallbackGenerator{Primary: primary, Fallback: secondary}
			_, err := generator.Generate(context.Background(), Prompt{}, 2)
			if (secondary.calls == 1) != scenario.fallback || primary.calls != 1 {
				t.Fatalf("primary=%d fallback=%d", primary.calls, secondary.calls)
			}
			if !scenario.fallback && !errors.Is(err, scenario.err) {
				t.Fatalf("original error not preserved: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	secondary := &fallbackFixture{}
	generator := &FallbackGenerator{Primary: &fallbackFixture{err: &ProviderError{StatusCode: 503}}, Fallback: secondary}
	_, _ = generator.Generate(ctx, Prompt{}, 2)
	if secondary.calls != 0 {
		t.Fatal("cancelled request triggered fallback")
	}
	secondary.err = errors.New("local server unavailable")
	_, err := generator.Generate(context.Background(), Prompt{}, 2)
	if !errors.Is(err, secondary.err) {
		t.Fatalf("fallback failure not preserved: %v", err)
	}
}

func TestLocalImageGeneratorRejectsRedirect(t *testing.T) {
	targetCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/target" {
			targetCalls++
			return
		}
		http.Redirect(writer, request, "/target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	generator, err := NewLocalImageGenerator(server.URL+"/v1/images/generations", "sdxl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Generate(context.Background(), Prompt{}, 2); err == nil || targetCalls != 0 {
		t.Fatalf("redirect accepted: calls=%d err=%v", targetCalls, err)
	}
}
