package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"checkout-api/handlers"
	"checkout-api/store"

	_ "checkout-api/docs"

	"github.com/jackc/pgx/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	ctx := context.Background()
	// Todo Bonus: this looks dangerous maybe you can save it in a .env file
	// then add it to .gitignore so that your secrets are not pushed to the server
	// try https://github.com/spf13/viper
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres:postgres@localhost:5432/postgres"
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		panic(err)
	}

	err = conn.Ping(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("successfully connected to db")

	// s := store.NewInMemStore()
	postgresStore := store.NewPostgresStore(conn)
	h := handlers.NewHandler(postgresStore)

	// cart
	http.HandleFunc("GET /user/cart", h.GetUserCart)
	http.HandleFunc("PATCH /user/cart/items/{item_id}", h.UpsertCartItem)
	http.HandleFunc("DELETE /user/cart/items/{item_id}", h.RemoveCartItem)

	// orders
	http.HandleFunc("POST /orders", h.CreateOrder)

	// items
	http.HandleFunc("GET /items", h.GetItems)
	http.HandleFunc("GET /items/{item_id}", h.GetItemByID)

	// users
	http.HandleFunc("POST /signup", h.CreateUser)
	http.HandleFunc("POST /login", h.LoginUser)

	// TODO: implement Get RefreshToken
	http.HandleFunc("GET /token", h.IssueJWT)

	// health
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := conn.Ping(r.Context()); err != nil {
			dbStatus = "unreachable"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	// @title Checkout API
	// @version 1.0
	// @description This is the backend for Alanis' Store.
	// @host localhost:8080
	// @BasePath /
	http.HandleFunc("GET /swagger/{any...}", httpSwagger.WrapHandler)

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
