package schedulepayout

import (
	"github.com/NodeFactoryIo/vedran/internal/script"
	"github.com/NodeFactoryIo/vedran/internal/ui"
	log "github.com/sirupsen/logrus"
	"net/url"
	"time"
)

func StartScheduledPayout(intervalInDays int32, secret string, reward float64, loadBalancerUrl *url.URL) {
	ticker := time.NewTicker(time.Hour * time.Duration(24*intervalInDays))
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				scheduledPayout(secret, reward, loadBalancerUrl)
			}
		}
	}()
}

func scheduledPayout(secret string, reward float64, loadBalancerUrl *url.URL) {
	log.Info("Starting automatic payout...")
	transactionDetails, err := script.ExecutePayout(secret, reward, loadBalancerUrl)
	if transactionDetails != nil {
		// display even if only part of transactions executed
		ui.DisplayTransactionsStatus(transactionDetails)
	}
	if err != nil {
		log.Errorf("Unable to execute payout, because of: %v", err)
		return
	} else {
		log.Info("Payout execution finished")
	}
}
