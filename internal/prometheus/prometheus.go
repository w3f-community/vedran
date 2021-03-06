package prometheus

import (
	"fmt"
	"github.com/NodeFactoryIo/vedran/internal/stats"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/NodeFactoryIo/vedran/internal/configuration"
	"github.com/NodeFactoryIo/vedran/internal/payout"
	"github.com/NodeFactoryIo/vedran/internal/repositories"
	schedulepayout "github.com/NodeFactoryIo/vedran/internal/schedule/payout"
	"github.com/NodeFactoryIo/vedran/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

const (
	nextPayoutDateLayout = "Mon, Jan 2 2006."

	FeeStatsIntervalEnv                  = "PROM_FEE_STATS_INTERVAL"
	DefaultFeeStatsCollectionInterval    = 12 * time.Hour
	NodeStatsIntervalEnv                 = "PROM_NODE_STATS_INTERVAL"
	DefaultNodeStatsCollectionInterval   = 15 * time.Second
	RequestStatsIntervalEnv              = "PROM_REQUEST_STATS_INTERVAL"
	DefaultRecordStatsCollectionInterval = 15 * time.Second
	PayoutStatsIntervalEnv               = "PROM_PAYOUT_STATS_INTERVAL"
	DefaultPayoutStatsCollectionInterval = 1 * time.Minute
)

var feeStatsCollectionInterval time.Duration
var nodeStatsCollectionInterval time.Duration
var requestStatsCollectionInterval time.Duration
var payoutStatsCollectionInterval time.Duration

var (
	activeNodes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vedran_number_of_active_nodes",
		Help: "The total number of active nodes serving requests",
	})
	penalizedNodes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vedran_number_of_penalized_nodes",
		Help: "The total number of nodes which are on cooldown",
	})
	successfulRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vedran_number_of_successful_requests",
		Help: "The total number of successful requests served via vedran",
	})
	failedRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vedran_number_of_failed_requests",
		Help: "The total number of successful requests served via vedran",
	})
	payoutDistribution = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vedran_payout_distribution",
			Help: "Payout distribution per polkadot address",
		},
		[]string{"address"},
	)
	payoutDate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vedran_payout_date",
			Help: "Payout date of next scheduled payout",
		},
		[]string{"date"},
	)
	payoutFeeAmount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vedran_lb_payout_fee",
			Help: "Payout fee for each last payout",
		},
		[]string{"date"})

	totalFee = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vedran_lb_payout_fee_total",
			Help: "",
		})

	nodeFees = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vedran_nodes_fee",
			Help: "Payout fee for each last payout",
		},
		[]string{"node"})
)

// RecordMetrics starts goroutines for recording metrics
func RecordMetrics(repos repositories.Repos) {
	version := promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vedran_version",
			Help: "App and golang version of vedran",
			ConstLabels: map[string]string{
				"go_version":     runtime.Version(),
				"vedran_version": version.Version,
			},
		},
	)
	version.Set(1)

	feeAsPercString := fmt.Sprintf(
		"%s%%",
		strconv.FormatFloat(float64(configuration.Config.Fee*100), 'f', -1, 32),
	)
	fee := promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vedran_lb_payout_fee_total_percentage",
			Help: "Percentage of fee that goes to load balancer",
			ConstLabels: map[string]string{
				"lb_fee": feeAsPercString,
			},
		})
	fee.Set(float64(configuration.Config.Fee))

	setUpCollectionIntervals()

	go recordPayoutDistribution(repos)
	go recordActiveNodeCount(repos.NodeRepo)
	go recordPenalizedNodeCount(repos.NodeRepo)
	go recordSuccessfulRequestCount(repos.RecordRepo)
	go recordFailedRequestCount(repos.RecordRepo)
	go recordPayoutDate(repos)
	go recordLbFeeAmount(repos.PayoutRepo)
	go recordNodeFees(repos.FeeRepo)
}

func recordNodeFees(repos repositories.FeeRepository) {
	for {
		fees, err := repos.GetAllFees()
		if err != nil {
			log.Errorf("Failed to fetch stats for fees because of: %v", err)
			time.Sleep(15 * time.Minute)
			continue
		}
		for _, fee := range *fees {
			nodeFees.With(prometheus.Labels{"node": fee.NodeId}).Set(float64(fee.TotalFee))
		}
		time.Sleep(feeStatsCollectionInterval)
	}
}

func recordLbFeeAmount(payoutRepo repositories.PayoutRepository) {
	for {
		payouts, err := payoutRepo.GetAll()
		if err != nil {
			log.Errorf("Failed recording stats for payout because of: %v", err)
			time.Sleep(15 * time.Minute)
			continue
		}
		totalFeeCollected := float64(0)
		for _, p := range *payouts {
			if p.LbFee != 0 {
				payoutFeeAmount.With(prometheus.Labels{
					"date": p.Timestamp.Format("2006-January-02"),
				}).Set(p.LbFee)
			}
			totalFeeCollected += p.LbFee
		}
		totalFee.Set(totalFeeCollected)
		time.Sleep(feeStatsCollectionInterval)
	}
}

func recordPayoutDistribution(repos repositories.Repos) {
	for {
		statistics, err := stats.CalculateStatisticsFromLastPayout(repos, time.Now())
		if err != nil {
			log.Errorf("Failed recording stats for payout because of: %v", err)
			time.Sleep(15 * time.Minute)
			continue
		}

		distributionByNode := payout.CalculatePayoutDistributionByNode(
			statistics,
			100,
			payout.LoadBalancerDistributionConfiguration{
				FeePercentage:       float64(configuration.Config.Fee),
				PayoutAddress:       "",
				DifferentFeeAddress: false,
			},
		)

		for address, distribution := range distributionByNode {
			floatDistribution, _ := strconv.ParseFloat(distribution.String(), 64)
			payoutDistribution.With(
				prometheus.Labels{"address": address},
			).Set(
				floatDistribution,
			)
		}

		time.Sleep(payoutStatsCollectionInterval)
	}
}

func recordPayoutDate(repos repositories.Repos) {
	for {
		date, err := schedulepayout.GetNextPayoutDate(configuration.Config.PayoutConfiguration, repos)
		if err != nil {
			payoutDate.With(prometheus.Labels{"date": "Scheduled payout not configured"}).Set(1)
		} else {
			payoutDate.With(prometheus.Labels{"date": date.Format(nextPayoutDateLayout)}).Set(1)
		}
		time.Sleep(feeStatsCollectionInterval)
	}
}

func recordActiveNodeCount(nodeRepo repositories.NodeRepository) {
	for {
		activeNodes.Set(float64(len(*nodeRepo.GetAllActiveNodes())))
		time.Sleep(nodeStatsCollectionInterval)
	}
}

func recordPenalizedNodeCount(nodeRepo repositories.NodeRepository) {
	for {
		nodes, _ := nodeRepo.GetPenalizedNodes()
		penalizedNodes.Set(float64(len(*nodes)))
		time.Sleep(nodeStatsCollectionInterval)
	}
}

func recordSuccessfulRequestCount(recordRepo repositories.RecordRepository) {
	for {
		count, _ := recordRepo.CountSuccessfulRequests()
		successfulRequests.Set(float64(count))
		time.Sleep(requestStatsCollectionInterval)
	}
}

func recordFailedRequestCount(recordRepo repositories.RecordRepository) {
	for {
		count, _ := recordRepo.CountFailedRequests()
		failedRequests.Set(float64(count))
		time.Sleep(requestStatsCollectionInterval)
	}
}

func setUpCollectionIntervals() {
	fsi := os.Getenv(FeeStatsIntervalEnv)
	if fsi != "" {
		feeStatsCollectionInterval, _ = time.ParseDuration(fsi)
	}
	if feeStatsCollectionInterval == 0 {
		feeStatsCollectionInterval = DefaultFeeStatsCollectionInterval
	}
	nsi := os.Getenv(NodeStatsIntervalEnv)
	if nsi != "" {
		nodeStatsCollectionInterval, _ = time.ParseDuration(nsi)
	}
	if nodeStatsCollectionInterval == 0 {
		nodeStatsCollectionInterval = DefaultNodeStatsCollectionInterval
	}
	rsi := os.Getenv(RequestStatsIntervalEnv)
	if rsi != "" {
		requestStatsCollectionInterval, _ = time.ParseDuration(rsi)
	}
	if requestStatsCollectionInterval == 0 {
		requestStatsCollectionInterval = DefaultRecordStatsCollectionInterval
	}
	psi := os.Getenv(PayoutStatsIntervalEnv)
	if psi != "" {
		payoutStatsCollectionInterval, _ = time.ParseDuration(psi)
	}
	if payoutStatsCollectionInterval == 0 {
		payoutStatsCollectionInterval = DefaultPayoutStatsCollectionInterval
	}
}
