package main

import (
	"checkout-api/handlers"
	"checkout-api/store"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system env")
	}

	merchantID := os.Getenv("XSOLLA_MERCHANT_ID")
	apiKey := os.Getenv("XSOLLA_API_KEY")

	// Get and convert the Project ID (since it's an int)
	projectIDStr := os.Getenv("XSOLLA_PROJECT_ID")
	projectID, _ := strconv.Atoi(projectIDStr)

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Connection string error:", err)
	}

	// Add this! If this fails, the server won't start with a broken DB
	if err := db.Ping(); err != nil {
		log.Fatal("Database is unreachable:", err)
	}

	h := &handlers.Handler{
		MerchantID:    merchantID,
		APIKey:        apiKey,
		ProjectID:     projectID,
		SigningSecret: os.Getenv("SIGNING_SECRET"),
		DB:            db,
		Store:         store.New(db),
	}
	// 5. Setup Routes
	mux := http.NewServeMux()

	// Example router setup
	mux.HandleFunc("GET /api/cart", h.GetCart)
	mux.HandleFunc("POST /api/cart/update", h.UpdateCartQuantity)
	mux.HandleFunc("DELETE /api/cart/clear", h.ClearCart)

	// Storefront routes
	mux.HandleFunc("GET /api/products", h.GetProducts)
	mux.HandleFunc("GET /api/inventory", h.GetInventory)

	// Payment & Webhook routes
	mux.HandleFunc("POST /api/payments/token", h.GetXsollaToken)
	mux.HandleFunc("POST /api/webhooks/xsolla", h.HandleXsollaWebhook)

	// 6. Wrap with CORS for your Vite frontend (localhost:5173)
	finalHandler := enableCORS(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Server starting on :%s\n", port)
	if err := http.ListenAndServe(":"+port, finalHandler); err != nil {
		log.Fatal(err)
	}
}

// enableCORS handles Cross-Origin Resource Sharing for the React frontend
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://xsolla-alanis-gamestore.vercel.app")
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
