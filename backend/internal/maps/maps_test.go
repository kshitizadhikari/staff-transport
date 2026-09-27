package maps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMapboxGeocoderSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "test-token" {
			t.Errorf("missing access token, got %q", r.URL.Query().Get("access_token"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"features":[{"place_name":"Kathmandu, Nepal","center":[85.324,27.7172]}]}`))
	}))
	defer server.Close()

	geocoder := &MapboxGeocoder{client: server.Client(), token: "test-token", endpoint: server.URL, country: "np"}
	location, err := geocoder.Geocode(context.Background(), "Kathmandu")
	if err != nil {
		t.Fatalf("geocode: %v", err)
	}
	if location.Latitude != 27.7172 || location.Longitude != 85.324 {
		t.Fatalf("unexpected coordinates: %+v", location)
	}
	if location.Address != "Kathmandu, Nepal" {
		t.Fatalf("unexpected address: %q", location.Address)
	}
}

func TestMapboxGeocoderNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"features":[]}`))
	}))
	defer server.Close()

	geocoder := &MapboxGeocoder{client: server.Client(), token: "test-token", endpoint: server.URL}
	if _, err := geocoder.Geocode(context.Background(), "nowhere"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMapboxGeocoderUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	geocoder := &MapboxGeocoder{client: server.Client(), token: "bad", endpoint: server.URL}
	if _, err := geocoder.Geocode(context.Background(), "Kathmandu"); err == nil {
		t.Fatal("expected error for unauthorized response")
	}
}

func TestMapboxGeocoderNotConfigured(t *testing.T) {
	geocoder := NewMapboxGeocoder("", "np")
	if _, err := geocoder.Geocode(context.Background(), "Kathmandu"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}
