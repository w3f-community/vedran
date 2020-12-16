package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/NodeFactoryIo/vedran/internal/whitelist"

	"github.com/NodeFactoryIo/vedran/internal/configuration"
	"github.com/NodeFactoryIo/vedran/internal/ip"
	"github.com/NodeFactoryIo/vedran/internal/loadbalancer"
	"github.com/NodeFactoryIo/vedran/internal/tunnel"
	"github.com/NodeFactoryIo/vedran/pkg/http-tunnel/server"
	"github.com/NodeFactoryIo/vedran/pkg/logger"
	"github.com/NodeFactoryIo/vedran/pkg/util"
	"github.com/NodeFactoryIo/vedran/pkg/util/random"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// load balancer related flags
	authSecret     string
	name           string
	certFile       string
	keyFile        string
	capacity       int64
	whitelistArray []string
	whitelistFile  string
	fee            float32
	selection      string
	serverPort     int32
	publicIP       string
	// payout related flags
	payoutPrivateKey           string
	payoutNumberOfDays         int32
	payoutTotalReward          string
	payoutTotalRewardAsFloat64 float64
	autoPayoutDisabled         bool
	// logging related flags
	logLevel string
	logFile  string
	// tunnel related flags
	tunnelServerPort string
	tunnelPortRange  string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts vedran load balancer",
	Run:   startCommand,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			log.Fatalf("Invalid log level %s", logLevel)
		}
		err = logger.SetupLogger(level, logFile)
		if err != nil {
			return err
		}
		return nil
	},
	Args: func(cmd *cobra.Command, args []string) error {
		// valid values are round-robin and random
		if selection != "round-robin" && selection != "random" {
			return errors.New("invalid selection option selected")
		}
		// all positive integers are valid, and -1 representing unlimited capacity
		if capacity < -1 {
			return errors.New("invalid capacity value")
		}
		// valid value is between 0-1
		if fee < 0 || fee > 1 {
			return errors.New("invalid fee value")
		}
		// well known ports and registered ports
		if !util.IsValidPortAsInt(serverPort) {
			return errors.New("invalid rpc server port number")
		}
		// valid format is PortMin:PortMax
		prt := strings.Split(tunnelPortRange, ":")
		if len(prt) != 2 {
			return errors.New("invalid port range, should be defined as \"PortMin:PortMax\"")
		}
		if !util.IsValidPortAsStr(prt[0]) {
			return errors.New("invalid port number provided for min port inside port range")
		}
		if !util.IsValidPortAsStr(prt[1]) {
			return errors.New("invalid port number provided for max port inside port range")
		}

		// valid certificates
		if (certFile != "" && keyFile == "") || (keyFile != "" && certFile == "") {
			return errors.New("both cert and key file flags need to be set for valid certificate")
		}

		minPort, _ := strconv.Atoi(prt[0])
		maxPort, _ := strconv.Atoi(prt[1])
		if capacity == -1 {
			capacity = int64(maxPort - minPort)
		} else if int64(maxPort-minPort) < capacity {
			return errors.New("port range too small for target capacity")
		}

		if whitelistArray != nil && whitelistFile != "" {
			return errors.New("only one flag for setting whitelisted nodes should be set")
		}

		autoPayoutDisabled = payoutNumberOfDays == 0 && payoutTotalReward == ""
		if !autoPayoutDisabled {
			if payoutNumberOfDays <= 0 {
				return errors.New("invalid payout interval")
			}
			rewardAsFloat64, err := strconv.ParseFloat(payoutTotalReward, 64)
			if err != nil {
				return errors.New("invalid total reward value")
			}
			payoutTotalRewardAsFloat64 = rewardAsFloat64
		}

		return nil
	},
}

func init() {
	startCmd.Flags().StringVar(
		&authSecret,
		"auth-secret",
		"",
		"[REQUIRED] Authentication secret used for generating tokens")

	startCmd.Flags().StringVar(
		&name,
		"name",
		fmt.Sprintf("load-balancer-%s", random.String(12, random.Alphabetic)),
		"[OPTIONAL] Public name for load balancer, autogenerated name used if omitted")

	startCmd.Flags().Int64Var(
		&capacity,
		"capacity",
		-1,
		"[OPTIONAL] Maximum number of nodes allowed to connect, where -1 represents no upper limit")

	startCmd.Flags().StringSliceVar(
		&whitelistArray,
		"whitelist",
		nil,
		"[OPTIONAL] Comma separated list of node id-s, if provided only these nodes will be allowed to connect."+
			"This flag can't be used together with --whitelist-file flag, only one option for setting whitelisted nodes can be used")

	startCmd.Flags().StringVar(
		&whitelistFile,
		"whitelist-file",
		"",
		"[OPTIONAL] Path to file with node id-s in each line that should be whitelisted."+
			"This flag can't be used together with --whitelist flag, only one option for setting whitelisted nodes can be used")

	startCmd.Flags().Float32Var(
		&fee,
		"fee",
		0.1,
		"[OPTIONAL] Value between 0-1 representing fee percentage")

	startCmd.Flags().StringVar(
		&selection,
		"selection",
		"round-robin",
		"[OPTIONAL] Type of selection used for choosing nodes (round-robin, random)")

	startCmd.Flags().StringVar(
		&certFile,
		"cert-file",
		"",
		"[OPTIONAL] SSL certificate file")

	startCmd.Flags().StringVar(
		&keyFile,
		"key-file",
		"",
		"[OPTIONAL] SSL matching private key")

	startCmd.Flags().Int32Var(
		&serverPort,
		"server-port",
		80,
		"[OPTIONAL] Port on which load balancer rpc server will be started")

	startCmd.Flags().StringVar(
		&publicIP,
		"public-ip",
		"",
		"[OPTIONAL] Public ip of load balancer")

	startCmd.Flags().StringVar(
		&logLevel,
		"log-level",
		"error",
		"[OPTIONAL] Level of logging (eg. info, warn, error)")

	startCmd.Flags().StringVar(
		&logFile,
		"log-file",
		"",
		"[OPTIONAL] Path to logfile (default stdout)")

	startCmd.Flags().StringVar(
		&tunnelServerPort,
		"tunnel-port",
		"5223",
		"[OPTIONAL] Address on which tunnel server is listening")

	startCmd.Flags().StringVar(
		&tunnelPortRange,
		"tunnel-port-range",
		"20000:30000",
		"[OPTIONAL] Range of ports which is used to open tunnels")

	startCmd.Flags().StringVar(
		&payoutPrivateKey,
		"private-key",
		"",
		"[REQUIRED] Loadbalancers wallet private key, used for sending funds on payout",
	)

	startCmd.Flags().Int32Var(
		&payoutNumberOfDays,
		"payout-interval",
		0,
		"[OPTIONAL] Payout interval in days, meaning each X days automatic payout will be executed")

	startCmd.Flags().StringVar(
		&payoutTotalReward,
		"payout-reward",
		"",
		"[OPTIONAL] Total reward pool in Planck",
	)

	_ = startCmd.MarkFlagRequired("private-key")

	RootCmd.AddCommand(startCmd)
}

func startCommand(_ *cobra.Command, _ []string) {
	DisplayBanner()

	// creating address pool
	pPool := &server.AddrPool{}
	err := pPool.Init(tunnelPortRange)
	if err != nil {
		log.Fatalf("Failed assigning port range because of: %v", err)
	}

	// defining tunnel server address
	var tunnelServerAddress string
	if publicIP == "" {
		IP, err := ip.Get()
		if err != nil {
			log.Fatal("Unable to fetch public IP address. Please set one explicitly!", err)
		}
		tunnelServerAddress = fmt.Sprintf("%s:%s", IP.String(), tunnelServerPort)
	} else {
		tunnelServerAddress = fmt.Sprintf("%s:%s", publicIP, tunnelServerPort)
	}
	log.Infof("Tunnel server will listen on %s and connect tunnels on port range %s", tunnelServerAddress, tunnelPortRange)

	// initializing whitelisting
	whitelistEnabled, err := whitelist.InitWhitelisting(whitelistArray, whitelistFile)
	if err != nil {
		log.Fatal("Unable to set whitelisted nodes ", err)
	}
	log.Debugf("Whitelisting set to: %t", whitelistEnabled)

	var payoutConfiguration *configuration.PayoutConfiguration
	if !autoPayoutDisabled {
		lbUrl, _ := url.Parse("http://" + publicIP + ":" + string(serverPort))
		payoutConfiguration = &configuration.PayoutConfiguration{
			PayoutNumberOfDays: int(payoutNumberOfDays),
			PayoutTotalReward:  payoutTotalRewardAsFloat64,
			LbURL:              lbUrl,
		}
	}

	tunnel.StartHttpTunnelServer(tunnelServerPort, pPool)
	loadbalancer.StartLoadBalancerServer(
		configuration.Configuration{
			AuthSecret:          authSecret,
			Name:                name,
			CertFile:            certFile,
			KeyFile:             keyFile,
			Capacity:            capacity,
			Fee:                 fee,
			Selection:           selection,
			Port:                serverPort,
			TunnelServerAddress: tunnelServerAddress,
			PortPool:            pPool,
			WhitelistEnabled:    whitelistEnabled,
			PayoutConfiguration: payoutConfiguration,
		},
		payoutPrivateKey,
	)
}
