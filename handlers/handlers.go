package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"checkout-api/models"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt/v5"
)

// TODO Bonus: this looks dangerous maybe you can save it in a .env file
// then add it to .gitignore so that your secrets are not pushed to the server
// try https://github.com/spf13/viper
var SigningSecret string = "5298365169"

// ItemStore defines the data operations the handler needs.
type ItemStore interface {
	GetItems(ctx context.Context) ([]*models.Item, error)
	GetItem(ctx context.Context, id int) (*models.Item, error)
	CreateOrder(ctx context.Context, userID int, items []models.LineItem, total int, status string) (*models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int, status string) error
	UpsertCartItem(ctx context.Context, userID int, itemID int, quantity int) error
	GetUserCart(ctx context.Context, userID int) ([]models.Cart, error)
	DeleteUserCart(ctx context.Context, userID int) error
	RemoveCartItem(ctx context.Context, userID int, itemID int) error
	SaveUser(ctx context.Context, email string, hash []byte) error
	FindUserByEmail(ctx context.Context, email string) (models.User, error)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store            ItemStore
	idempotencyCache map[string]*IdempotencyRecord
}

// NewHandler creates a Handler with the given store.
func NewHandler(s ItemStore) *Handler {
	return &Handler{
		store:            s,
		idempotencyCache: make(map[string]*IdempotencyRecord),
	}
}

type IdempotencyRecord struct {
	Response   []byte
	StatusCode int
	Expiry     time.Time
}

// mockProcessPayment simulates a payment provider call.
func mockProcessPayment(amount int) PaymentResult {
	if amount > 0 && amount < 1000000 {
		return PaymentResult{
			Success:       true,
			TransactionID: fmt.Sprintf("txn_%d", time.Now().UnixNano()),
		}
	}
	return PaymentResult{
		Success: false,
		Error:   "Payment declined",
	}
}

func (h *Handler) UpsertCartItem(w http.ResponseWriter, r *http.Request) {
	// TODO: this looks like it can be extracted as a commonly used
	// Explore the middleware pattern in net/http and see if you can extract authentication logic
	// into it's own handler(middleware)

	// get authorization header
	authorizationHeaderStr := r.Header.Get("Authorization")
	if authorizationHeaderStr == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// BeArER xxxx.yyyy.zzzz
	scheme := "bearer "
	if len(authorizationHeaderStr) < len(scheme) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userScheme := authorizationHeaderStr[:len(scheme)] // BeArER
	if !strings.EqualFold(scheme, userScheme) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userJWT := authorizationHeaderStr[len(scheme):]
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(
		userJWT,
		&claims,
		func(t *jwt.Token) (any, error) {
			return []byte(SigningSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		fmt.Printf("failed to parse jwt %q", err.Error())
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(claims.Subject)
	if err != nil {
		fmt.Printf("failed to parse jwt %q", err.Error())
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// user is now authenticated

	itemIDStr := r.PathValue("item_id")
	if itemIDStr == "" {
		http.Error(w, "missing item_id", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
			Message: "item_id must be integer",
		})
		return
	}

	var req UpsertCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Quantity < 0 {
		http.Error(w, "quantity must be greater than 0", http.StatusBadRequest)
		return
	}

	if err := h.store.UpsertCartItem(r.Context(), userID, itemID, req.Quantity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	// TODO: Protect this method
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		http.Error(w, "missing X-User-ID header", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid X-User-ID header", http.StatusBadRequest)
		return
	}

	itemIDStr := r.PathValue("item_id")
	if itemIDStr == "" {
		http.Error(w, "missing item_id", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
			Message: "item_id must be integer",
		})
		return
	}

	if err := h.store.RemoveCartItem(r.Context(), userID, itemID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary User Cart
// @Description Users cart Object, User identification is by http header X-User-ID
// @Tags Cart
// @Produce json
// @Success 200 {object} models.Cart
// @Failure 400 {object} APIError
// @Router /user/cart [get]

func (h *Handler) GetUserCart(w http.ResponseWriter, r *http.Request) {
	// TODO: protect this method
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		http.Error(w, "missing X-User-ID header", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid X-User-ID header", http.StatusBadRequest)
		return
	}

	cart, err := h.store.GetUserCart(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// TODO: instead of returning Cart
	// return CartResponse instead
	// tip: refactor the GetUserCart method
	writeJSON(w, http.StatusOK, cart)
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	// TODO: protect this method
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		http.Error(w, "missing X-User-ID header", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid X-User-ID header", http.StatusBadRequest)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	if record, exists := h.idempotencyCache[idempotencyKey]; exists {
		if time.Now().Before(record.Expiry) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(record.StatusCode)
			w.Write(record.Response)
			return
		}
		delete(h.idempotencyCache, idempotencyKey)
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.LineItems) == 0 {
		http.Error(w, "items must not be empty", http.StatusBadRequest)
		return
	}

	items := make([]models.LineItem, 0, len(req.LineItems))
	for _, i := range req.LineItems {
		items = append(items, models.LineItem{
			ItemID:   i.ItemID,
			Quantity: i.Quantity,
			Price:    i.Price,
		})
	}

	order, err := h.store.CreateOrder(r.Context(), userID, items, req.Total, "pending")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paymentResult := mockProcessPayment(req.Total)

	status := "paid"
	if !paymentResult.Success {
		status = "failed"
	}

	if err := h.store.UpdateOrderStatus(r.Context(), order.ID, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	order.Status = status

	responseData := map[string]any{
		"order":   order,
		"payment": paymentResult,
	}

	statusCode := http.StatusCreated
	if !paymentResult.Success {
		statusCode = http.StatusPaymentRequired
	}

	responseBody, _ := json.Marshal(responseData)
	h.idempotencyCache[idempotencyKey] = &IdempotencyRecord{
		Response:   responseBody,
		StatusCode: statusCode,
		Expiry:     time.Now().Add(24 * time.Hour),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(responseBody)
}

// GetItems handles GET /items — returns all available items.
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.GetItems(r.Context())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, items)
	return
}

// GetItemByID handles GET /items/{id} — returns a single i`tem.
func (h *Handler) GetItemByID(w http.ResponseWriter, r *http.Request) {
	itemIDStr := r.PathValue("item_id")
	if itemIDStr == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
			Message: "item_id must be an integer",
		})
		return
	}

	item, err := h.store.GetItem(r.Context(), itemID)
	if err != nil {
		fmt.Printf("error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if item == nil {
		writeJSON(w, http.StatusNotFound, nil)
		return
	}

	writeJSON(w, http.StatusOK, item)
	return
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// validate email
	_, err = mail.ParseAddress(req.Email)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
			Message: "invalid email",
		})
		return
	}

	pwlen := len(req.Password)
	// validate password
	if pwlen < 12 || pwlen > 25 {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
			Message: "password is too short or too long",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = h.store.SaveUser(r.Context(), req.Email, hash)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, nil)
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	user, err := h.store.FindUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{
				Message: "user does not exist",
			})
			return
		}
		fmt.Printf("cannot query %q", err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword(user.Hash, []byte(req.Password))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// issue jwt
	fifteenAfter := time.Now().Add(15 * time.Minute)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(fifteenAfter),
		Subject:   strconv.Itoa(user.ID),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	signedString, err := token.SignedString([]byte(SigningSecret))
	if err != nil {
		fmt.Printf("cannot generate signed string %q", err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// store session(refresh token)
	// TODO: do it yourself
	// generate a random string(bonus: if you use a CSPRNG to generate a random sequence of bytes)
	// insert into refresh_tokens (token_value, is_active) values ("sOmERANdomlYGeNERATEDstRing", 1)

	writeJSON(w, http.StatusOK, AuthResponse{
		JWT:          signedString,
		RefreshToken: "sOmERANdomlYGeNERATEDstRing",
	})
}

func (h *Handler) IssueJWT(w http.ResponseWriter, r *http.Request) {
	// TODO: implement issueing of new JWT with refresh token
	// check if refresh_token exists in the db and still active
	// generate a new JWT
	// generate a new random string (bonus: if you use a CSPRNG to generate a random sequence of bytes) as refresh_token
	// save new refresh token in db
	// deactivate old refresh token

}

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
