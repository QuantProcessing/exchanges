package execution

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidTransition = errors.New("invalid execution transition")
	ErrOverfill          = errors.New("execution overfill")
)

type PositionIDMode string

const (
	PositionIDModeNetting PositionIDMode = "netting"
	PositionIDModeHedging PositionIDMode = "hedging"
)

type Config struct {
	Cache          *cache.Cache
	AllowOverfills bool
	PositionIDMode PositionIDMode
}

type Manager struct {
	mu               sync.Mutex
	cache            *cache.Cache
	allowOverfills   bool
	positionIDMode   PositionIDMode
	submits          map[model.ClientOrderID]model.SubmitOrder
	heldChildren     map[parentOrderKey][]model.SubmitOrder
	orderListMembers map[orderListKey][]model.ClientOrderID
	orderListKinds   map[orderListKey]model.OrderListKind
	orderListFills   map[orderProgressKey]decimal.Decimal
	positionIDs      map[positionKey]model.PositionID
}

type OrderListStatus string

const (
	OrderListStatusInitialized OrderListStatus = "initialized"
	OrderListStatusOpen        OrderListStatus = "open"
	OrderListStatusClosed      OrderListStatus = "closed"
)

type OrderListActions struct {
	Submit []model.SubmitOrder
	Modify []model.ModifyOrder
	Cancel []model.CancelOrder
}

type OrderListSnapshot struct {
	AccountID     model.AccountID
	OrderListID   model.OrderListID
	Kind          model.OrderListKind
	Status        OrderListStatus
	MemberCount   int
	OpenCount     int
	TerminalCount int
	HeldCount     int
	Members       []model.ClientOrderID
	HeldChildren  []HeldOrderListChildren
	FillProgress  []OrderListFillProgress
	Orders        []model.OrderStatusReport
}

type HeldOrderListChildren struct {
	ParentClientOrderID model.ClientOrderID
	Orders              []model.SubmitOrder
}

type OrderListFillProgress struct {
	OrderID        model.OrderID
	FilledQuantity decimal.Decimal
}

type parentOrderKey struct {
	accountID     model.AccountID
	clientOrderID model.ClientOrderID
}

type orderListKey struct {
	accountID   model.AccountID
	orderListID model.OrderListID
}

type orderProgressKey struct {
	accountID model.AccountID
	orderID   model.OrderID
}

type positionKey struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	strategyID   model.StrategyID
}

func NewManager(cfg Config) *Manager {
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	mode := cfg.PositionIDMode
	if mode == "" {
		mode = PositionIDModeNetting
	}
	return &Manager{
		cache:            c,
		allowOverfills:   cfg.AllowOverfills,
		positionIDMode:   mode,
		submits:          make(map[model.ClientOrderID]model.SubmitOrder),
		heldChildren:     make(map[parentOrderKey][]model.SubmitOrder),
		orderListMembers: make(map[orderListKey][]model.ClientOrderID),
		orderListKinds:   make(map[orderListKey]model.OrderListKind),
		orderListFills:   make(map[orderProgressKey]decimal.Decimal),
		positionIDs:      make(map[positionKey]model.PositionID),
	}
}

func (m *Manager) CacheSubmitCommand(order model.SubmitOrder) error {
	if err := order.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submits[order.ClientOrderID] = order
	return nil
}

func (m *Manager) SubmitCommand(clientOrderID model.ClientOrderID) (model.SubmitOrder, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.submits[clientOrderID]
	return order, ok
}

func (m *Manager) PopSubmitCommand(clientOrderID model.ClientOrderID) (model.SubmitOrder, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.submits[clientOrderID]
	if ok {
		delete(m.submits, clientOrderID)
	}
	return order, ok
}

func (m *Manager) ApplyOrderReport(report model.OrderStatusReport) error {
	if err := report.Validate(); err != nil {
		return err
	}
	if existing, ok := m.cache.Order(report.AccountID, report.OrderID); ok {
		if err := validateOrderTransition(existing, report); err != nil {
			return err
		}
	}
	if err := m.cache.PutOrder(report); err != nil {
		return err
	}
	return m.replayDeferredFills(report.AccountID, report.OrderID)
}

func (m *Manager) ApplyFill(fill model.FillReport) (bool, error) {
	if err := fill.Validate(); err != nil {
		return false, err
	}
	if _, ok := m.cache.FillByTradeID(fill.AccountID, fill.TradeID); ok {
		return false, nil
	}
	order, ok := m.cache.Order(fill.AccountID, fill.OrderID)
	if !ok && fill.ClientOrderID != "" {
		order, ok = m.cache.OrderByClientID(fill.AccountID, fill.ClientOrderID)
	}
	if !ok && fill.IsLegFill() {
		if fill.PositionID == "" {
			fill.PositionID = m.determineLegPositionID(fill)
		}
		return m.cache.PutFill(fill)
	}
	if !ok {
		_, err := m.cache.PutDeferredFill(fill)
		return false, err
	}
	nextFilled := order.FilledQuantity.Add(fill.Quantity)
	if order.Quantity.IsPositive() && nextFilled.GreaterThan(order.Quantity) && !m.allowOverfills {
		return false, fmt.Errorf("%w: filled %s exceeds quantity %s", ErrOverfill, nextFilled, order.Quantity)
	}
	applied, err := m.cache.PutFill(fill)
	if err != nil || !applied {
		return applied, err
	}
	updated := order
	updated.FilledQuantity = nextFilled
	if order.Quantity.IsPositive() {
		updated.LeavesQuantity = order.Quantity.Sub(nextFilled)
		if updated.LeavesQuantity.IsNegative() {
			updated.LeavesQuantity = decimal.Zero
		}
	}
	updated.AveragePrice = averageFillPrice(order, fill, nextFilled)
	if updated.LeavesQuantity.IsZero() {
		updated.Status = model.OrderStatusFilled
	} else {
		updated.Status = model.OrderStatusPartiallyFilled
	}
	updated.LastUpdatedTime = fill.Timestamp
	return true, m.cache.PutOrder(updated)
}

func (m *Manager) replayDeferredFills(accountID model.AccountID, orderID model.OrderID) error {
	fills := m.cache.DeferredFillsForOrder(accountID, orderID)
	if len(fills) == 0 {
		return nil
	}
	for _, fill := range fills {
		if _, err := m.ApplyFill(fill); err != nil {
			return err
		}
	}
	m.cache.ClearDeferredFillsForOrder(accountID, orderID)
	return nil
}

func (m *Manager) DeterminePositionID(accountID model.AccountID, instrumentID model.InstrumentID, strategyID model.StrategyID) model.PositionID {
	if m.positionIDMode != PositionIDModeHedging {
		return model.PositionID(instrumentID.String())
	}
	key := positionKey{accountID: accountID, instrumentID: instrumentID, strategyID: strategyID}
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.positionIDs[key]; ok {
		return id
	}
	id := model.PositionID(fmt.Sprintf("%s:%s", instrumentID, strategyID))
	m.positionIDs[key] = id
	return id
}

func (m *Manager) determineLegPositionID(fill model.FillReport) model.PositionID {
	if fill.ClientOrderID != "" {
		return model.PositionID(fmt.Sprintf("%s:%s", fill.InstrumentID, fill.ClientOrderID))
	}
	if fill.VenueOrderID != "" {
		return model.PositionID(fmt.Sprintf("%s:%s", fill.InstrumentID, fill.VenueOrderID))
	}
	return model.PositionID(fmt.Sprintf("%s:%s", fill.InstrumentID, fill.TradeID))
}

func (m *Manager) IndexOrderList(list model.OrderList) error {
	if err := list.Validate(); err != nil {
		return err
	}
	kind := list.Kind()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, order := range list.Orders {
		key := orderListKey{accountID: order.AccountID, orderListID: order.OrderListID}
		m.orderListMembers[key] = append(m.orderListMembers[key], order.ClientOrderID)
		m.orderListKinds[key] = kind
		if order.ParentClientOrderID == "" {
			continue
		}
		parent := parentOrderKey{accountID: order.AccountID, clientOrderID: order.ParentClientOrderID}
		m.heldChildren[parent] = append(m.heldChildren[parent], order)
	}
	return nil
}

func (m *Manager) OrderListSnapshot(accountID model.AccountID, orderListID model.OrderListID) (OrderListSnapshot, bool) {
	m.mu.Lock()
	listKey := orderListKey{accountID: accountID, orderListID: orderListID}
	members, ok := m.orderListMembers[listKey]
	if !ok {
		m.mu.Unlock()
		return OrderListSnapshot{}, false
	}
	members = append([]model.ClientOrderID(nil), members...)
	kind := m.orderListKinds[listKey]
	heldByParent := make(map[model.ClientOrderID][]model.SubmitOrder)
	heldCount := 0
	for key, children := range m.heldChildren {
		if key.accountID != accountID {
			continue
		}
		for _, child := range children {
			if child.OrderListID != orderListID {
				continue
			}
			heldByParent[key.clientOrderID] = append(heldByParent[key.clientOrderID], child)
			heldCount++
		}
	}
	fillProgress := make(map[model.OrderID]decimal.Decimal)
	for key, filled := range m.orderListFills {
		if key.accountID == accountID {
			fillProgress[key.orderID] = filled
		}
	}
	m.mu.Unlock()

	orders := m.cache.OrdersByOrderListID(accountID, orderListID)
	orderIDs := make(map[model.OrderID]struct{}, len(orders))
	openCount := 0
	terminalCount := 0
	for _, order := range orders {
		orderIDs[order.OrderID] = struct{}{}
		if order.Status.IsTerminal() {
			terminalCount++
		} else if order.Status.IsOpen() {
			openCount++
		}
	}
	snapshot := OrderListSnapshot{
		AccountID:     accountID,
		OrderListID:   orderListID,
		Kind:          kind,
		Status:        deriveOrderListStatus(openCount, terminalCount, heldCount),
		MemberCount:   len(members),
		OpenCount:     openCount,
		TerminalCount: terminalCount,
		HeldCount:     heldCount,
		Members:       members,
		Orders:        orders,
	}
	for parentID, children := range heldByParent {
		children = append([]model.SubmitOrder(nil), children...)
		sort.Slice(children, func(i, j int) bool {
			return children[i].ClientOrderID < children[j].ClientOrderID
		})
		snapshot.HeldChildren = append(snapshot.HeldChildren, HeldOrderListChildren{
			ParentClientOrderID: parentID,
			Orders:              children,
		})
	}
	sort.Slice(snapshot.HeldChildren, func(i, j int) bool {
		return snapshot.HeldChildren[i].ParentClientOrderID < snapshot.HeldChildren[j].ParentClientOrderID
	})
	for orderID, filled := range fillProgress {
		if _, ok := orderIDs[orderID]; !ok {
			continue
		}
		snapshot.FillProgress = append(snapshot.FillProgress, OrderListFillProgress{
			OrderID:        orderID,
			FilledQuantity: filled,
		})
	}
	sort.Slice(snapshot.FillProgress, func(i, j int) bool {
		return snapshot.FillProgress[i].OrderID < snapshot.FillProgress[j].OrderID
	})
	return snapshot, true
}

func (m *Manager) OrderListSnapshots(accountID model.AccountID) []OrderListSnapshot {
	m.mu.Lock()
	listIDs := make([]model.OrderListID, 0)
	seen := make(map[model.OrderListID]struct{})
	for key := range m.orderListMembers {
		if key.accountID != accountID {
			continue
		}
		if _, ok := seen[key.orderListID]; ok {
			continue
		}
		seen[key.orderListID] = struct{}{}
		listIDs = append(listIDs, key.orderListID)
	}
	m.mu.Unlock()
	sort.Slice(listIDs, func(i, j int) bool { return listIDs[i] < listIDs[j] })
	snapshots := make([]OrderListSnapshot, 0, len(listIDs))
	for _, listID := range listIDs {
		snapshot, ok := m.OrderListSnapshot(accountID, listID)
		if ok {
			snapshots = append(snapshots, snapshot)
		}
	}
	return snapshots
}

func (m *Manager) HandleOrderListProgress(order model.OrderStatusReport) (OrderListActions, error) {
	if err := order.Validate(); err != nil {
		return OrderListActions{}, err
	}
	if order.OrderListID != "" {
		defer m.cleanupClosedOrderList(order.AccountID, order.OrderListID)
	}
	m.mu.Lock()
	members := append([]model.ClientOrderID(nil), m.orderListMembers[orderListKey{
		accountID:   order.AccountID,
		orderListID: order.OrderListID,
	}]...)
	if order.Status.IsTerminal() && order.Status != model.OrderStatusFilled {
		delete(m.heldChildren, parentOrderKey{
			accountID:     order.AccountID,
			clientOrderID: order.ClientOrderID,
		})
	}
	m.mu.Unlock()

	actions := OrderListActions{}
	if order.OrderListID != "" && order.Contingency == model.ContingencyTypeOUO {
		actions = m.handleOuoProgress(order, members)
	}
	if order.Status != model.OrderStatusFilled {
		return actions, nil
	}
	m.mu.Lock()
	children := append([]model.SubmitOrder(nil), m.heldChildren[parentOrderKey{
		accountID:     order.AccountID,
		clientOrderID: order.ClientOrderID,
	}]...)
	delete(m.heldChildren, parentOrderKey{accountID: order.AccountID, clientOrderID: order.ClientOrderID})
	m.mu.Unlock()
	actions.Submit = append(actions.Submit, children...)
	if order.OrderListID == "" || order.Contingency != model.ContingencyTypeOCO {
		return actions, nil
	}
	for _, clientOrderID := range members {
		if clientOrderID == order.ClientOrderID {
			continue
		}
		sibling, ok := m.cache.OrderByClientID(order.AccountID, clientOrderID)
		if !ok || !sibling.Status.IsOpen() {
			continue
		}
		actions.Cancel = append(actions.Cancel, model.CancelOrder{
			AccountID:     sibling.AccountID,
			InstrumentID:  sibling.InstrumentID,
			ClientOrderID: sibling.ClientOrderID,
			OrderID:       sibling.OrderID,
		})
	}
	return actions, nil
}

func (m *Manager) handleOuoProgress(order model.OrderStatusReport, members []model.ClientOrderID) OrderListActions {
	if !order.FilledQuantity.IsPositive() {
		return OrderListActions{}
	}
	key := orderProgressKey{accountID: order.AccountID, orderID: order.OrderID}
	m.mu.Lock()
	previous := m.orderListFills[key]
	if order.FilledQuantity.LessThanOrEqual(previous) {
		m.mu.Unlock()
		return OrderListActions{}
	}
	fillDelta := order.FilledQuantity.Sub(previous)
	m.orderListFills[key] = order.FilledQuantity
	m.mu.Unlock()

	actions := OrderListActions{}
	for _, clientOrderID := range members {
		if clientOrderID == "" || clientOrderID == order.ClientOrderID {
			continue
		}
		sibling, ok := m.cache.OrderByClientID(order.AccountID, clientOrderID)
		if !ok || !sibling.Status.IsOpen() {
			continue
		}
		nextQuantity := sibling.Quantity.Sub(fillDelta)
		if !nextQuantity.IsPositive() || nextQuantity.LessThanOrEqual(sibling.FilledQuantity) {
			actions.Cancel = append(actions.Cancel, model.CancelOrder{
				AccountID:     sibling.AccountID,
				InstrumentID:  sibling.InstrumentID,
				ClientOrderID: sibling.ClientOrderID,
				OrderID:       sibling.OrderID,
			})
			continue
		}
		if nextQuantity.Equal(sibling.Quantity) {
			continue
		}
		actions.Modify = append(actions.Modify, model.ModifyOrder{
			AccountID:     sibling.AccountID,
			InstrumentID:  sibling.InstrumentID,
			ClientOrderID: sibling.ClientOrderID,
			OrderID:       sibling.OrderID,
			Quantity:      nextQuantity,
		})
	}
	return actions
}

func deriveOrderListStatus(openCount int, terminalCount int, heldCount int) OrderListStatus {
	if openCount > 0 || heldCount > 0 {
		return OrderListStatusOpen
	}
	if terminalCount > 0 {
		return OrderListStatusClosed
	}
	return OrderListStatusInitialized
}

func (m *Manager) cleanupClosedOrderList(accountID model.AccountID, orderListID model.OrderListID) {
	if m == nil || orderListID == "" {
		return
	}
	snapshot, ok := m.OrderListSnapshot(accountID, orderListID)
	if !ok || snapshot.Status != OrderListStatusClosed {
		return
	}
	orderIDs := make(map[model.OrderID]struct{}, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		orderIDs[order.OrderID] = struct{}{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, children := range m.heldChildren {
		if key.accountID != accountID {
			continue
		}
		kept := children[:0]
		for _, child := range children {
			if child.OrderListID != orderListID {
				kept = append(kept, child)
			}
		}
		if len(kept) == 0 {
			delete(m.heldChildren, key)
		} else {
			m.heldChildren[key] = kept
		}
	}
	for key := range m.orderListFills {
		if key.accountID != accountID {
			continue
		}
		if _, ok := orderIDs[key.orderID]; ok {
			delete(m.orderListFills, key)
		}
	}
}

func validateOrderTransition(existing model.OrderStatusReport, next model.OrderStatusReport) error {
	if existing.Status.IsTerminal() && existing.Status != next.Status {
		return fmt.Errorf("%w: cannot transition order %s from %s to %s", ErrInvalidTransition, existing.OrderID, existing.Status, next.Status)
	}
	return nil
}

func averageFillPrice(order model.OrderStatusReport, fill model.FillReport, nextFilled decimal.Decimal) decimal.Decimal {
	if nextFilled.IsZero() {
		return decimal.Zero
	}
	if order.FilledQuantity.IsZero() || order.AveragePrice.IsZero() {
		return fill.Price
	}
	previousNotional := order.AveragePrice.Mul(order.FilledQuantity)
	fillNotional := fill.Price.Mul(fill.Quantity)
	return previousNotional.Add(fillNotional).Div(nextFilled)
}
