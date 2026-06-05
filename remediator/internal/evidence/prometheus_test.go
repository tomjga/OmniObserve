package evidence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGather_ReturnsOnlyQueriesWithData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expr := r.URL.Query().Get("query")
		// Only the gRPC queries return data; HTTP queries come back empty.
		if strings.Contains(expr, "rpc_server_duration") {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1780000000,"0.42"]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	got := NewPrometheus(srv.URL).Gather(context.Background(), "product-catalog")

	if len(got) != 2 {
		t.Fatalf("got %d metrics, want 2 (the two gRPC queries)", len(got))
	}
	for _, m := range got {
		if !strings.Contains(m.Name, "gRPC") {
			t.Errorf("unexpected metric %q (HTTP queries had no data)", m.Name)
		}
		if m.Value != "0.42" {
			t.Errorf("value = %q, want 0.42", m.Value)
		}
	}
}

func TestGather_QuerySubstitutesService(t *testing.T) {
	var sawJob string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("query"), `job="cart"`) {
			sawJob = "cart"
		}
		_, _ = w.Write([]byte(`{"data":{"result":[]}}`))
	}))
	defer srv.Close()

	NewPrometheus(srv.URL).Gather(context.Background(), "cart")
	if sawJob != "cart" {
		t.Error("service name was not substituted into the query")
	}
}
