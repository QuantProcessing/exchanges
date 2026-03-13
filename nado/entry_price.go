package nado

import (
	"slices"
	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/nado/sdk"

	"github.com/shopspring/decimal"
)

func (a *Adapter) GetNetEntryPriceAndRealizedPnL(productID int64, quantity float64, side exchanges.PositionSide, matchesResp *nado.ArchiveMatchesResponse) (float64, float64, error) {
	if matchesResp == nil {
		return 0, 0, nil
	}
	var subIdx []string
	TotalRealizedPnL := decimal.NewFromFloat(0)
	for _, match := range matchesResp.Matches {
		if match.PostBalance.Base.Perp.ProductID != productID {
			continue
		}

		amount := parseX18(match.PostBalance.Base.Perp.Balance.Amount)

		// 遇到持仓为0的时候，要是记录了开仓记录，要么就是卖光了
		if amount.IsZero() {
			break
		}

		// matched
		subIdx = slices.Insert(subIdx, 0, match.SubmissionIdx)

		// 计算收益
		realizedPnLF := parseX18(match.RealizedPnL)
		TotalRealizedPnL = TotalRealizedPnL.Add(realizedPnLF)

		// 找到初始的开仓记录就可以停了
		if match.PreBalance.Base.Perp.Balance.Amount == "0" {
			break
		}
	}
	realizedPnL, _ := TotalRealizedPnL.Float64()

	if len(subIdx) == 0 {
		return 0, 0, nil
	}

	entryPrice := decimal.NewFromFloat(0)
	totalAmount := decimal.NewFromFloat(0)
	totalSpend := decimal.NewFromFloat(0)

	for _, idx := range subIdx {
		for _, tx := range matchesResp.Txs {
			if tx.SubmissionIdx != idx {
				continue
			}
			// matched
			// 因为是onyway模式，所以不管买卖，其实都是吃单方，都是taker
			if tx.TxInfo.MatchOrders.Taker.Order.Sender != a.sender {
				continue
			}
			// 使用卖方的挂单价格
			price := parseX18(tx.TxInfo.MatchOrders.Maker.Order.PriceX18)
			// 吃单数量
			amount := parseX18(tx.TxInfo.MatchOrders.Taker.Order.Amount)

			amountDec := amount
			priceDec := price

			cmpZero := totalAmount.Cmp(decimal.NewFromFloat(0))
			// 暂未持仓
			if cmpZero == 0 {
				totalAmount = amountDec
				totalSpend = amountDec.Mul(priceDec)
				entryPrice = priceDec
			} else {
				// 判断方向: 如果 amount 与 totalAmount 同号，则是加仓
				isIncrease := false
				if totalAmount.Sign() > 0 && amount.IsPositive() {
					isIncrease = true
				} else if totalAmount.Sign() < 0 && amount.IsNegative() {
					isIncrease = true
				} else if totalAmount.Sign() == 0 { // Should be covered by cmpZero, but for safety
					isIncrease = true
				}

				if isIncrease {
					// 加仓: Cost 增加 (amount * price)
					totalSpend = totalSpend.Add(amountDec.Mul(priceDec))
					totalAmount = totalAmount.Add(amountDec)
					entryPrice = totalSpend.Div(totalAmount)
				} else {
					// 减仓: Cost 减少 (amount * entryPrice)
					// amount 是异号，所以 Add 相当于减去 value
					totalSpend = totalSpend.Add(amountDec.Mul(entryPrice))
					totalAmount = totalAmount.Add(amountDec)

					// 如果持仓归零
					if totalAmount.IsZero() {
						totalSpend = decimal.Zero
						entryPrice = decimal.Zero
					}
				}
			}

			break
		}
	}

	if totalAmount.Equal(decimal.NewFromFloat(0)) {
		return 0, 0, nil
	}
	ep, _ := entryPrice.Float64()
	return ep, realizedPnL, nil
}

func (a *SpotAdapter) GetNetEntryPriceAndRealizedPnL(productID int64, quantity float64, side exchanges.PositionSide, matchesResp *nado.ArchiveMatchesResponse) (float64, float64, error) {
	if matchesResp == nil {
		return 0, 0, nil
	}
	var subIdx []string
	TotalRealizedPnL := decimal.NewFromFloat(0)
	for _, match := range matchesResp.Matches {
		if match.PostBalance.Base.Perp.ProductID != productID {
			continue
		}

		amount := parseX18(match.PostBalance.Base.Perp.Balance.Amount)

		// 遇到持仓为0的时候，要是记录了开仓记录，要么就是卖光了
		if amount.IsZero() {
			break
		}

		// matched
		subIdx = slices.Insert(subIdx, 0, match.SubmissionIdx)

		// 计算收益
		realizedPnLF := parseX18(match.RealizedPnL)
		TotalRealizedPnL = TotalRealizedPnL.Add(realizedPnLF)

		// 找到初始的开仓记录就可以停了
		if match.PreBalance.Base.Perp.Balance.Amount == "0" {
			break
		}
	}
	realizedPnL, _ := TotalRealizedPnL.Float64()

	if len(subIdx) == 0 {
		return 0, 0, nil
	}

	entryPrice := decimal.NewFromFloat(0)
	totalAmount := decimal.NewFromFloat(0)
	totalSpend := decimal.NewFromFloat(0)

	for _, idx := range subIdx {
		for _, tx := range matchesResp.Txs {
			if tx.SubmissionIdx != idx {
				continue
			}
			// matched
			// 因为是onyway模式，所以不管买卖，其实都是吃单方，都是taker
			if tx.TxInfo.MatchOrders.Taker.Order.Sender != a.sender {
				continue
			}
			// 使用卖方的挂单价格
			price := parseX18(tx.TxInfo.MatchOrders.Maker.Order.PriceX18)
			// 吃单数量
			amount := parseX18(tx.TxInfo.MatchOrders.Taker.Order.Amount)

			amountDec := amount
			priceDec := price

			cmpZero := totalAmount.Cmp(decimal.NewFromFloat(0))
			// 暂未持仓
			if cmpZero == 0 {
				totalAmount = amountDec
				totalSpend = amountDec.Mul(priceDec)
				entryPrice = priceDec
			} else {
				// 判断方向: 如果 amount 与 totalAmount 同号，则是加仓
				isIncrease := false
				if totalAmount.Sign() > 0 && amount.IsPositive() {
					isIncrease = true
				} else if totalAmount.Sign() < 0 && amount.IsNegative() {
					isIncrease = true
				} else if totalAmount.Sign() == 0 { // Should be covered by cmpZero, but for safety
					isIncrease = true
				}

				if isIncrease {
					// 加仓: Cost 增加 (amount * price)
					totalSpend = totalSpend.Add(amountDec.Mul(priceDec))
					totalAmount = totalAmount.Add(amountDec)
					entryPrice = totalSpend.Div(totalAmount)
				} else {
					// 减仓: Cost 减少 (amount * entryPrice)
					// amount 是异号，所以 Add 相当于减去 value
					totalSpend = totalSpend.Add(amountDec.Mul(entryPrice))
					totalAmount = totalAmount.Add(amountDec)

					// 如果持仓归零
					if totalAmount.IsZero() {
						totalSpend = decimal.Zero
						entryPrice = decimal.Zero
					}
				}
			}

			break
		}
	}

	if totalAmount.Equal(decimal.NewFromFloat(0)) {
		return 0, 0, nil
	}
	ep, _ := entryPrice.Float64()
	return ep, realizedPnL, nil
}
