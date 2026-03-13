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

	http.HandleFunc("/user/cart/items/", h.UpdateOrRemoveItemFromCart)
	http.HandleFunc("/user/cart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.CreateUserCartAndAddItems(w, r)
		} else if r.Method == http.MethodGet {
			h.GetUserCart(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/user/orders", h.CreateOrderFromCart)

	http.HandleFunc("/items", h.GetItems)
	http.HandleFunc("/items/", h.GetItemByID)

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
