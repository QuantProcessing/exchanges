#!/usr/bin/env python3
"""NautilusTrader-side version of the same order-book imbalance demo.

This file is intentionally a usage comparison artifact. It shows the strategy
shape a Nautilus user writes; wiring it into a BacktestEngine or TradingNode
follows the standard Nautilus examples under `.omx/references/nautilus_trader`.
"""

from __future__ import annotations

from decimal import Decimal

from nautilus_trader.config import StrategyConfig
from nautilus_trader.model.data import OrderBookDepth10
from nautilus_trader.model.enums import OrderSide
from nautilus_trader.model.enums import TimeInForce
from nautilus_trader.model.events import OrderAccepted
from nautilus_trader.model.events import OrderFilled
from nautilus_trader.model.identifiers import InstrumentId
from nautilus_trader.trading.strategy import Strategy


class ImbalanceConfig(StrategyConfig, frozen=True):
    instrument_id: InstrumentId
    trade_size: Decimal = Decimal("0.01")
    imbalance_ratio: Decimal = Decimal("2")


class ImbalanceStrategy(Strategy):
    def __init__(self, config: ImbalanceConfig) -> None:
        super().__init__(config)
        self.instrument = None
        self.submitted = False
        self.accepted = 0
        self.filled = 0

    def on_start(self) -> None:
        self.instrument = self.cache.instrument(self.config.instrument_id)
        if self.instrument is None:
            self.log.error(f"Instrument missing: {self.config.instrument_id}")
            self.stop()
            return

        self.subscribe_order_book_depth(
            instrument_id=self.config.instrument_id,
            depth=2,
        )

    def on_order_book_depth(self, depth: OrderBookDepth10) -> None:
        if self.submitted or len(depth.bids) == 0 or len(depth.asks) == 0:
            return

        bid = depth.bids[0]
        ask = depth.asks[0]
        bid_size = bid.size.as_decimal()
        ask_size = ask.size.as_decimal()
        if bid_size <= ask_size * self.config.imbalance_ratio:
            return

        self.submitted = True
        order = self.order_factory.limit(
            instrument_id=self.config.instrument_id,
            order_side=OrderSide.BUY,
            quantity=self.instrument.make_qty(self.config.trade_size),
            price=self.instrument.make_price(ask.price.as_decimal()),
            time_in_force=TimeInForce.GTC,
        )
        self.submit_order(order)

    def on_order_accepted(self, event: OrderAccepted) -> None:
        self.accepted += 1
        self.log.info(
            f"accepted client_order_id={event.client_order_id} venue_order_id={event.venue_order_id}",
        )

    def on_order_filled(self, event: OrderFilled) -> None:
        self.filled += 1
        self.log.info(
            f"filled client_order_id={event.client_order_id} qty={event.last_qty} price={event.last_px}",
        )

    def on_stop(self) -> None:
        self.unsubscribe_order_book_depth(self.config.instrument_id)
