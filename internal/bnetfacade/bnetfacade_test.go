package bnetfacade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBridgeGetRefusesNonLocal(t *testing.T) {
	for _, addr := range []string{"example.com:80", "8.8.8.8:53", "192.168.1.1:6119"} {
		_, err := BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser)
		if !errors.Is(err, ErrNotLocal) {
			t.Errorf("BridgeGet(%q): got %v, want ErrNotLocal", addr, err)
		}
	}
}

func TestBridgeGetRefusesBadPath(t *testing.T) {
	for _, path := range []string{"/api/health", "/other", "/web-apifoo", "web-api/v1"} {
		_, err := BridgeGet(context.Background(), "127.0.0.1:6119", path, PriorityUser)
		if !errors.Is(err, ErrForbiddenPath) {
			t.Errorf("BridgeGet(path=%q): got %v, want ErrForbiddenPath", path, err)
		}
	}
}

func TestBridgeGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/web-api/v1/profile" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"test"}`)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()
	data, err := BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser)
	if err != nil {
		t.Fatalf("BridgeGet: %v", err)
	}
	if string(data) != `{"name":"test"}` {
		t.Errorf("got %q, want %q", data, `{"name":"test"}`)
	}
}

func TestDecodeBridgeJSON_UTF8(t *testing.T) {
	raw := []byte(`{"map":"Lost Temple"}`)
	var out struct{ Map string }
	if err := DecodeBridgeJSON(raw, &out); err != nil {
		t.Fatalf("DecodeBridgeJSON: %v", err)
	}
	if out.Map != "Lost Temple" {
		t.Errorf("got %q, want %q", out.Map, "Lost Temple")
	}
}

func TestDecodeBridgeJSON_CP949(t *testing.T) {
	// EUC-KR encoding of "서울" (Seoul) is 0xBC AD 0xBF EF.
	cp949Seoul := []byte{0xBC, 0xAD, 0xBF, 0xEF}
	raw := append([]byte(`{"map":"`), cp949Seoul...)
	raw = append(raw, []byte(`"}`)...)

	var out struct{ Map string }
	if err := DecodeBridgeJSON(raw, &out); err != nil {
		t.Fatalf("DecodeBridgeJSON cp949: %v", err)
	}
	if out.Map != "서울" {
		t.Errorf("got %q, want %q", out.Map, "서울")
	}
}

func TestDecodeBridgeJSON_Latin1(t *testing.T) {
	// ISO-8859-1 encoding of "café" uses 0xE9 for é.
	raw := []byte(`{"map":"caf` + string([]byte{0xe9}) + `"}`)
	var out struct{ Map string }
	if err := DecodeBridgeJSON(raw, &out); err != nil {
		t.Fatalf("DecodeBridgeJSON latin1: %v", err)
	}
	if out.Map != "café" {
		t.Errorf("got %q, want %q", out.Map, "café")
	}
}

func TestDownloadReplayRefusesBadPath(t *testing.T) {
	for _, path := range []string{
		"/other-bucket/file.rep",
		"/starcraft-user-uploads-prod/S2-replays/foo.rep",
		"starcraft-user-uploads-prod/S1-replays/foo.rep",
	} {
		_, err := DownloadReplay(context.Background(), path, PriorityUser)
		if !errors.Is(err, ErrForbiddenPath) {
			t.Errorf("DownloadReplay(path=%q): got %v, want ErrForbiddenPath", path, err)
		}
	}
}

func TestValidateReplay_TooShort(t *testing.T) {
	err := validateReplay([]byte("short"))
	if !errors.Is(err, ErrInvalidReplay) {
		t.Errorf("validateReplay(short): got %v, want ErrInvalidReplay", err)
	}
}

func TestValidateReplay_BadMagic(t *testing.T) {
	data := make([]byte, 32)
	copy(data[12:16], "XXXX")
	err := validateReplay(data)
	if !errors.Is(err, ErrInvalidReplay) {
		t.Errorf("validateReplay(bad magic): got %v, want ErrInvalidReplay", err)
	}
}

func TestValidateReplay_Valid(t *testing.T) {
	data := make([]byte, 32)
	copy(data[12:16], "seRS")
	if err := validateReplay(data); err != nil {
		t.Errorf("validateReplay(valid): unexpected error: %v", err)
	}
}

func TestDownloadReplayValidation(t *testing.T) {
	validReplay := make([]byte, 64)
	copy(validReplay[12:16], "seRS")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/starcraft-user-uploads-prod/S1-replays/valid.rep":
			w.Write(validReplay)
		case "/starcraft-user-uploads-prod/S1-replays/short.rep":
			w.Write([]byte("short"))
		case "/starcraft-user-uploads-prod/S1-replays/badmagic.rep":
			bad := make([]byte, 32)
			w.Write(bad)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Override the GCS URL for testing by using DownloadReplayFrom.
	t.Run("valid", func(t *testing.T) {
		data, err := downloadReplayFrom(context.Background(),
			srv.URL+"/starcraft-user-uploads-prod/S1-replays/valid.rep")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 64 {
			t.Errorf("got %d bytes, want 64", len(data))
		}
	})

	t.Run("too_short", func(t *testing.T) {
		_, err := downloadReplayFrom(context.Background(),
			srv.URL+"/starcraft-user-uploads-prod/S1-replays/short.rep")
		if !errors.Is(err, ErrInvalidReplay) {
			t.Errorf("got %v, want ErrInvalidReplay", err)
		}
	})

	t.Run("bad_magic", func(t *testing.T) {
		_, err := downloadReplayFrom(context.Background(),
			srv.URL+"/starcraft-user-uploads-prod/S1-replays/badmagic.rep")
		if !errors.Is(err, ErrInvalidReplay) {
			t.Errorf("got %v, want ErrInvalidReplay", err)
		}
	})
}
