package main

import (
	"fmt"
	"log"
	"net/http"

	"checkout-api/handlers"
	"checkout-api/store"
)

func main() {
	s := store.NewStore()
	h := handlers.NewHandler(s)

	http.HandleFunc("/items", h.GetItems)
	http.HandleFunc("/items/", h.GetItemByID)
	http.HandleFunc("/orders", h.CreateOrder)

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
