// Command http is a minimal net/http integration example. It intentionally
// uses offline generation only; real services should authenticate callers,
// use coordinated mode where required, and persist IDs in a database.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/dev-gopi/uniqidx"
)

func main() {
	gen, err := uniqidx.NewGenerator(context.Background(), uniqidx.Config{Mode: uniqidx.ModeOffline})
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("POST /v1/ids", func(w http.ResponseWriter, r *http.Request) {
		id, err := gen.Next(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id.Text()})
	})
	http.HandleFunc("GET /v1/ids/{id}/validate", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"valid": uniqidx.ValidText(id)})
	})
	log.Println("example server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
