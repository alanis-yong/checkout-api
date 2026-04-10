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

	"checkout-api/internal/cart"
	"checkout-api/models"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
)

var validate = validator.New()

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
	DeleteUserCart(ctx context.Context, userID int) error
	SaveUser(ctx context.Context, email string, hash []byte) error
	FindUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserCart(ctx context.Context, userID int) (*cart.Cart, error)
	SaveCart(ctx context.Context, c *cart.Cart) error
	GetUserOrders(ctx context.Context, userID int, cursor int, limit int) ([]models.Order, error)
	RemoveCartItem(ctx context.Context, userID int, itemID int) error
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

type OrdersResponse struct {
	Orders     []OrderResponse `json:"orders"`
	NextCursor string          `json:"next_cursor,omitempty"`
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

// @Summary Add or Update Cart Item
// @Description Adds an item to the user's cart or updates the quantity if it exists
// @Tags Cart
// @Accept json
// @Produce json
// @Security Bearer
// @Param item_id path int true "Item ID"
// @Param body body UpsertCartItemRequest true "Quantity"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorMessageResponse
// @Router /cart/{item_id} [put]
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
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{Message: "item_id must be integer"})
		return
	}

	var req UpsertCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{
			Message: "Validation failed: quantity must be greater than 0",
		})
		return
	}

	item, err := h.store.GetItem(r.Context(), itemID)
	if err != nil || item == nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	c, err := h.store.GetUserCart(r.Context(), userID)
	if err != nil {
		c = cart.New(userID)
	}

	if err := c.AddItem(itemID, req.Quantity, item.Price); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: err.Error()})
		return
	}

	// 5. PERSIST: Save the whole Aggregate back
	if err := h.store.SaveCart(r.Context(), c); err != nil {
		http.Error(w, "failed to save cart", http.StatusInternalServerError)
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

// @Summary Create Order
// @Description Converts cart items into a permanent order
// @Tags Orders
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Idempotency Key"
// @Param body body CreateOrderRequest true "Order Details"
// @Success 201 {object} models.Order
// @Failure 402 {object} map[string]interface{} "Payment Required"
// @Router /orders [post]
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

// @Summary Get all items
// @Description Get a list of all products in the store
// @Tags Items
// @Produce json
// @Success 200 {array} models.Item
// @Router /items [get]
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

// @Summary Login
// @Description Authenticate user and return JWT
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body AuthRequest true "Credentials"
// @Success 200 {object} AuthResponse
// @Failure 401 "Unauthorized"
// @Router /login [post]
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

func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	// 1. Get userID from path
	userIDStr := r.PathValue("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// 2. Parse pagination parameters from Query String (?limit=10&cursor=50)
	limitStr := r.URL.Query().Get("limit")
	cursorStr := r.URL.Query().Get("cursor")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10 // Default page size
	}

	// If cursor is empty (first page), we use 0 or a very large number
	// depending on if you sort ASC or DESC.
	cursor, _ := strconv.Atoi(cursorStr)

	// 3. Fetch data from the Store
	orders, err := h.store.GetUserOrders(r.Context(), userID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to fetch orders", http.StatusInternalServerError)
		return
	}

	// 4. Determine the "Next Cursor"
	// If we got items back, the cursor for the NEXT page is the ID of the LAST item.
	var nextCursor string
	if len(orders) > 0 {
		lastOrder := orders[len(orders)-1]
		nextCursor = strconv.Itoa(lastOrder.ID)
	}

	// 5. Build the Response
	// (Assuming you have a helper to map models.Order to OrderResponse)
	resp := OrdersResponse{
		Orders:     mapOrdersToResponse(orders),
		NextCursor: nextCursor,
	}

	writeJSON(w, http.StatusOK, resp)
}

func mapOrdersToResponse(orders []models.Order) []OrderResponse {
	out := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, OrderResponse{
			ID:     o.ID,
			UserID: o.UserID,
			Total:  o.Total,
			Status: o.Status,
		})
	}
	return out
}
