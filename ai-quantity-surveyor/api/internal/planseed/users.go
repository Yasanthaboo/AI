package planseed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type User struct {
	ID      string
	Email   string
	PlanID  string
	Created bool
}

type adminUser struct {
	ID               string         `json:"id"`
	Email            string         `json:"email"`
	AppMetadata      map[string]any `json:"app_metadata"`
	EmailConfirmedAt *string        `json:"email_confirmed_at"`
}

func EnsureUsers(ctx context.Context, config Config) ([]User, error) {
	if !config.Enabled {
		return nil, nil
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	perform := func(method, path string, body any, result any) error {
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			if err != nil {
				return errors.New("could not encode Supabase admin request")
			}
		}
		request, err := http.NewRequestWithContext(ctx, method, config.supabaseURL+"/auth/v1/admin/users"+path, bytes.NewReader(data))
		if err != nil {
			return errors.New("could not build Supabase admin request")
		}
		request.Header.Set("apikey", config.adminKey)
		request.Header.Set("Authorization", "Bearer "+config.adminKey)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return errors.New("Supabase administrative request failed; check endpoint, connectivity and credentials")
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("Supabase administrative request returned HTTP %d; account credentials were not logged", response.StatusCode)
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(result); err != nil {
			return errors.New("Supabase administrative response was invalid or too large")
		}
		return nil
	}
	found := make(map[string]adminUser)
	complete := false
	for page := 1; page <= 100; page++ {
		var result struct {
			Users []adminUser `json:"users"`
		}
		if err := perform(http.MethodGet, "?page="+strconv.Itoa(page)+"&per_page=100", nil, &result); err != nil {
			return nil, err
		}
		for _, user := range result.Users {
			for _, account := range config.accounts {
				if strings.EqualFold(user.Email, account.email) {
					if _, duplicate := found[account.email]; duplicate {
						return nil, errors.New("duplicate preview identities found; manual admin review is required")
					}
					found[account.email] = user
				}
			}
		}
		if len(result.Users) < 100 {
			complete = true
			break
		}
	}
	if !complete {
		return nil, errors.New("Supabase account lookup exceeded the safe paging bound")
	}
	for _, account := range config.accounts {
		if user, exists := found[account.email]; exists {
			if user.ID == "" || user.AppMetadata["plan_preview_seed"] != marker || user.AppMetadata["plan_preview_tier"] != account.planID || user.EmailConfirmedAt == nil {
				return nil, fmt.Errorf("refusing to adopt or overwrite an unrelated/unconfirmed %s preview identity; resolve the email collision in Supabase", account.planID)
			}
		}
	}
	users := make([]User, 0, 3)
	for _, account := range config.accounts {
		user, exists := found[account.email]
		if !exists {
			body := map[string]any{"email": account.email, "password": account.password, "email_confirm": true, "app_metadata": map[string]string{"plan_preview_seed": marker, "plan_preview_tier": account.planID}, "user_metadata": map[string]string{"display_name": strings.ToUpper(account.planID[:1]) + account.planID[1:] + " Plan Preview"}}
			if err := perform(http.MethodPost, "", body, &user); err != nil {
				return nil, err
			}
			if user.ID == "" || !strings.EqualFold(user.Email, account.email) || user.AppMetadata["plan_preview_seed"] != marker || user.AppMetadata["plan_preview_tier"] != account.planID || user.EmailConfirmedAt == nil {
				return nil, errors.New("created Supabase preview identity did not match the requested confirmed account")
			}
		}
		users = append(users, User{ID: user.ID, Email: account.email, PlanID: account.planID, Created: !exists})
	}
	return users, nil
}
