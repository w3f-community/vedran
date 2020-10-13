package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NodeFactoryIo/vedran/internal/configuration"
	"github.com/NodeFactoryIo/vedran/internal/httptunnel"
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
	authSecret string
	name       string
	capacity   int64
	whitelist  []string
	fee        float32
	selection  string
	serverPort int32
	publicIP   string
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
		&whitelist,
		"whitelist",
		nil,
		"[OPTIONAL] Comma separated list of node id-s, if provided only these nodes will be allowed to connect")

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

	startCmd.Flags().Int32Var(
		&serverPort,
		"server-port",
		4000,
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

	RootCmd.AddCommand(startCmd)
}

func startCommand(_ *cobra.Command, _ []string) {
	DisplayBanner()

	var tunnelURL string
	if publicIP == "" {
		IP, err := ip.Get()
		if err != nil {
			log.Fatal("Unable to fetch public IP address. Please set one explicitly!", err)
		}
		tunnelURL = fmt.Sprintf("%s:%s", IP.String(), tunnelServerPort)
		log.Infof("Tunnel server will listen on %s and connect tunnels on port range %s", tunnelURL, tunnelPortRange)
	}

	tunnel.StartTunnelServer(tunnelServerPort, tunnelPortRange)

	pPool := &server.AddrPool{}
	err := pPool.Init(tunnelPortRange)
	if err != nil {
		log.Fatal("Failed assigning port range because of: %v", err)
	}

	httptunnel.StartHttpTunnelServer(tunnelServerPort, pPool)
	loadbalancer.StartLoadBalancerServer(configuration.Configuration{
		AuthSecret: authSecret,
		Name:       name,
		Capacity:   capacity,
		Whitelist:  whitelist,
		Fee:        fee,
		Selection:  selection,
		Port:       serverPort,
		TunnelURL:  tunnelURL,
		PortPool:   pPool,
	})
}
