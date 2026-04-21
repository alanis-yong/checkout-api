package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"checkout-api/handlers"
	"checkout-api/store"

	_ "github.com/lib/pq"
)

func main() {
	// 1. Database connection using your Docker settings
	// Note: 'host' matches the service name in your docker-compose.yml
	dsn := "host=xsolla-gamestore-db user=alanis password=testing12345 dbname=gamestore_db port=5432 sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}
	defer db.Close()

	// 2. Initialize the store
	queries := store.New(db)

	// 3. --- SEED THE DATABASE ---
	// This reads your virtual-items.json and syncs it to Postgres at startup
	fmt.Println("🔄 Syncing database with virtual-items.json...")
	if err := queries.SeedDatabase(context.Background()); err != nil {
		// We log the error but don't stop the server, just in case
		// the file is missing but the DB already has data.
		log.Printf("⚠️ Seeding warning: %v", err)
	}

	// 4. Initialize Handler with your Xsolla credentials
	h := &handlers.Handler{
		MerchantID: "879363",
		APIKey:     "080ded9939889e5b8c567ae039cb026fedd4ecf8",
		ProjectID:  304862,
		Store:      queries,
	}

	// 5. Setup Routes
	mux := http.NewServeMux()

	// Storefront routes
	mux.HandleFunc("GET /api/products", h.GetProducts)
	mux.HandleFunc("GET /api/inventory", h.GetInventory)

	// Payment & Webhook routes
	mux.HandleFunc("POST /api/payments/token", h.GetXsollaToken)
	mux.HandleFunc("POST /api/webhooks/xsolla", h.HandleXsollaWebhook)

	// 6. Wrap with CORS for your Vite frontend (localhost:5173)
	finalHandler := enableCORS(mux)

	fmt.Println("🚀 Server starting on :8080")
	if err := http.ListenAndServe(":8080", finalHandler); err != nil {
		log.Fatal(err)
	}
}

// enableCORS handles Cross-Origin Resource Sharing for the React frontend
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
