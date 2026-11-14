package slack

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// stubSlack returns an APIClient whose requests go to handle instead of Slack.
func stubSlack(t *testing.T, handle func(r *http.Request, body []byte) *http.Response) *APIClient {
	t.Helper()
	return newAPIClient("xoxb-test", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		resp := handle(r, body)
		resp.Request = r
		return resp, nil
	})})
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestPostMessageSendsTheRequestSlackExpects(t *testing.T) {
	for _, threadTS := range []string{"", "123.4"} {
		var got map[string]any
		c := stubSlack(t, func(r *http.Request, body []byte) *http.Response {
			if r.URL.String() != "https://slack.com/api/chat.postMessage" {
				t.Errorf("url = %s", r.URL)
			}
			if r.Header.Get("Authorization") != "Bearer xoxb-test" {
				t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				t.Errorf("content type = %q", r.Header.Get("Content-Type"))
			}
			_ = json.Unmarshal(body, &got)
			return jsonResponse(http.StatusOK, `{"ok":true}`)
		})
		if err := c.PostMessage(context.Background(), "C1", "hello", threadTS); err != nil {
			t.Fatal(err)
		}
		if got["channel"] != "C1" || got["text"] != "hello" {
			t.Fatalf("payload = %v", got)
		}
		if _, has := got["thread_ts"]; has != (threadTS != "") {
			t.Fatalf("thread_ts present = %v for threadTS %q: %v", has, threadTS, got)
		}
	}
}

func TestListUserGroupMembers(t *testing.T) {
	c := stubSlack(t, func(r *http.Request, body []byte) *http.Response {
		form, _ := url.ParseQuery(string(body))
		if r.URL.Path != "/api/usergroups.users.list" || form.Get("usergroup") != "S1" || form.Get("team_id") != "T1" {
			t.Errorf("request = %s %v", r.URL.Path, form)
		}
		return jsonResponse(http.StatusOK, `{"ok":true,"users":["U1","U2"]}`)
	})
	users, err := c.ListUserGroupMembers(context.Background(), "T1", "S1")
	if err != nil || strings.Join(users, ",") != "U1,U2" {
		t.Fatalf("users = %v, %v", users, err)
	}
}

func TestCallReportsFailuresByMethod(t *testing.T) {
	cases := map[string]struct {
		resp     *http.Response
		wantErr  []string
		notInErr string
	}{
		"slack error code": {
			resp:    jsonResponse(http.StatusOK, `{"ok":false,"error":"channel_not_found"}`),
			wantErr: []string{"slack chat.postMessage failed: channel_not_found"},
		},
		"rate limited": {
			resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": {"30"}},
				Body:       io.NopCloser(strings.NewReader("")),
			},
			wantErr: []string{"chat.postMessage", "429", "30"},
		},
		"server error with an html body": {
			resp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": {"text/html"}},
				Body:       io.NopCloser(strings.NewReader("<html>oops</html>")),
			},
			wantErr:  []string{"chat.postMessage", "500"},
			notInErr: "decode",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := stubSlack(t, func(*http.Request, []byte) *http.Response { return tc.resp })
			err := c.PostMessage(context.Background(), "C1", "hello", "")
			if err == nil {
				t.Fatal("want an error")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
			if tc.notInErr != "" && strings.Contains(err.Error(), tc.notInErr) {
				t.Errorf("error %q should not be a %s error", err, tc.notInErr)
			}
		})
	}
}

func TestDisplayNamePreference(t *testing.T) {
	cases := map[string]struct {
		user    string
		want    string
		wantErr bool
	}{
		"display name first":       {user: `{"real_name":"R","profile":{"display_name":"D","real_name":"P"}}`, want: "D"},
		"then profile real name":   {user: `{"real_name":"R","profile":{"display_name":"","real_name":"P"}}`, want: "P"},
		"then top-level real name": {user: `{"real_name":"R","profile":{}}`, want: "R"},
		"no name at all":           {user: `{"profile":{}}`, wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := stubSlack(t, func(r *http.Request, body []byte) *http.Response {
				if form, _ := url.ParseQuery(string(body)); r.URL.Path != "/api/users.info" || form.Get("user") != "U1" {
					t.Errorf("request = %s %s", r.URL.Path, body)
				}
				return jsonResponse(http.StatusOK, `{"ok":true,"user":`+tc.user+`}`)
			})
			got, err := c.DisplayName(context.Background(), "U1")
			if tc.wantErr != (err != nil) || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestIsWorkspaceAdmin(t *testing.T) {
	for user, want := range map[string]bool{
		`{"is_admin":true}`:  true,
		`{"is_owner":true}`:  true,
		`{"is_admin":false}`: false,
	} {
		c := stubSlack(t, func(*http.Request, []byte) *http.Response {
			return jsonResponse(http.StatusOK, `{"ok":true,"user":`+user+`}`)
		})
		if got, err := c.IsWorkspaceAdmin(context.Background(), "U1"); err != nil || got != want {
			t.Fatalf("user %s: got %v, %v; want %v", user, got, err, want)
		}
	}
}

// Every workspace's client shares one http.Client, so connections are reused.
func TestTeamResolvingClientSharesOneHTTPClient(t *testing.T) {
	tc := NewTeamResolvingClient(nil, "xoxb-fallback")
	a, err := tc.apiClient(context.Background(), "T1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := tc.apiClient(context.Background(), "T2")
	if a.httpClient == nil || a.httpClient != b.httpClient {
		t.Fatalf("clients do not share an http.Client: %p vs %p", a.httpClient, b.httpClient)
	}
}

func TestCallSurfacesTransportAndDecodeErrors(t *testing.T) {
	failing := newAPIClient("xoxb-test", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset")
	})})
	if err := failing.PostMessage(context.Background(), "C1", "hi", ""); err == nil || !strings.Contains(err.Error(), "send slack chat.postMessage") {
		t.Fatalf("transport failure: %v", err)
	}

	garbled := stubSlack(t, func(*http.Request, []byte) *http.Response { return jsonResponse(http.StatusOK, "not json") })
	if err := garbled.PostMessage(context.Background(), "C1", "hi", ""); err == nil || !strings.Contains(err.Error(), "decode slack chat.postMessage") {
		t.Fatalf("garbled body: %v", err)
	}

	// Slack failures reach callers of every method, not just PostMessage.
	rejecting := stubSlack(t, func(*http.Request, []byte) *http.Response {
		return jsonResponse(http.StatusOK, `{"ok":false,"error":"missing_scope"}`)
	})
	if _, err := rejecting.ListUserGroupMembers(context.Background(), "T1", "S1"); err == nil || !strings.Contains(err.Error(), "usergroups.users.list failed: missing_scope") {
		t.Errorf("ListUserGroupMembers: %v", err)
	}
	if _, err := rejecting.DisplayName(context.Background(), "U1"); err == nil || !strings.Contains(err.Error(), "users.info failed: missing_scope") {
		t.Errorf("DisplayName: %v", err)
	}
	if _, err := rejecting.IsWorkspaceAdmin(context.Background(), "U1"); err == nil || !strings.Contains(err.Error(), "users.info failed: missing_scope") {
		t.Errorf("IsWorkspaceAdmin: %v", err)
	}
}
