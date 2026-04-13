package store

import (
	"checkout-api/internal/cart"
	"checkout-api/models"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
)

// PostgresStore is an in-memory store for items and orders.
type PostgresStore struct {
	conn *pgx.Conn
	mu   sync.Mutex
}

// NewPostgresStore creates a Store pre-loaded with seed data.
func NewPostgresStore(conn *pgx.Conn) *PostgresStore {
	s := &PostgresStore{
		conn: conn,
	}
	return s
}

func (s *PostgresStore) DB() *Query {
	return &Query{
		DBTX: s.conn,
	}
}

func (s *PostgresStore) WithTx(tx pgx.Tx) *Query {
	return &Query{
		DBTX: tx,
	}
}

func (s *PostgresStore) GetItems(ctx context.Context) ([]*models.Item, error) {
	rows, err := s.DB().GetItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to select all items", err)
	}
	defer rows.Close()

	items := make([]*models.Item, 0)
	for rows.Next() {
		var item models.Item
		err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Price, &item.Stock, &item.CreatedAt)
		if err != nil {
			fmt.Printf("unable to scan row: %v", err)
			return nil, fmt.Errorf("unable to scan row: %w", err)
		}
		items = append(items, &item)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) GetItem(ctx context.Context, id int) (*models.Item, error) {
	var item models.Item
	err := s.DB().GetItemByID(ctx, id).Scan(&item.ID, &item.Name, &item.Description, &item.Price, &item.Stock, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *PostgresStore) CreateOrder(ctx context.Context, userID int, items []models.LineItem, total int, status string) (*models.Order, error) {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.WithTx(tx)

	// Pessimistic lock: acquire row-level lock on each item and verify sufficient stock.
	for _, lineItem := range items {
		var item models.Item
		err := q.GetItemByIDForUpdate(ctx, lineItem.ItemID).Scan(&item.ID, &item.Name, &item.Description, &item.Price, &item.Stock, &item.CreatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("item %d not found", lineItem.ItemID)
			}
			return nil, fmt.Errorf("failed to lock item %d: %w", lineItem.ItemID, err)
		}
		if item.Stock < lineItem.Quantity {
			return nil, fmt.Errorf("insufficient stock for item %d: have %d, need %d", lineItem.ItemID, item.Stock, lineItem.Quantity)
		}
	}

	var orderID int
	if err := q.InsertOrderReturning(ctx, userID, total, status).Scan(&orderID); err != nil {
		return nil, fmt.Errorf("failed to insert order: %w", err)
	}

	for _, lineItem := range items {
		if _, err := q.InsertLineItem(ctx, orderID, lineItem.ItemID, lineItem.Price, lineItem.Quantity); err != nil {
			return nil, fmt.Errorf("failed to insert line item for item %d: %w", lineItem.ItemID, err)
		}
		if _, err := q.DecrementItemStock(ctx, lineItem.ItemID, lineItem.Quantity); err != nil {
			return nil, fmt.Errorf("failed to decrement stock for item %d: %w", lineItem.ItemID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.Order{
		ID:     orderID,
		UserID: userID,
		Items:  items,
		Total:  total,
		Status: status,
	}, nil
}

func (s *PostgresStore) UpdateOrderStatus(ctx context.Context, orderID int, status string) error {
	_, err := s.DB().UpdateOrderStatus(ctx, orderID, status)
	return err
}

func (s *PostgresStore) UpsertCartItem(ctx context.Context, userID int, itemID int, quantity int) error {
	_, err := s.DB().UpsertCart(ctx, userID, itemID, quantity)
	return err
}

func (s *PostgresStore) GetUserCart(ctx context.Context, userID int) (*cart.Cart, error) {
	rows, err := s.DB().GetCartByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 1. Create a temporary struct to hold the row data
	type cartRow struct {
		itemID   int
		quantity int
	}
	var tempRows []cartRow

	// 2. Quickly scan everything and close the connection handle
	for rows.Next() {
		var r cartRow
		if err := rows.Scan(&r.itemID, &r.quantity); err != nil {
			rows.Close()
			return nil, err
		}
		tempRows = append(tempRows, r)
	}
	rows.Close() // <--- CRITICAL: Connection is now free!

	newCart := cart.New(userID)

	// 3. Now it is safe to call GetItem
	for _, r := range tempRows {
		item, err := s.GetItem(ctx, r.itemID)
		if err == nil && item != nil {
			newCart.AddItem(r.itemID, r.quantity, item.Price)
		}
	}

	return newCart, nil
}

func (s *PostgresStore) DeleteUserCart(ctx context.Context, userID int) error {
	_, err := s.DB().DeleteCartByUserID(ctx, userID)
	return err
}

func (s *PostgresStore) RemoveCartItem(ctx context.Context, userID int, itemID int) error {
	_, err := s.DB().DeleteItemFromUserCart(ctx, userID, itemID)
	return err
}

func (s *PostgresStore) SaveUser(ctx context.Context, email string, hash []byte) error {
	_, err := s.DB().InsertUser(ctx, email, hash)
	return err
}

func (s *PostgresStore) FindUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	row := s.DB().GetUserByEmail(ctx, email)
	err := row.Scan(&user.ID, &user.Email, &user.Hash)
	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (s *PostgresStore) GetUserOrders(ctx context.Context, userID int, limit int, cursor string) ([]models.Order, string, error) {
	var rows pgx.Rows
	var err error

	// 1. Determine which Query method to use
	if cursor == "" {
		rows, err = s.DB().GetOrdersByUserIDFirstPage(ctx, userID, limit)
	} else {
		cursorID, convErr := strconv.Atoi(cursor)
		if convErr != nil {
			return nil, "", fmt.Errorf("invalid cursor format: %w", convErr)
		}
		rows, err = s.DB().GetOrdersByUserIDWithCursor(ctx, userID, cursorID, limit)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	// 2. Scan the rows
	var orders []models.Order
	for rows.Next() {
		var o models.Order
		// Note: Matches the 4 columns in your Query methods: id, user_id, total, status
		err := rows.Scan(
			&o.ID,
			&o.UserID,
			&o.Total,
			&o.Status,
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to scan order row: %w", err)
		}
		orders = append(orders, o)
	}

	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("row iteration error: %w", err)
	}

	// 3. Determine the next_cursor
	var nextCursor string
	if len(orders) > 0 && len(orders) == limit {
		// Use the ID of the last element as the cursor for the next request
		nextCursor = strconv.Itoa(orders[len(orders)-1].ID)
	}

	return orders, nextCursor, nil
}

func (s *PostgresStore) SaveCart(ctx context.Context, c *cart.Cart) error {
	s.mu.Lock()         // Lock at the start
	defer s.mu.Unlock() // Unlock at the end
	// Check if connection is still alive
	if err := s.conn.Ping(ctx); err != nil {
		fmt.Printf("Connection lost: %v\n", err)
		return err
	}

	fmt.Printf("Saving cart for user %d with %d items\n", c.UserID(), len(c.Items()))

	for _, item := range c.Items() {
		commandTag, err := s.DB().UpsertCart(ctx, c.UserID(), item.ItemID(), item.Quantity())
		if err != nil {
			fmt.Printf("Upsert Error: %v\n", err)
			return err
		}

		// IMPORTANT: Check if the DB actually did something
		if commandTag.RowsAffected() == 0 {
			fmt.Printf("Warning: 0 rows affected for item %d. Check your table constraints!\n", item.ItemID())
		} else {
			fmt.Printf("Successfully saved item %d. Rows affected: %d\n", item.ItemID(), commandTag.RowsAffected())
		}
	}
	return nil
}
