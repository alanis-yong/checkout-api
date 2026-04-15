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
	GetUserOrders(ctx context.Context, userID int, limit int, cursor string) ([]models.Order, string, error)
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
	NextCursor string          `json:"next_cursor"`
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			writeJSON(w, http.StatusUnauthorized, ErrorMessageResponse{Message: "unauthorized"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		var claims jwt.RegisteredClaims
		_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
			return []byte(SigningSecret), nil
		})

		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ErrorMessageResponse{Message: "invalid token"})
			return
		}

		userID, _ := strconv.Atoi(claims.Subject)

		// Inject the REAL ID into the request context
		ctx := context.WithValue(r.Context(), "userID", int(userID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

// UpsertCartItem adds or updates an item in the cart
// @Summary      Add or Update Cart Item
// @Description  Adds an item to the user's cart or updates the quantity
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Param        item_id  path      int                           true  "Item ID"
// @Param        body     body      handlers.UpsertCartItemRequest true  "Quantity"
// @Success      204      {object}  nil
// @Failure      400      {object}  handlers.ErrorMessageResponse
// @Security     Bearer
// @Router       /user/cart/items/{item_id} [patch]
func (h *Handler) UpsertCartItem(w http.ResponseWriter, r *http.Request) {
	// 1. Get the authenticated UserID from the request context.
	// This value is injected by the AuthMiddleware.
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorMessageResponse{Message: "unauthorized"})
		return
	}

	// 2. Extract and validate the item_id from the URL path.
	itemIDStr := r.PathValue("item_id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorMessageResponse{Message: "item_id must be integer"})
		return
	}

	// 3. Decode the incoming JSON body.
	var req UpsertCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: "invalid request body"})
		return
	}

	// 4. Validate the request structure (e.g., quantity > 0) using the validator tags.
	if err := validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{
			Message: "Validation failed: quantity must be greater than 0",
		})
		return
	}

	// 5. Verify the item actually exists in your catalog before adding it to a cart.
	item, err := h.store.GetItem(r.Context(), itemID)
	if err != nil || item == nil {
		writeJSON(w, http.StatusNotFound, ErrorMessageResponse{Message: "item not found"})
		return
	}

	// 6. Fetch the user's current cart. If it doesn't exist, initialize a new one.
	c, err := h.store.GetUserCart(r.Context(), userID)
	if err != nil {
		// If the error is simply "not found", we create a new cart aggregate.
		c = cart.New(userID)
	}

	// 7. Add the item to the cart aggregate.
	// This handles the logic of "Add new" vs "Update existing quantity".
	if err := c.AddItem(itemID, req.Quantity, item.Price); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: err.Error()})
		return
	}

	// 8. Persist the updated cart back to the database.
	if err := h.store.SaveCart(r.Context(), c); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorMessageResponse{Message: "failed to save cart"})
		return
	}

	// 9. Return 204 No Content on success.
	w.WriteHeader(http.StatusNoContent)
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

// @Summary Get User Cart
// @Description Fetches the cart for the currently authenticated user.
// @Tags Cart
// @Produce json
// @Security Bearer
// @Success 200 {object} models.Cart
// @Failure 404 {object} APIError "Cart not found"
// @Router /user/cart [get]
func (h *Handler) GetUserCart(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	userID, ok := val.(int)

	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorMessageResponse{Message: "unauthorized"})
		return
	}

	c, err := h.store.GetUserCart(r.Context(), userID)
	if err != nil {
		status := h.mapErrorToStatus(err)
		writeJSON(w, status, ErrorMessageResponse{Message: err.Error()})
		return
	}

	// --- ADD THIS CHECK HERE ---
	// If the cart exists but has 0 items, treat it as "Not Found"
	if len(c.Items()) == 0 {
		writeJSON(w, http.StatusNotFound, ErrorMessageResponse{Message: "cart not found"})
		return
	}
	// ---------------------------

	response := CartResponse{
		UserID: c.UserID(),
		Items:  mapLineItemsToResponse(c.Items()),
	}

	writeJSON(w, http.StatusOK, response)
}

// @Summary Create Order
// @Description Converts cart items into a permanent order. Returns 402 if mock payment fails.
// @Tags Orders
// @Accept json
// @Produce json
// @Security Bearer
// @Param Idempotency-Key header string true "Idempotency Key"
// @Param body body CreateOrderRequest true "Order Details"
// @Success 201 {object} map[string]interface{} "Order Created Successfully"
// @Failure 400 {object} ErrorMessageResponse "Bad Request"
// @Failure 402 {object} ErrorMessageResponse "Payment Failed"
// @Failure 500 {object} ErrorMessageResponse "Internal Server Error"
// @Router /orders [post]
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Identify the user
	val := r.Context().Value("userID")
	userID, ok := val.(int)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorMessageResponse{Message: "unauthorized"})
		return
	}

	// 2. Idempotency: Ensure we don't process the same request twice
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: "Idempotency-Key header is required"})
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

	// 3. Decode Request: Parse the incoming JSON
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: "invalid request body"})
		return
	}

	// --- STEP 4: THE GATEKEEPER (MOCK PAYMENT) ---
	// We check payment BEFORE we touch the database.
	paymentResult := mockProcessPayment(req.Total)

	if !paymentResult.Success {
		// Use your custom error variable
		errToMap := cart.ErrPaymentFailed

		// Use your mapping function to get the 402 Status code
		statusCode := h.mapErrorToStatus(errToMap)

		response := ErrorMessageResponse{Message: errToMap.Error()}
		responseBody, _ := json.Marshal(response)

		// Cache the failure so a retry returns the same 402
		h.saveToIdempotency(idempotencyKey, statusCode, responseBody)

		writeJSON(w, statusCode, response)
		return // STOP: The database is never reached
	}

	// --- STEP 5: VALIDATION & MAPPING ---
	if len(req.LineItems) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorMessageResponse{Message: "items must not be empty"})
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

	// --- STEP 6: DATABASE PERSISTENCE ---
	// Payment passed, so we save the order as "paid"
	order, err := h.store.CreateOrder(r.Context(), userID, items, req.Total, "paid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorMessageResponse{Message: err.Error()})
		return
	}

	// --- STEP 7: SUCCESS RESPONSE ---
	responseData := map[string]any{
		"order":   order,
		"payment": paymentResult,
	}

	responseBody, _ := json.Marshal(responseData)
	h.saveToIdempotency(idempotencyKey, http.StatusCreated, responseBody)

	writeJSON(w, http.StatusCreated, responseData)
}

func (h *Handler) saveToIdempotency(key string, status int, body []byte) {
	h.idempotencyCache[key] = &IdempotencyRecord{
		Response:   body,
		StatusCode: status,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
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

// GetUserOrders returns a paginated list of orders for a user.
// @Summary      Get user orders
// @Description  Get a list of orders for a specific user with cursor pagination
// @Tags         Orders
// @Produce      json
// @Param        id      path      int     true  "User ID"
// @Param        limit   query     int     false "Limit"
// @Param        cursor  query     string  false "Cursor"
// @Success      200     {object}  OrdersResponse
// @Failure      400     {object}  ErrorMessageResponse
// @Router       /users/{id}/orders [get]
func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	// 1. Get UserID from path
	idStr := r.PathValue("id")
	userID, _ := strconv.Atoi(idStr)

	// 2. Parse pagination params
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}
	cursor := r.URL.Query().Get("cursor")

	// 3. Fetch data from store — ASK FOR limit + 1
	// We want to find 3 items so we can prove a 3rd exists, even if we only show 2.
	orders, _, err := h.store.GetUserOrders(r.Context(), userID, limit+1, cursor)
	if err != nil {
		status := h.mapErrorToStatus(err)
		writeJSON(w, status, ErrorMessageResponse{Message: err.Error()})
		return
	}

	// 4. Logic to determine the "Fact" of the next page
	finalNextCursor := ""
	if len(orders) > limit {
		// FACT: We found 3 items, so there is definitely a next page.

		// We set the cursor to the ID of the 2nd item (limit-1)
		// This tells the frontend: "Next time, start looking for IDs smaller than this one."
		lastVisibleItem := orders[limit-1]
		finalNextCursor = strconv.Itoa(lastVisibleItem.ID)

		// IMPORTANT: Chop off the 3rd item so the user only sees 2!
		orders = orders[:limit]
	} else {
		// FACT: We found 2 or fewer items. No next page exists.
		finalNextCursor = ""
	}

	// 5. Map to response
	response := OrdersResponse{
		Orders:     mapOrdersToResponse(orders),
		NextCursor: finalNextCursor,
	}

	writeJSON(w, http.StatusOK, response)
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

func mapLineItemsToResponse(items []cart.LineItem) []CartItemResponse {
	res := make([]CartItemResponse, 0, len(items))
	for _, item := range items {
		res = append(res, CartItemResponse{
			ID:       item.ItemID(),
			Price:    item.Price(),
			Quantity: item.Quantity(),
			// Note: Name, Description, and Stock would need to be
			// fetched from the ItemStore if you want them in the response!
		})
	}
	return res
}

func (h *Handler) mapErrorToStatus(err error) int {
	switch {
	// 404: If the cart isn't in the DB
	case errors.Is(err, cart.ErrCartNotFound):
		return http.StatusNotFound

	// 402: If the payment failed during checkout
	// Adjust the package name (cart.) if you moved this to an orders package
	case errors.Is(err, cart.ErrPaymentFailed):
		return http.StatusPaymentRequired

	// 400: Optional - for validation or bad input
	case errors.Is(err, cart.ErrInvalidQuantity):
		return http.StatusBadRequest

	// 500: The "catch-all" for things you didn't expect
	default:
		return http.StatusInternalServerError
	}
}
