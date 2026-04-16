package okx

import (
	"fmt"

	okxsdk "github.com/QuantProcessing/exchanges/okx/sdk"
)

func okxOrderActionError(action string, result okxsdk.OrderId) error {
	if result.SCode == "" || result.SCode == "0" {
		return nil
	}
	if result.SubCode != "" {
		return fmt.Errorf("okx %s rejected: sCode=%s subCode=%s sMsg=%s", action, result.SCode, result.SubCode, result.SMsg)
	}
	return fmt.Errorf("okx %s rejected: sCode=%s sMsg=%s", action, result.SCode, result.SMsg)
}

func okxInstrumentIDCode(inst okxsdk.Instrument, instID string) (int64, error) {
	if inst.InstIdCode == nil {
		return 0, fmt.Errorf("missing instIdCode for %s", instID)
	}
	return *inst.InstIdCode, nil
}
