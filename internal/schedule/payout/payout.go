package schedulepayout

import (
	"github.com/NodeFactoryIo/vedran/internal/repositories"
	"github.com/NodeFactoryIo/vedran/internal/script"
	"github.com/NodeFactoryIo/vedran/internal/ui"
	log "github.com/sirupsen/logrus"
	"net/url"
	"time"
)

type PayoutConfiguration struct {
	PayoutNumberOfDays int
	PayoutTotalReward float64
	LbURL *url.URL
}

// StartScheduledPayout checks every 24 hours how many days have passed since last payout.
// If number of passed days is equal or bigger than defined interval in configuration, start automatic payout
func StartScheduledPayout(configuration PayoutConfiguration, privateKey string, repos repositories.Repos) {
	ticker := time.NewTicker(time.Hour * 24)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				checkForPayout(
					configuration.PayoutNumberOfDays,
					privateKey,
					configuration.PayoutTotalReward,
					configuration.LbURL,
					repos,
				)
			}
		}
	}()
}

func checkForPayout(intervalInDays int, privateKey string, reward float64, loadBalancerUrl *url.URL, repos repositories.Repos) {
	daysSinceLastPayout, lastPayoutTimestamp, err := numOfDaysSinceLastPayout(repos)
	if err != nil {
		log.Error("Unable to calculate number of days since last payout", err)
		return
	}

	if daysSinceLastPayout >= intervalInDays {
		go startPayout(privateKey, reward, loadBalancerUrl)
	} else {
		log.Infof(
			"Last payout was %s, next payout will be in %d days",
			lastPayoutTimestamp.Format("2006-January-02"),
			intervalInDays - daysSinceLastPayout,
		)
	}
}

func startPayout(privateKey string, reward float64, loadBalancerUrl *url.URL)  {
	log.Info("Starting automatic payout...")
	transactionDetails, err := script.ExecutePayout(privateKey, reward, loadBalancerUrl)
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

func numOfDaysSinceLastPayout(repos repositories.Repos) (int, *time.Time, error) {
	latestPayout, err := repos.PayoutRepo.FindLatestPayout()
	if err != nil {
		return 0, nil, err
	}
	daysSinceLastPayout := time.Now().Sub(latestPayout.Timestamp) / (24 * time.Hour)
	return int(daysSinceLastPayout), &latestPayout.Timestamp, nil
}
